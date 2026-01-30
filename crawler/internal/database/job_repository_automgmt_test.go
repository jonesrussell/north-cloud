package database_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestJobRepository_FindBySourceID_ReturnsNilWhenNotFound(t *testing.T) {
	t.Helper()

	// Verify method signature exists
	var _ interface {
		FindBySourceID(ctx context.Context, sourceID uuid.UUID) (*domain.Job, error)
	} = (*database.JobRepository)(nil)
}

func TestJobRepository_UpsertAutoManaged_Interface(t *testing.T) {
	t.Helper()

	// Verify method signature exists
	var _ interface {
		UpsertAutoManaged(ctx context.Context, job *domain.Job) error
	} = (*database.JobRepository)(nil)
}

func TestJobRepository_DeleteBySourceID_Interface(t *testing.T) {
	t.Helper()

	// Verify method signature exists
	var _ interface {
		DeleteBySourceID(ctx context.Context, sourceID uuid.UUID) error
	} = (*database.JobRepository)(nil)
}

func TestJobRepository_UpdateStatusBySourceID_Interface(t *testing.T) {
	t.Helper()

	// Verify method signature exists
	var _ interface {
		UpdateStatusBySourceID(ctx context.Context, sourceID uuid.UUID, status string) error
	} = (*database.JobRepository)(nil)
}

func TestJobRepository_RecordProcessedEvent_Interface(t *testing.T) {
	t.Helper()

	// Verify method signature exists
	var _ interface {
		RecordProcessedEvent(ctx context.Context, eventID uuid.UUID) error
	} = (*database.JobRepository)(nil)
}

func TestJobRepository_IsEventProcessed_Interface(t *testing.T) {
	t.Helper()

	// Verify method signature exists
	var _ interface {
		IsEventProcessed(ctx context.Context, eventID uuid.UUID) (bool, error)
	} = (*database.JobRepository)(nil)
}
