package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/search/internal/api"
	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/search/internal/logger"
	"github.com/jonesrussell/north-cloud/search/internal/service"
	"github.com/north-cloud/infrastructure/profiling"
)

func main() {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log := logger.New(cfg.Logging)
	log.Info("Starting search service",
		"version", cfg.Service.Version,
		"port", cfg.Service.Port,
		"debug", cfg.Service.Debug,
	)

	// Initialize Elasticsearch client
	log.Info("Connecting to Elasticsearch", "url", cfg.Elasticsearch.URL)
	esClient, err := elasticsearch.NewClient(&cfg.Elasticsearch)
	if err != nil {
		log.Fatal("Failed to create Elasticsearch client", "error", err)
	}
	log.Info("Successfully connected to Elasticsearch")

	// Initialize search service
	searchService := service.NewSearchService(esClient, cfg, log)
	log.Info("Search service initialized")

	// Initialize API handler
	handler := api.NewHandler(searchService, log)

	// Create and start HTTP server
	server := api.NewServer(handler, cfg, log)

	// Start server in goroutine
	go func() {
		if startErr := server.Start(); startErr != nil {
			log.Fatal("Failed to start HTTP server", "error", startErr)
		}
	}()

	log.Info("Search service started successfully",
		"port", cfg.Service.Port,
		"elasticsearch_pattern", cfg.Elasticsearch.ClassifiedContentPattern,
	)

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutdown signal received, gracefully shutting down...")

	// Create shutdown context with timeout
	const shutdownTimeoutSeconds = 10
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeoutSeconds*time.Second)

	// Shutdown HTTP server
	shutdownErr := server.Shutdown(ctx)
	cancel() // Always cancel context

	if shutdownErr != nil {
		log.Error("Server forced to shutdown", "error", shutdownErr)
		os.Exit(1)
	}

	log.Info("Search service exited cleanly")
}
