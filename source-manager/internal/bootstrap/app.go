// Package bootstrap handles application initialization and lifecycle management
// for the source-manager service.
package bootstrap

import (
	"context"
	"fmt"
	"os"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/infrastructure/profiling"
	"github.com/jonesrussell/north-cloud/infrastructure/provider/anthropic"
	"github.com/jonesrussell/north-cloud/source-manager/internal/aiverify"
	"github.com/jonesrussell/north-cloud/source-manager/internal/icpstore"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

const version = "dev"

// Start initializes and starts the source-manager application.
func Start() error {
	// Phase 0: Start profiling server (if enabled)
	profiling.StartPprofServer()
	if pyroProfiler, pyroErr := profiling.StartPyroscope("source-manager"); pyroErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Pyroscope failed to start: %v\n", pyroErr)
	} else if pyroProfiler != nil {
		defer func() {
			if stopErr := pyroProfiler.Stop(); stopErr != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Pyroscope failed to stop: %v\n", stopErr)
			}
		}()
	}

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

	icpStore, err := icpstore.New(cfg.ICP.SegmentsPath, cfg.ICP.ReloadInterval, log)
	if err != nil {
		return fmt.Errorf("failed to load ICP seed: %w", err)
	}
	icpCtx, icpCancel := context.WithCancel(context.Background())
	defer icpCancel()
	go icpStore.Run(icpCtx)

	// Phase 4: Setup and run HTTP server
	server := SetupHTTPServer(cfg, db, publisher, icpStore, log)

	// Phase 4.5: Verification worker (optional, disabled by default)
	if cfg.Verification.AIEnabled {
		verifyClient := anthropic.New(cfg.Verification.AnthropicAPIKey, cfg.Verification.AnthropicModel)
		verifier := aiverify.NewLLMVerifier(verifyClient)
		verificationRepo := repository.NewVerificationRepository(db.DB(), log)
		worker := aiverify.NewWorker(verificationRepo, verifier, aiverify.WorkerConfig{
			Interval:            cfg.Verification.Interval,
			BatchSize:           cfg.Verification.BatchSize,
			AutoVerifyThreshold: cfg.Verification.AutoVerifyThreshold,
			AutoRejectThreshold: cfg.Verification.AutoRejectThreshold,
		}, log)

		verifyCtx, verifyCancel := context.WithCancel(context.Background())
		defer verifyCancel()
		go worker.Run(verifyCtx)
	}

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
