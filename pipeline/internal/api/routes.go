package api

import (
	"github.com/gin-gonic/gin"
	infragin "github.com/north-cloud/infrastructure/gin"
)

// SetupRoutes configures all API routes.
// Ingest endpoints are public (internal service-to-service calls within Docker network).
// Read endpoints are protected with JWT.
func SetupRoutes(router *gin.Engine, ingestHandler *IngestHandler, funnelHandler *FunnelHandler, jwtSecret string) {
	public, protected := infragin.SetupAPIRoutesWithPublic(router, jwtSecret)

	// Event ingest (write path) — public, called by other services
	public.POST("/events", ingestHandler.IngestEvent)
	public.POST("/events/batch", ingestHandler.IngestBatch)

	// Funnel (read path) — protected
	protected.GET("/funnel", funnelHandler.GetFunnel)
}
