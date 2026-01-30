// crawler/internal/job/migrator.go
package job

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// MigrationRepository defines the interface for migration-related job operations.
type MigrationRepository interface {
	FindManualJobs(ctx context.Context, limit int) ([]*domain.Job, error)
	UpdateMigrationStatus(ctx context.Context, jobID string, status string) error
	UpsertAutoManaged(ctx context.Context, job *domain.Job) error
	CountByMigrationStatus(ctx context.Context) (map[string]int, error)
}

// MigrationResult holds the results of a migration operation.
type MigrationResult struct {
	Processed int      `json:"processed"`
	Migrated  int      `json:"migrated"`
	Orphaned  int      `json:"orphaned"`
	Skipped   int      `json:"skipped"`
	Errors    []string `json:"errors,omitempty"`
}

// Migrator handles Phase 3 migration of manual jobs to auto-managed.
type Migrator struct {
	repo             MigrationRepository
	sourceClient     sources.Client
	scheduleComputer *ScheduleComputer
	log              infralogger.Logger
}

// NewMigrator creates a new job migrator.
func NewMigrator(
	repo MigrationRepository,
	sourceClient sources.Client,
	scheduleComputer *ScheduleComputer,
	log infralogger.Logger,
) *Migrator {
	return &Migrator{
		repo:             repo,
		sourceClient:     sourceClient,
		scheduleComputer: scheduleComputer,
		log:              log,
	}
}

// MigrateJobs migrates manual jobs to auto-managed in batches.
func (m *Migrator) MigrateJobs(ctx context.Context, batchSize int) (*MigrationResult, error) {
	jobs, err := m.repo.FindManualJobs(ctx, batchSize)
	if err != nil {
		return nil, fmt.Errorf("find manual jobs: %w", err)
	}

	result := &MigrationResult{
		Processed: len(jobs),
		Errors:    make([]string, 0),
	}

	for _, j := range jobs {
		migrateErr := m.migrateJob(ctx, j, result)
		if migrateErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("job %s: %v", j.ID, migrateErr))
		}
	}

	m.logInfo("Migration batch complete",
		infralogger.Int("processed", result.Processed),
		infralogger.Int("migrated", result.Migrated),
		infralogger.Int("orphaned", result.Orphaned),
		infralogger.Int("skipped", result.Skipped),
	)

	return result, nil
}

// migrateJob migrates a single job.
func (m *Migrator) migrateJob(ctx context.Context, j *domain.Job, result *MigrationResult) error {
	// Parse source ID
	sourceID, parseErr := uuid.Parse(j.SourceID)
	if parseErr != nil {
		// Invalid source ID - mark as orphaned
		if updateErr := m.repo.UpdateMigrationStatus(ctx, j.ID, domain.MigrationStatusOrphaned); updateErr != nil {
			return fmt.Errorf("update status: %w", updateErr)
		}
		result.Orphaned++
		return nil
	}

	// Fetch source from source-manager
	source, fetchErr := m.sourceClient.GetSource(ctx, sourceID)
	if fetchErr != nil {
		if errors.Is(fetchErr, sources.ErrSourceNotFound) {
			// Source doesn't exist - mark as orphaned
			if updateErr := m.repo.UpdateMigrationStatus(ctx, j.ID, domain.MigrationStatusOrphaned); updateErr != nil {
				return fmt.Errorf("update status: %w", updateErr)
			}
			result.Orphaned++
			m.logInfo("Job marked as orphaned - source not found",
				infralogger.String("job_id", j.ID),
				infralogger.String("source_id", j.SourceID),
			)
			return nil
		}
		return fmt.Errorf("fetch source: %w", fetchErr)
	}

	// Compute schedule from source metadata
	schedule := m.scheduleComputer.ComputeSchedule(ScheduleInput{
		RateLimit: source.RateLimit,
		MaxDepth:  source.MaxDepth,
		Priority:  source.Priority,
	})

	// Update job to auto-managed
	j.AutoManaged = true
	j.Priority = schedule.NumericPriority
	j.IntervalMinutes = &schedule.IntervalMinutes
	j.IntervalType = schedule.IntervalType
	j.SourceName = &source.Name

	// If source is disabled, pause the job
	if !source.Enabled {
		j.Status = "paused"
	}

	// Upsert the updated job
	if upsertErr := m.repo.UpsertAutoManaged(ctx, j); upsertErr != nil {
		return fmt.Errorf("upsert auto-managed: %w", upsertErr)
	}

	// Mark as migrated
	if updateErr := m.repo.UpdateMigrationStatus(ctx, j.ID, domain.MigrationStatusMigrated); updateErr != nil {
		return fmt.Errorf("update status: %w", updateErr)
	}

	result.Migrated++
	m.logInfo("Job migrated to auto-managed",
		infralogger.String("job_id", j.ID),
		infralogger.String("source_name", source.Name),
		infralogger.Int("interval_minutes", schedule.IntervalMinutes),
	)

	return nil
}

// GetMigrationStats returns current migration statistics.
func (m *Migrator) GetMigrationStats(ctx context.Context) (map[string]int, error) {
	return m.repo.CountByMigrationStatus(ctx)
}

// logInfo logs at info level if logger is not nil.
func (m *Migrator) logInfo(msg string, fields ...infralogger.Field) {
	if m.log != nil {
		m.log.Info(msg, fields...)
	}
}
