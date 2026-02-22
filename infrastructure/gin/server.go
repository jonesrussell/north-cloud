package gin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/north-cloud/infrastructure/logger"
)

// Server represents an HTTP server with lifecycle management.
type Server struct {
	router *gin.Engine
	server *http.Server
	logger logger.Logger
	config *Config
}

// NewServer creates a new HTTP server with the given configuration.
// The setupRoutes function is called to configure service-specific routes
// after standard middleware has been applied.
func NewServer(cfg *Config, log logger.Logger, setupRoutes func(*gin.Engine)) *Server {
	cfg.SetDefaults()

	// Set Gin mode based on debug flag
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Apply standard middleware in correct order
	// 1. Recovery first to catch panics
	router.Use(RecoveryMiddleware(log))

	// 2. Request ID + context-scoped logger
	router.Use(RequestIDLoggerMiddleware(log))

	// 3. Request logging (picks up request_id set by step 2)
	router.Use(LoggerMiddleware(log))

	// 4. CORS handling
	router.Use(CORSMiddleware(cfg.CORS))

	// Call service-specific route setup
	if setupRoutes != nil {
		setupRoutes(router)
	}

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return &Server{
		router: router,
		server: httpServer,
		logger: log,
		config: cfg,
	}
}

// Router returns the underlying Gin engine for additional configuration.
func (s *Server) Router() *gin.Engine {
	return s.router
}

// HTTPServer returns the underlying http.Server.
func (s *Server) HTTPServer() *http.Server {
	return s.server
}

// Config returns the server configuration.
func (s *Server) Config() *Config {
	return s.config
}

// Start starts the HTTP server in a blocking manner.
// Returns when the server is shut down or encounters an error.
func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server",
		logger.String("address", s.server.Addr),
		logger.String("service", s.config.ServiceName),
		logger.String("version", s.config.ServiceVersion),
		logger.Duration("read_timeout", s.server.ReadTimeout),
		logger.Duration("write_timeout", s.server.WriteTimeout),
	)

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// StartAsync starts the HTTP server in a goroutine and returns immediately.
// Returns an error channel that will receive any server errors.
func (s *Server) StartAsync() <-chan error {
	errCh := make(chan error, 1)

	go func() {
		if err := s.Start(); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	return errCh
}

// Shutdown gracefully shuts down the server with the configured timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server",
		logger.Duration("timeout", s.config.ShutdownTimeout),
	)

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	s.logger.Info("HTTP server stopped gracefully")
	return nil
}

// ShutdownWithTimeout gracefully shuts down the server with a custom timeout.
func (s *Server) ShutdownWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.Shutdown(ctx)
}

// RunWithGracefulShutdown starts the server and handles graceful shutdown
// on SIGINT or SIGTERM signals or when the context is cancelled.
// This is a convenience method for the common case of running a server.
func (s *Server) RunWithGracefulShutdown(ctx context.Context) error {
	// Start server in goroutine
	errCh := s.StartAsync()

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Wait for signal, error, or context cancellation
	select {
	case err := <-errCh:
		return err
	case sig := <-sigCh:
		s.logger.Info("Shutdown signal received",
			logger.String("signal", sig.String()),
		)
	case <-ctx.Done():
		s.logger.Info("Context cancelled, shutting down")
	}

	// Perform graceful shutdown - use fresh context since the original may be cancelled
	//nolint:contextcheck // Intentional: need fresh context for shutdown when original is cancelled
	return s.Shutdown(context.Background())
}

// Run is a convenience method that creates a context and runs the server
// with graceful shutdown handling.
func (s *Server) Run() error {
	return s.RunWithGracefulShutdown(context.Background())
}
