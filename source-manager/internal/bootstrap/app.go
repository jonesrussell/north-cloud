// Package bootstrap handles application initialization and lifecycle management
// for the source-manager service.
package bootstrap

import (
	"fmt"

	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

const version = "dev"

// Start initializes and starts the source-manager application.
func Start() error {
	// Phase 0: Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Phase 1: Load config and create logger
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log, err := CreateLogger(cfg, version)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	// Phase 2: Setup database
	db, err := SetupDatabase(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("Failed to close database", infralogger.Error(closeErr))
		}
	}()

	// Phase 3: Setup event publisher (optional)
	publisher := SetupEventPublisher(cfg, log)

	// Phase 4: Setup and run HTTP server
	server := SetupHTTPServer(cfg, db, publisher, log)

	log.Info("Starting HTTP server",
		infralogger.String("host", cfg.Server.Host),
		infralogger.Int("port", cfg.Server.Port),
	)

	if runErr := server.Run(); runErr != nil {
		log.Error("Server error", infralogger.Error(runErr))
		return fmt.Errorf("server error: %w", runErr)
	}

	log.Info("Server exited")
	return nil
}
