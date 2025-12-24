// Package httpd implements the HTTP server for the crawler service.
package httpd

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
)

// Start starts the HTTP server and runs until interrupted.
// It handles graceful shutdown on SIGINT or SIGTERM signals.
func Start() error {
	// Phase 1: Initialize dependencies
	deps, err := newCommandDeps()
	if err != nil {
		return fmt.Errorf("failed to initialize dependencies: %w", err)
	}

	// Phase 2: Setup storage
	storageResult, err := createStorage(deps.Config, deps.Logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	// Phase 3: Create search manager
	searchManager := storage.NewSearchManager(storageResult.Storage, deps.Logger)

	// Phase 4: Setup jobs handler and scheduler
	jobsHandler, dbScheduler, db := setupJobsAndScheduler(deps, storageResult)
	if db != nil {
		defer db.Close()
	}

	// Phase 5: Start HTTP server
	server, errChan, err := startHTTPServer(deps, searchManager, jobsHandler)
	if err != nil {
		return err
	}

	// Phase 6: Run server until interrupted
	return runServerUntilInterrupt(deps.Logger, server, dbScheduler, errChan)
}
