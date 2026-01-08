package api

import (
	"os"

	"github.com/gin-gonic/gin"
	infrajwt "github.com/north-cloud/infrastructure/jwt"
	"github.com/north-cloud/infrastructure/monitoring"
)

// SetupRoutes configures all API routes
func SetupRoutes(router *gin.Engine, handler *Handler) {
	// Health and readiness checks
	router.GET("/health", handler.HealthCheck)
	router.GET("/ready", handler.ReadyCheck)

	// Memory health endpoint
	router.GET("/health/memory", func(c *gin.Context) {
		monitoring.MemoryHealthHandler(c.Writer, c.Request)
	})

	// API v1 routes - protected with JWT
	v1 := router.Group("/api/v1")
	// Add JWT middleware if JWT secret is configured
	if jwtSecret := os.Getenv("AUTH_JWT_SECRET"); jwtSecret != "" {
		v1.Use(infrajwt.Middleware(jwtSecret))
	}

	// Classification endpoints
	classify := v1.Group("/classify")
	classify.POST("", handler.Classify)                                    // POST /api/v1/classify
	classify.POST("/batch", handler.ClassifyBatch)                         // POST /api/v1/classify/batch
	classify.POST("/reclassify/:content_id", handler.ReclassifyDocument)   // POST /api/v1/classify/reclassify/:content_id
	classify.GET("/:content_id", handler.GetClassificationResult)         // GET /api/v1/classify/:content_id

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
}
