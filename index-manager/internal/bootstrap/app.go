// Package bootstrap handles application initialization and lifecycle management
// for the index-manager service.
package bootstrap

import (
	"fmt"

	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

// Start initializes and starts the index-manager application.
func Start() error {
	// Phase 0: Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Phase 1: Load config and create logger
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log, err := CreateLogger(cfg)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting Index Manager Service",
		infralogger.String("name", cfg.Service.Name),
		infralogger.String("version", cfg.Service.Version),
		infralogger.Int("port", cfg.Service.Port),
	)

	// Phase 2: Setup Elasticsearch
	esClient, err := SetupElasticsearch(cfg)
	if err != nil {
		return fmt.Errorf("failed to setup Elasticsearch: %w", err)
	}
	log.Info("Elasticsearch client initialized")

	// Phase 3: Setup database
	db, err := SetupDatabase(cfg)
	if err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("Failed to close database connection", infralogger.Error(closeErr))
		}
	}()
	log.Info("Database connection established")

	// Phase 3b: Check for mapping version drift
	CheckMappingVersionDrift(db, log)

	// Phase 4: Setup and run HTTP server
	server := SetupHTTPServer(cfg, esClient, db, log)

	if runErr := server.Run(); runErr != nil {
		log.Error("Server error", infralogger.Error(runErr))
		return fmt.Errorf("server error: %w", runErr)
	}

	log.Info("Index Manager Service stopped")
	return nil
}
