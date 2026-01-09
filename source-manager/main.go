package main

import (
	"flag"
	"os"

	"github.com/jonesrussell/north-cloud/source-manager/internal/api"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/database"
	"github.com/jonesrussell/north-cloud/source-manager/internal/logger"
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

	// Create infrastructure logger for the gin server
	infraLog, logErr := infralogger.New(infralogger.Config{
		Level:       "info",
		Format:      "json",
		Development: cfg.Debug,
	})
	if logErr != nil {
		appLogger.Error("Failed to create infrastructure logger",
			logger.Error(logErr),
		)
		os.Exit(1)
	}
	defer func() { _ = infraLog.Sync() }()

	// Initialize server using infrastructure gin
	server := api.NewServer(sourceRepo, cfg, appLogger, infraLog)

	appLogger.Info("Starting HTTP server",
		logger.String("host", cfg.Server.Host),
		logger.Int("port", cfg.Server.Port),
	)

	// Run server with graceful shutdown
	if runErr := server.Run(); runErr != nil {
		appLogger.Error("Server error",
			logger.Error(runErr),
		)
		os.Exit(1)
	}

	appLogger.Info("Server exited")
}
