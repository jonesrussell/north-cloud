package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// listChannels returns all custom channels (Layer 2)
// GET /api/v1/channels?enabled_only=true
func (r *Router) listChannels(c *gin.Context) {
	ctx := c.Request.Context()

	const queryTrue = "true"
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

// createChannel creates a new custom channel with rules
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

// previewChannel returns a preview of articles that would match this channel's rules
// GET /api/v1/channels/:id/preview
// Note: This is a placeholder - full implementation would query Elasticsearch
func (r *Router) previewChannel(c *gin.Context) {
	ctx := c.Request.Context()

	channelID, ok := parseUUID(c, "id", "channel")
	if !ok {
		return
	}

	channel, err := r.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		handleRepositoryError(c, err, "channel", "get")
		return
	}

	// Build response with channel details and rules summary
	// Full implementation would query Elasticsearch for matching articles
	response := gin.H{
		"channel": channel,
		"rules_summary": gin.H{
			"include_topics": channel.Rules.IncludeTopics,
			"exclude_topics": channel.Rules.ExcludeTopics,
			"min_quality":    channel.Rules.MinQualityScore,
			"content_types":  channel.Rules.ContentTypes,
			"rules_is_empty": channel.Rules.IsEmpty(),
			"rules_version":  channel.RulesVersion,
		},
		"matching_count":  0,       // Would be populated by ES query
		"sample_articles": []any{}, // Would be populated by ES query
		"note":            "Preview endpoint - full ES query not implemented yet",
	}

	c.JSON(http.StatusOK, response)
}
