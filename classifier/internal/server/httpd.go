package server

import (
	"context"
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/classifier/internal/bootstrap"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

// StartHTTPServer starts the HTTP server for the classifier service (blocking).
// Returns error on failure; caller should os.Exit(1).
func StartHTTPServer() error {
	profiling.StartPprofServer()
	if pyroProfiler, pyroErr := profiling.StartPyroscope("classifier"); pyroErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Pyroscope failed to start: %v\n", pyroErr)
	} else if pyroProfiler != nil {
		defer func() {
			if stopErr := pyroProfiler.Stop(); stopErr != nil {
				fmt.Fprintf(os.Stderr, "WARNING: Pyroscope failed to stop: %v\n", stopErr)
			}
		}()
	}

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := bootstrap.CreateLogger(cfg)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	logger.Info("Starting classifier HTTP server",
		infralogger.Int("port", cfg.Service.Port),
		infralogger.Bool("debug", cfg.Service.Debug),
	)

	comps, err := bootstrap.NewHTTPComponents(cfg, logger)
	if err != nil {
		return fmt.Errorf("setup components: %w", err)
	}
	defer func() {
		_ = comps.DB.Close()
		_ = comps.InfraLog.Sync()
	}()

	if runErr := comps.Server.Run(); runErr != nil {
		logger.Error("Server error", infralogger.Error(runErr))
		return fmt.Errorf("server: %w", runErr)
	}
	return nil
}

// StartHTTPServerWithStop starts the HTTP server and returns a stop function.
// Used when running concurrently with processor.
func StartHTTPServerWithStop() (func(), error) {
	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		return nil, err
	}

	logger, err := bootstrap.CreateLogger(cfg)
	if err != nil {
		return nil, err
	}

	logger.Info("Starting classifier HTTP server",
		infralogger.Int("port", cfg.Service.Port),
		infralogger.Bool("debug", cfg.Service.Debug),
	)

	comps, err := bootstrap.NewHTTPComponents(cfg, logger)
	if err != nil {
		return nil, err
	}

	serverErrors := make(chan error, 1)
	go func() {
		if startErr := comps.Server.Start(); startErr != nil {
			serverErrors <- startErr
		}
	}()

	go func() {
		if serverErr := <-serverErrors; serverErr != nil {
			logger.Error("Server error", infralogger.Error(serverErr))
		}
	}()

	stopFunc := func() {
		logger.Info("Stopping HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), bootstrap.HTTPShutdownTimeout())
		defer cancel()

		if shutdownErr := comps.Server.Shutdown(shutdownCtx); shutdownErr != nil {
			logger.Error("Graceful shutdown failed", infralogger.Error(shutdownErr))
		} else {
			logger.Info("HTTP server stopped gracefully")
		}

		_ = comps.DB.Close()
		_ = comps.InfraLog.Sync()
	}

	return stopFunc, nil
}
