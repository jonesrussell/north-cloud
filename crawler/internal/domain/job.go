// Package domain provides domain models used across the application.
package domain

import (
	"time"
)

// Job represents a crawling job.
type Job struct {
	// Identity
	ID         string  `db:"id"          json:"id"`
	SourceID   string  `db:"source_id"   json:"source_id"`
	SourceName *string `db:"source_name" json:"source_name,omitempty"`
	URL        string  `db:"url"         json:"url"`

	// Interval-based scheduling (replaces cron)
	IntervalMinutes *int       `db:"interval_minutes" json:"interval_minutes,omitempty"` // NULL = run once
	IntervalType    string     `db:"interval_type"    json:"interval_type"`              // 'minutes', 'hours', 'days'
	NextRunAt       *time.Time `db:"next_run_at"      json:"next_run_at,omitempty"`      // Auto-calculated

	// Legacy cron field (deprecated, kept for rollback)
	ScheduleTime    *string `db:"schedule_time"    json:"schedule_time,omitempty"`
	ScheduleEnabled bool    `db:"schedule_enabled" json:"schedule_enabled"`

	// Job control
	IsPaused            bool `db:"is_paused"             json:"is_paused"`
	MaxRetries          int  `db:"max_retries"           json:"max_retries"`
	RetryBackoffSeconds int  `db:"retry_backoff_seconds" json:"retry_backoff_seconds"`
	CurrentRetryCount   int  `db:"current_retry_count"   json:"current_retry_count"`

	// Distributed locking
	LockToken      *string    `db:"lock_token"       json:"lock_token,omitempty"`
	LockAcquiredAt *time.Time `db:"lock_acquired_at" json:"lock_acquired_at,omitempty"`

	// State tracking
	Status string `db:"status" json:"status"` // pending, scheduled, running, paused, cancelled, completed, failed

	// Scheduler version (1=V1 interval-based, 2=V2 Redis Streams)
	SchedulerVersion int `db:"scheduler_version" json:"scheduler_version"`

	// Timestamps
	CreatedAt   time.Time  `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"   json:"updated_at"`
	StartedAt   *time.Time `db:"started_at"   json:"started_at,omitempty"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at,omitempty"`
	PausedAt    *time.Time `db:"paused_at"    json:"paused_at,omitempty"`
	CancelledAt *time.Time `db:"cancelled_at" json:"cancelled_at,omitempty"`

	// Metadata
	ErrorMessage *string  `db:"error_message" json:"error_message,omitempty"`
	Metadata     JSONBMap `db:"metadata"      json:"metadata,omitempty"`

	// Auto-managed job lifecycle
	AutoManaged   bool       `db:"auto_managed"    json:"auto_managed"`
	Priority      int        `db:"priority"        json:"priority"`
	FailureCount  int        `db:"failure_count"   json:"failure_count"`
	LastFailureAt *time.Time `db:"last_failure_at" json:"last_failure_at,omitempty"`
	BackoffUntil  *time.Time `db:"backoff_until"   json:"backoff_until,omitempty"`

	// Phase 3 migration tracking
	MigrationStatus *string `db:"migration_status" json:"migration_status,omitempty"`
}

// Migration status values (Phase 3)
const (
	MigrationStatusMigrated = "migrated" // Successfully converted to auto-managed
	MigrationStatusOrphaned = "orphaned" // Source not found, marked for review
	MigrationStatusSkipped  = "skipped"  // Intentionally left as manual
)

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
