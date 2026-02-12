package api

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all API routes
func SetupRoutes(router *gin.Engine, handler *Handler) {
	// Health routes are handled by the infrastructure/gin package (exposes /health)
	// No manual health route needed here

	// API v1 routes
	v1 := router.Group("/api/v1")
	// Index management endpoints
	indexes := v1.Group("/indexes")
	indexes.POST("", handler.CreateIndex)                      // POST /api/v1/indexes
	indexes.GET("", handler.ListIndices)                       // GET /api/v1/indexes
	indexes.GET("/:index_name", handler.GetIndex)              // GET /api/v1/indexes/:index_name
	indexes.DELETE("/:index_name", handler.DeleteIndex)        // DELETE /api/v1/indexes/:index_name
	indexes.GET("/:index_name/health", handler.GetIndexHealth) // GET /api/v1/indexes/:index_name/health
	indexes.POST("/:index_name/migrate", handler.MigrateIndex) // POST /api/v1/indexes/:index_name/migrate

	// Document management endpoints
	indexes.GET("/:index_name/documents", handler.QueryDocuments)                   // GET /api/v1/indexes/:index_name/documents
	indexes.GET("/:index_name/documents/:document_id", handler.GetDocument)         // GET /api/v1/indexes/:index_name/documents/:document_id
	indexes.PUT("/:index_name/documents/:document_id", handler.UpdateDocument)      // PUT /api/v1/indexes/:index_name/documents/:document_id
	indexes.DELETE("/:index_name/documents/:document_id", handler.DeleteDocument)   // DELETE /api/v1/indexes/:index_name/documents/:document_id
	indexes.POST("/:index_name/documents/bulk-delete", handler.BulkDeleteDocuments) // POST /api/v1/indexes/:index_name/documents/bulk-delete

	// Bulk operations
	bulk := v1.Group("/indexes/bulk")
	bulk.POST("/create", handler.BulkCreateIndexes)   // POST /api/v1/indexes/bulk/create
	bulk.DELETE("/delete", handler.BulkDeleteIndexes) // DELETE /api/v1/indexes/bulk/delete

	// Source-based operations
	sources := v1.Group("/sources")
	sources.POST("/:source_name/indexes", handler.CreateIndexesForSource)   // POST /api/v1/sources/:source_name/indexes
	sources.GET("/:source_name/indexes", handler.ListIndexesForSource)      // GET /api/v1/sources/:source_name/indexes
	sources.DELETE("/:source_name/indexes", handler.DeleteIndexesForSource) // DELETE /api/v1/sources/:source_name/indexes

	// Statistics
	v1.GET("/stats", handler.GetStats) // GET /api/v1/stats

	// Aggregation routes
	aggregations := v1.Group("/aggregations")
	aggregations.GET("/crime", handler.GetCrimeAggregation)       // GET /api/v1/aggregations/crime
	aggregations.GET("/location", handler.GetLocationAggregation) // GET /api/v1/aggregations/location
	aggregations.GET("/overview", handler.GetOverviewAggregation) // GET /api/v1/aggregations/overview
	aggregations.GET("/mining", handler.GetMiningAggregation)     // GET /api/v1/aggregations/mining
	aggregations.GET("/source-health", handler.GetSourceHealth)   // GET /api/v1/aggregations/source-health
}
