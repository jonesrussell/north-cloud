package scheduler

import (
	"sync"
	"time"
)

// SchedulerMetrics holds real-time metrics for the scheduler.
type SchedulerMetrics struct {
	mu sync.RWMutex

	// Job counts by state
	JobsScheduled int64
	JobsRunning   int64
	JobsCompleted int64
	JobsFailed    int64
	JobsCancelled int64

	// Execution metrics
	TotalExecutions   int64
	AverageDurationMs float64
	SuccessRate       float64

	// Operational metrics
	LastCheckAt       time.Time
	LastMetricsUpdate time.Time
	StaleLocksCleared int64
}

// IncrementScheduled atomically increments the scheduled jobs counter.
func (m *SchedulerMetrics) IncrementScheduled() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.JobsScheduled++
}

// IncrementRunning atomically increments the running jobs counter.
func (m *SchedulerMetrics) IncrementRunning() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.JobsRunning++
}

// DecrementRunning atomically decrements the running jobs counter.
func (m *SchedulerMetrics) DecrementRunning() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.JobsRunning--
}

// IncrementCompleted atomically increments the completed jobs counter.
func (m *SchedulerMetrics) IncrementCompleted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.JobsCompleted++
}

// IncrementFailed atomically increments the failed jobs counter.
func (m *SchedulerMetrics) IncrementFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.JobsFailed++
}

// IncrementCancelled atomically increments the cancelled jobs counter.
func (m *SchedulerMetrics) IncrementCancelled() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.JobsCancelled++
}

// IncrementTotalExecutions atomically increments the total executions counter.
func (m *SchedulerMetrics) IncrementTotalExecutions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalExecutions++
}

// UpdateAggregateMetrics updates aggregate metrics (avg duration, success rate).
func (m *SchedulerMetrics) UpdateAggregateMetrics(avgDuration, successRate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AverageDurationMs = avgDuration
	m.SuccessRate = successRate
	m.LastMetricsUpdate = time.Now()
}

// UpdateLastCheck updates the last check timestamp.
func (m *SchedulerMetrics) UpdateLastCheck() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LastCheckAt = time.Now()
}

// AddStaleLocksCleared adds to the stale locks cleared counter.
func (m *SchedulerMetrics) AddStaleLocksCleared(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StaleLocksCleared += int64(count)
}

// Snapshot returns a copy of the current metrics (thread-safe).
func (m *SchedulerMetrics) Snapshot() SchedulerMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return SchedulerMetrics{
		JobsScheduled:     m.JobsScheduled,
		JobsRunning:       m.JobsRunning,
		JobsCompleted:     m.JobsCompleted,
		JobsFailed:        m.JobsFailed,
		JobsCancelled:     m.JobsCancelled,
		TotalExecutions:   m.TotalExecutions,
		AverageDurationMs: m.AverageDurationMs,
		SuccessRate:       m.SuccessRate,
		LastCheckAt:       m.LastCheckAt,
		LastMetricsUpdate: m.LastMetricsUpdate,
		StaleLocksCleared: m.StaleLocksCleared,
	}
}
