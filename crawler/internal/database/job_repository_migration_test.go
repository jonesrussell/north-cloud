// crawler/internal/database/job_repository_migration_test.go
package database_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestJobRepository_FindManualJobs_Interface(t *testing.T) {
	t.Helper()

	// Verify method signature exists
	var _ interface {
		FindManualJobs(ctx context.Context, limit int) ([]*domain.Job, error)
	} = (*database.JobRepository)(nil)
}

func TestJobRepository_UpdateMigrationStatus_Interface(t *testing.T) {
	t.Helper()

	// Verify method signature exists
	var _ interface {
		UpdateMigrationStatus(ctx context.Context, jobID string, status string) error
	} = (*database.JobRepository)(nil)
}

func TestJobRepository_CountByMigrationStatus_Interface(t *testing.T) {
	t.Helper()

	// Verify method signature exists
	var _ interface {
		CountByMigrationStatus(ctx context.Context) (map[string]int, error)
	} = (*database.JobRepository)(nil)
}
