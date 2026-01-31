// crawler/internal/job/migrator_test.go
package job_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
)

// MockMigrationRepository implements job.MigrationRepository for testing.
type MockMigrationRepository struct {
	ManualJobs      []*domain.Job
	UpdatedStatuses map[string]string
}

func NewMockMigrationRepository() *MockMigrationRepository {
	return &MockMigrationRepository{
		ManualJobs:      make([]*domain.Job, 0),
		UpdatedStatuses: make(map[string]string),
	}
}

func (m *MockMigrationRepository) FindManualJobs(_ context.Context, limit int) ([]*domain.Job, error) {
	if limit > len(m.ManualJobs) {
		limit = len(m.ManualJobs)
	}
	return m.ManualJobs[:limit], nil
}

func (m *MockMigrationRepository) UpdateMigrationStatus(_ context.Context, jobID, status string) error {
	m.UpdatedStatuses[jobID] = status
	return nil
}

func (m *MockMigrationRepository) UpsertAutoManaged(_ context.Context, _ *domain.Job) error {
	return nil
}

func (m *MockMigrationRepository) CountByMigrationStatus(_ context.Context) (map[string]int, error) {
	counts := make(map[string]int)
	for _, status := range m.UpdatedStatuses {
		counts[status]++
	}
	counts["pending"] = len(m.ManualJobs) - len(m.UpdatedStatuses)
	return counts, nil
}

func TestMigrator_MigrateJob_SourceExists(t *testing.T) {
	t.Helper()

	repo := NewMockMigrationRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()
	migrator := job.NewMigrator(repo, sourceClient, scheduleComputer, nil)

	sourceID := uuid.New()
	jobID := uuid.New().String()

	// Add source to mock
	sourceClient.sources[sourceID] = &sources.Source{
		ID:        sourceID,
		Name:      "Test Source",
		URL:       "https://example.com",
		RateLimit: 10,
		MaxDepth:  2,
		Enabled:   true,
		Priority:  "normal",
	}

	// Add manual job to migrate
	repo.ManualJobs = append(repo.ManualJobs, &domain.Job{
		ID:          jobID,
		SourceID:    sourceID.String(),
		URL:         "https://example.com",
		AutoManaged: false,
	})

	result, err := migrator.MigrateJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Migrated != 1 {
		t.Errorf("expected 1 migrated, got %d", result.Migrated)
	}

	if repo.UpdatedStatuses[jobID] != domain.MigrationStatusMigrated {
		t.Errorf("expected status migrated, got %s", repo.UpdatedStatuses[jobID])
	}
}

func TestMigrator_MigrateJob_SourceNotFound(t *testing.T) {
	t.Helper()

	repo := NewMockMigrationRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()
	migrator := job.NewMigrator(repo, sourceClient, scheduleComputer, nil)

	jobID := uuid.New().String()
	sourceID := uuid.New()

	// No source in mock (simulates orphaned job)
	repo.ManualJobs = append(repo.ManualJobs, &domain.Job{
		ID:          jobID,
		SourceID:    sourceID.String(),
		URL:         "https://orphaned.example.com",
		AutoManaged: false,
	})

	result, err := migrator.MigrateJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Orphaned != 1 {
		t.Errorf("expected 1 orphaned, got %d", result.Orphaned)
	}

	if repo.UpdatedStatuses[jobID] != domain.MigrationStatusOrphaned {
		t.Errorf("expected status orphaned, got %s", repo.UpdatedStatuses[jobID])
	}
}

func TestMigrator_MigrateJob_SourceDisabled(t *testing.T) {
	t.Helper()

	repo := NewMockMigrationRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()
	migrator := job.NewMigrator(repo, sourceClient, scheduleComputer, nil)

	sourceID := uuid.New()
	jobID := uuid.New().String()

	// Add disabled source
	sourceClient.sources[sourceID] = &sources.Source{
		ID:       sourceID,
		Name:     "Disabled Source",
		URL:      "https://disabled.example.com",
		Enabled:  false,
		Priority: "normal",
	}

	repo.ManualJobs = append(repo.ManualJobs, &domain.Job{
		ID:          jobID,
		SourceID:    sourceID.String(),
		URL:         "https://disabled.example.com",
		AutoManaged: false,
		Status:      "pending",
	})

	result, err := migrator.MigrateJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still migrate, but job should be paused
	if result.Migrated != 1 {
		t.Errorf("expected 1 migrated, got %d", result.Migrated)
	}
}
