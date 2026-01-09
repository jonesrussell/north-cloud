package api

import (
	"time"

	"github.com/gin-gonic/gin"
	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
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
func NewServer(handler *Handler, config ServerConfig, _ Logger, infraLog logger.Logger) *infragin.Server {
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
func SetupServiceRoutes(router *gin.Engine, handler *Handler) {
	// API v1 routes
	v1 := router.Group("/api/v1")

	// Index management endpoints
	indexes := v1.Group("/indexes")
	indexes.POST("", handler.CreateIndex)                      // POST /api/v1/indexes
	indexes.GET("", handler.ListIndices)                       // GET /api/v1/indexes
	indexes.GET("/:index_name", handler.GetIndex)              // GET /api/v1/indexes/:index_name
	indexes.DELETE("/:index_name", handler.DeleteIndex)        // DELETE /api/v1/indexes/:index_name
	indexes.GET("/:index_name/health", handler.GetIndexHealth) // GET /api/v1/indexes/:index_name/health

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
}
