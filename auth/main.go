package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/auth/internal/api"
	"github.com/jonesrussell/north-cloud/auth/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
	"github.com/north-cloud/infrastructure/server"
)

func main() {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting auth service",
		logger.String("name", cfg.Service.Name),
		logger.Int("port", cfg.Service.Port),
		logger.Bool("debug", cfg.Service.Debug),
	)

	// Create server
	srv, err := api.NewServer(cfg, log)
	if err != nil {
		log.Error("Failed to create server", logger.Error(err))
		os.Exit(1)
	}

	// Run server with graceful shutdown
	if err := server.RunWithGracefulShutdown(context.Background(), srv, log); err != nil {
		log.Error("Server error", logger.Error(err))
		os.Exit(1)
	}
}
