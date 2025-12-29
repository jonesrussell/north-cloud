// Package domain provides domain models used across the application.
package domain

import (
	"time"
)

// Job represents a crawling job.
type Job struct {
	// Identity
	ID         string  `json:"id" db:"id"`
	SourceID   string  `json:"source_id" db:"source_id"`
	SourceName *string `json:"source_name,omitempty" db:"source_name"`
	URL        string  `json:"url" db:"url"`

	// Interval-based scheduling (replaces cron)
	IntervalMinutes *int       `json:"interval_minutes,omitempty" db:"interval_minutes"` // NULL = run once
	IntervalType    string     `json:"interval_type" db:"interval_type"`                 // 'minutes', 'hours', 'days'
	NextRunAt       *time.Time `json:"next_run_at,omitempty" db:"next_run_at"`           // Auto-calculated

	// Legacy cron field (deprecated, kept for rollback)
	ScheduleTime    *string `json:"schedule_time,omitempty" db:"schedule_time"`
	ScheduleEnabled bool    `json:"schedule_enabled" db:"schedule_enabled"`

	// Job control
	IsPaused            bool `json:"is_paused" db:"is_paused"`
	MaxRetries          int  `json:"max_retries" db:"max_retries"`
	RetryBackoffSeconds int  `json:"retry_backoff_seconds" db:"retry_backoff_seconds"`
	CurrentRetryCount   int  `json:"current_retry_count" db:"current_retry_count"`

	// Distributed locking
	LockToken      *string    `json:"lock_token,omitempty" db:"lock_token"`
	LockAcquiredAt *time.Time `json:"lock_acquired_at,omitempty" db:"lock_acquired_at"`

	// State tracking
	Status string `json:"status" db:"status"` // pending, scheduled, running, paused, cancelled, completed, failed

	// Timestamps
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	PausedAt    *time.Time `json:"paused_at,omitempty" db:"paused_at"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty" db:"cancelled_at"`

	// Metadata
	ErrorMessage *string                `json:"error_message,omitempty" db:"error_message"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}

// Item represents a crawled item from a job.
type Item struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	URL       string    `json:"url"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
