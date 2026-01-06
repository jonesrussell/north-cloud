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
	"github.com/jonesrussell/north-cloud/search/internal/logging"
	"github.com/jonesrussell/north-cloud/search/internal/service"
	infraconfig "github.com/north-cloud/infrastructure/config"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

const shutdownTimeout = 10 * time.Second

func main() {
	os.Exit(run())
}

func run() int {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		return 1
	}

	// Initialize logger
	log, err := logger.New(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()

	// Create logger adapter for services
	logAdapter := logging.NewAdapter(log)

	log.Info("Starting search service",
		logger.String("name", cfg.Service.Name),
		logger.String("version", cfg.Service.Version),
		logger.Int("port", cfg.Service.Port),
		logger.Bool("debug", cfg.Service.Debug),
	)

	// Initialize Elasticsearch client
	log.Info("Connecting to Elasticsearch", logger.String("url", cfg.Elasticsearch.URL))
	esClient, esErr := elasticsearch.NewClient(&cfg.Elasticsearch)
	if esErr != nil {
		log.Error("Failed to create Elasticsearch client", logger.Error(esErr))
		return 1
	}
	log.Info("Successfully connected to Elasticsearch")

	// Initialize search service
	searchService := service.NewSearchService(esClient, cfg, logAdapter)
	log.Info("Search service initialized")

	// Initialize API handler
	handler := api.NewHandler(searchService, logAdapter)

	// Create and start HTTP server
	server := api.NewServer(handler, cfg, logAdapter)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if startErr := server.Start(); startErr != nil {
			serverErr <- startErr
		}
	}()

	log.Info("Search service started successfully",
		logger.Int("port", cfg.Service.Port),
		logger.String("elasticsearch_pattern", cfg.Elasticsearch.ClassifiedContentPattern),
	)

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case srvErr := <-serverErr:
		log.Error("Server error", logger.Error(srvErr))
		return 1
	case <-quit:
		log.Info("Shutdown signal received, gracefully shutting down...")
	}

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if shutdownErr := server.Shutdown(ctx); shutdownErr != nil {
		log.Error("Server forced to shutdown", logger.Error(shutdownErr))
		return 1
	}

	log.Info("Search service exited cleanly")
	return 0
}
