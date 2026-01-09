package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Handlers provides HTTP handlers for the API
type Handlers struct {
	statsService *StatsService
	logger       infralogger.Logger
	version      string
}

// NewHandlers creates a new handlers instance
func NewHandlers(statsService *StatsService, log infralogger.Logger, version string) *Handlers {
	return &Handlers{
		statsService: statsService,
		logger:       log,
		version:      version,
	}
}

// GetStats handles GET /api/v1/stats
func (h *Handlers) GetStats(c *gin.Context) {
	stats, err := h.statsService.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get stats",
			infralogger.Error(err),
			infralogger.String("path", c.Request.URL.Path),
			infralogger.String("method", c.Request.Method),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve statistics",
		})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetRecentArticles handles GET /api/v1/articles/recent
func (h *Handlers) GetRecentArticles(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	articles, err := h.statsService.GetRecentArticles(c.Request.Context(), limit)
	if err != nil {
		h.logger.Error("Failed to get recent articles",
			infralogger.Error(err),
			infralogger.String("path", c.Request.URL.Path),
			infralogger.String("method", c.Request.Method),
			infralogger.Int("limit", limit),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve recent articles",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"articles": articles,
		"count":    len(articles),
	})
}

// Health handles GET /health
func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "publisher",
		"version": h.version,
	})
}
