// Package domain provides domain models used across the application.
package domain

import (
	"time"
)

// JobExecution represents a single execution of a job.
// This tracks the history of each job run with detailed metrics.
type JobExecution struct {
	// Identity
	ID              string `json:"id" db:"id"`
	JobID           string `json:"job_id" db:"job_id"`
	ExecutionNumber int    `json:"execution_number" db:"execution_number"` // Nth execution of this job

	// Status
	Status string `json:"status" db:"status"` // running, completed, failed, cancelled

	// Timing
	StartedAt   time.Time  `json:"started_at" db:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	DurationMs  *int64     `json:"duration_ms,omitempty" db:"duration_ms"` // Auto-calculated on completion

	// Results
	ItemsCrawled int     `json:"items_crawled" db:"items_crawled"`
	ItemsIndexed int     `json:"items_indexed" db:"items_indexed"`
	ErrorMessage *string `json:"error_message,omitempty" db:"error_message"`
	StackTrace   *string `json:"stack_trace,omitempty" db:"stack_trace"`

	// Resource tracking
	CPUTimeMs    *int64 `json:"cpu_time_ms,omitempty" db:"cpu_time_ms"`
	MemoryPeakMB *int   `json:"memory_peak_mb,omitempty" db:"memory_peak_mb"`

	// Retry tracking
	RetryAttempt int `json:"retry_attempt" db:"retry_attempt"` // 0 = first try, 1+ = retry

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}

// JobStats represents aggregate statistics for a job.
type JobStats struct {
	TotalExecutions   int        `json:"total_executions"`
	SuccessfulRuns    int        `json:"successful_runs"`
	FailedRuns        int        `json:"failed_runs"`
	AverageDurationMs float64    `json:"average_duration_ms"`
	LastExecutionAt   *time.Time `json:"last_execution_at"`
	NextScheduledAt   *time.Time `json:"next_scheduled_at"`
	SuccessRate       float64    `json:"success_rate"` // 0.0 to 1.0
}

// AggregateStats represents system-wide scheduler statistics.
type AggregateStats struct {
	TotalExecutions int64   `json:"total_executions"`
	AvgDurationMs   float64 `json:"average_duration_ms"`
	SuccessRate     float64 `json:"success_rate"` // 0.0 to 1.0
	FailureRate     float64 `json:"failure_rate"` // 0.0 to 1.0
	ActiveJobs      int64   `json:"active_jobs"`
	ScheduledJobs   int64   `json:"scheduled_jobs"`
	CompletedToday  int64   `json:"completed_today"`
	FailedToday     int64   `json:"failed_today"`
}
