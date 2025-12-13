package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/gosources/internal/api"
	"github.com/jonesrussell/gosources/internal/config"
	"github.com/jonesrussell/gosources/internal/database"
	"github.com/jonesrussell/gosources/internal/logger"
	"github.com/jonesrussell/gosources/internal/repository"
)

var (
	version                = "dev"
	defaultShutdownTimeout = 10
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		tempLogger, _ := logger.NewLogger(true)
		tempLogger.Error("Failed to load config",
			logger.String("config_path", configPath),
			logger.Error(err),
		)
		_ = tempLogger.Sync()
		os.Exit(1)
	}

	// Initialize logger
	appLogger, err := logger.NewLogger(cfg.Debug)
	if err != nil {
		tempLogger, _ := logger.NewLogger(true)
		tempLogger.Error("Failed to create logger",
			logger.Error(err),
		)
		_ = tempLogger.Sync()
		os.Exit(1)
	}
	defer func() {
		_ = appLogger.Sync()
	}()

	appLogger = appLogger.With(
		logger.String("service", "gosources"),
		logger.String("version", version),
	)

	// Initialize database
	db, err := database.New(cfg, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to database",
			logger.Error(err),
		)
		os.Exit(1)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			appLogger.Error("Failed to close database",
				logger.Error(err),
			)
		}
	}()

	// Initialize repository
	sourceRepo := repository.NewSourceRepository(db.DB(), appLogger)

	// Initialize router
	router := api.NewRouter(sourceRepo, appLogger)

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	go func() {
		appLogger.Info("Starting HTTP server",
			logger.String("host", cfg.Server.Host),
			logger.Int("port", cfg.Server.Port),
		)

		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			appLogger.Error("HTTP server failed",
				logger.Error(serveErr),
			)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down server")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(defaultShutdownTimeout)*time.Second)
	defer cancel()

	if shutdownErr := srv.Shutdown(ctx); shutdownErr != nil {
		appLogger.Error("Server forced to shutdown",
			logger.Error(shutdownErr),
		)
	}

	appLogger.Info("Server exited")
}
