// Package main is the entry point for the publisher service.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	infralogger "github.com/north-cloud/infrastructure/logger"
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
		// CLI output (not operational log)
		fmt.Printf("Publisher version %s\n", version)
		os.Exit(0)
	case "help", "-h", "--help":
		printUsage()
		os.Exit(0)
	default:
		// CLI output (not operational log)
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

// startBoth starts both the API server and router concurrently
func startBoth() {
	// Initialize logger for operational logs
	log, err := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	log = log.With(infralogger.String("service", "publisher"))

	log.Info("Starting both services",
		infralogger.String("version", version),
		infralogger.String("mode", "both"),
	)

	// Start API server
	apiStop, err := runAPIServerWithStop()
	if err != nil {
		log.Error("Failed to start API server", infralogger.Error(err))
		os.Exit(1)
	}

	// Start router
	routerStop, err := runRouterWithStop()
	if err != nil {
		log.Error("Failed to start router", infralogger.Error(err))
		apiStop() // Clean up API server if router fails
		os.Exit(1)
	}

	log.Info("Both services started successfully")

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdown
	log.Info("Shutdown signal received", infralogger.String("signal", sig.String()))

	// Stop both services
	routerStop()
	apiStop()

	log.Info("All services stopped successfully")
}

func printUsage() {
	// CLI help output (not operational log)
	fmt.Println("Publisher Service - Multi-command CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  publisher [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  both       Start both HTTP API server and router (default)")
	fmt.Println("  api        Start the HTTP API server only")
	fmt.Println("  router     Start the background router service only")
	fmt.Println("  version    Print version information")
	fmt.Println("  help       Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  publisher                # Start both API server and router (default)")
	fmt.Println("  publisher both           # Same as above")
	fmt.Println("  publisher api            # Start API server only on port 8070")
	fmt.Println("  publisher router         # Start router service only")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  Database:")
	fmt.Println("    POSTGRES_PUBLISHER_HOST      - PostgreSQL host (default: localhost)")
	fmt.Println("    POSTGRES_PUBLISHER_PORT      - PostgreSQL port (default: 5432)")
	fmt.Println("    POSTGRES_PUBLISHER_USER      - PostgreSQL user (default: postgres)")
	fmt.Println("    POSTGRES_PUBLISHER_PASSWORD  - PostgreSQL password")
	fmt.Println("    POSTGRES_PUBLISHER_DB        - PostgreSQL database (default: publisher)")
	fmt.Println()
	fmt.Println("  API Server:")
	fmt.Println("    PUBLISHER_PORT               - HTTP port (default: 8070)")
	fmt.Println("    AUTH_JWT_SECRET              - JWT secret for authentication (optional)")
	fmt.Println("    GIN_MODE                     - Gin mode: debug|release (default: debug)")
	fmt.Println()
	fmt.Println("  Router Service:")
	fmt.Println("    ELASTICSEARCH_URL            - Elasticsearch URL (default: http://localhost:9200)")
	fmt.Println("    REDIS_ADDR                   - Redis address (default: localhost:6379)")
	fmt.Println("    REDIS_PASSWORD               - Redis password (optional)")
	fmt.Println("    PUBLISHER_ROUTER_CHECK_INTERVAL - Check interval (default: 5m)")
	fmt.Println("    PUBLISHER_ROUTER_BATCH_SIZE     - Batch size (default: 100)")
}
