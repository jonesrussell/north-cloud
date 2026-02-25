package metrics

import (
	"context"
)

// MetricsTracker defines the interface for tracking metrics.
// This interface allows for easy testing and potential future implementations.
type MetricsTracker interface {
	// IncrementPosted increments the posted content counter for a city
	IncrementPosted(ctx context.Context, city string) error
	// IncrementSkipped increments the skipped content counter for a city
	IncrementSkipped(ctx context.Context, city string) error
	// IncrementErrors increments the error counter for a city
	IncrementErrors(ctx context.Context, city string) error
	// AddRecentItem adds an item to the recent items list
	// Accepts any to allow flexibility (can be RecentItem or map[string]any)
	AddRecentItem(ctx context.Context, item any) error
	// GetStats returns aggregated statistics
	GetStats(ctx context.Context) (*Stats, error)
	// GetRecentItems returns recently posted items
	GetRecentItems(ctx context.Context, limit int) ([]RecentItem, error)
	// UpdateLastSync updates the last sync timestamp
	UpdateLastSync(ctx context.Context) error
}
