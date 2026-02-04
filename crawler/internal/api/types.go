// Package api implements the HTTP API for the crawler service.
package api

import (
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
)

// SchedulerInterface defines the scheduler operations needed by job handlers.
type SchedulerInterface interface {
	CancelJob(jobID string) error
	GetMetrics() scheduler.SchedulerMetrics
	GetDistribution() *scheduler.Distribution
}

// CreateJobRequest represents a job creation request.
type CreateJobRequest struct {
	SourceID   string `binding:"required" json:"source_id"`
	SourceName string `json:"source_name"`
	URL        string `binding:"required" json:"url"`

	// Interval-based scheduling (new)
	IntervalMinutes *int   `json:"interval_minutes"` // NULL = run once immediately
	IntervalType    string `json:"interval_type"`    // 'minutes', 'hours', 'days'
	ScheduleEnabled bool   `json:"schedule_enabled"`

	// Retry configuration (new)
	MaxRetries          *int `json:"max_retries"`           // Default: 3
	RetryBackoffSeconds *int `json:"retry_backoff_seconds"` // Default: 60

	// Legacy cron field (deprecated, maintained for backward compatibility)
	ScheduleTime string `json:"schedule_time"`

	// Metadata (new)
	Metadata map[string]any `json:"metadata"`
}

// UpdateJobRequest represents a job update request.
type UpdateJobRequest struct {
	SourceID   string `json:"source_id"`
	SourceName string `json:"source_name"`
	URL        string `json:"url"`

	// Interval-based scheduling (new)
	IntervalMinutes *int   `json:"interval_minutes"`
	IntervalType    string `json:"interval_type"`
	ScheduleEnabled *bool  `json:"schedule_enabled"`

	// Retry configuration (new)
	MaxRetries          *int `json:"max_retries"`
	RetryBackoffSeconds *int `json:"retry_backoff_seconds"`

	// Legacy cron field (deprecated)
	ScheduleTime string `json:"schedule_time"`

	// Status
	Status string `json:"status"`

	// Metadata (new)
	Metadata map[string]any `json:"metadata"`
}

// JobStatsResponse represents aggregate statistics for a job.
type JobStatsResponse struct {
	TotalExecutions   int        `json:"total_executions"`
	SuccessfulRuns    int        `json:"successful_runs"`
	FailedRuns        int        `json:"failed_runs"`
	AverageDurationMs float64    `json:"average_duration_ms"`
	LastExecutionAt   *time.Time `json:"last_execution_at"`
	NextScheduledAt   *time.Time `json:"next_scheduled_at"`
	SuccessRate       float64    `json:"success_rate"` // 0.0 to 1.0
}

// SchedulerMetricsResponse represents scheduler-wide metrics.
type SchedulerMetricsResponse struct {
	Jobs struct {
		Scheduled int64 `json:"scheduled"`
		Running   int64 `json:"running"`
		Completed int64 `json:"completed"`
		Failed    int64 `json:"failed"`
		Cancelled int64 `json:"cancelled"`
	} `json:"jobs"`
	Executions struct {
		Total             int64   `json:"total"`
		AverageDurationMs float64 `json:"average_duration_ms"`
		SuccessRate       float64 `json:"success_rate"`
	} `json:"executions"`
	LastCheckAt       time.Time `json:"last_check_at"`
	LastMetricsUpdate time.Time `json:"last_metrics_update"`
	StaleLocksCleared int64     `json:"stale_locks_cleared"`
}

// ExecutionsListResponse represents a paginated list of executions.
type ExecutionsListResponse struct {
	Executions any `json:"executions"` // []*domain.JobExecution
	Total      int `json:"total"`
	Limit      int `json:"limit"`
	Offset     int `json:"offset"`
}
