package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gopost/integration/internal/logger"
	"github.com/redis/go-redis/v9"
)

// Tracker implements MetricsTracker interface using Redis
type Tracker struct {
	client redis.UniversalClient
	keys   *RedisKeys
	logger logger.Logger
	cities []string // For GetStats aggregation
}

// NewTracker creates a new metrics tracker
func NewTracker(client redis.UniversalClient, cities []string, log logger.Logger) *Tracker {
	return &Tracker{
		client: client,
		keys:   NewRedisKeys(KeyPrefixMetrics),
		logger: log,
		cities: cities,
	}
}

// IncrementPosted increments the posted articles counter for a city
func (t *Tracker) IncrementPosted(ctx context.Context, city string) error {
	key := t.keys.Posted(city)
	ttl := MetricsTTLDays * 24 * time.Hour

	// Use pipeline for atomic operation with TTL
	pipe := t.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		t.logger.Warn("Failed to increment posted counter",
			logger.String("city", city),
			logger.String("redis_key", key),
			logger.Error(err),
		)
		return fmt.Errorf("increment posted counter: %w", err)
	}

	return nil
}

// IncrementSkipped increments the skipped articles counter for a city
func (t *Tracker) IncrementSkipped(ctx context.Context, city string) error {
	key := t.keys.Skipped(city)
	ttl := MetricsTTLDays * 24 * time.Hour

	// Use pipeline for atomic operation with TTL
	pipe := t.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		t.logger.Warn("Failed to increment skipped counter",
			logger.String("city", city),
			logger.String("redis_key", key),
			logger.Error(err),
		)
		return fmt.Errorf("increment skipped counter: %w", err)
	}

	return nil
}

// IncrementErrors increments the error counter for a city
func (t *Tracker) IncrementErrors(ctx context.Context, city string) error {
	key := t.keys.Errors(city)
	ttl := MetricsTTLDays * 24 * time.Hour

	// Use pipeline for atomic operation with TTL
	pipe := t.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		t.logger.Warn("Failed to increment error counter",
			logger.String("city", city),
			logger.String("redis_key", key),
			logger.Error(err),
		)
		return fmt.Errorf("increment error counter: %w", err)
	}

	return nil
}

// AddRecentArticle adds an article to the recent articles list
func (t *Tracker) AddRecentArticle(ctx context.Context, article interface{}) error {
	// Convert interface{} to RecentArticle
	var recentArticle RecentArticle

	switch v := article.(type) {
	case RecentArticle:
		recentArticle = v
	case map[string]interface{}:
		// Handle map conversion
		if id, ok := v["id"].(string); ok {
			recentArticle.ID = id
		}
		if title, ok := v["title"].(string); ok {
			recentArticle.Title = title
		}
		if url, ok := v["url"].(string); ok {
			recentArticle.URL = url
		}
		if city, ok := v["city"].(string); ok {
			recentArticle.City = city
		}
		if postedAtStr, ok := v["posted_at"].(string); ok {
			if postedAt, err := time.Parse(time.RFC3339, postedAtStr); err == nil {
				recentArticle.PostedAt = postedAt
			} else {
				recentArticle.PostedAt = time.Now()
			}
		} else {
			recentArticle.PostedAt = time.Now()
		}
	default:
		// Try JSON marshal/unmarshal as fallback
		data, err := json.Marshal(article)
		if err != nil {
			return fmt.Errorf("marshal article: %w", err)
		}
		if err := json.Unmarshal(data, &recentArticle); err != nil {
			return fmt.Errorf("unmarshal article: %w", err)
		}
	}

	// Serialize article to JSON
	data, err := json.Marshal(recentArticle)
	if err != nil {
		return fmt.Errorf("marshal article: %w", err)
	}

	key := KeyRecentArticles
	ttl := RecentArticlesTTLDays * 24 * time.Hour

	// Use pipeline for atomic operations: LPUSH, LTRIM, EXPIRE
	pipe := t.client.Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, MaxRecentArticles-1) // Keep last 100
	pipe.Expire(ctx, key, ttl)

	_, err = pipe.Exec(ctx)
	if err != nil {
		t.logger.Warn("Failed to add recent article",
			logger.String("article_id", recentArticle.ID),
			logger.String("city", recentArticle.City),
			logger.Error(err),
		)
		return fmt.Errorf("add recent article: %w", err)
	}

	return nil
}

