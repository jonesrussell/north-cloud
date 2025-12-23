package api

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all API routes
func SetupRoutes(router *gin.Engine, handler *Handler) {
	// Health and readiness checks
	router.GET("/api/v1/health", handler.HealthCheck)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Index management endpoints
		indexes := v1.Group("/indexes")
		{
			indexes.POST("", handler.CreateIndex)                      // POST /api/v1/indexes
			indexes.GET("", handler.ListIndices)                       // GET /api/v1/indexes
			indexes.GET("/:index_name", handler.GetIndex)              // GET /api/v1/indexes/:index_name
			indexes.DELETE("/:index_name", handler.DeleteIndex)        // DELETE /api/v1/indexes/:index_name
			indexes.GET("/:index_name/health", handler.GetIndexHealth) // GET /api/v1/indexes/:index_name/health
		}

		// Bulk operations
		bulk := v1.Group("/indexes/bulk")
		{
			bulk.POST("/create", handler.BulkCreateIndexes)   // POST /api/v1/indexes/bulk/create
			bulk.DELETE("/delete", handler.BulkDeleteIndexes) // DELETE /api/v1/indexes/bulk/delete
		}

		// Source-based operations
		sources := v1.Group("/sources")
		{
			sources.POST("/:source_name/indexes", handler.CreateIndexesForSource)   // POST /api/v1/sources/:source_name/indexes
			sources.GET("/:source_name/indexes", handler.ListIndexesForSource)      // GET /api/v1/sources/:source_name/indexes
			sources.DELETE("/:source_name/indexes", handler.DeleteIndexesForSource) // DELETE /api/v1/sources/:source_name/indexes
		}

		// Statistics
		v1.GET("/stats", handler.GetStats) // GET /api/v1/stats
	}
}
