package metrics

import (
	"context"
)

// MetricsTracker defines the interface for tracking metrics.
// This interface allows for easy testing and potential future implementations.
type MetricsTracker interface {
	// IncrementPosted increments the posted articles counter for a city
	IncrementPosted(ctx context.Context, city string) error
	// IncrementSkipped increments the skipped articles counter for a city
	IncrementSkipped(ctx context.Context, city string) error
	// IncrementErrors increments the error counter for a city
	IncrementErrors(ctx context.Context, city string) error
	// AddRecentArticle adds an article to the recent articles list
	// Accepts any to allow flexibility (can be RecentArticle or map[string]any)
	AddRecentArticle(ctx context.Context, article any) error
	// GetStats returns aggregated statistics
	GetStats(ctx context.Context) (*Stats, error)
	// GetRecentArticles returns recent posted articles
	GetRecentArticles(ctx context.Context, limit int) ([]RecentArticle, error)
	// UpdateLastSync updates the last sync timestamp
	UpdateLastSync(ctx context.Context) error
}
