package api

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all API routes
func SetupRoutes(router *gin.Engine, handler *Handler) {
	// Health checks (no /api/v1 prefix for standard health endpoints)
	router.GET("/health", handler.HealthCheck)
	router.GET("/ready", handler.ReadinessCheck)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Health checks
		v1.GET("/health", handler.HealthCheck)
		v1.GET("/ready", handler.ReadinessCheck)

		// Search endpoints
		search := v1.Group("/search")
		{
			search.POST("", handler.Search) // POST for complex searches
			search.GET("", handler.Search)  // GET for simple searches
		}
	}
}
