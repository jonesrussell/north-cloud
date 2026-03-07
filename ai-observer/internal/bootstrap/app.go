// Package bootstrap handles initialization for the ai-observer service.
package bootstrap

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/north-cloud/infrastructure/logger"
)

// Start initializes and runs the ai-observer service.
func Start() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log, err := CreateLogger(cfg)
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting AI Observer",
		logger.String("service", cfg.Service.Name),
		logger.String("version", cfg.Service.Version),
		logger.Bool("dry_run", cfg.Observer.DryRun),
	)

	if !cfg.Observer.Enabled {
		log.Info("AI Observer is disabled via AI_OBSERVER_ENABLED — exiting")
		return nil
	}

	// TODO Task 7: wire scheduler here
	log.Info("AI Observer scaffold running — scheduler not yet wired")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("AI Observer stopped")
	return nil
}
