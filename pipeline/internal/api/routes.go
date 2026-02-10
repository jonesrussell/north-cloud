package api

import (
	"github.com/gin-gonic/gin"
	infragin "github.com/north-cloud/infrastructure/gin"
)

// SetupRoutes configures all API routes.
func SetupRoutes(router *gin.Engine, ingestHandler *IngestHandler, funnelHandler *FunnelHandler, jwtSecret string) {
	v1 := infragin.ProtectedGroup(router, "/api/v1", jwtSecret)

	// Event ingest (write path)
	v1.POST("/events", ingestHandler.IngestEvent)
	v1.POST("/events/batch", ingestHandler.IngestBatch)

	// Funnel (read path)
	v1.GET("/funnel", funnelHandler.GetFunnel)
}
