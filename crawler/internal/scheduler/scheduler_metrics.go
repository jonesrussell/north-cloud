package scheduler

import (
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// collectMetrics periodically updates scheduler metrics.
func (s *IntervalScheduler) collectMetrics() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.metricsInterval)
	defer ticker.Stop()

	s.logger.Info("Metrics collector started", infralogger.Duration("interval", s.metricsInterval))
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Metrics collector stopping")
			return
		case <-ticker.C:
			s.updateMetrics()
		}
	}
}

// updateMetrics calculates and updates scheduler metrics.
func (s *IntervalScheduler) updateMetrics() {
	// Get aggregate stats from database
	stats, err := s.executionRepo.GetAggregateStats(s.ctx)
	if err != nil {
		s.logger.Error("Failed to get aggregate stats", infralogger.Error(err))
		return
	}

	s.metrics.UpdateAggregateMetrics(stats.AvgDurationMs, stats.SuccessRate)

	s.logger.Debug("Metrics updated",
		infralogger.Float64("avg_duration_ms", stats.AvgDurationMs),
		infralogger.Float64("success_rate", stats.SuccessRate),
		infralogger.Int64("active_jobs", stats.ActiveJobs),
		infralogger.Int64("scheduled_jobs", stats.ScheduledJobs),
	)
}

// GetMetrics returns a snapshot of current scheduler metrics.
func (s *IntervalScheduler) GetMetrics() SchedulerMetrics {
	return s.metrics.Snapshot()
}
