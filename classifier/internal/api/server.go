package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Default timeout values.
const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
)

// ServerConfig holds server configuration.
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Debug        bool
}

// NewServer creates a new HTTP server using the infrastructure gin package.
func NewServer(handler *Handler, serverCfg ServerConfig, cfg *config.Config, infraLog infralogger.Logger) *infragin.Server {
	// Set timeout defaults if not provided
	readTimeout := serverCfg.ReadTimeout
	if readTimeout == 0 {
		readTimeout = defaultReadTimeout
	}
	writeTimeout := serverCfg.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = defaultWriteTimeout
	}

	// Build server using infrastructure gin package
	server := infragin.NewServerBuilder(cfg.Service.Name, serverCfg.Port).
		WithLogger(infraLog).
		WithDebug(serverCfg.Debug).
		WithVersion(cfg.Service.Version).
		WithTimeouts(readTimeout, writeTimeout, defaultIdleTimeout).
		WithRoutes(func(router *gin.Engine) {
			// Setup service-specific routes (health routes added by builder)
			SetupServiceRoutes(router, handler, cfg)
		}).
		Build()

	return server
}

// SetupServiceRoutes configures service-specific API routes (not health routes).
// Health routes are handled by the infrastructure gin package.
func SetupServiceRoutes(router *gin.Engine, handler *Handler, cfg *config.Config) {
	// API v1 routes - protected with JWT
	v1 := infragin.ProtectedGroup(router, "/api/v1", cfg.Auth.JWTSecret)

	// Classification endpoints
	classify := v1.Group("/classify")
	classify.POST("", handler.Classify)                                  // POST /api/v1/classify
	classify.POST("/batch", handler.ClassifyBatch)                       // POST /api/v1/classify/batch
	classify.POST("/reclassify/:content_id", handler.ReclassifyDocument) // POST /api/v1/classify/reclassify/:content_id
	classify.GET("/:content_id", handler.GetClassificationResult)        // GET /api/v1/classify/:content_id

	// Rules management endpoints
	rules := v1.Group("/rules")
	rules.GET("", handler.ListRules)         // GET /api/v1/rules
	rules.POST("", handler.CreateRule)       // POST /api/v1/rules
	rules.PUT("/:id", handler.UpdateRule)    // PUT /api/v1/rules/:id
	rules.DELETE("/:id", handler.DeleteRule) // DELETE /api/v1/rules/:id

	// Source reputation endpoints
	sources := v1.Group("/sources")
	sources.GET("", handler.ListSources)                // GET /api/v1/sources
	sources.GET("/:name", handler.GetSource)            // GET /api/v1/sources/:name
	sources.PUT("/:name", handler.UpdateSource)         // PUT /api/v1/sources/:name
	sources.GET("/:name/stats", handler.GetSourceStats) // GET /api/v1/sources/:name/stats

	// Statistics endpoints
	stats := v1.Group("/stats")
	stats.GET("", handler.GetStats)                      // GET /api/v1/stats
	stats.GET("/topics", handler.GetTopicStats)          // GET /api/v1/stats/topics
	stats.GET("/sources", handler.GetSourceDistribution) // GET /api/v1/stats/sources

	// Metrics endpoints
	metrics := v1.Group("/metrics")
	metrics.GET("/ml-health", handler.GetMLHealth) // GET /api/v1/metrics/ml-health

	// Keep original health check handlers for backward compatibility (/ready)
	router.GET("/ready", handler.ReadyCheck)
}
