package api

import (
	"time"

	"github.com/gin-gonic/gin"
	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/search/internal/config"
)

// Default timeout values.
const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
)

// ServerDeps holds optional dependencies for health checks.
type ServerDeps struct {
	ESPing func() error
}

// NewServer creates a new HTTP server using the infrastructure gin package.
func NewServer(handler *Handler, cfg *config.Config, infraLog infralogger.Logger, deps *ServerDeps) *infragin.Server {
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
	builder := infragin.NewServerBuilder(cfg.Service.Name, cfg.Service.Port).
		WithLogger(infraLog).
		WithDebug(cfg.Service.Debug).
		WithVersion(cfg.Service.Version).
		WithTimeouts(defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout).
		WithCORS(corsConfig).
		WithMetrics()

	// Wire dependency health checks
	if deps != nil && deps.ESPing != nil {
		builder = builder.WithElasticsearchHealthCheck(deps.ESPing)
	}

	server := builder.
		WithRoutes(func(router *gin.Engine) {
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

	// Public feed (no auth): stable URL for static-site consumers at build time
	router.GET("/feed.json", handler.PublicFeed)

	// Community search (consumed by Minoo at /api/communities/search)
	communities := router.Group("/api/communities")
	communities.GET("/search", handler.SearchCommunities)

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

		// Topic-filtered feeds (no auth): /api/v1/feeds/{slug}
		feeds := v1.Group("/feeds")
		feeds.GET("/:slug", handler.TopicFeed)
	}
}
