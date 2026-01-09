package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/logging"
	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
)

// Default timeout values.
const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
)

// NewServer creates a new HTTP server using the infrastructure gin package.
func NewServer(handler *Handler, cfg *config.Config, _ logging.Logger, infraLog logger.Logger) *infragin.Server {
	// Build CORS config from service config
	corsConfig := infragin.CORSConfig{
		Enabled:          cfg.CORS.Enabled,
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           time.Duration(cfg.CORS.MaxAge) * time.Second,
	}

	// Build server using infrastructure gin package
	server := infragin.NewServerBuilder(cfg.Service.Name, cfg.Service.Port).
		WithLogger(infraLog).
		WithDebug(cfg.Service.Debug).
		WithVersion(cfg.Service.Version).
		WithTimeouts(defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout).
		WithCORS(corsConfig).
		WithRoutes(func(router *gin.Engine) {
			// Setup service-specific routes (health routes added by builder)
			SetupServiceRoutes(router, handler)
		}).
		Build()

	return server
}

// SetupServiceRoutes configures service-specific API routes (not health routes).
// Health routes are handled by the infrastructure gin package.
func SetupServiceRoutes(router *gin.Engine, handler *Handler) {
	// Additional readiness endpoint
	router.GET("/ready", handler.ReadinessCheck)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Health checks within API v1
		v1.GET("/health", handler.HealthCheck)
		v1.GET("/ready", handler.ReadinessCheck)

		// Search endpoints
		search := v1.Group("/search")
		search.POST("", handler.Search) // POST for complex searches
		search.GET("", handler.Search)  // GET for simple searches
	}
}
