// Package api implements the HTTP API for the crawler service.
package api

import (
	"github.com/gin-gonic/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/sse"
)

// SSEHandler handles Server-Sent Events endpoints.
type SSEHandler struct {
	broker sse.Broker
	logger infralogger.Logger
}

// NewSSEHandler creates a new SSE handler.
func NewSSEHandler(broker sse.Broker, logger infralogger.Logger) *SSEHandler {
	return &SSEHandler{
		broker: broker,
		logger: logger,
	}
}

// HandleCrawlerEvents handles GET /api/crawler/events endpoint.
// Streams job:status, job:progress, and job:completed events.
func (h *SSEHandler) HandleCrawlerEvents(c *gin.Context) {
	sse.Handler(h.broker, h.logger, sse.WithJobFilter())(c)
}

// HandleHealthEvents handles GET /api/health/events endpoint.
// Streams health:status events.
func (h *SSEHandler) HandleHealthEvents(c *gin.Context) {
	sse.Handler(h.broker, h.logger, sse.WithHealthFilter())(c)
}

// HandleMetricsEvents handles GET /api/metrics/events endpoint.
// Streams metrics:update and pipeline:stage events.
func (h *SSEHandler) HandleMetricsEvents(c *gin.Context) {
	sse.Handler(h.broker, h.logger, sse.WithMetricsFilter())(c)
}