// GetStats returns aggregated statistics using Redis pipeline for atomic reads
func (t *Tracker) GetStats(ctx context.Context) (*Stats, error) {
	pipe := t.client.Pipeline()

	// Queue all reads in pipeline for atomic operation
	postedCmds := make(map[string]*redis.StringCmd)
	skippedCmds := make(map[string]*redis.StringCmd)
	errorCmds := make(map[string]*redis.StringCmd)

	for _, city := range t.cities {
		postedCmds[city] = pipe.Get(ctx, t.keys.Posted(city))
		skippedCmds[city] = pipe.Get(ctx, t.keys.Skipped(city))
		errorCmds[city] = pipe.Get(ctx, t.keys.Errors(city))
	}

	// Get last sync timestamp
	lastSyncCmd := pipe.Get(ctx, KeyLastSync)

	// Execute pipeline atomically
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("execute pipeline: %w", err)
	}

	// Build stats from results
	stats := &Stats{
		Cities: make([]CityStats, 0, len(t.cities)),
	}

	var totalPosted, totalSkipped, totalErrors int64

	for _, city := range t.cities {
		cityStats := CityStats{Name: city}

		// Get posted count (default to 0 if key doesn't exist)
		if postedVal, err := postedCmds[city].Int64(); err == nil {
			cityStats.Posted = postedVal
			totalPosted += postedVal
		}

		// Get skipped count (default to 0 if key doesn't exist)
		if skippedVal, err := skippedCmds[city].Int64(); err == nil {
			cityStats.Skipped = skippedVal
			totalSkipped += skippedVal
		}

		// Get error count (default to 0 if key doesn't exist)
		if errorVal, err := errorCmds[city].Int64(); err == nil {
			cityStats.Errors = errorVal
			totalErrors += errorVal
		}

		stats.Cities = append(stats.Cities, cityStats)
	}

	stats.TotalPosted = totalPosted
	stats.TotalSkipped = totalSkipped
	stats.TotalErrors = totalErrors

	// Get last sync timestamp
	if lastSyncStr, err := lastSyncCmd.Result(); err == nil && lastSyncStr != "" {
		if lastSync, err := time.Parse(time.RFC3339, lastSyncStr); err == nil {
			stats.LastSync = lastSync
		}
	}

	// If no last sync found, use zero time
	if stats.LastSync.IsZero() {
		stats.LastSync = time.Time{}
	}

	return stats, nil
}

// GetRecentArticles returns recent posted articles
func (t *Tracker) GetRecentArticles(ctx context.Context, limit int) ([]RecentArticle, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > MaxRecentArticles {
		limit = MaxRecentArticles
	}

	key := KeyRecentArticles

	// Get articles from list (0 to limit-1)
	results, err := t.client.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		if err == redis.Nil {
			return []RecentArticle{}, nil
		}
		return nil, fmt.Errorf("get recent articles: %w", err)
	}

	articles := make([]RecentArticle, 0, len(results))
	for _, result := range results {
		var article RecentArticle
		if err := json.Unmarshal([]byte(result), &article); err != nil {
			t.logger.Warn("Failed to unmarshal recent article",
				logger.Error(err),
			)
			continue
		}
		articles = append(articles, article)
	}

	return articles, nil
}

// UpdateLastSync updates the last sync timestamp
func (t *Tracker) UpdateLastSync(ctx context.Context) error {
	key := KeyLastSync
	now := time.Now().Format(time.RFC3339)

	err := t.client.Set(ctx, key, now, 0).Err() // No expiration for last sync
	if err != nil {
		t.logger.Warn("Failed to update last sync",
			logger.Error(err),
		)
		return fmt.Errorf("update last sync: %w", err)
	}

	return nil
}
