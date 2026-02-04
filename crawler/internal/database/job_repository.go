package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// ErrJobNotFoundBySourceID is returned when no job exists for a given source ID.
// This is a sentinel error that callers can check with errors.Is().
var ErrJobNotFoundBySourceID = errors.New("job not found for source_id")

// jobInsertColumns lists columns for job INSERT operations.
const jobInsertColumns = `id, source_id, source_name, url,
	schedule_time, schedule_enabled,
	interval_minutes, interval_type,
	is_paused, max_retries, retry_backoff_seconds,
	status, metadata`

// jobSelectBase lists columns for job SELECT queries (without auto-managed fields).
const jobSelectBase = `id, source_id, source_name, url,
	schedule_time, schedule_enabled,
	interval_minutes, interval_type, next_run_at,
	is_paused, max_retries, retry_backoff_seconds, current_retry_count,
	lock_token, lock_acquired_at,
	status, scheduler_version,
	created_at, updated_at, started_at, completed_at,
	paused_at, cancelled_at,
	error_message, metadata`

// jobSelectAutoManaged extends jobSelectBase with auto-managed fields.
const jobSelectAutoManaged = jobSelectBase + `,
	auto_managed, priority, failure_count, last_failure_at, backoff_until`

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
	query := `INSERT INTO jobs (` + jobInsertColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at, next_run_at`

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
		domain.MetadataPtr(job.Metadata),
	).Scan(&job.CreatedAt, &job.UpdatedAt, &job.NextRunAt)

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// CreateOrUpdate inserts a new job or updates an existing one by source_id.
// Returns wasInserted=true for new jobs, false when updating an existing job.
func (r *JobRepository) CreateOrUpdate(ctx context.Context, job *domain.Job) (bool, error) {
	query := `INSERT INTO jobs (` + jobInsertColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (source_id) DO UPDATE SET
			source_name = EXCLUDED.source_name,
			url = EXCLUDED.url,
			schedule_time = EXCLUDED.schedule_time,
			schedule_enabled = EXCLUDED.schedule_enabled,
			interval_minutes = EXCLUDED.interval_minutes,
			interval_type = EXCLUDED.interval_type,
			is_paused = EXCLUDED.is_paused,
			max_retries = EXCLUDED.max_retries,
			retry_backoff_seconds = EXCLUDED.retry_backoff_seconds,
			status = CASE
				WHEN jobs.status = 'running' THEN jobs.status
				ELSE EXCLUDED.status
			END,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id, created_at, updated_at, next_run_at
	`

	originalID := job.ID
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
		domain.MetadataPtr(job.Metadata),
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt, &job.NextRunAt)

	if err != nil {
		return false, fmt.Errorf("failed to create or update job: %w", err)
	}

	// Insert returns our id; update returns the existing row's id
	wasInserted := job.ID == originalID
	return wasInserted, nil
}

// ListJobsParams contains parameters for listing jobs.
type ListJobsParams struct {
	Status    string // Optional status filter
	SourceID  string // Optional source_id filter
	Search    string // Optional search term (source_name, url)
	SortBy    string // Column to sort by (already validated)
	SortOrder string // "asc" or "desc" (already validated)
	Limit     int
	Offset    int
}

// CountJobsParams contains parameters for counting jobs.
type CountJobsParams struct {
	Status   string // Optional status filter
	SourceID string // Optional source_id filter
	Search   string // Optional search term
}

// GetByID retrieves a job by its ID.
func (r *JobRepository) GetByID(ctx context.Context, id string) (*domain.Job, error) {
	var job domain.Job
	query := `SELECT ` + jobSelectBase + `
		FROM jobs
		WHERE id = $1`

	err := r.db.GetContext(ctx, &job, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("job not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return &job, nil
}

// List retrieves jobs with filtering, sorting, and pagination.
func (r *JobRepository) List(ctx context.Context, params ListJobsParams) ([]*domain.Job, error) {
	var jobs []*domain.Job
	var conditions []string
	var args []any
	argIndex := 1

	// Build WHERE conditions
	if params.Status != "" {
		// Handle comma-separated status values
		statuses := strings.Split(params.Status, ",")
		if len(statuses) == 1 {
			conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
			args = append(args, params.Status)
			argIndex++
		} else {
			// Build IN clause for multiple statuses
			placeholders := make([]string, len(statuses))
			for i, s := range statuses {
				placeholders[i] = fmt.Sprintf("$%d", argIndex)
				args = append(args, strings.TrimSpace(s))
				argIndex++
			}
			conditions = append(conditions, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ", ")))
		}
	}

	if params.SourceID != "" {
		conditions = append(conditions, fmt.Sprintf("source_id = $%d", argIndex))
		args = append(args, params.SourceID)
		argIndex++
	}

	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(COALESCE(source_name, '') ILIKE $%d OR url ILIKE $%d)",
			argIndex, argIndex,
		))
		args = append(args, "%"+params.Search+"%")
		argIndex++
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Default sort
	sortBy := params.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := params.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	query := fmt.Sprintf(`SELECT %s
		FROM jobs
		%s
		ORDER BY %s %s NULLS LAST
		LIMIT $%d OFFSET $%d
	`, jobSelectBase, whereClause, sortBy, strings.ToUpper(sortOrder), argIndex, argIndex+1)

	args = append(args, params.Limit, params.Offset)

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

	result, execErr := r.db.ExecContext(
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
		domain.MetadataPtr(job.Metadata),
		job.ID,
	)

	return execRequireRows(result, execErr, fmt.Errorf("job not found: %s", job.ID))
}

