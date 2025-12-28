package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// listChannels returns all channels
// GET /api/v1/channels?enabled_only=true
func (r *Router) listChannels(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	enabledOnly := c.Query("enabled_only") == "true"

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
		if errors.Is(err, models.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Channel with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create channel",
		})
		return
	}

	c.JSON(http.StatusCreated, channel)
}

// getChannel retrieves a channel by ID
// GET /api/v1/channels/:id
func (r *Router) getChannel(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid channel ID format",
		})
		return
	}

	channel, err := r.repo.GetChannelByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Channel not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get channel",
		})
		return
	}

	c.JSON(http.StatusOK, channel)
}

// updateChannel updates a channel
// PUT /api/v1/channels/:id
func (r *Router) updateChannel(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid channel ID format",
		})
		return
	}

	var req models.ChannelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		if errors.Is(err, models.ErrNoFieldsToUpdate) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "At least one field must be provided for update",
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	channel, err := r.repo.UpdateChannel(ctx, id, &req)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Channel not found",
			})
			return
		}
		if errors.Is(err, models.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Channel with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update channel",
		})
		return
	}

	c.JSON(http.StatusOK, channel)
}

// deleteChannel deletes a channel
// DELETE /api/v1/channels/:id
func (r *Router) deleteChannel(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid channel ID format",
		})
		return
	}

	err = r.repo.DeleteChannel(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Channel not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete channel",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Channel deleted successfully",
	})
}
