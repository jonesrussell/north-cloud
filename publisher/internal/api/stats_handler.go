package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// getStatsOverview returns overall publishing statistics
// GET /api/v1/stats/overview?period=today|week|month
func (r *Router) getStatsOverview(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse period parameter
	period := c.Query("period")
	if period == "" {
		period = "today"
	}

	var startDate *time.Time
	now := time.Now()

	switch period {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		startDate = &start
	case "week":
		start := now.AddDate(0, 0, -7)
		startDate = &start
	case "month":
		start := now.AddDate(0, -1, 0)
		startDate = &start
	case "all":
		// No start date filter
		startDate = nil
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid period. Valid values: today, week, month, all",
		})
		return
	}

	// Get stats from repository
	stats, err := r.repo.GetPublishStats(ctx, startDate, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get stats",
		})
		return
	}

	// Calculate total
	total := 0
	for _, count := range stats {
		total += count
	}

	c.JSON(http.StatusOK, gin.H{
		"period":            period,
		"total_articles":    total,
		"by_channel":        stats,
		"channel_count":     len(stats),
		"generated_at":      now,
	})
}

// getChannelStats returns statistics per channel
// GET /api/v1/stats/channels?since=2025-12-01
func (r *Router) getChannelStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse since parameter
	sinceParam := c.Query("since")
	var since *time.Time

	if sinceParam != "" {
		parsedTime, err := time.Parse("2006-01-02", sinceParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid date format. Use YYYY-MM-DD",
			})
			return
		}
		since = &parsedTime
	} else {
		// Default to last 30 days
		thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
		since = &thirtyDaysAgo
	}

	// Get all enabled channels
	channels, err := r.repo.ListChannels(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list channels",
		})
		return
	}

	// Get publish count for each channel
	channelStats := make([]gin.H, 0, len(channels))
	for _, channel := range channels {
		count, err := r.repo.GetPublishCountByChannel(ctx, channel.Name, since)
		if err != nil {
			// Log error but continue
			count = 0
		}

		channelStats = append(channelStats, gin.H{
			"channel_id":          channel.ID,
			"channel_name":        channel.Name,
			"channel_description": channel.Description,
			"article_count":       count,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"channels": channelStats,
		"since":    since,
		"count":    len(channelStats),
	})
}

// getRouteStats returns statistics per route
// GET /api/v1/stats/routes
func (r *Router) getRouteStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Get all enabled routes with details
	routes, err := r.repo.ListRoutesWithDetails(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list routes",
		})
		return
	}

	// For now, return route info (can be enhanced with per-route publish counts)
	routeStats := make([]gin.H, 0, len(routes))
	for _, route := range routes {
		routeStats = append(routeStats, gin.H{
			"route_id":            route.ID,
			"source_name":         route.SourceName,
			"source_index":        route.SourceIndexPattern,
			"channel_name":        route.ChannelName,
			"min_quality_score":   route.MinQualityScore,
			"topics":              route.Topics,
			"enabled":             route.Enabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"routes": routeStats,
		"count":  len(routeStats),
	})
}

// listPublishHistory returns publish history with optional filters
// GET /api/v1/publish-history?channel_name=articles:crime&limit=100&offset=0
func (r *Router) listPublishHistory(c *gin.Context) {
	ctx := c.Request.Context()

	// Bind query parameters to filter struct
	var filter models.PublishHistoryFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid query parameters",
			"details": err.Error(),
		})
		return
	}

	// Set defaults
	if filter.Limit == 0 {
		filter.Limit = 100
	}

	history, err := r.repo.ListPublishHistory(ctx, &filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get publish history",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"count":   len(history),
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}

// getPublishHistoryByArticle returns all publish history for a specific article
// GET /api/v1/publish-history/:article_id
func (r *Router) getPublishHistoryByArticle(c *gin.Context) {
	ctx := c.Request.Context()

	articleID := c.Param("article_id")
	if articleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Article ID is required",
		})
		return
	}

	history, err := r.repo.GetPublishHistoryByArticleID(ctx, articleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get publish history",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"article_id": articleID,
		"history":    history,
		"count":      len(history),
	})
}
