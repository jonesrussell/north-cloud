package scheduler

import (
	"time"
)

// SchedulerOption is a functional option for configuring the IntervalScheduler.
type SchedulerOption func(*IntervalScheduler)

// WithCheckInterval sets how often the scheduler polls for jobs ready to run.
// Default: 10 seconds
func WithCheckInterval(interval time.Duration) SchedulerOption {
	return func(s *IntervalScheduler) {
		s.checkInterval = interval
	}
}

// WithLockDuration sets how long job locks are valid before being considered stale.
// Default: 5 minutes
func WithLockDuration(duration time.Duration) SchedulerOption {
	return func(s *IntervalScheduler) {
		s.lockDuration = duration
	}
}

// WithMetricsInterval sets how often metrics are collected and updated.
// Default: 30 seconds
func WithMetricsInterval(interval time.Duration) SchedulerOption {
	return func(s *IntervalScheduler) {
		s.metricsInterval = interval
	}
}

// WithStaleLockCheckInterval sets how often to check for and clear stale locks.
// Default: 1 minute
func WithStaleLockCheckInterval(interval time.Duration) SchedulerOption {
	return func(s *IntervalScheduler) {
		s.staleLockCheckInterval = interval
	}
}
