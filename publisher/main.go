// Package main is the entry point for the publisher service.
package main

import (
	"log"
	"os"
)

const (
	minArgsCount = 2
)

var (
	// version can be set at build time via -ldflags
	version = "dev"
)

func main() {
	if len(os.Args) < minArgsCount {
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
		log.Printf("Publisher version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		log.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	log.Println("Publisher Service - Multi-command CLI")
	log.Println()
	log.Println("Usage:")
	log.Println("  publisher <command>")
	log.Println()
	log.Println("Commands:")
	log.Println("  api       Start the HTTP API server")
	log.Println("  router    Start the background router service")
	log.Println("  version   Print version information")
	log.Println("  help      Show this help message")
	log.Println()
	log.Println("Examples:")
	log.Println("  publisher api          # Start API server on port 8070")
	log.Println("  publisher router       # Start router service")
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
