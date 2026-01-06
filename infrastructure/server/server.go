// Package server provides HTTP server utilities including graceful shutdown.
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/north-cloud/infrastructure/logger"
)

// DefaultShutdownTimeout is the default timeout for graceful shutdown.
const DefaultShutdownTimeout = 30 * time.Second

// Config holds server configuration.
type Config struct {
	Address         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// SetDefaults applies default values to the config.
func (c *Config) SetDefaults() {
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 30 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 60 * time.Second
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}
}

// New creates a new HTTP server with the given configuration and handler.
func New(cfg Config, handler http.Handler) *http.Server {
	cfg.SetDefaults()
	return &http.Server{
		Addr:         cfg.Address,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}
}

// RunWithGracefulShutdown runs the server and handles graceful shutdown on
// SIGINT or SIGTERM signals or when the context is cancelled.
func RunWithGracefulShutdown(ctx context.Context, srv *http.Server, log logger.Logger) error {
	return RunWithGracefulShutdownTimeout(ctx, srv, log, DefaultShutdownTimeout)
}

// RunWithGracefulShutdownTimeout runs the server with a custom shutdown timeout.
func RunWithGracefulShutdownTimeout(ctx context.Context, srv *http.Server, log logger.Logger, shutdownTimeout time.Duration) error {
	// Create error channel for server errors
	errCh := make(chan error, 1)

	// Start server in goroutine
	go func() {
		log.Info("Starting HTTP server", logger.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Wait for signal, error, or context cancellation
	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigCh:
		log.Info("Shutdown signal received", logger.String("signal", sig.String()))
	case <-ctx.Done():
		log.Info("Context cancelled, shutting down")
	}

	// Perform graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	log.Info("Shutting down HTTP server", logger.Duration("timeout", shutdownTimeout))
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	log.Info("HTTP server stopped gracefully")
	return nil
}

// ListenAndServe starts the server without graceful shutdown handling.
// Use RunWithGracefulShutdown for production servers.
func ListenAndServe(srv *http.Server) error {
	return srv.ListenAndServe()
}

// Shutdown performs graceful shutdown of the server.
func Shutdown(ctx context.Context, srv *http.Server) error {
	return srv.Shutdown(ctx)
}

// ShutdownWithTimeout performs graceful shutdown with a timeout.
func ShutdownWithTimeout(srv *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return srv.Shutdown(ctx)
}
