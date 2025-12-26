package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jonesrussell/auth/internal/api"
	"github.com/jonesrussell/auth/internal/config"
	"github.com/jonesrussell/auth/internal/database"
	"github.com/jonesrussell/auth/internal/logger"
	"github.com/jonesrussell/auth/internal/middleware"
	"github.com/jonesrussell/auth/internal/repository"
	infracontext "github.com/north-cloud/infrastructure/context"
)

var (
	version = "dev"
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
		logger.String("service", "auth"),
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
				logger.Error(closeErr),
			)
		}
	}()

	// Initialize repository
	userRepo := repository.NewUserRepository(db.DB(), appLogger)

	// Initialize JWT middleware
	jwtMiddleware := middleware.NewJWTMiddleware(cfg, appLogger)

	// Initialize router
	router := api.NewRouter(userRepo, jwtMiddleware, cfg, appLogger)

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
	ctx, cancel := infracontext.WithShutdownTimeout()
	defer cancel()

	if shutdownErr := srv.Shutdown(ctx); shutdownErr != nil {
		appLogger.Error("Server forced to shutdown",
			logger.Error(shutdownErr),
		)
	}

	appLogger.Info("Server exited")
}