// Delete removes a job from the database.
func (r *JobRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM jobs WHERE id = $1`

	result, execErr := r.db.ExecContext(ctx, query, id)
	return execRequireRows(result, execErr, fmt.Errorf("job not found: %s", id))
}

// Count returns the total number of jobs matching the given filters.
func (r *JobRepository) Count(ctx context.Context, params CountJobsParams) (int, error) {
	var count int
	var conditions []string
	var args []any
	argIndex := 1

	if params.Status != "" {
		// Handle comma-separated status values
		statuses := strings.Split(params.Status, ",")
		if len(statuses) == 1 {
			conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
			args = append(args, params.Status)
			argIndex++
		} else {
			// Build IN clause for multiple statuses
			placeholders := make([]string, len(statuses))
			for i, s := range statuses {
				placeholders[i] = fmt.Sprintf("$%d", argIndex)
				args = append(args, strings.TrimSpace(s))
				argIndex++
			}
			conditions = append(conditions, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ", ")))
		}
	}

	if params.SourceID != "" {
		conditions = append(conditions, fmt.Sprintf("source_id = $%d", argIndex))
		args = append(args, params.SourceID)
		argIndex++
	}

	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(COALESCE(source_name, '') ILIKE $%d OR url ILIKE $%d)",
			argIndex, argIndex,
		))
		args = append(args, "%"+params.Search+"%")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM jobs %s", whereClause)

	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	return count, nil
}

// GetJobsReadyToRun retrieves all V1 scheduler jobs that are ready to be executed.
// This method is used by the V1 interval scheduler and only returns jobs with
// scheduler_version = 1 (or NULL for backward compatibility).
// Returns jobs where:
//   - next_run_at is in the past (for scheduled jobs), OR
//   - schedule_enabled = false and status = 'pending' (for immediate jobs)
func (r *JobRepository) GetJobsReadyToRun(ctx context.Context) ([]*domain.Job, error) {
	var jobs []*domain.Job
	query := `SELECT ` + jobSelectBase + `
		FROM jobs
		WHERE is_paused = false
		  AND status IN ('pending', 'scheduled')
		  AND lock_token IS NULL
		  AND (scheduler_version = 1 OR scheduler_version IS NULL)
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

// GetScheduledJobs returns all scheduled jobs (next_run_at IS NOT NULL, not paused).
// Used to rebuild the bucket map on scheduler startup.
func (r *JobRepository) GetScheduledJobs(ctx context.Context) ([]*domain.Job, error) {
	query := `SELECT ` + jobSelectBase + `
		FROM jobs
		WHERE next_run_at IS NOT NULL
		AND is_paused = false
		AND status IN ('pending', 'scheduled')
		ORDER BY next_run_at
	`

	var jobs []*domain.Job
	err := r.db.SelectContext(ctx, &jobs, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled jobs: %w", err)
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

	result, execErr := r.db.ExecContext(ctx, query, jobID)
	return execRequireRows(result, execErr, fmt.Errorf("job not found or not in scheduled state: %s", jobID))
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

	result, execErr := r.db.ExecContext(ctx, query, jobID)
	return execRequireRows(result, execErr, fmt.Errorf("job not found or not in paused state: %s", jobID))
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

	result, execErr := r.db.ExecContext(ctx, query, jobID)
	return execRequireRows(result, execErr, fmt.Errorf("job not found or already completed/failed/cancelled: %s", jobID))
}

// FindBySourceID retrieves a job by its source ID.
// Returns nil, nil if no job exists for the source.
func (r *JobRepository) FindBySourceID(ctx context.Context, sourceID uuid.UUID) (*domain.Job, error) {
	var job domain.Job
	query := `SELECT ` + jobSelectAutoManaged + `
		FROM jobs
		WHERE source_id = $1`

	err := r.db.GetContext(ctx, &job, query, sourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJobNotFoundBySourceID
		}
		return nil, fmt.Errorf("find job by source_id: %w", err)
	}

	return &job, nil
}

// UpsertAutoManaged creates or updates an auto-managed job.
// Uses source_id as the unique key for upsert.
func (r *JobRepository) UpsertAutoManaged(ctx context.Context, job *domain.Job) error {
	query := `
		INSERT INTO jobs (
			id, source_id, source_name, url,
			interval_minutes, interval_type, next_run_at,
			status, auto_managed, priority,
			failure_count, last_failure_at, backoff_until,
			schedule_enabled, is_paused,
			max_retries, retry_backoff_seconds
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (source_id) DO UPDATE SET
			source_name = EXCLUDED.source_name,
			url = EXCLUDED.url,
			interval_minutes = EXCLUDED.interval_minutes,
			interval_type = EXCLUDED.interval_type,
			next_run_at = COALESCE(jobs.next_run_at, EXCLUDED.next_run_at),
			status = CASE
				WHEN jobs.status IN ('running', 'scheduled') THEN jobs.status
				ELSE EXCLUDED.status
			END,
			priority = EXCLUDED.priority,
			failure_count = EXCLUDED.failure_count,
			last_failure_at = EXCLUDED.last_failure_at,
			backoff_until = EXCLUDED.backoff_until,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		job.ID,
		job.SourceID,
		job.SourceName,
		job.URL,
		job.IntervalMinutes,
		job.IntervalType,
		job.NextRunAt,
		job.Status,
		job.AutoManaged,
		job.Priority,
		job.FailureCount,
		job.LastFailureAt,
		job.BackoffUntil,
		job.ScheduleEnabled,
		job.IsPaused,
		job.MaxRetries,
		job.RetryBackoffSeconds,
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return fmt.Errorf("upsert auto-managed job: %w", err)
	}

	return nil
}

// DeleteBySourceID deletes a job by its source ID.
func (r *JobRepository) DeleteBySourceID(ctx context.Context, sourceID uuid.UUID) error {
	query := `DELETE FROM jobs WHERE source_id = $1`

	_, err := r.db.ExecContext(ctx, query, sourceID)
	if err != nil {
		return fmt.Errorf("delete job by source_id: %w", err)
	}

	return nil
}

// UpdateStatusBySourceID updates a job's status by source ID.
func (r *JobRepository) UpdateStatusBySourceID(ctx context.Context, sourceID uuid.UUID, status string) error {
	query := `
		UPDATE jobs
		SET status = $1, updated_at = NOW()
		WHERE source_id = $2
	`

	_, err := r.db.ExecContext(ctx, query, status, sourceID)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	return nil
}

// FindManualJobs returns jobs that are not auto-managed and need migration.
// Excludes already-migrated jobs.
func (r *JobRepository) FindManualJobs(ctx context.Context, limit int) ([]*domain.Job, error) {
	query := `
		SELECT id, source_id, source_name, url, interval_minutes, interval_type,
		       schedule_time, next_run_at, status,
		       is_paused, max_retries, retry_backoff_seconds,
		       created_at, updated_at, schedule_enabled, scheduler_version,
		       auto_managed, priority, failure_count, last_failure_at, backoff_until,
		       migration_status
		FROM jobs
		WHERE (auto_managed = false OR auto_managed IS NULL)
		  AND (migration_status IS NULL OR migration_status NOT IN ('migrated', 'skipped'))
		ORDER BY created_at ASC
		LIMIT $1
	`

	var jobs []*domain.Job
	if selectErr := r.db.SelectContext(ctx, &jobs, query, limit); selectErr != nil {
		return nil, fmt.Errorf("find manual jobs: %w", selectErr)
	}

	return jobs, nil
}

// UpdateMigrationStatus updates the migration status for a job.
func (r *JobRepository) UpdateMigrationStatus(ctx context.Context, jobID, status string) error {
	query := `
		UPDATE jobs
		SET migration_status = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, execErr := r.db.ExecContext(ctx, query, status, jobID)
	return execRequireRows(result, execErr, fmt.Errorf("job not found: %s", jobID))
}

// CountByMigrationStatus returns counts of jobs grouped by migration status.
func (r *JobRepository) CountByMigrationStatus(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT
			COALESCE(migration_status, 'pending') as status,
			COUNT(*) as count
		FROM jobs
		GROUP BY migration_status
	`

	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count by migration status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if scanErr := rows.Scan(&status, &count); scanErr != nil {
			return nil, fmt.Errorf("scan row: %w", scanErr)
		}
		counts[status] = count
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate rows: %w", rowsErr)
	}

	return counts, nil
}
