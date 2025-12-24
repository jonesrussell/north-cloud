package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jonesrussell/north-cloud/classifier/cmd/processor"
	"github.com/jonesrussell/north-cloud/classifier/internal/server"
)

const version = "1.0.0"

func main() {
	// Get command from args, default to "httpd"
	command := "httpd"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "httpd":
		// Run HTTP server
		server.StartHTTPServer()
	case "processor", "worker":
		// Run background processor
		fmt.Printf("Classifier Service v%s - Processor Mode\n", version)
		if err := processor.Start(); err != nil {
			log.Fatalf("Processor failed: %v", err)
		}
	case "version":
		fmt.Printf("Classifier Service v%s\n", version)
		os.Exit(0)
	case "help", "-h", "--help":
		fmt.Printf("Classifier Service v%s\n", version)
		fmt.Println("\nUsage: classifier [command]")
		fmt.Println("\nCommands:")
		fmt.Println("  httpd      Start HTTP API server (default)")
		fmt.Println("  processor  Start background processor")
		fmt.Println("  version    Show version")
		fmt.Println("  help       Show this help message")
		os.Exit(0)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Run 'classifier help' for usage information")
		os.Exit(1)
	}
}
