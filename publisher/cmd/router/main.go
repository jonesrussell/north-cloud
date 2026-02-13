package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/jonesrussell/north-cloud/publisher/internal/discovery"
	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

const (
	shutdownTimeout      = 30 * time.Second
	gracefulShutdownWait = 5 * time.Second
)

func main() {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Initialize logger using infrastructure logger
	appLogger, loggerErr := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if loggerErr != nil {
		os.Stderr.WriteString("Failed to create logger, exiting\n")
		os.Exit(1)
	}
	defer func() {
		_ = appLogger.Sync()
	}()

	appLogger = appLogger.With(
		infralogger.String("service", "publisher-router"),
	)

	appLogger.Info("Starting Publisher Router Service...")

	// Load configuration
	cfg := LoadConfig()

	// Initialize database connection
	db, dbErr := database.NewPostgresConnection(cfg.Database)
	if dbErr != nil {
		appLogger.Fatal("Failed to connect to database", infralogger.Error(dbErr))
	}
	defer db.Close()
	appLogger.Info("Database connection established")

	// Initialize repository
	repo := database.NewRepository(db)

	// Initialize Elasticsearch client
	esClient := initElasticsearchClient(cfg.ESURL)

	// Initialize Redis client
	redisClient := initRedisClient(cfg.RedisAddr, cfg.RedisPassword)
	defer redisClient.Close()

	// Initialize discovery service
	discoveryService := discovery.NewService(esClient, appLogger)

	// Initialize router service
	routerConfig := router.Config{
		PollInterval:      cfg.PollInterval,
		DiscoveryInterval: cfg.DiscoveryInterval,
		BatchSize:         cfg.BatchSize,
	}
	routerService := router.NewService(repo, discoveryService, esClient, redisClient, routerConfig, appLogger, nil)

	// Setup graceful shutdown
	serviceCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start router service in goroutine
	errChan := make(chan error, 1)
	go func() {
		startErr := routerService.Start(serviceCtx)
		if startErr != nil && !errors.Is(startErr, context.Canceled) {
			errChan <- startErr
		}
	}()

	appLogger.Info("Router service started",
		infralogger.Duration("poll_interval", cfg.PollInterval),
		infralogger.Duration("discovery_interval", cfg.DiscoveryInterval),
		infralogger.Int("batch_size", cfg.BatchSize),
	)

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		appLogger.Info("Received shutdown signal")
		cancel()
	case serviceErr := <-errChan:
		appLogger.Error("Router service error", infralogger.Error(serviceErr))
		cancel()
	}

	// Wait for graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Give the service time to finish current operations
	done := make(chan struct{})
	go func() {
		time.Sleep(gracefulShutdownWait)
		close(done)
	}()

	select {
	case <-done:
		appLogger.Info("Router service stopped gracefully")
	case <-shutdownCtx.Done():
		appLogger.Warn("Shutdown timeout exceeded, forcing exit")
	}
}
