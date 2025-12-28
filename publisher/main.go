// Package main is the entry point for the publisher service.
package main

import (
	"fmt"
	"os"
)

var (
	// version can be set at build time via -ldflags
	version = "dev"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "api":
		runAPIServer()
	case "router":
		runRouter()
	case "version":
		fmt.Printf("Publisher version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Publisher Service - Multi-command CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  publisher <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  api       Start the HTTP API server")
	fmt.Println("  router    Start the background router service")
	fmt.Println("  version   Print version information")
	fmt.Println("  help      Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  publisher api          # Start API server on port 8070")
	fmt.Println("  publisher router       # Start router service")
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
