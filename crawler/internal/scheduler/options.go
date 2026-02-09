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

// WithExecutionTimeout sets the maximum time a job execution can run before
// its context is cancelled. This prevents jobs from hanging indefinitely.
// Default: 1 hour
func WithExecutionTimeout(timeout time.Duration) SchedulerOption {
	return func(s *IntervalScheduler) {
		s.executionTimeout = timeout
	}
}

// WithStuckJobCheckInterval sets how often to check for and recover stuck jobs.
// Default: 2 minutes
func WithStuckJobCheckInterval(interval time.Duration) SchedulerOption {
	return func(s *IntervalScheduler) {
		s.stuckJobCheckInterval = interval
	}
}

// WithLoadBalancing enables or disables load-balanced placement.
// Default is true (enabled).
func WithLoadBalancing(enabled bool) SchedulerOption {
	return func(s *IntervalScheduler) {
		if !enabled {
			s.bucketMap = nil
		}
	}
}
