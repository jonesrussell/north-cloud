package main

import (
	"flag"
	"os"

	"github.com/jonesrussell/north-cloud/source-manager/internal/api"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/database"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

var (
	version = "dev"
)

func main() {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	var configPath string
	flag.StringVar(&configPath, "config", "config.yml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		tempLogger, _ := infralogger.New(infralogger.Config{
			Level:       "debug",
			Format:      "json",
			Development: true,
		})
		tempLogger.Error("Failed to load config",
			infralogger.String("config_path", configPath),
			infralogger.Error(err),
		)
		_ = tempLogger.Sync()
		os.Exit(1)
	}

	// Initialize logger using infrastructure logger
	// Always use JSON format for consistency
	appLogger, err := infralogger.New(infralogger.Config{
		Level:       "info",
		Format:      "json",
		Development: cfg.Debug,
	})
	if err != nil {
		tempLogger, _ := infralogger.New(infralogger.Config{
			Level:       "debug",
			Format:      "json",
			Development: true,
		})
		tempLogger.Error("Failed to create logger",
			infralogger.Error(err),
		)
		_ = tempLogger.Sync()
		os.Exit(1)
	}
	defer func() {
		_ = appLogger.Sync()
	}()

	appLogger = appLogger.With(
		infralogger.String("service", "gosources"),
		infralogger.String("version", version),
	)

	// Initialize database
	db, err := database.New(cfg, appLogger)
	if err != nil {
		appLogger.Error("Failed to connect to database",
			infralogger.Error(err),
		)
		os.Exit(1)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			appLogger.Error("Failed to close database",
				infralogger.Error(closeErr),
			)
		}
	}()

	// Initialize repository
	sourceRepo := repository.NewSourceRepository(db.DB(), appLogger)

	// Initialize server using infrastructure gin
	server := api.NewServer(sourceRepo, cfg, appLogger)

	appLogger.Info("Starting HTTP server",
		infralogger.String("host", cfg.Server.Host),
		infralogger.Int("port", cfg.Server.Port),
	)

	// Run server with graceful shutdown
	if runErr := server.Run(); runErr != nil {
		appLogger.Error("Server error",
			infralogger.Error(runErr),
		)
		os.Exit(1)
	}

	appLogger.Info("Server exited")
}
