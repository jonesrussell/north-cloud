// Package main is the entry point for the publisher service.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

const (
	minArgsCount = 1
)

var (
	// version can be set at build time via -ldflags
	version = "dev"
)

func main() {
	// Get command from args, default to "both" (api + router)
	command := "both"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "both", "all":
		// Run both API server and router concurrently
		startBoth()
	case "api":
		runAPIServer()
	case "router":
		runRouter()
	case "version":
		log.Printf("Publisher version %s\n", version)
		os.Exit(0)
	case "help", "-h", "--help":
		printUsage()
		os.Exit(0)
	default:
		log.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

// startBoth starts both the API server and router concurrently
func startBoth() {
	log.Printf("Publisher Service v%s - Starting API Server and Router\n", version)

	// Start API server
	apiStop, err := runAPIServerWithStop()
	if err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}

	// Start router
	routerStop, err := runRouterWithStop()
	if err != nil {
		log.Fatalf("Failed to start router: %v", err)
		apiStop() // Clean up API server if router fails
		os.Exit(1)
	}

	log.Println("Both services started successfully")

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdown
	log.Printf("Received signal %v, shutting down both services...", sig)

	// Stop both services
	routerStop()
	apiStop()

	log.Println("All services stopped successfully")
}

func printUsage() {
	log.Println("Publisher Service - Multi-command CLI")
	log.Println()
	log.Println("Usage:")
	log.Println("  publisher [command]")
	log.Println()
	log.Println("Commands:")
	log.Println("  both       Start both HTTP API server and router (default)")
	log.Println("  api        Start the HTTP API server only")
	log.Println("  router     Start the background router service only")
	log.Println("  version    Print version information")
	log.Println("  help       Show this help message")
	log.Println()
	log.Println("Examples:")
	log.Println("  publisher                # Start both API server and router (default)")
	log.Println("  publisher both           # Same as above")
	log.Println("  publisher api            # Start API server only on port 8070")
	log.Println("  publisher router         # Start router service only")
	log.Println()
	log.Println("Environment Variables:")
	log.Println("  Database:")
	log.Println("    POSTGRES_PUBLISHER_HOST      - PostgreSQL host (default: localhost)")
	log.Println("    POSTGRES_PUBLISHER_PORT      - PostgreSQL port (default: 5432)")
	log.Println("    POSTGRES_PUBLISHER_USER      - PostgreSQL user (default: postgres)")
	log.Println("    POSTGRES_PUBLISHER_PASSWORD  - PostgreSQL password")
	log.Println("    POSTGRES_PUBLISHER_DB        - PostgreSQL database (default: publisher)")
	log.Println()
	log.Println("  API Server:")
	log.Println("    PUBLISHER_PORT               - HTTP port (default: 8070)")
	log.Println("    AUTH_JWT_SECRET              - JWT secret for authentication (optional)")
	log.Println("    GIN_MODE                     - Gin mode: debug|release (default: debug)")
	log.Println()
	log.Println("  Router Service:")
	log.Println("    ELASTICSEARCH_URL            - Elasticsearch URL (default: http://localhost:9200)")
	log.Println("    REDIS_ADDR                   - Redis address (default: localhost:6379)")
	log.Println("    REDIS_PASSWORD               - Redis password (optional)")
	log.Println("    PUBLISHER_ROUTER_CHECK_INTERVAL - Check interval (default: 5m)")
	log.Println("    PUBLISHER_ROUTER_BATCH_SIZE     - Batch size (default: 100)")
}
