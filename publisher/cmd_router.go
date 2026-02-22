package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/jonesrussell/north-cloud/publisher/internal/discovery"
	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/pipeline"
	"github.com/north-cloud/infrastructure/profiling"
)

func runRouter() {
	stop, err := runRouterWithStop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start router: %v\n", err)
		os.Exit(1)
	}

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	stop()
}

// runRouterWithStop starts the router service and returns a stop function
// This allows the router to run concurrently with other services
func runRouterWithStop() (func(), error) {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()
	var pyroProfiler *profiling.PyroscopeProfiler
	if p, pyroErr := profiling.StartPyroscope("publisher-router"); pyroErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Pyroscope failed to start: %v\n", pyroErr)
	} else {
		pyroProfiler = p
	}

	// Initialize logger using infrastructure logger
	appLogger, loggerErr := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if loggerErr != nil {
		return nil, fmt.Errorf("failed to create logger: %w", loggerErr)
	}

	appLogger = appLogger.With(
		infralogger.String("service", "publisher-router"),
		infralogger.String("version", version),
	)

	appLogger.Info("Starting Publisher Router Service...")

	// Load configuration
	cfg := LoadRouterConfig()

	// Initialize database connection
	db, dbErr := database.NewPostgresConnection(cfg.Database)
	if dbErr != nil {
		_ = appLogger.Sync()
		return nil, fmt.Errorf("failed to connect to database: %w", dbErr)
	}

	appLogger.Info("Database connection established")

	// Initialize repository
	repo := database.NewRepository(db)

	// Initialize Elasticsearch client
	esClient := initElasticsearchClient(cfg.ESURL)

	// Initialize Redis client
	redisClient := initRedisClient(cfg.RedisAddr, cfg.RedisPassword)

	// Initialize discovery service
	discoveryService := discovery.NewService(esClient, appLogger)

	// Initialize pipeline client (fire-and-forget; no-op when URL is empty)
	pipelineClient := pipeline.NewClient(cfg.PipelineURL, "publisher")

	// Initialize router service
	routerConfig := router.Config{
		PollInterval:      cfg.PollInterval,
		DiscoveryInterval: cfg.DiscoveryInterval,
		BatchSize:         cfg.BatchSize,
	}
	routerService := router.NewService(repo, discoveryService, esClient, redisClient, routerConfig, appLogger, pipelineClient)

	// Setup graceful shutdown context
	serviceCtx, cancel := context.WithCancel(context.Background())

	// Start router service in goroutine
	go func() {
		startErr := routerService.Start(serviceCtx)
		if startErr != nil && !errors.Is(startErr, context.Canceled) {
			appLogger.Error("Router service error", infralogger.Error(startErr))
		}
	}()

	appLogger.Info("Router service started",
		infralogger.Duration("poll_interval", cfg.PollInterval),
		infralogger.Duration("discovery_interval", cfg.DiscoveryInterval),
		infralogger.Int("batch_size", cfg.BatchSize),
	)

	// Return stop function
	stop := func() {
		appLogger.Info("Stopping router service")
		cancel()

		// Wait for graceful shutdown with timeout
		const shutdownTimeout = 30 * time.Second
		const gracefulShutdownWait = 5 * time.Second
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

		if pyroProfiler != nil {
			if stopErr := pyroProfiler.Stop(); stopErr != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Pyroscope failed to stop: %v\n", stopErr)
			}
		}
		redisClient.Close()
		db.Close()
		_ = appLogger.Sync()
	}

	return stop, nil
}
