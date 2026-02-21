// Package bootstrap handles application initialization and lifecycle management
// for the pipeline service.
package bootstrap

import (
	"fmt"

	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

// Start initializes and runs the pipeline service.
func Start() error {
	profiling.StartPprofServer()
	profiling.StartPyroscope("pipeline") //nolint:errcheck // env-gated, non-critical

	cfg, configErr := LoadConfig()
	if configErr != nil {
		return fmt.Errorf("config: %w", configErr)
	}

	log, logErr := CreateLogger(cfg)
	if logErr != nil {
		return fmt.Errorf("logger: %w", logErr)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting Pipeline Service",
		infralogger.String("name", cfg.Service.Name),
		infralogger.String("version", cfg.Service.Version),
		infralogger.Int("port", cfg.Service.Port),
	)

	db, dbErr := SetupDatabase(cfg)
	if dbErr != nil {
		return fmt.Errorf("database: %w", dbErr)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("Failed to close database", infralogger.Error(closeErr))
		}
	}()
	log.Info("Database connection established")

	server := SetupHTTPServer(cfg, db, log)

	if runErr := server.Run(); runErr != nil {
		log.Error("Server error", infralogger.Error(runErr))
		return fmt.Errorf("server: %w", runErr)
	}

	log.Info("Pipeline Service stopped")
	return nil
}
