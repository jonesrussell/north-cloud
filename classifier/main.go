package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jonesrussell/north-cloud/classifier/cmd/processor"
	"github.com/jonesrussell/north-cloud/classifier/internal/server"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const version = "1.0.0"

// initLogger creates a logger for CLI commands
func initLogger() infralogger.Logger {
	log, err := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	return log.With(infralogger.String("service", "classifier"))
}

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
		if err := server.StartHTTPServer(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "processor", "worker":
		// Run background processor only (operational log)
		log := initLogger()
		log.Info("Starting processor",
			infralogger.String("version", version),
			infralogger.String("mode", "processor"),
		)
		if err := processor.Start(); err != nil {
			log.Error("Processor failed", infralogger.Error(err))
			os.Exit(1)
		}
	case "version":
		// CLI output (not operational log)
		fmt.Printf("Classifier Service v%s\n", version)
		os.Exit(0)
	case "help", "-h", "--help":
		// CLI output (not operational log)
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
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command) //nolint:gosec // G705: command is CLI arg, stderr only
		fmt.Fprintln(os.Stderr, "Run 'classifier help' for usage information")
		os.Exit(1)
	}
}

// startBoth starts both the HTTP server and processor concurrently
func startBoth() {
	log := initLogger()
	log.Info("Starting both services",
		infralogger.String("version", version),
		infralogger.String("mode", "both"),
	)

	// Start HTTP server
	httpStop, err := server.StartHTTPServerWithStop()
	if err != nil {
		log.Error("Failed to start HTTP server", infralogger.Error(err))
		os.Exit(1)
	}

	// Start processor
	processorStop, err := processor.StartWithStop()
	if err != nil {
		log.Error("Failed to start processor", infralogger.Error(err))
		httpStop() // Clean up HTTP server if processor fails
		os.Exit(1)
	}

	log.Info("Both services started successfully")

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdown
	log.Info("Shutdown signal received",
		infralogger.String("signal", sig.String()),
	)

	// Stop both services
	processorStop()
	httpStop()

	log.Info("All services stopped successfully")
}
