package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	version string
}

// NewHealthHandler creates a HealthHandler that reports the given version.
func NewHealthHandler(version string) *HealthHandler {
	return &HealthHandler{version: version}
}

// HealthCheck returns service health status.
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"version":   h.version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
