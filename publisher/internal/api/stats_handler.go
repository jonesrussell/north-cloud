package api

import (
	"net/http"
	"strconv"
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
		"period":         period,
		"total_articles": total,
		"by_channel":     stats,
		"channel_count":  len(stats),
		"generated_at":   now,
	})
}

// getChannelStats returns statistics per custom channel (Layer 2)
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

	// Get all enabled channels (Layer 2 custom channels)
	channels, err := r.repo.ListChannels(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list channels",
		})
		return
	}

	// Get publish count for each channel using redis_channel name
	channelStats := make([]gin.H, 0, len(channels))
	for i := range channels {
		count, countErr := r.repo.GetPublishCountByChannel(ctx, channels[i].RedisChannel, since)
		if countErr != nil {
			// Log error but continue
			count = 0
		}

		channelStats = append(channelStats, gin.H{
			"channel_id":    channels[i].ID,
			"name":          channels[i].Name,
			"slug":          channels[i].Slug,
			"redis_channel": channels[i].RedisChannel,
			"description":   channels[i].Description,
			"rules":         channels[i].Rules,
			"article_count": count,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"channels": channelStats,
		"since":    since,
		"count":    len(channelStats),
	})
}

// getActiveChannels returns custom channels (Layer 2) with their publish activity
// GET /api/v1/stats/channels/active
func (r *Router) getActiveChannels(c *gin.Context) {
	ctx := c.Request.Context()

	// Get all channels (Layer 2 custom channels)
	channels, err := r.repo.ListChannels(ctx, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list channels",
		})
		return
	}

	// Get channel stats (publish counts and last published dates)
	channelStats, statsErr := r.repo.GetChannelStats(ctx)
	if statsErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get channel stats",
		})
		return
	}

	// Build response
	activeChannels := make([]gin.H, 0, len(channels))
	for i := range channels {
		// Look up stats by redis_channel name
		stats, hasStats := channelStats[channels[i].RedisChannel]

		channelInfo := gin.H{
			"id":            channels[i].ID,
			"name":          channels[i].Name,
			"slug":          channels[i].Slug,
			"redis_channel": channels[i].RedisChannel,
			"description":   channels[i].Description,
			"rules":         channels[i].Rules,
			"enabled":       channels[i].Enabled,
			"has_published": hasStats,
		}

		if hasStats {
			channelInfo["total_published"] = stats.TotalPublished
			if stats.LastPublished != nil {
				channelInfo["last_published_at"] = stats.LastPublished.Format(time.RFC3339)
			}
		} else {
			channelInfo["total_published"] = 0
		}

		activeChannels = append(activeChannels, channelInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"channels": activeChannels,
		"count":    len(activeChannels),
		"note":     "Layer 2 custom channels. Layer 1 automatic topic channels (articles:{topic}) are not tracked here.",
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

// getRecentArticles returns recent published articles
// GET /api/v1/articles/recent?limit=50
func (r *Router) getRecentArticles(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse limit parameter
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	const maxLimit = 100
	if limit > maxLimit {
		limit = maxLimit
	}

	// Use publish history to get recent articles
	filter := &models.PublishHistoryFilter{
		Limit:  limit,
		Offset: 0,
	}

	history, err := r.repo.ListPublishHistory(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get recent articles",
		})
		return
	}

	// Transform publish_history to match frontend expectations
	articles := make([]gin.H, 0, len(history))
	for i := range history {
		articles = append(articles, gin.H{
			"id":        history[i].ID,
			"title":     history[i].ArticleTitle,
			"url":       history[i].ArticleURL,
			"city":      "", // Publish history doesn't have city - could extract from channel if needed
			"posted_at": history[i].PublishedAt,
			// Include additional fields for compatibility
			"article_id":    history[i].ArticleID,
			"article_title": history[i].ArticleTitle,
			"article_url":   history[i].ArticleURL,
			"channel_name":  history[i].ChannelName,
			"published_at":  history[i].PublishedAt,
			"quality_score": history[i].QualityScore,
			"topics":        history[i].Topics,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"articles": articles,
		"count":    len(articles),
	})
}

// clearAllPublishHistory deletes all publish history entries
// DELETE /api/v1/publish-history
func (r *Router) clearAllPublishHistory(c *gin.Context) {
	ctx := c.Request.Context()

	count, err := r.repo.DeleteAllPublishHistory(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to clear publish history",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Publish history cleared successfully",
		"deleted": count,
	})
}
