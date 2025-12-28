//nolint:dupl // Similar structure to sources_handler.go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// listChannels returns all channels
// GET /api/v1/channels?enabled_only=true
func (r *Router) listChannels(c *gin.Context) {
	ctx := c.Request.Context()

	const queryTrue = "true"
	// Parse query parameters
	enabledOnly := c.Query("enabled_only") == queryTrue

	channels, err := r.repo.ListChannels(ctx, enabledOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list channels",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"channels": channels,
		"count":    len(channels),
	})
}

// createChannel creates a new channel
// POST /api/v1/channels
func (r *Router) createChannel(c *gin.Context) {
	ctx := c.Request.Context()

	var req models.ChannelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	channel, err := r.repo.CreateChannel(ctx, &req)
	if err != nil {
		handleRepositoryError(c, err, "channel", "create")
		return
	}

	c.JSON(http.StatusCreated, channel)
}

// getChannel retrieves a channel by ID
// GET /api/v1/channels/:id
func (r *Router) getChannel(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := parseUUID(c, "id", "channel")
	if !ok {
		return
	}

	channel, err := r.repo.GetChannelByID(ctx, id)
	if err != nil {
		handleRepositoryError(c, err, "channel", "get")
		return
	}

	c.JSON(http.StatusOK, channel)
}

// updateChannel updates a channel
// PUT /api/v1/channels/:id
func (r *Router) updateChannel(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := parseUUID(c, "id", "channel")
	if !ok {
		return
	}

	var req models.ChannelUpdateRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": bindErr.Error(),
		})
		return
	}

	// Validate request
	if validateErr := req.Validate(); validateErr != nil {
		handleValidationError(c, validateErr)
		return
	}

	channel, err := r.repo.UpdateChannel(ctx, id, &req)
	if err != nil {
		handleRepositoryError(c, err, "channel", "update")
		return
	}

	c.JSON(http.StatusOK, channel)
}

// deleteChannel deletes a channel
// DELETE /api/v1/channels/:id
func (r *Router) deleteChannel(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := parseUUID(c, "id", "channel")
	if !ok {
		return
	}

	err := r.repo.DeleteChannel(ctx, id)
	if err != nil {
		handleRepositoryError(c, err, "channel", "delete")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Channel deleted successfully",
	})
}
