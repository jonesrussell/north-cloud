package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// ExecutionRepository handles database operations for job executions.
type ExecutionRepository struct {
	db *sqlx.DB
}

// NewExecutionRepository creates a new execution repository.
func NewExecutionRepository(db *sqlx.DB) *ExecutionRepository {
	return &ExecutionRepository{db: db}
}

// Create inserts a new execution record into the database.
func (r *ExecutionRepository) Create(ctx context.Context, execution *domain.JobExecution) error {
	query := `
		INSERT INTO job_executions (
			id, job_id, execution_number, status,
			started_at, items_crawled, items_indexed,
			retry_attempt, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		execution.ID,
		execution.JobID,
		execution.ExecutionNumber,
		execution.Status,
		execution.StartedAt,
		execution.ItemsCrawled,
		execution.ItemsIndexed,
		execution.RetryAttempt,
		execution.Metadata,
	)

	if err != nil {
		return fmt.Errorf("failed to create execution: %w", err)
	}

	return nil
}

// GetByID retrieves an execution by its ID.
func (r *ExecutionRepository) GetByID(ctx context.Context, id string) (*domain.JobExecution, error) {
	var execution domain.JobExecution
	query := `
		SELECT id, job_id, execution_number, status,
		       started_at, completed_at, duration_ms,
		       items_crawled, items_indexed,
		       error_message, stack_trace,
		       cpu_time_ms, memory_peak_mb,
		       retry_attempt, metadata
		FROM job_executions
		WHERE id = $1
	`

	err := r.db.GetContext(ctx, &execution, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("execution not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	return &execution, nil
}

// Update updates an existing execution record.
func (r *ExecutionRepository) Update(ctx context.Context, execution *domain.JobExecution) error {
	query := `
		UPDATE job_executions
		SET status = $1,
		    completed_at = $2,
		    duration_ms = $3,
		    items_crawled = $4,
		    items_indexed = $5,
		    error_message = $6,
		    stack_trace = $7,
		    cpu_time_ms = $8,
		    memory_peak_mb = $9,
		    metadata = $10
		WHERE id = $11
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		execution.Status,
		execution.CompletedAt,
		execution.DurationMs,
		execution.ItemsCrawled,
		execution.ItemsIndexed,
		execution.ErrorMessage,
		execution.StackTrace,
		execution.CPUTimeMs,
		execution.MemoryPeakMB,
		execution.Metadata,
		execution.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("execution not found: %s", execution.ID)
	}

	return nil
}

// Delete removes an execution from the database.
func (r *ExecutionRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM job_executions WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete execution: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("execution not found: %s", id)
	}

	return nil
}

// ListByJobID retrieves all executions for a specific job with pagination.
func (r *ExecutionRepository) ListByJobID(ctx context.Context, jobID string, limit, offset int) ([]*domain.JobExecution, error) {
	var executions []*domain.JobExecution
	query := `
		SELECT id, job_id, execution_number, status,
		       started_at, completed_at, duration_ms,
		       items_crawled, items_indexed,
		       error_message, stack_trace,
		       cpu_time_ms, memory_peak_mb,
		       retry_attempt, metadata
		FROM job_executions
		WHERE job_id = $1
		ORDER BY started_at DESC
		LIMIT $2 OFFSET $3
	`

	err := r.db.SelectContext(ctx, &executions, query, jobID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}

	if executions == nil {
		executions = []*domain.JobExecution{}
	}

	return executions, nil
}

