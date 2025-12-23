package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
		INSERT INTO jobs (id, source_id, source_name, url, schedule_time, schedule_enabled, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		job.ID,
		job.SourceID,
		job.SourceName,
		job.URL,
		job.ScheduleTime,
		job.ScheduleEnabled,
		job.Status,
	).Scan(&job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// GetByID retrieves a job by its ID.
func (r *JobRepository) GetByID(ctx context.Context, id string) (*domain.Job, error) {
	var job domain.Job
	query := `
		SELECT id, source_id, source_name, url, schedule_time, schedule_enabled, status,
		       created_at, updated_at, started_at, completed_at, error_message
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

	if status != "" {
		query = `
			SELECT id, source_id, source_name, url, schedule_time, schedule_enabled, status,
			       created_at, updated_at, started_at, completed_at, error_message
			FROM jobs
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []any{status, limit, offset}
	} else {
		query = `
			SELECT id, source_id, source_name, url, schedule_time, schedule_enabled, status,
			       created_at, updated_at, started_at, completed_at, error_message
			FROM jobs
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
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
		SET source_id = $1, source_name = $2, url = $3, schedule_time = $4,
		    schedule_enabled = $5, status = $6, started_at = $7, completed_at = $8,
		    error_message = $9
		WHERE id = $10
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		job.SourceID,
		job.SourceName,
		job.URL,
		job.ScheduleTime,
		job.ScheduleEnabled,
		job.Status,
		job.StartedAt,
		job.CompletedAt,
		job.ErrorMessage,
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
