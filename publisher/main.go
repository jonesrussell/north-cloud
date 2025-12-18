package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gopost/integration/internal/api"
	"github.com/gopost/integration/internal/config"
	"github.com/gopost/integration/internal/integration"
	"github.com/gopost/integration/internal/logger"
	"github.com/gopost/integration/internal/metrics"
	"github.com/gopost/integration/internal/sources"
	infracontext "github.com/north-cloud/infrastructure/context"
	"github.com/redis/go-redis/v9"
)

var (
	// version can be set at build time via -ldflags
	version = "dev"
)

const (
	// DefaultShutdownTimeoutSeconds is the default shutdown timeout in seconds
	DefaultShutdownTimeoutSeconds = 30
)

func initializeLogger(cfg *config.Config) (logger.Logger, error) {
	appLogger, err := logger.NewLogger(cfg.Debug)
	if err != nil {
		return nil, err
	}

	// Add service context fields to all log entries
	appLogger = appLogger.With(
		logger.String("service", "gopost"),
		logger.String("version", version),
	)

	return appLogger, nil
}

func handleFlushCache(service *integration.Service, appLogger logger.Logger) {
	const flushCacheTimeout = 30 * time.Second
	ctx, cancel := infracontext.WithTimeout(flushCacheTimeout)
	defer cancel()

	if err := service.FlushCache(ctx); err != nil {
		appLogger.Error("Failed to flush cache",
			logger.Error(err),
		)
		_ = appLogger.Sync()
		os.Exit(1)
	}

	appLogger.Info("Cache flushed successfully")
	_ = appLogger.Sync()
}

// loadConfiguration loads and validates configuration
func loadConfiguration(configPath string, appLogger logger.Logger) (*config.Config, error) {
	baseCfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// If sources service is enabled, try to fetch cities from it
	var cfg *config.Config
	if baseCfg.Sources.Enabled {
		sourcesClient := sources.NewClient(&baseCfg.Sources, appLogger)
		cfg, err = config.LoadWithSources(configPath, sourcesClient)
		if err != nil {
			appLogger.Warn("Failed to load config with sources, falling back to config file",
				logger.Error(err),
			)
			cfg = baseCfg
		} else {
			appLogger.Info("Loaded cities from sources service",
				logger.Int("city_count", len(cfg.Cities)),
			)
		}
	} else {
		cfg = baseCfg
	}

	return cfg, nil
}

// initializeServices initializes Redis, metrics tracker, and integration service
func initializeServices(cfg *config.Config, appLogger logger.Logger) (*integration.Service, *metrics.Tracker, error) {
	// Initialize Redis for metrics (shared with dedup, but using different key prefixes)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.URL,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	ctx, cancel := infracontext.WithPingTimeout()
	defer cancel()
	if pingErr := redisClient.Ping(ctx).Err(); pingErr != nil {
		return nil, nil, fmt.Errorf("connect to Redis: %w", pingErr)
	}

	// Get city names for metrics tracker
	cityNames := make([]string, 0, len(cfg.Cities))
	for _, city := range cfg.Cities {
		cityNames = append(cityNames, city.Name)
	}

	// Create metrics tracker
	metricsTracker := metrics.NewTracker(redisClient, cityNames, appLogger)

	// Create integration service with metrics tracker
	service, err := integration.NewService(cfg, metricsTracker, appLogger)
	if err != nil {
		return nil, nil, fmt.Errorf("create integration service: %w", err)
	}

	return service, metricsTracker, nil
}

// startServers starts the HTTP server and worker
func startServers(
	cfg *config.Config,
	service *integration.Service,
	metricsTracker *metrics.Tracker,
	appLogger logger.Logger,
	version string,
	configPath string,
) (*http.Server, context.CancelFunc, chan error, chan error) {
	// Create stats service and API router
	statsService := api.NewStatsService(metricsTracker, appLogger)
	router := api.NewRouter(statsService, appLogger, version)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start HTTP server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		appLogger.Info("Starting HTTP server",
			logger.String("address", cfg.Server.Address),
		)
		if listenErr := srv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			serverErr <- listenErr
		}
	}()

	// Start worker in goroutine
	workerCtx, workerCancel := context.WithCancel(context.Background())

	workerErr := make(chan error, 1)
	go func() {
		appLogger.Info("Starting integration service",
			logger.String("config_path", configPath),
			logger.Bool("debug", cfg.Debug),
		)
		workerErr <- service.Run(workerCtx)
	}()

	return srv, workerCancel, serverErr, workerErr
}

