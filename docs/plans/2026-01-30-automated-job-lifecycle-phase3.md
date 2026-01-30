# Automated Job Lifecycle - Phase 3 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate existing manual jobs to auto-managed jobs and deprecate manual job creation via API.

**Architecture:** A Migrator service identifies jobs with `auto_managed=false` or `NULL`, validates the corresponding source exists in source-manager, and converts them to auto-managed with computed schedules. The manual job creation endpoint (`POST /api/v1/jobs`) receives deprecation warnings first, then is disabled in a future version. Orphaned jobs (no matching source) are marked deprecated, not deleted.

**Tech Stack:** Go 1.24+, PostgreSQL, HTTP client for source-manager API

---

## Prerequisites

- Phase 2 complete (EventService, ScheduleComputer, auto_managed fields)
- All Phase 2 migrations applied (009_add_auto_managed_jobs)
- Tests passing, linter clean

---

## Task 1: Create Migration Status Column

**Files:**
- Create: `crawler/migrations/010_add_job_migration_status.up.sql`
- Create: `crawler/migrations/010_add_job_migration_status.down.sql`

### Step 1: Create up migration

```sql
-- crawler/migrations/010_add_job_migration_status.up.sql
-- Migration: Add migration status tracking for Phase 3
-- Description: Track migration state for gradual job conversion

BEGIN;

-- Add column to track migration status
-- Values: NULL (not migrated), 'migrated', 'orphaned', 'skipped'
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS migration_status VARCHAR(20);

-- Index for efficient migration queries
CREATE INDEX IF NOT EXISTS idx_jobs_migration_status
    ON jobs (migration_status)
    WHERE migration_status IS NOT NULL;

-- Documentation
COMMENT ON COLUMN jobs.migration_status IS 'Phase 3 migration status: migrated, orphaned, or skipped';

COMMIT;
```

### Step 2: Create down migration

```sql
-- crawler/migrations/010_add_job_migration_status.down.sql
-- Migration: Remove migration status column
-- Description: Rollback for 010_add_job_migration_status.up.sql

BEGIN;

DROP INDEX IF EXISTS idx_jobs_migration_status;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS migration_status;

COMMIT;
```

### Step 3: Verify migration syntax

Run: `cat crawler/migrations/010_add_job_migration_status.up.sql`
Expected: Valid SQL syntax displayed

### Step 4: Commit

```bash
git add crawler/migrations/010_add_job_migration_status.up.sql crawler/migrations/010_add_job_migration_status.down.sql
git commit -m "feat(crawler): add migration status column for Phase 3

Track job migration state: migrated, orphaned, or skipped.
Supports gradual conversion of manual jobs to auto-managed.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: Add MigrationStatus to Job Domain Model

**Files:**
- Modify: `crawler/internal/domain/job.go`

### Step 1: Add MigrationStatus field to Job struct

Add this field after the BackoffUntil field:

```go
// Phase 3 migration tracking
MigrationStatus *string `db:"migration_status" json:"migration_status,omitempty"`
```

### Step 2: Add constants for migration status values

Add these constants after the existing job status constants:

```go
// Migration status values (Phase 3)
const (
	MigrationStatusMigrated = "migrated" // Successfully converted to auto-managed
	MigrationStatusOrphaned = "orphaned" // Source not found, marked for review
	MigrationStatusSkipped  = "skipped"  // Intentionally left as manual
)
```

### Step 3: Verify the changes compile

Run: `cd crawler && go build ./...`
Expected: Build succeeds

### Step 4: Commit

```bash
git add crawler/internal/domain/job.go
git commit -m "feat(crawler): add MigrationStatus field to Job domain model

Support Phase 3 migration tracking with status values:
- migrated: Successfully converted to auto-managed
- orphaned: Source not found
- skipped: Intentionally left manual

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: Extend JobRepository for Migration Queries

**Files:**
- Modify: `crawler/internal/database/job_repository.go`
- Create: `crawler/internal/database/job_repository_migration_test.go`

### Step 1: Write failing tests for new repository methods

```go
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
```

### Step 2: Run tests to verify they fail

