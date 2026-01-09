package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/auth/internal/api"
	"github.com/jonesrussell/north-cloud/auth/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

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
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		return 1
	}

	// Validate configuration
	if validationErr := cfg.Validate(); validationErr != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", validationErr)
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

	log.Info("Starting auth service",
		logger.String("name", cfg.Service.Name),
		logger.Int("port", cfg.Service.Port),
		logger.Bool("debug", cfg.Service.Debug),
	)

	// Create server
	srv, srvErr := api.NewServer(cfg, log)
	if srvErr != nil {
		log.Error("Failed to create server", logger.Error(srvErr))
		return 1
	}

	// Run server with graceful shutdown
	if runErr := srv.Run(); runErr != nil {
		log.Error("Server error", logger.Error(runErr))
		return 1
	}

	return 0
}
