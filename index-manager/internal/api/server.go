package api

import (
	"time"

	"github.com/gin-gonic/gin"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Default timeout values.
const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
	serviceVersion      = "1.0.0"
)

// ServerConfig holds server configuration.
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Debug        bool
	ServiceName  string
}

// NewServer creates a new HTTP server using the infrastructure gin package.
func NewServer(handler *Handler, config ServerConfig, infraLog infralogger.Logger) *infragin.Server {
	// Set timeout defaults if not provided
	readTimeout := config.ReadTimeout
	if readTimeout == 0 {
		readTimeout = defaultReadTimeout
	}
	writeTimeout := config.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = defaultWriteTimeout
	}
	serviceName := config.ServiceName
	if serviceName == "" {
		serviceName = "index-manager"
	}

	// Build server using infrastructure gin package
	server := infragin.NewServerBuilder(serviceName, config.Port).
		WithLogger(infraLog).
		WithDebug(config.Debug).
		WithVersion(serviceVersion).
		WithTimeouts(readTimeout, writeTimeout, defaultIdleTimeout).
		WithRoutes(func(router *gin.Engine) {
			// Setup service-specific routes (health routes added by builder)
			SetupServiceRoutes(router, handler)
		}).
		Build()

	return server
}

// SetupServiceRoutes configures service-specific API routes (not health routes).
// Health routes are handled by the infrastructure gin package.
// Delegates to SetupRoutes to avoid duplication.
func SetupServiceRoutes(router *gin.Engine, handler *Handler) {
	SetupRoutes(router, handler)
}
