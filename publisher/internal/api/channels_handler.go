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

// testPublish simulates publishing to a channel and returns preview of articles that would be published
// GET /api/v1/channels/:id/test-publish
func (r *Router) testPublish(c *gin.Context) {
	ctx := c.Request.Context()

	channelID, ok := parseUUID(c, "id", "channel")
	if !ok {
		return
	}

	// Get channel details
	channel, err := r.repo.GetChannelByID(ctx, channelID)
	if err != nil {
		handleRepositoryError(c, err, "channel", "get")
		return
	}

	// Get all enabled routes for this channel
	routes, err := r.repo.GetRoutesByChannelID(ctx, channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get routes for channel",
		})
		return
	}

	if len(routes) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"channel_name":    channel.Name,
			"routes_count":    0,
			"estimated_count": 0,
			"message":         "No enabled routes found for this channel",
			"sample_articles": []gin.H{},
		})
		return
	}

	// Aggregate statistics across all routes
	totalEstimated := 0
	sampleArticles := []gin.H{}
	routeStats := []gin.H{}

	for _, route := range routes {
		// For each route, simulate article count (in real implementation, would query Elasticsearch)
		// Using similar logic to previewRoute
		routeCount := 50 + (len(route.Topics) * 25) // Simulated: more topics = more articles
		if route.MinQualityScore > 70 {
			routeCount = routeCount / 2 // Higher quality threshold = fewer articles
		}
		totalEstimated += routeCount

		// Add route statistics
		routeStats = append(routeStats, gin.H{
			"route_id":          route.ID,
			"source_name":       route.SourceName,
			"min_quality_score": route.MinQualityScore,
			"topics":            route.Topics,
			"estimated_count":   routeCount,
		})

		// Add sample articles for this route (limit to 3 per route to avoid overwhelming response)
		for i := 0; i < 3 && i < routeCount; i++ {
			sampleArticles = append(sampleArticles, gin.H{
				"title":          generateSampleTitle(route.Topics),
				"quality_score":  70 + (i * 8), // Varying quality scores
				"topics":         route.Topics,
				"published_date": "2026-01-02T14:30:00Z",
				"url":            "https://example.com/article-" + route.ID.String() + "-" + string(rune(i)),
				"source":         route.SourceName,
				"route_id":       route.ID,
			})
		}
	}

	// Limit total sample articles to 10
	if len(sampleArticles) > 10 {
		sampleArticles = sampleArticles[:10]
	}

	response := gin.H{
		"channel_name":    channel.Name,
		"channel_id":      channelID,
		"routes_count":    len(routes),
		"estimated_count": totalEstimated,
		"route_stats":     routeStats,
		"sample_articles": sampleArticles,
		"message":         "Test publish simulation completed",
	}

	c.JSON(http.StatusOK, response)
}

// generateSampleTitle generates a sample article title based on topics
func generateSampleTitle(topics []string) string {
	if len(topics) == 0 {
		return "Sample Article Title"
	}

	// Use first topic to generate title
	topic := topics[0]
	titles := map[string][]string{
		"crime": {
			"Crime Report: Downtown Incident",
			"Breaking: Major Arrest Made",
			"Local Police Update",
		},
		"local": {
			"Local Community News Update",
			"City Council Meeting Summary",
			"Neighborhood Events This Week",
		},
		"news": {
			"Breaking News: Important Update",
			"Today's Top Stories",
			"News Brief: Latest Developments",
		},
		"breaking": {
			"Breaking: Urgent Update",
			"Alert: Breaking News",
			"Urgent: Breaking Story",
		},
	}

	if topicTitles, ok := titles[topic]; ok {
		return topicTitles[0] // Use first title for consistency
	}

	return "Sample " + topic + " Article"
}
