// Package app provides the main application lifecycle management for the publisher service.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	"github.com/jonesrussell/north-cloud/publisher/internal/integration"
	"github.com/jonesrussell/north-cloud/publisher/internal/logger"
	"github.com/jonesrussell/north-cloud/publisher/internal/metrics"
	"github.com/jonesrussell/north-cloud/publisher/internal/sources"
	infracontext "github.com/north-cloud/infrastructure/context"
	"github.com/redis/go-redis/v9"
)

const (
	// DefaultShutdownTimeout is the default timeout for graceful shutdown
	DefaultShutdownTimeout = 30 * time.Second
	// FlushCacheTimeout is the timeout for cache flush operations
	FlushCacheTimeout = 30 * time.Second
)

// App represents the publisher application with all its dependencies
type App struct {
	config         *config.Config
	logger         logger.Logger
	redisClient    redis.UniversalClient
	metricsTracker *metrics.Tracker
	service        *integration.Service
	httpServer     *http.Server
	version        string
	configPath     string
}

// Options contains configuration for creating a new App
type Options struct {
	ConfigPath string
	Version    string
}

// New creates a new App instance with all dependencies initialized
func New(opts Options) (*App, error) {
	// Load configuration
	cfg, appLogger, err := loadConfigAndLogger(opts.ConfigPath)
	if err != nil {
		return nil, err
	}

	// Add service context to logger
	appLogger = appLogger.With(
		logger.String("service", "gopost"),
		logger.String("version", opts.Version),
	)

	// Initialize Redis client (shared between metrics and dedup)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.URL,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	ctx, cancel := infracontext.WithPingTimeout()
	defer cancel()
	if pingErr := redisClient.Ping(ctx).Err(); pingErr != nil {
		_ = appLogger.Sync()
		return nil, fmt.Errorf("connect to Redis: %w", pingErr)
	}

	// Get city names for metrics tracker
	cityNames := make([]string, 0, len(cfg.Cities))
	for _, city := range cfg.Cities {
		cityNames = append(cityNames, city.Name)
	}

	// Create metrics tracker
	metricsTracker := metrics.NewTracker(redisClient, cityNames, appLogger)

	// Create integration service with shared Redis client
	service, err := integration.NewService(cfg, integration.ServiceDeps{
		RedisClient: redisClient,
		Metrics:     metricsTracker,
		Logger:      appLogger,
	})
	if err != nil {
		_ = appLogger.Sync()
		return nil, fmt.Errorf("create integration service: %w", err)
	}

	return &App{
		config:         cfg,
		logger:         appLogger,
		redisClient:    redisClient,
		metricsTracker: metricsTracker,
		service:        service,
		version:        opts.Version,
		configPath:     opts.ConfigPath,
	}, nil
}

// loadConfigAndLogger loads configuration and creates the logger
func loadConfigAndLogger(configPath string) (*config.Config, logger.Logger, error) {
	// Load base config first to determine debug mode
	baseCfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	// Create logger based on debug mode
	appLogger, err := logger.NewLogger(baseCfg.Debug)
	if err != nil {
		return nil, nil, fmt.Errorf("create logger: %w", err)
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

	return cfg, appLogger, nil
}

// Run starts the application and blocks until shutdown
func (a *App) Run(ctx context.Context) error {
	// Note: HTTP server is now handled by separate 'api' command
	// This app only runs the integration service worker

	// Channels for tracking goroutine errors
	workerErr := make(chan error, 1)

	// Start worker with cancellable context
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	go func() {
		a.logger.Info("Starting integration service",
			logger.String("config_path", a.configPath),
			logger.Bool("debug", a.config.Debug),
		)
		workerErr <- a.service.Run(workerCtx)
	}()

	// Wait for shutdown signal or error
	return a.waitForShutdown(workerCancel, nil, workerErr)
}

// waitForShutdown handles graceful shutdown
func (a *App) waitForShutdown(workerCancel context.CancelFunc, serverErr, workerErr chan error) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var shutdownErr error

	select {
	case sig := <-sigChan:
		a.logger.Info("Shutting down gracefully",
			logger.String("signal", sig.String()),
		)
		a.shutdown(workerCancel, workerErr, true)

	case err := <-serverErr:
		if serverErr != nil {
			a.logger.Error("Server error", logger.Error(err))
			workerCancel()
			a.waitForWorker(workerErr)
			shutdownErr = err
		}

	case err := <-workerErr:
		if err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Error("Worker error", logger.Error(err))
			shutdownErr = err
		}
		a.shutdownHTTPServer()
	}

	a.logger.Info("Service stopped")
	return shutdownErr
}

// shutdown performs graceful shutdown of all components
func (a *App) shutdown(workerCancel context.CancelFunc, workerErr chan error, waitForWorker bool) {
	workerCancel()
	a.shutdownHTTPServer()

	if waitForWorker {
		a.waitForWorker(workerErr)
	}
}

// shutdownHTTPServer gracefully shuts down the HTTP server
func (a *App) shutdownHTTPServer() {
	if a.httpServer == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()

	if err := a.httpServer.Shutdown(ctx); err != nil {
		a.logger.Error("Server shutdown error", logger.Error(err))
	} else {
		a.logger.Info("HTTP server stopped")
	}
}

// waitForWorker waits for the worker goroutine to finish
func (a *App) waitForWorker(workerErr chan error) {
	err := <-workerErr
	if err != nil && !errors.Is(err, context.Canceled) {
		a.logger.Error("Worker error", logger.Error(err))
	} else {
		a.logger.Info("Worker stopped")
	}
}

// FlushCache flushes the Redis deduplication cache
func (a *App) FlushCache(ctx context.Context) error {
	return a.service.FlushCache(ctx)
}

// Close cleans up resources
func (a *App) Close() error {
	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil {
			a.logger.Warn("Failed to close Redis client", logger.Error(err))
		}
	}
	return a.logger.Sync()
}

// Logger returns the application logger
func (a *App) Logger() logger.Logger {
	return a.logger
}
