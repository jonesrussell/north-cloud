// Package api provides HTTP handlers for the pipeline observability service.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/pipeline/internal/domain"
)

// period24Hours is the duration for the "24h" period option.
const period24Hours = 24 * time.Hour

// periodDays7 is the number of days for the "7d" period option.
const periodDays7 = 7

// periodDays30 is the number of days for the "30d" period option.
const periodDays30 = 30

// FunnelQuerier defines the funnel query operations needed by the handler.
type FunnelQuerier interface {
	GetFunnel(ctx context.Context, from, to time.Time) (*domain.FunnelResponse, error)
}

// FunnelHandler handles funnel query HTTP requests.
type FunnelHandler struct {
	svc FunnelQuerier
}

// NewFunnelHandler creates a new funnel handler.
func NewFunnelHandler(svc FunnelQuerier) *FunnelHandler {
	return &FunnelHandler{svc: svc}
}

// GetFunnel handles GET /api/v1/funnel.
func (h *FunnelHandler) GetFunnel(c *gin.Context) {
	period := c.DefaultQuery("period", "today")

	from, to := resolvePeriod(period)

	response, queryErr := h.svc.GetFunnel(c.Request.Context(), from, to)
	if queryErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": queryErr.Error()})
		return
	}

	response.Period = period
	response.Timezone = "UTC"

	c.JSON(http.StatusOK, response)
}

// resolvePeriod converts a period string to a time range.
func resolvePeriod(period string) (from, to time.Time) {
	now := time.Now().UTC()
	to = now

	switch period {
	case "24h":
		from = now.Add(-period24Hours)
	case "7d":
		from = now.AddDate(0, 0, -periodDays7)
	case "30d":
		from = now.AddDate(0, 0, -periodDays30)
	default: // "today"
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}

	return from, to
}