func main() {
	var configPath string
	var flushCache bool
	flag.StringVar(&configPath, "config", "config.yml", "Path to configuration file")
	flag.BoolVar(&flushCache, "flush-cache", false, "Flush Redis deduplication cache and exit")
	flag.Parse()

	// Load base config to determine debug mode
	baseCfg, err := config.Load(configPath)
	if err != nil {
		// Use a temporary logger for early errors before config is loaded
		tempLogger, _ := logger.NewLogger(true)
		tempLogger.Error("Failed to load config",
			logger.String("config_path", configPath),
			logger.Error(err),
		)
		_ = tempLogger.Sync()
		os.Exit(1)
	}

	// Create logger based on debug mode from config
	appLogger, err := initializeLogger(baseCfg)
	if err != nil {
		// Fallback to temporary logger if logger creation fails
		tempLogger, _ := logger.NewLogger(true)
		tempLogger.Error("Failed to create logger",
			logger.Error(err),
		)
		_ = tempLogger.Sync()
		os.Exit(1)
	}
	defer func() {
		if syncErr := appLogger.Sync(); syncErr != nil {
			// Can't log this error since logger might be closed
			_ = syncErr
		}
	}()

	// Load full configuration (with sources if enabled)
	cfg, err := loadConfiguration(configPath, appLogger)
	if err != nil {
		appLogger.Error("Failed to load configuration",
			logger.Error(err),
		)
		_ = appLogger.Sync()
		os.Exit(1)
	}

	// Initialize services
	service, metricsTracker, err := initializeServices(cfg, appLogger)
	if err != nil {
		appLogger.Error("Failed to initialize services",
			logger.Error(err),
		)
		_ = appLogger.Sync()
		os.Exit(1)
	}

	// Handle flush-cache flag
	if flushCache {
		handleFlushCache(service, appLogger)
		return
	}

	// Start servers
	srv, workerCancel, serverErr, workerErr := startServers(cfg, service, metricsTracker, appLogger, version, configPath)
	defer workerCancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		appLogger.Info("Shutting down gracefully",
			logger.String("signal", sig.String()),
		)
		workerCancel()

		// Shutdown HTTP server with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), DefaultShutdownTimeoutSeconds*time.Second)
		defer shutdownCancel()

		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			appLogger.Error("Server shutdown error",
				logger.Error(shutdownErr),
			)
		} else {
			appLogger.Info("HTTP server stopped")
		}

		// Wait for worker to finish
		if workerErrVal := <-workerErr; workerErrVal != nil && !errors.Is(workerErrVal, context.Canceled) {
			appLogger.Error("Worker error",
				logger.Error(workerErrVal),
			)
		} else {
			appLogger.Info("Worker stopped")
		}
	case serverErrVal := <-serverErr:
		appLogger.Error("Server error",
			logger.Error(serverErrVal),
		)
		workerCancel()
		if workerErrVal := <-workerErr; workerErrVal != nil && !errors.Is(workerErrVal, context.Canceled) {
			appLogger.Error("Worker error",
				logger.Error(workerErrVal),
			)
		}
	case workerErrVal := <-workerErr:
		if workerErrVal != nil && !errors.Is(workerErrVal, context.Canceled) {
			appLogger.Error("Worker error",
				logger.Error(workerErrVal),
			)
		}
		workerCancel()

		// Shutdown HTTP server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), DefaultShutdownTimeoutSeconds*time.Second)
		defer shutdownCancel()

		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			appLogger.Error("Server shutdown error",
				logger.Error(shutdownErr),
			)
		}
	}

	appLogger.Info("Service stopped")
}
