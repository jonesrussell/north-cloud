package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// JobRepository handles database operations for jobs.
type JobRepository struct {
	db *sqlx.DB
}

// NewJobRepository creates a new job repository.
func NewJobRepository(db *sqlx.DB) *JobRepository {
	return &JobRepository{db: db}
}

// Create inserts a new job into the database.
func (r *JobRepository) Create(ctx context.Context, job *domain.Job) error {
	query := `
		INSERT INTO jobs (
			id, source_id, source_name, url,
			schedule_time, schedule_enabled,
			interval_minutes, interval_type,
			is_paused, max_retries, retry_backoff_seconds,
			status, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at, next_run_at
	`

	// Convert Metadata to pointer for driver.Valuer interface
	var metadataPtr *domain.JSONBMap
	if job.Metadata != nil {
		metadataPtr = &job.Metadata
	}

	err := r.db.QueryRowContext(
		ctx,
		query,
		job.ID,
		job.SourceID,
		job.SourceName,
		job.URL,
		job.ScheduleTime,
		job.ScheduleEnabled,
		job.IntervalMinutes,
		job.IntervalType,
		job.IsPaused,
		job.MaxRetries,
		job.RetryBackoffSeconds,
		job.Status,
		metadataPtr,
	).Scan(&job.CreatedAt, &job.UpdatedAt, &job.NextRunAt)

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// GetByID retrieves a job by its ID.
func (r *JobRepository) GetByID(ctx context.Context, id string) (*domain.Job, error) {
	var job domain.Job
	query := `
		SELECT id, source_id, source_name, url,
		       schedule_time, schedule_enabled,
		       interval_minutes, interval_type, next_run_at,
		       is_paused, max_retries, retry_backoff_seconds, current_retry_count,
		       lock_token, lock_acquired_at,
		       status,
		       created_at, updated_at, started_at, completed_at,
		       paused_at, cancelled_at,
		       error_message, metadata
		FROM jobs
		WHERE id = $1
	`

	err := r.db.GetContext(ctx, &job, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("job not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return &job, nil
}

// List retrieves all jobs with optional filtering.
func (r *JobRepository) List(ctx context.Context, status string, limit, offset int) ([]*domain.Job, error) {
	var jobs []*domain.Job
	var query string
	var args []any

	selectFields := `
		id, source_id, source_name, url,
		schedule_time, schedule_enabled,
		interval_minutes, interval_type, next_run_at,
		is_paused, max_retries, retry_backoff_seconds, current_retry_count,
		lock_token, lock_acquired_at,
		status,
		created_at, updated_at, started_at, completed_at,
		paused_at, cancelled_at,
		error_message, metadata
	`

	if status != "" {
		query = fmt.Sprintf(`
			SELECT %s
			FROM jobs
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`, selectFields)
		args = []any{status, limit, offset}
	} else {
		query = fmt.Sprintf(`
			SELECT %s
			FROM jobs
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`, selectFields)
		args = []any{limit, offset}
	}

	err := r.db.SelectContext(ctx, &jobs, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	if jobs == nil {
		jobs = []*domain.Job{}
	}

	return jobs, nil
}

// Update updates an existing job.
func (r *JobRepository) Update(ctx context.Context, job *domain.Job) error {
	query := `
		UPDATE jobs
		SET source_id = $1, source_name = $2, url = $3,
		    schedule_time = $4, schedule_enabled = $5,
		    interval_minutes = $6, interval_type = $7, next_run_at = $8,
		    is_paused = $9, max_retries = $10, retry_backoff_seconds = $11,
		    current_retry_count = $12,
		    lock_token = $13, lock_acquired_at = $14,
		    status = $15,
		    started_at = $16, completed_at = $17,
		    paused_at = $18, cancelled_at = $19,
		    error_message = $20, metadata = $21
		WHERE id = $22
	`

	// Convert Metadata to pointer for driver.Valuer interface
	var metadataPtr *domain.JSONBMap
	if job.Metadata != nil {
		metadataPtr = &job.Metadata
	}

	result, err := r.db.ExecContext(
		ctx,
		query,
		job.SourceID,
		job.SourceName,
		job.URL,
		job.ScheduleTime,
		job.ScheduleEnabled,
		job.IntervalMinutes,
		job.IntervalType,
		job.NextRunAt,
		job.IsPaused,
		job.MaxRetries,
		job.RetryBackoffSeconds,
		job.CurrentRetryCount,
		job.LockToken,
		job.LockAcquiredAt,
		job.Status,
		job.StartedAt,
		job.CompletedAt,
		job.PausedAt,
		job.CancelledAt,
		job.ErrorMessage,
		metadataPtr,
		job.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", job.ID)
	}

	return nil
}

// Delete removes a job from the database.
func (r *JobRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM jobs WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

// Count returns the total number of jobs, optionally filtered by status.
func (r *JobRepository) Count(ctx context.Context, status string) (int, error) {
	var count int
	var query string
	var args []any

	if status != "" {
		query = `SELECT COUNT(*) FROM jobs WHERE status = $1`
		args = []any{status}
	} else {
		query = `SELECT COUNT(*) FROM jobs`
		args = []any{}
	}

	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	return count, nil
}

// GetJobsReadyToRun retrieves all jobs that are ready to be executed.
// Returns jobs where:
//   - next_run_at is in the past (for scheduled jobs), OR
//   - schedule_enabled = false and status = 'pending' (for immediate jobs)
func (r *JobRepository) GetJobsReadyToRun(ctx context.Context) ([]*domain.Job, error) {
	var jobs []*domain.Job
	query := `
		SELECT id, source_id, source_name, url,
		       schedule_time, schedule_enabled,
		       interval_minutes, interval_type, next_run_at,
		       is_paused, max_retries, retry_backoff_seconds, current_retry_count,
		       lock_token, lock_acquired_at,
		       status,
		       created_at, updated_at, started_at, completed_at,
		       paused_at, cancelled_at,
		       error_message, metadata
		FROM jobs
		WHERE is_paused = false
		  AND status IN ('pending', 'scheduled')
		  AND lock_token IS NULL
		  AND (
		      -- Scheduled jobs: next_run_at is in the past
		      (next_run_at IS NOT NULL AND next_run_at <= NOW())
		      OR
		      -- Immediate jobs: no schedule, ready to run immediately
		      (schedule_enabled = false AND next_run_at IS NULL AND status = 'pending')
		  )
		ORDER BY 
		  CASE 
		    WHEN schedule_enabled = false THEN 0  -- Immediate jobs first
		    ELSE 1
		  END,
		  next_run_at ASC NULLS LAST
		LIMIT 100
	`

	err := r.db.SelectContext(ctx, &jobs, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs ready to run: %w", err)
	}

	if jobs == nil {
		jobs = []*domain.Job{}
	}

	return jobs, nil
}

// AcquireLock attempts to acquire a distributed lock for a job.
// Uses atomic compare-and-swap to prevent race conditions.
// Returns true if lock was acquired, false if already locked by another instance.
func (r *JobRepository) AcquireLock(
	ctx context.Context,
	jobID string,
	token uuid.UUID,
	now time.Time,
	duration time.Duration,
) (bool, error) {
	query := `
		UPDATE jobs
		SET lock_token = $1,
		    lock_acquired_at = $2
		WHERE id = $3
		  AND lock_token IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, token.String(), now, jobID)
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected > 0, nil
}

// ReleaseLock releases the distributed lock for a job.
func (r *JobRepository) ReleaseLock(ctx context.Context, jobID string) error {
	query := `
		UPDATE jobs
		SET lock_token = NULL,
		    lock_acquired_at = NULL
		WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	return nil
}

// ClearStaleLocks clears locks that are older than the specified cutoff time.
// This prevents jobs from being stuck indefinitely if a scheduler instance crashes.
// Returns the number of locks cleared.
func (r *JobRepository) ClearStaleLocks(ctx context.Context, cutoff time.Time) (int, error) {
	query := `
		UPDATE jobs
		SET lock_token = NULL,
		    lock_acquired_at = NULL
		WHERE lock_token IS NOT NULL
		  AND lock_acquired_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to clear stale locks: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// PauseJob pauses a scheduled job.
// Only scheduled jobs can be paused.
func (r *JobRepository) PauseJob(ctx context.Context, jobID string) error {
	query := `
		UPDATE jobs
		SET is_paused = true,
		    status = 'paused',
		    paused_at = NOW(),
		    next_run_at = NULL
		WHERE id = $1
		  AND status = 'scheduled'
	`

	result, err := r.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to pause job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found or not in scheduled state: %s", jobID)
	}

	return nil
}

// ResumeJob resumes a paused job.
// Only paused jobs can be resumed.
func (r *JobRepository) ResumeJob(ctx context.Context, jobID string) error {
	query := `
		UPDATE jobs
		SET is_paused = false,
		    status = 'scheduled',
		    paused_at = NULL
		WHERE id = $1
		  AND status = 'paused'
	`

	result, err := r.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to resume job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found or not in paused state: %s", jobID)
	}

	return nil
}

// CancelJob cancels a job.
// Can cancel scheduled, running, or paused jobs.
func (r *JobRepository) CancelJob(ctx context.Context, jobID string) error {
	query := `
		UPDATE jobs
		SET status = 'cancelled',
		    cancelled_at = NOW(),
		    next_run_at = NULL
		WHERE id = $1
		  AND status IN ('scheduled', 'running', 'paused', 'pending')
	`

	result, err := r.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found or already completed/failed/cancelled: %s", jobID)
	}

	return nil
}