Run: `cd crawler && go test ./internal/database/... -run TestJobRepository_.*Migration.* -v`
Expected: FAIL with compile errors (methods don't exist)

### Step 3: Add repository methods

Add these methods to `crawler/internal/database/job_repository.go`:

```go
// FindManualJobs returns jobs that are not auto-managed and need migration.
// Excludes already-migrated jobs.
func (r *JobRepository) FindManualJobs(ctx context.Context, limit int) ([]*domain.Job, error) {
	query := `
		SELECT id, source_id, source_name, url, interval_minutes, interval_type,
		       cron_expression, schedule_time, next_run_at, last_run_at, status,
		       is_paused, max_retries, retry_count, retry_backoff_seconds, metadata,
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
func (r *JobRepository) UpdateMigrationStatus(ctx context.Context, jobID string, status string) error {
	query := `
		UPDATE jobs
		SET migration_status = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := r.db.ExecContext(ctx, query, status, jobID)
	if err != nil {
		return fmt.Errorf("update migration status: %w", err)
	}

	rowsAffected, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return fmt.Errorf("get rows affected: %w", affectedErr)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", jobID)
	}

	return nil
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
```

### Step 4: Run tests to verify they pass

Run: `cd crawler && go test ./internal/database/... -run TestJobRepository_.*Migration.* -v`
Expected: PASS

### Step 5: Commit

```bash
git add crawler/internal/database/job_repository.go crawler/internal/database/job_repository_migration_test.go
git commit -m "feat(crawler): add migration query methods to JobRepository

Add FindManualJobs, UpdateMigrationStatus, and CountByMigrationStatus
for Phase 3 job migration support.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: Create Migrator Service

**Files:**
- Create: `crawler/internal/job/migrator.go`
- Create: `crawler/internal/job/migrator_test.go`

### Step 1: Write failing tests

```go
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
	ManualJobs []*domain.Job
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

func (m *MockMigrationRepository) UpdateMigrationStatus(_ context.Context, jobID string, status string) error {
	m.UpdatedStatuses[jobID] = status
	return nil
}

func (m *MockMigrationRepository) UpsertAutoManaged(_ context.Context, j *domain.Job) error {
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
	sourceClient.Sources[sourceID] = &sources.Source{
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
	sourceClient.Sources[sourceID] = &sources.Source{
		ID:        sourceID,
		Name:      "Disabled Source",
		URL:       "https://disabled.example.com",
		Enabled:   false,
		Priority:  "normal",
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
```

### Step 2: Run tests to verify they fail

Run: `cd crawler && go test ./internal/job/... -run TestMigrator -v`
Expected: FAIL with "undefined: job.NewMigrator"

### Step 3: Write implementation

```go
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

	for _, job := range jobs {
		migrateErr := m.migrateJob(ctx, job, result)
		if migrateErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("job %s: %v", job.ID, migrateErr))
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
func (m *Migrator) migrateJob(ctx context.Context, job *domain.Job, result *MigrationResult) error {
	// Parse source ID
	sourceID, parseErr := uuid.Parse(job.SourceID)
	if parseErr != nil {
		// Invalid source ID - mark as orphaned
		if updateErr := m.repo.UpdateMigrationStatus(ctx, job.ID, domain.MigrationStatusOrphaned); updateErr != nil {
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
			if updateErr := m.repo.UpdateMigrationStatus(ctx, job.ID, domain.MigrationStatusOrphaned); updateErr != nil {
				return fmt.Errorf("update status: %w", updateErr)
			}
			result.Orphaned++
			m.logInfo("Job marked as orphaned - source not found",
				infralogger.String("job_id", job.ID),
				infralogger.String("source_id", job.SourceID),
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
	job.AutoManaged = true
	job.Priority = schedule.NumericPriority
	job.IntervalMinutes = &schedule.IntervalMinutes
	job.IntervalType = schedule.IntervalType
	job.SourceName = &source.Name

	// If source is disabled, pause the job
	if !source.Enabled {
		job.Status = "paused"
	}

	// Upsert the updated job
	if upsertErr := m.repo.UpsertAutoManaged(ctx, job); upsertErr != nil {
		return fmt.Errorf("upsert auto-managed: %w", upsertErr)
	}

	// Mark as migrated
	if updateErr := m.repo.UpdateMigrationStatus(ctx, job.ID, domain.MigrationStatusMigrated); updateErr != nil {
		return fmt.Errorf("update status: %w", updateErr)
	}

	result.Migrated++
	m.logInfo("Job migrated to auto-managed",
		infralogger.String("job_id", job.ID),
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
```

### Step 4: Run tests to verify they pass

Run: `cd crawler && go test ./internal/job/... -run TestMigrator -v`
Expected: PASS (all 3 tests)

### Step 5: Run linter

Run: `cd crawler && golangci-lint run ./internal/job/...`
Expected: 0 issues

### Step 6: Commit

```bash
git add crawler/internal/job/migrator.go crawler/internal/job/migrator_test.go
git commit -m "feat(crawler): add Migrator service for Phase 3 job migration

Migrate manual jobs to auto-managed with computed schedules:
- Validates source exists in source-manager
- Marks orphaned jobs (no source) for review
- Computes schedule from source metadata
- Pauses jobs for disabled sources

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: Add Migration API Endpoint

**Files:**
- Create: `crawler/internal/api/migration_handler.go`
- Create: `crawler/internal/api/migration_handler_test.go`
- Modify: `crawler/cmd/httpd/httpd.go`

### Step 1: Write failing tests

```go
// crawler/internal/api/migration_handler_test.go
package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
)

// MockMigrator implements the migrator interface for testing.
type MockMigrator struct {
	MigrateResult *job.MigrationResult
	Stats         map[string]int
}

func (m *MockMigrator) MigrateJobs(_ context.Context, _ int) (*job.MigrationResult, error) {
	return m.MigrateResult, nil
}

func (m *MockMigrator) GetMigrationStats(_ context.Context) (map[string]int, error) {
	return m.Stats, nil
}

func TestMigrationHandler_RunMigration(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	migrator := &MockMigrator{
		MigrateResult: &job.MigrationResult{
			Processed: 5,
			Migrated:  3,
			Orphaned:  2,
		},
	}

	handler := api.NewMigrationHandler(migrator, nil)
	router.POST("/api/v1/jobs/migrate", handler.RunMigration)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/migrate", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMigrationHandler_GetStats(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	migrator := &MockMigrator{
		Stats: map[string]int{
			"pending":  10,
			"migrated": 5,
			"orphaned": 2,
		},
	}

	handler := api.NewMigrationHandler(migrator, nil)
	router.GET("/api/v1/jobs/migration-stats", handler.GetStats)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/migration-stats", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
```

### Step 2: Run tests to verify they fail

Run: `cd crawler && go test ./internal/api/... -run TestMigrationHandler -v`
Expected: FAIL with "undefined: api.NewMigrationHandler"

### Step 3: Write implementation

```go
// crawler/internal/api/migration_handler.go
package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Default migration batch size.
const defaultMigrationBatchSize = 100

// MigratorInterface defines the interface for job migration operations.
type MigratorInterface interface {
	MigrateJobs(ctx context.Context, batchSize int) (*job.MigrationResult, error)
	GetMigrationStats(ctx context.Context) (map[string]int, error)
}

// MigrationHandler handles Phase 3 migration endpoints.
type MigrationHandler struct {
	migrator MigratorInterface
	log      infralogger.Logger
}

// NewMigrationHandler creates a new migration handler.
func NewMigrationHandler(migrator MigratorInterface, log infralogger.Logger) *MigrationHandler {
	return &MigrationHandler{
		migrator: migrator,
		log:      log,
	}
}

// RunMigration runs a batch of job migrations.
// POST /api/v1/jobs/migrate?batch_size=100
func (h *MigrationHandler) RunMigration(c *gin.Context) {
	batchSize := defaultMigrationBatchSize
	if batchStr := c.Query("batch_size"); batchStr != "" {
		parsed, parseErr := strconv.Atoi(batchStr)
		if parseErr == nil && parsed > 0 {
			batchSize = parsed
		}
	}

	result, err := h.migrator.MigrateJobs(c.Request.Context(), batchSize)
	if err != nil {
		h.logError("Migration failed", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "migration failed",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetStats returns current migration statistics.
// GET /api/v1/jobs/migration-stats
func (h *MigrationHandler) GetStats(c *gin.Context) {
	stats, err := h.migrator.GetMigrationStats(c.Request.Context())
	if err != nil {
		h.logError("Failed to get migration stats", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get stats",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"migration_status": stats,
	})
}

// logError logs at error level if logger is not nil.
func (h *MigrationHandler) logError(msg string, fields ...infralogger.Field) {
	if h.log != nil {
		h.log.Error(msg, fields...)
	}
}
```

### Step 4: Run tests to verify they pass

Run: `cd crawler && go test ./internal/api/... -run TestMigrationHandler -v`
Expected: PASS

### Step 5: Commit

```bash
git add crawler/internal/api/migration_handler.go crawler/internal/api/migration_handler_test.go
git commit -m "feat(crawler): add migration API endpoints

Add POST /api/v1/jobs/migrate for batch migration
Add GET /api/v1/jobs/migration-stats for progress tracking

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: Wire Up Migration Handler in httpd.go

**Files:**
- Modify: `crawler/cmd/httpd/httpd.go`

### Step 1: Add setupMigrator function

Add this function after setupEventConsumer:

```go
// setupMigrator creates the migrator service for Phase 3 job migration.
func setupMigrator(deps *CommandDeps, jobRepo *database.JobRepository) *job.Migrator {
	sourceManagerCfg := deps.Config.GetSourceManagerConfig()
	sourceClient := sources.NewHTTPClient(sourceManagerCfg.URL, nil)
	scheduleComputer := job.NewScheduleComputer()

	return job.NewMigrator(jobRepo, sourceClient, scheduleComputer, deps.Logger)
}
```

### Step 2: Update JobsSchedulerResult to include Migrator

Add to the struct:

```go
Migrator *job.Migrator
```

### Step 3: Update setupJobsAndScheduler to create migrator

In setupJobsAndScheduler, after creating jobRepo, add:

```go
migrator := setupMigrator(deps, jobRepo)
```

And include it in the return:

```go
return &JobsSchedulerResult{
	// ... existing fields
	Migrator: migrator,
}, nil
```

### Step 4: Register migration routes

In the route setup section (after jobs routes), add:

```go
// Migration routes (Phase 3)
migrationHandler := api.NewMigrationHandler(jsResult.Migrator, deps.Logger)
apiV1.POST("/jobs/migrate", migrationHandler.RunMigration)
apiV1.GET("/jobs/migration-stats", migrationHandler.GetStats)
```

### Step 5: Verify build

Run: `cd crawler && go build ./...`
Expected: Build succeeds

### Step 6: Run linter

Run: `cd crawler && golangci-lint run ./...`
Expected: 0 issues

### Step 7: Commit

```bash
git add crawler/cmd/httpd/httpd.go
git commit -m "feat(crawler): wire up migration handler in httpd.go

Register Phase 3 migration endpoints:
- POST /api/v1/jobs/migrate
- GET /api/v1/jobs/migration-stats

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: Add Deprecation Warning to Manual Job Creation

**Files:**
- Modify: `crawler/internal/api/jobs_handler.go`

### Step 1: Add deprecation header to CreateJob

In the CreateJob handler, add a deprecation warning header at the start of successful responses:

```go
// Add deprecation warning header
c.Header("Deprecation", "true")
c.Header("Sunset", "2026-06-01")
c.Header("X-Deprecation-Notice", "POST /api/v1/jobs is deprecated. Use source-manager to create sources; jobs are created automatically.")
```

### Step 2: Add deprecation log message

After successful job creation, log a deprecation warning:

```go
h.logger.Warn("Deprecated API usage: manual job creation",
	infralogger.String("job_id", newJob.ID),
	infralogger.String("source_id", req.SourceID),
)
```

### Step 3: Verify build

Run: `cd crawler && go build ./...`
Expected: Build succeeds

### Step 4: Run linter

Run: `cd crawler && golangci-lint run ./internal/api/...`
Expected: 0 issues

### Step 5: Commit

```bash
git add crawler/internal/api/jobs_handler.go
git commit -m "feat(crawler): add deprecation warning to manual job creation

Add HTTP headers and logging for deprecated POST /api/v1/jobs endpoint.
Sunset date: 2026-06-01

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: Integration Test Documentation

**Files:**
- Create: `docs/plans/phase3-testing.md`

### Step 1: Create Phase 3 test documentation

```markdown
# Phase 3 Integration Test Results

## Date: YYYY-MM-DD

## Test: Job Migration to Auto-Managed

### Prerequisites

1. Phase 2 complete and all migrations applied
2. Redis running with source events enabled
3. PostgreSQL running
4. source-manager running
5. crawler running with `REDIS_EVENTS_ENABLED=true`

---

### Test 1: Migration of Job with Valid Source

1. Create a manual job (using deprecated endpoint):
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "{existing-source-uuid}",
    "url": "https://example.com",
    "interval_minutes": 60,
    "schedule_enabled": true
  }'
```

2. Note the deprecation headers in response

3. Run migration:
```bash
curl -X POST http://localhost:8060/api/v1/jobs/migrate?batch_size=10
```

4. Verify job is now auto-managed:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id} | jq '.auto_managed, .migration_status'
```

**Expected:**
- [ ] `auto_managed = true`
- [ ] `migration_status = "migrated"`
- [ ] `interval_minutes` computed from source metadata

---

### Test 2: Migration of Orphaned Job

1. Create a job with non-existent source ID:
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "00000000-0000-0000-0000-000000000000",
    "url": "https://orphan.example.com",
    "interval_minutes": 120
  }'
```

2. Run migration:
```bash
curl -X POST http://localhost:8060/api/v1/jobs/migrate?batch_size=10
```

3. Check migration status:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id} | jq '.migration_status'
```

**Expected:**
- [ ] `migration_status = "orphaned"`
- [ ] `auto_managed` unchanged (still false)

---

### Test 3: Migration Stats

1. Check migration progress:
```bash
curl http://localhost:8060/api/v1/jobs/migration-stats
```

**Expected:**
```json
{
  "migration_status": {
    "pending": N,
    "migrated": N,
    "orphaned": N
  }
}
```

---

### Test 4: Deprecation Headers

1. Create a job via deprecated endpoint:
```bash
curl -v -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{...}'
```

**Expected Headers:**
- [ ] `Deprecation: true`
- [ ] `Sunset: 2026-06-01`
- [ ] `X-Deprecation-Notice: ...`

---

### Test 5: Idempotent Migration

1. Run migration twice:
```bash
curl -X POST http://localhost:8060/api/v1/jobs/migrate
curl -X POST http://localhost:8060/api/v1/jobs/migrate
```

2. Verify already-migrated jobs aren't reprocessed:
```bash
curl http://localhost:8060/api/v1/jobs/migration-stats
```

**Expected:**
- [ ] Second run shows 0 processed (all already migrated)

---

## Verification Commands

```bash
# Check manual jobs remaining
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler \
  -c "SELECT COUNT(*) FROM jobs WHERE auto_managed = false OR auto_managed IS NULL;"

# Check migration status distribution
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler \
  -c "SELECT COALESCE(migration_status, 'pending') as status, COUNT(*) FROM jobs GROUP BY migration_status;"

# List orphaned jobs
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler \
  -c "SELECT id, source_id, url FROM jobs WHERE migration_status = 'orphaned';"
```

---

## Notes

Phase 3 migrates existing manual jobs to auto-managed without data loss:
- Valid sources: Jobs become auto-managed with computed schedules
- Missing sources: Jobs marked "orphaned" for operator review
- Already migrated: Skipped in subsequent runs (idempotent)

Next steps:
1. Review orphaned jobs and either delete or link to valid sources
2. After all jobs migrated, disable `POST /api/v1/jobs` endpoint (Phase 4)
```

### Step 2: Commit

```bash
git add docs/plans/phase3-testing.md
git commit -m "docs: add Phase 3 integration test documentation

Document migration testing:
- Job with valid source → migrated
- Job with missing source → orphaned
- Migration stats endpoint
- Deprecation headers
- Idempotent migration

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Phase 3 Complete - Summary

### Files Created
- `crawler/migrations/010_add_job_migration_status.up.sql`
- `crawler/migrations/010_add_job_migration_status.down.sql`
- `crawler/internal/job/migrator.go`
- `crawler/internal/job/migrator_test.go`
- `crawler/internal/api/migration_handler.go`
- `crawler/internal/api/migration_handler_test.go`
- `crawler/internal/database/job_repository_migration_test.go`
- `docs/plans/phase3-testing.md`

### Files Modified
- `crawler/internal/domain/job.go` - Added MigrationStatus field
- `crawler/internal/database/job_repository.go` - Added migration query methods
- `crawler/internal/api/jobs_handler.go` - Added deprecation warning
- `crawler/cmd/httpd/httpd.go` - Wired up migration handler

### Success Criteria
- [ ] Manual jobs can be migrated to auto-managed
- [ ] Orphaned jobs (no source) are marked, not deleted
- [ ] Migration is idempotent (safe to run multiple times)
- [ ] Deprecation warnings on manual job creation
- [ ] Migration stats endpoint for progress tracking
- [ ] All existing tests pass
- [ ] Linter passes with 0 issues

---

**Next Phase:** Phase 4 will disable manual job creation endpoint and require all jobs to be auto-managed.
