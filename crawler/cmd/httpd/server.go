package httpd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/constants"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	infracontext "github.com/north-cloud/infrastructure/context"
)

// startHTTPServer creates and starts the HTTP server.
// Returns the server and an error channel for server errors.
func startHTTPServer(
	deps *CommandDeps,
	searchManager api.SearchManager,
	jobsHandler *api.JobsHandler,
) (*http.Server, chan error, error) {
	server, _, err := api.StartHTTPServer(deps.Logger, searchManager, deps.Config, jobsHandler)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start server in goroutine
	deps.Logger.Info("Starting HTTP server", "addr", deps.Config.GetServerConfig().Address)
	errChan := make(chan error, errorChannelBufferSize)
	go func() {
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errChan <- serveErr
		}
	}()

	return server, errChan, nil
}

// runServerUntilInterrupt runs the server until interrupted by signal or error.
func runServerUntilInterrupt(
	log logger.Interface,
	server *http.Server,
	dbScheduler *job.DBScheduler,
	errChan chan error,
) error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, signalChannelBufferSize)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal or error
	select {
	case serverErr := <-errChan:
		log.Error("Server error", "error", serverErr)
		return fmt.Errorf("server error: %w", serverErr)
	case sig := <-sigChan:
		return shutdownServer(log, server, dbScheduler, sig)
	}
}

// shutdownServer performs graceful shutdown of the server and scheduler.
func shutdownServer(
	log logger.Interface,
	server *http.Server,
	dbScheduler *job.DBScheduler,
	sig os.Signal,
) error {
	log.Info("Shutdown signal received", "signal", sig.String())
	shutdownCtx, cancel := infracontext.WithTimeout(constants.DefaultShutdownTimeout)
	defer cancel()

	// Stop scheduler first
	if dbScheduler != nil {
		log.Info("Stopping database scheduler")
		if err := dbScheduler.Stop(); err != nil {
			log.Error("Failed to stop scheduler", "error", err)
		}
	}

	// Stop HTTP server
	log.Info("Stopping HTTP server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Failed to stop server", "error", err)
		return fmt.Errorf("failed to stop server: %w", err)
	}

	log.Info("Server stopped successfully")
	return nil
}
