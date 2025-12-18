package api

import (
	"context"

	"github.com/gopost/integration/internal/logger"
	"github.com/gopost/integration/internal/metrics"
)

// StatsService provides business logic for statistics operations
type StatsService struct {
	tracker MetricsTracker
	logger  logger.Logger
}

// MetricsTracker interface for dependency injection
type MetricsTracker interface {
	GetStats(ctx context.Context) (*metrics.Stats, error)
	GetRecentArticles(ctx context.Context, limit int) ([]metrics.RecentArticle, error)
}

// NewStatsService creates a new stats service
func NewStatsService(tracker MetricsTracker, log logger.Logger) *StatsService {
	return &StatsService{
		tracker: tracker,
		logger:  log,
	}
}

// GetStats returns aggregated statistics
func (s *StatsService) GetStats(ctx context.Context) (*metrics.Stats, error) {
	return s.tracker.GetStats(ctx)
}

// GetRecentArticles returns recent posted articles with limit validation
func (s *StatsService) GetRecentArticles(ctx context.Context, limit int) ([]metrics.RecentArticle, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return s.tracker.GetRecentArticles(ctx, limit)
}