// CountByJobID returns the total number of executions for a job.
func (r *ExecutionRepository) CountByJobID(ctx context.Context, jobID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM job_executions WHERE job_id = $1`

	err := r.db.GetContext(ctx, &count, query, jobID)
	if err != nil {
		return 0, fmt.Errorf("failed to count executions: %w", err)
	}

	return count, nil
}

// GetLatestByJobID retrieves the most recent execution for a job.
func (r *ExecutionRepository) GetLatestByJobID(ctx context.Context, jobID string) (*domain.JobExecution, error) {
	var execution domain.JobExecution
	query := `
		SELECT id, job_id, execution_number, status,
		       started_at, completed_at, duration_ms,
		       items_crawled, items_indexed,
		       error_message, stack_trace,
		       cpu_time_ms, memory_peak_mb,
		       retry_attempt, metadata
		FROM job_executions
		WHERE job_id = $1
		ORDER BY started_at DESC
		LIMIT 1
	`

	err := r.db.GetContext(ctx, &execution, query, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no executions found for job: %s", jobID)
		}
		return nil, fmt.Errorf("failed to get latest execution: %w", err)
	}

	return &execution, nil
}

// GetJobStats returns aggregate statistics for a specific job.
func (r *ExecutionRepository) GetJobStats(ctx context.Context, jobID string) (*domain.JobStats, error) {
	var stats domain.JobStats

	query := `
		SELECT
			COUNT(*) as total_executions,
			COUNT(*) FILTER (WHERE status = 'completed') as successful_runs,
			COUNT(*) FILTER (WHERE status = 'failed') as failed_runs,
			COALESCE(AVG(duration_ms) FILTER (WHERE duration_ms IS NOT NULL), 0) as average_duration_ms,
			MAX(started_at) as last_execution_at
		FROM job_executions
		WHERE job_id = $1
	`

	err := r.db.QueryRowContext(ctx, query, jobID).Scan(
		&stats.TotalExecutions,
		&stats.SuccessfulRuns,
		&stats.FailedRuns,
		&stats.AverageDurationMs,
		&stats.LastExecutionAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get job stats: %w", err)
	}

	// Calculate success rate
	if stats.TotalExecutions > 0 {
		stats.SuccessRate = float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions)
	}

	// Get next scheduled run from jobs table
	var nextRunAt *time.Time
	jobQuery := `SELECT next_run_at FROM jobs WHERE id = $1`
	err = r.db.QueryRowContext(ctx, jobQuery, jobID).Scan(&nextRunAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get next run time: %w", err)
	}
	stats.NextScheduledAt = nextRunAt

	return &stats, nil
}

// GetAggregateStats returns system-wide execution statistics.
func (r *ExecutionRepository) GetAggregateStats(ctx context.Context) (*domain.AggregateStats, error) {
	var stats domain.AggregateStats

	// Get execution stats
	execQuery := `
		SELECT
			COUNT(*) as total_executions,
			COALESCE(AVG(duration_ms) FILTER (WHERE duration_ms IS NOT NULL), 0) as avg_duration_ms,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM job_executions
	`

	var completed, failed int64
	err := r.db.QueryRowContext(ctx, execQuery).Scan(
		&stats.TotalExecutions,
		&stats.AvgDurationMs,
		&completed,
		&failed,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get aggregate execution stats: %w", err)
	}

	// Calculate rates
	total := completed + failed
	if total > 0 {
		stats.SuccessRate = float64(completed) / float64(total)
		stats.FailureRate = float64(failed) / float64(total)
	}

	// Get job counts
	jobQuery := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'running') as active_jobs,
			COUNT(*) FILTER (WHERE status = 'scheduled') as scheduled_jobs
		FROM jobs
	`

	err = r.db.QueryRowContext(ctx, jobQuery).Scan(
		&stats.ActiveJobs,
		&stats.ScheduledJobs,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get job counts: %w", err)
	}

	// Get today's stats
	todayQuery := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'completed') as completed_today,
			COUNT(*) FILTER (WHERE status = 'failed') as failed_today
		FROM job_executions
		WHERE started_at >= CURRENT_DATE
	`

	err = r.db.QueryRowContext(ctx, todayQuery).Scan(
		&stats.CompletedToday,
		&stats.FailedToday,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get today's stats: %w", err)
	}

	return &stats, nil
}

// GetFailureRate returns the failure rate within a time window.
func (r *ExecutionRepository) GetFailureRate(ctx context.Context, window time.Duration) (float64, error) {
	cutoff := time.Now().Add(-window)

	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM job_executions
		WHERE started_at >= $1
	`

	var completed, failed int
	err := r.db.QueryRowContext(ctx, query, cutoff).Scan(&completed, &failed)
	if err != nil {
		return 0, fmt.Errorf("failed to get failure rate: %w", err)
	}

	total := completed + failed
	if total == 0 {
		return 0, nil
	}

	return float64(failed) / float64(total), nil
}

// GetStuckJobs returns jobs that have been running longer than the threshold.
func (r *ExecutionRepository) GetStuckJobs(ctx context.Context, threshold time.Duration) ([]*domain.Job, error) {
	var jobs []*domain.Job
	cutoff := time.Now().Add(-threshold)

	query := `
		SELECT j.id, j.source_id, j.source_name, j.url,
		       j.schedule_time, j.schedule_enabled,
		       j.interval_minutes, j.interval_type, j.next_run_at,
		       j.is_paused, j.max_retries, j.retry_backoff_seconds, j.current_retry_count,
		       j.lock_token, j.lock_acquired_at,
		       j.status,
		       j.created_at, j.updated_at, j.started_at, j.completed_at,
		       j.paused_at, j.cancelled_at,
		       j.error_message, j.metadata
		FROM jobs j
		INNER JOIN job_executions e ON j.id = e.job_id
		WHERE j.status = 'running'
		  AND e.status = 'running'
		  AND e.started_at < $1
	`

	err := r.db.SelectContext(ctx, &jobs, query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to get stuck jobs: %w", err)
	}

	if jobs == nil {
		jobs = []*domain.Job{}
	}

	return jobs, nil
}

// CleanupOldExecutions removes old execution records based on the retention policy.
// Keeps the 100 most recent executions per job OR executions from the last 30 days.
// Returns the number of executions deleted.
func (r *ExecutionRepository) CleanupOldExecutions(ctx context.Context) (int, error) {
	// Call the database function created in the migration
	query := `SELECT cleanup_old_executions()`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old executions: %w", err)
	}

	// Get count of deleted rows
	// Note: The function doesn't return a count, so we'll return 0
	// You could modify the SQL function to return a count if needed
	return 0, nil
}
