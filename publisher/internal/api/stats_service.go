package api

import (
	"context"
	"errors"

	"github.com/jonesrussell/north-cloud/publisher/internal/metrics"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const defaultLimit = 50

// StatsService provides business logic for statistics operations
type StatsService struct {
	tracker MetricsTracker
	logger  infralogger.Logger
}

// MetricsTracker interface for dependency injection
type MetricsTracker interface {
	GetStats(ctx context.Context) (*metrics.Stats, error)
	GetRecentItems(ctx context.Context, limit int) ([]metrics.RecentItem, error)
}

// NewStatsService creates a new stats service
func NewStatsService(tracker MetricsTracker, log infralogger.Logger) *StatsService {
	return &StatsService{
		tracker: tracker,
		logger:  log,
	}
}

// GetStats returns aggregated statistics
func (s *StatsService) GetStats(ctx context.Context) (*metrics.Stats, error) {
	if s.tracker == nil {
		s.logger.Error("Metrics tracker is nil")
		return nil, errors.New("metrics tracker not initialized")
	}
	return s.tracker.GetStats(ctx)
}

// GetRecentItems returns recently posted items with limit validation
func (s *StatsService) GetRecentItems(ctx context.Context, limit int) ([]metrics.RecentItem, error) {
	if s.tracker == nil {
		s.logger.Error("Metrics tracker is nil")
		return nil, errors.New("metrics tracker not initialized")
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > metrics.MaxRecentItems {
		limit = metrics.MaxRecentItems
	}
	return s.tracker.GetRecentItems(ctx, limit)
}
