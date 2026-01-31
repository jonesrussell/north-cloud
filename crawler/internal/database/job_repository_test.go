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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
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

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
