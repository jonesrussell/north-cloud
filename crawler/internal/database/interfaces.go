package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// JobRepositoryInterface defines the contract for job data access.
type JobRepositoryInterface interface {
	// Basic CRUD operations
	Create(ctx context.Context, job *domain.Job) error
	CreateOrUpdate(ctx context.Context, job *domain.Job) (bool, error)
	GetByID(ctx context.Context, id string) (*domain.Job, error)
	List(ctx context.Context, params ListJobsParams) ([]*domain.Job, error)
	Update(ctx context.Context, job *domain.Job) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, params CountJobsParams) (int, error)

	// Scheduler operations
	GetJobsReadyToRun(ctx context.Context) ([]*domain.Job, error)
	GetScheduledJobs(ctx context.Context) ([]*domain.Job, error)
	AcquireLock(ctx context.Context, jobID string, token uuid.UUID, now time.Time, duration time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, jobID string) error
	ClearStaleLocks(ctx context.Context, cutoff time.Time) (int, error)

	// Job control operations
	PauseJob(ctx context.Context, jobID string) error
	ResumeJob(ctx context.Context, jobID string) error
	CancelJob(ctx context.Context, jobID string) error

	// Analytics
	CountByStatus(ctx context.Context) (map[string]int, error)
}

// ExecutionRepositoryInterface defines the contract for execution history data access.
type ExecutionRepositoryInterface interface {
	// Basic CRUD operations
	Create(ctx context.Context, execution *domain.JobExecution) error
	GetByID(ctx context.Context, id string) (*domain.JobExecution, error)
	Update(ctx context.Context, execution *domain.JobExecution) error
	Delete(ctx context.Context, id string) error

	// Query operations
	ListByJobID(ctx context.Context, jobID string, limit, offset int) ([]*domain.JobExecution, error)
	CountByJobID(ctx context.Context, jobID string) (int, error)
	GetLatestByJobID(ctx context.Context, jobID string) (*domain.JobExecution, error)

	// Analytics operations
	GetJobStats(ctx context.Context, jobID string) (*domain.JobStats, error)
	GetAggregateStats(ctx context.Context) (*domain.AggregateStats, error)
	GetTodayStats(ctx context.Context) (crawledToday int64, indexedToday int64, err error)
	GetFailureRate(ctx context.Context, window time.Duration) (float64, error)
	GetStuckJobs(ctx context.Context, threshold time.Duration) ([]*domain.Job, error)

	// Maintenance operations
	CleanupOldExecutions(ctx context.Context) (int, error)
}
