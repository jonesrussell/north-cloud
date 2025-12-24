package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jonesrussell/north-cloud/classifier/cmd/processor"
	"github.com/jonesrussell/north-cloud/classifier/internal/server"
)

const version = "1.0.0"

func main() {
	// Get command from args, default to "both" (httpd + processor)
	command := "both"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "both", "all":
		// Run both HTTP server and processor concurrently
		startBoth()
	case "httpd":
		// Run HTTP server only
		server.StartHTTPServer()
	case "processor", "worker":
		// Run background processor only
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
		fmt.Println("  both       Start both HTTP API server and processor (default)")
		fmt.Println("  httpd      Start HTTP API server only")
		fmt.Println("  processor  Start background processor only")
		fmt.Println("  version    Show version")
		fmt.Println("  help       Show this help message")
		os.Exit(0)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Run 'classifier help' for usage information")
		os.Exit(1)
	}
}

// startBoth starts both the HTTP server and processor concurrently
func startBoth() {
	fmt.Printf("Classifier Service v%s - Starting HTTP Server and Processor\n", version)

	// Start HTTP server
	httpStop, err := server.StartHTTPServerWithStop()
	if err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}

	// Start processor
	processorStop, err := processor.StartWithStop()
	if err != nil {
		log.Fatalf("Failed to start processor: %v", err)
		httpStop() // Clean up HTTP server if processor fails
		os.Exit(1)
	}

	log.Println("Both services started successfully")

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdown
	log.Printf("Received signal %v, shutting down both services...", sig)

	// Stop both services
	processorStop()
	httpStop()

	log.Println("All services stopped successfully")
}
