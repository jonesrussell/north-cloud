package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/logging"
)

// Server holds the HTTP server
type Server struct {
	router *gin.Engine
	server *http.Server
	logger logging.Logger
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Debug        bool
}

// NewServer creates a new HTTP server
func NewServer(handler *Handler, cfg *config.Config, log logging.Logger) *Server {
	// Set Gin mode
	if !cfg.Service.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Add middleware
	router.Use(RecoveryMiddleware(log))
	router.Use(LoggerMiddleware(log))
	router.Use(CORSMiddleware(&cfg.CORS))

	// Setup routes
	SetupRoutes(router, handler)

	// Create HTTP server
	const (
		readTimeoutSeconds  = 30
		writeTimeoutSeconds = 60
	)
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Service.Port),
		Handler:      router,
		ReadTimeout:  readTimeoutSeconds * time.Second,
		WriteTimeout: writeTimeoutSeconds * time.Second,
	}

	return &Server{
		router: router,
		server: server,
		logger: log,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server", "addr", s.server.Addr)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info("HTTP server stopped")
	return nil
}
