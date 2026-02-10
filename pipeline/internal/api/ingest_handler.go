// Package api provides HTTP handlers for the pipeline observability service.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

// Ingester defines the ingest operations needed by the handler.
type Ingester interface {
	Ingest(ctx context.Context, req *domain.IngestRequest) error
	IngestBatch(ctx context.Context, req *domain.BatchIngestRequest) (int, error)
	GetFunnel(ctx context.Context, from, to time.Time) (*domain.FunnelResponse, error)
}

// IngestHandler handles event ingestion HTTP requests.
type IngestHandler struct {
	svc Ingester
}

// NewIngestHandler creates a new ingest handler.
func NewIngestHandler(svc Ingester) *IngestHandler {
	return &IngestHandler{svc: svc}
}

// IngestEvent handles POST /api/v1/events.
func (h *IngestHandler) IngestEvent(c *gin.Context) {
	var req domain.IngestRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	if ingestErr := h.svc.Ingest(c.Request.Context(), &req); ingestErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": ingestErr.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "ingested"})
}

// IngestBatch handles POST /api/v1/events/batch.
func (h *IngestHandler) IngestBatch(c *gin.Context) {
	var req domain.BatchIngestRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	ingested, ingestErr := h.svc.IngestBatch(c.Request.Context(), &req)
	if ingestErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    ingestErr.Error(),
			"ingested": ingested,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":   "ingested",
		"ingested": ingested,
	})
}
