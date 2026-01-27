// Package domain provides domain models used across the application.
package domain

import (
	"time"
)

// JobExecution represents a single execution of a job.
// This tracks the history of each job run with detailed metrics.
type JobExecution struct {
	// Identity
	ID              string `db:"id"               json:"id"`
	JobID           string `db:"job_id"           json:"job_id"`
	ExecutionNumber int    `db:"execution_number" json:"execution_number"` // Nth execution of this job

	// Status
	Status string `db:"status" json:"status"` // running, completed, failed, cancelled

	// Timing
	StartedAt   time.Time  `db:"started_at"   json:"started_at"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at,omitempty"`
	DurationMs  *int64     `db:"duration_ms"  json:"duration_ms,omitempty"` // Auto-calculated on completion

	// Results
	ItemsCrawled int     `db:"items_crawled" json:"items_crawled"`
	ItemsIndexed int     `db:"items_indexed" json:"items_indexed"`
	ErrorMessage *string `db:"error_message" json:"error_message,omitempty"`
	StackTrace   *string `db:"stack_trace"   json:"stack_trace,omitempty"`

	// Resource tracking
	CPUTimeMs    *int64 `db:"cpu_time_ms"    json:"cpu_time_ms,omitempty"`
	MemoryPeakMB *int   `db:"memory_peak_mb" json:"memory_peak_mb,omitempty"`

	// Retry tracking
	RetryAttempt int `db:"retry_attempt" json:"retry_attempt"` // 0 = first try, 1+ = retry

	// Log archival
	LogObjectKey *string `db:"log_object_key" json:"log_object_key,omitempty"` // MinIO object key for archived logs
	LogSizeBytes *int64  `db:"log_size_bytes" json:"log_size_bytes,omitempty"` // Size of archived logs (compressed)
	LogLineCount *int    `db:"log_line_count" json:"log_line_count,omitempty"` // Number of log lines

	// Metadata
	Metadata JSONBMap `db:"metadata" json:"metadata,omitempty"`
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
