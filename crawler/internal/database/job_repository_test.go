package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestJobRepository_CreateOrUpdate_Insert(t *testing.T) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewJobRepository(db)
	ctx := context.Background()

	jobID := "new-job-123"
	createdAt := time.Now()
	updatedAt := time.Now()

	mock.ExpectQuery("INSERT INTO jobs").
		WithArgs(
			jobID,
			"80729e12-5127-48f5-9f5c-dcc2647c6fe6",
			sqlmock.AnyArg(),
			"https://calgaryherald.com",
			sqlmock.AnyArg(),
			false,
			sqlmock.AnyArg(),
			"minutes",
			false,
			3,
			60,
			"pending",
			sqlmock.AnyArg(),
		).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "created_at", "updated_at", "next_run_at"}).
				AddRow(jobID, createdAt, updatedAt, nil),
		)

	job := &domain.Job{
		ID:                  jobID,
		SourceID:            "80729e12-5127-48f5-9f5c-dcc2647c6fe6",
		URL:                 "https://calgaryherald.com",
		ScheduleEnabled:     false,
		MaxRetries:          3,
		RetryBackoffSeconds: 60,
		IntervalType:        "minutes",
		Status:              "pending",
	}

	wasInserted, createErr := repo.CreateOrUpdate(ctx, job)
	if createErr != nil {
		t.Fatalf("CreateOrUpdate() error = %v", createErr)
	}

	if !wasInserted {
		t.Error("expected wasInserted=true for new job")
	}

	if job.ID != jobID {
		t.Errorf("expected job.ID=%s, got %s", jobID, job.ID)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestJobRepository_CreateOrUpdate_UpdateExisting(t *testing.T) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewJobRepository(db)
	ctx := context.Background()

	newJobID := "attempted-new-id"
	existingJobID := "existing-job-456"
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now()

	mock.ExpectQuery("INSERT INTO jobs").
		WithArgs(
			sqlmock.AnyArg(),
			"80729e12-5127-48f5-9f5c-dcc2647c6fe6",
			sqlmock.AnyArg(),
			"https://calgaryherald.com",
			sqlmock.AnyArg(),
			false,
			sqlmock.AnyArg(),
			"minutes",
			false,
			3,
			60,
			"pending",
			sqlmock.AnyArg(),
		).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "created_at", "updated_at", "next_run_at"}).
				AddRow(existingJobID, createdAt, updatedAt, nil),
		)

	job := &domain.Job{
		ID:                  newJobID,
		SourceID:            "80729e12-5127-48f5-9f5c-dcc2647c6fe6",
		URL:                 "https://calgaryherald.com",
		ScheduleEnabled:     false,
		MaxRetries:          3,
		RetryBackoffSeconds: 60,
		IntervalType:        "minutes",
		Status:              "pending",
	}

	wasInserted, createErr := repo.CreateOrUpdate(ctx, job)
	if createErr != nil {
		t.Fatalf("CreateOrUpdate() error = %v", createErr)
	}

	if wasInserted {
		t.Error("expected wasInserted=false when updating existing job")
	}

	if job.ID != existingJobID {
		t.Errorf("expected job.ID=%s (existing row), got %s", existingJobID, job.ID)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestJobRepository_List_WithSorting(t *testing.T) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewJobRepository(db)
	ctx := context.Background()

	// Define columns to match jobSelectBase
	cols := []string{
		"id", "source_id", "source_name", "url",
		"schedule_time", "schedule_enabled",
		"interval_minutes", "interval_type", "next_run_at",
		"is_paused", "max_retries", "retry_backoff_seconds", "current_retry_count",
		"lock_token", "lock_acquired_at",
		"status", "scheduler_version",
		"created_at", "updated_at", "started_at", "completed_at",
		"paused_at", "cancelled_at",
		"error_message", "metadata",
	}

	// Expect query with ORDER BY next_run_at ASC NULLS LAST (no WHERE clause)
	mock.ExpectQuery("SELECT .+ FROM jobs\\s+ORDER BY next_run_at ASC NULLS LAST").
		WithArgs(50, 0).
		WillReturnRows(sqlmock.NewRows(cols))

	params := database.ListJobsParams{
		Limit:     50,
		Offset:    0,
		SortBy:    "next_run_at",
		SortOrder: "asc",
	}

	jobs, listErr := repo.List(ctx, params)
	if listErr != nil {
		t.Fatalf("List() error = %v", listErr)
	}

	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestJobRepository_List_WithStatusFilter(t *testing.T) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewJobRepository(db)
	ctx := context.Background()

	cols := []string{
		"id", "source_id", "source_name", "url",
		"schedule_time", "schedule_enabled",
		"interval_minutes", "interval_type", "next_run_at",
		"is_paused", "max_retries", "retry_backoff_seconds", "current_retry_count",
		"lock_token", "lock_acquired_at",
		"status", "scheduler_version",
		"created_at", "updated_at", "started_at", "completed_at",
		"paused_at", "cancelled_at",
		"error_message", "metadata",
	}

	mock.ExpectQuery("SELECT .+ FROM jobs\\s+WHERE status = \\$1").
		WithArgs("running", 25, 0).
		WillReturnRows(sqlmock.NewRows(cols))

	params := database.ListJobsParams{
		Status:    "running",
		Limit:     25,
		Offset:    0,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	jobs, listErr := repo.List(ctx, params)
	if listErr != nil {
		t.Fatalf("List() error = %v", listErr)
	}

	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestJobRepository_Count_WithFilters(t *testing.T) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewJobRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM jobs WHERE status = \\$1 AND source_id = \\$2").
		WithArgs("running", "src-123").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	params := database.CountJobsParams{
		Status:   "running",
		SourceID: "src-123",
	}

	count, countErr := repo.Count(ctx, params)
	if countErr != nil {
		t.Fatalf("Count() error = %v", countErr)
	}

	if count != 42 {
		t.Errorf("expected count=42, got %d", count)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}
