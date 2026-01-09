package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
)

// Server represents the HTTP server
type Server struct {
	router *gin.Engine
	server *http.Server
	logger Logger
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Debug        bool
}

// NewServer creates a new HTTP server
func NewServer(handler *Handler, serverCfg ServerConfig, logger Logger, cfg *config.Config) *Server {
	// Set Gin mode based on debug flag
	if !serverCfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
	router.Use(LoggerMiddleware(logger))
	router.Use(CORSMiddleware())

	// Setup routes
	SetupRoutes(router, handler, cfg)

	// Create HTTP server
	addr := fmt.Sprintf(":%d", serverCfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  serverCfg.ReadTimeout,
		WriteTimeout: serverCfg.WriteTimeout,
	}

	return &Server{
		router: router,
		server: server,
		logger: logger,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("Starting HTTP server",
		"address", s.server.Addr,
		"read_timeout", s.server.ReadTimeout,
		"write_timeout", s.server.WriteTimeout,
	)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info("HTTP server shut down successfully")
	return nil
}

// LoggerMiddleware creates a Gin middleware for logging
func LoggerMiddleware(logger Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request
		c.Next()

		// Log after request
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		logger.Info("HTTP request",
			"method", method,
			"path", path,
			"status", statusCode,
			"duration_ms", duration.Milliseconds(),
			"client_ip", c.ClientIP(),
		)

		// Log errors if any
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				logger.Error("Request error",
					"method", method,
					"path", path,
					"error", err.Error(),
				)
			}
		}
	}
}

// CORSMiddleware creates a Gin middleware for CORS
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
