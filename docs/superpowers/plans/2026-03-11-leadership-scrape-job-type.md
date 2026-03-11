# Leadership Scrape Job Type Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `leadership_scrape` job type to the crawler's interval scheduler so leadership scraping runs automatically alongside regular crawl jobs.

**Architecture:** Add a `type` column to the jobs table (defaulting to `"crawl"` for backward compatibility). Extend the domain model, database repository SQL, API request types, and scheduler's `runJob()` dispatcher to route `leadership_scrape` jobs to the existing `scraper.Scraper` instead of the crawler. The scraper gets its config (source-manager URL, JWT token) from the app config already available in bootstrap.

**Tech Stack:** Go 1.23, PostgreSQL (golang-migrate), gin HTTP framework, existing `internal/scraper` package.

**GitHub Issue:** waaseyaa/minoo#188

**Design decision — single leadership_scrape job:** The `jobs` table has a UNIQUE constraint on `source_id`. Leadership scrape jobs use a placeholder `source_id = "leadership-scrape"`, meaning only one leadership_scrape job can exist at a time. This is intentional — the scraper already processes all communities with linked sources in a single run.

---

## Chunk 1: Schema + Domain Model + Database Repository

### Task 1: Add `type` column migration

**Files:**
- Create: `crawler/migrations/021_add_job_type.up.sql`
- Create: `crawler/migrations/021_add_job_type.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- Add job type column to distinguish crawl vs leadership_scrape jobs.
-- Default 'crawl' ensures all existing jobs remain unchanged.
ALTER TABLE jobs ADD COLUMN type VARCHAR(50) NOT NULL DEFAULT 'crawl';

-- Index for filtering jobs by type
CREATE INDEX idx_jobs_type ON jobs(type);
```

- [ ] **Step 2: Write the down migration**

```sql
DROP INDEX IF EXISTS idx_jobs_type;
ALTER TABLE jobs DROP COLUMN IF EXISTS type;
```

- [ ] **Step 3: Commit**

```bash
git add crawler/migrations/021_add_job_type.up.sql crawler/migrations/021_add_job_type.down.sql
git commit -m "feat(#188): add job type column migration"
```

---

### Task 2: Add `Type` field to domain model

**Files:**
- Modify: `crawler/internal/domain/job.go:9-15` (add Type field to Job struct)
- Create: `crawler/internal/domain/job_test.go`

- [ ] **Step 1: Add the Type field**

Add `Type` field after the `URL` field (line 14) in the `Job` struct:

```go
Type   string  `db:"type"        json:"type"`
```

- [ ] **Step 2: Add job type constants**

Add after the migration status constants (after line 72):

```go
// Job type values
const (
	JobTypeCrawl            = "crawl"
	JobTypeLeadershipScrape = "leadership_scrape"
)

// ValidJobType returns true if the given type is a known job type.
func ValidJobType(t string) bool {
	return t == JobTypeCrawl || t == JobTypeLeadershipScrape
}
```

- [ ] **Step 3: Write unit test**

Create `crawler/internal/domain/job_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestValidJobType(t *testing.T) {
	tests := []struct {
		name     string
		jobType  string
		expected bool
	}{
		{"crawl is valid", domain.JobTypeCrawl, true},
		{"leadership_scrape is valid", domain.JobTypeLeadershipScrape, true},
		{"empty is invalid", "", false},
		{"unknown is invalid", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.ValidJobType(tt.jobType); got != tt.expected {
				t.Errorf("ValidJobType(%q) = %v, want %v", tt.jobType, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 4: Run test**

Run: `cd /home/fsd42/dev/north-cloud && go test ./crawler/internal/domain/ -v -run TestValidJobType`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add crawler/internal/domain/job.go crawler/internal/domain/job_test.go
git commit -m "feat(#188): add Type field and constants to Job domain model"
```

---

### Task 3: Add `type` to database repository SQL

**Files:**
- Modify: `crawler/internal/database/job_repository.go`

The `type` column must appear in all SQL constants and queries that read/write jobs. Without this, the field is silently ignored.

- [ ] **Step 1: Add `type` to `jobInsertColumns`**

Change (line 21-25):

```go
const jobInsertColumns = `id, source_id, source_name, url,
	schedule_time, schedule_enabled,
	interval_minutes, interval_type,
	is_paused, max_retries, retry_backoff_seconds,
	status, metadata`
```

to:

```go
const jobInsertColumns = `id, source_id, source_name, url, type,
	schedule_time, schedule_enabled,
	interval_minutes, interval_type,
	is_paused, max_retries, retry_backoff_seconds,
	status, metadata`
```

- [ ] **Step 2: Add `type` to `jobSelectBase`**

Change (line 28-36):

```go
const jobSelectBase = `id, source_id, source_name, url,
	schedule_time, schedule_enabled,
	interval_minutes, interval_type, next_run_at,
	is_paused, max_retries, retry_backoff_seconds, current_retry_count,
	lock_token, lock_acquired_at,
	status, scheduler_version,
	created_at, updated_at, started_at, completed_at,
	paused_at, cancelled_at,
	error_message, metadata`
```

to:

```go
const jobSelectBase = `id, source_id, source_name, url, type,
	schedule_time, schedule_enabled,
	interval_minutes, interval_type, next_run_at,
	is_paused, max_retries, retry_backoff_seconds, current_retry_count,
	lock_token, lock_acquired_at,
	status, scheduler_version,
	created_at, updated_at, started_at, completed_at,
	paused_at, cancelled_at,
	error_message, metadata`
```

- [ ] **Step 3: Update `Create` method — add `type` parameter**

The VALUES now has 14 params instead of 13. Update (lines 53-81):

```go
func (r *JobRepository) Create(ctx context.Context, job *domain.Job) error {
	query := `INSERT INTO jobs (` + jobInsertColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING created_at, updated_at, next_run_at`

	err := r.db.QueryRowContext(
		ctx,
		query,
		job.ID,
		job.SourceID,
		job.SourceName,
		job.URL,
		job.Type,
		job.ScheduleTime,
		job.ScheduleEnabled,
		job.IntervalMinutes,
		job.IntervalType,
		job.IsPaused,
		job.MaxRetries,
		job.RetryBackoffSeconds,
		job.Status,
		domain.MetadataPtr(job.Metadata),
	).Scan(&job.CreatedAt, &job.UpdatedAt, &job.NextRunAt)

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Update `CreateOrUpdate` method — add `type` parameter + ON CONFLICT SET**

The VALUES now has 14 params. Add `type = EXCLUDED.type` to the ON CONFLICT SET clause. Update (lines 85-133):

```go
func (r *JobRepository) CreateOrUpdate(ctx context.Context, job *domain.Job) (bool, error) {
	query := `INSERT INTO jobs (` + jobInsertColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (source_id) DO UPDATE SET
			source_name = EXCLUDED.source_name,
			url = EXCLUDED.url,
			type = EXCLUDED.type,
			schedule_time = EXCLUDED.schedule_time,
			schedule_enabled = EXCLUDED.schedule_enabled,
			interval_minutes = EXCLUDED.interval_minutes,
			interval_type = EXCLUDED.interval_type,
			is_paused = EXCLUDED.is_paused,
			max_retries = EXCLUDED.max_retries,
			retry_backoff_seconds = EXCLUDED.retry_backoff_seconds,
			status = CASE
				WHEN jobs.status = 'running' THEN jobs.status
				ELSE EXCLUDED.status
			END,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id, created_at, updated_at, next_run_at
	`

	originalID := job.ID
	err := r.db.QueryRowContext(
		ctx,
		query,
		job.ID,
		job.SourceID,
		job.SourceName,
		job.URL,
		job.Type,
		job.ScheduleTime,
		job.ScheduleEnabled,
		job.IntervalMinutes,
		job.IntervalType,
		job.IsPaused,
		job.MaxRetries,
		job.RetryBackoffSeconds,
		job.Status,
		domain.MetadataPtr(job.Metadata),
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt, &job.NextRunAt)

	if err != nil {
		return false, fmt.Errorf("failed to create or update job: %w", err)
	}

	wasInserted := job.ID == originalID
	return wasInserted, nil
}
```

- [ ] **Step 5: Update `Update` method — add `type` parameter**

Bump all parameter indices by 1 after `url`. Update (lines 257-301):

```go
func (r *JobRepository) Update(ctx context.Context, job *domain.Job) error {
	query := `
		UPDATE jobs
		SET source_id = $1, source_name = $2, url = $3, type = $4,
		    schedule_time = $5, schedule_enabled = $6,
		    interval_minutes = $7, interval_type = $8, next_run_at = $9,
		    is_paused = $10, max_retries = $11, retry_backoff_seconds = $12,
		    current_retry_count = $13,
		    lock_token = $14, lock_acquired_at = $15,
		    status = $16,
		    started_at = $17, completed_at = $18,
		    paused_at = $19, cancelled_at = $20,
		    error_message = $21, metadata = $22
		WHERE id = $23
	`

	result, execErr := r.db.ExecContext(
		ctx,
		query,
		job.SourceID,
		job.SourceName,
		job.URL,
		job.Type,
		job.ScheduleTime,
		job.ScheduleEnabled,
		job.IntervalMinutes,
		job.IntervalType,
		job.NextRunAt,
		job.IsPaused,
		job.MaxRetries,
		job.RetryBackoffSeconds,
		job.CurrentRetryCount,
		job.LockToken,
		job.LockAcquiredAt,
		job.Status,
		job.StartedAt,
		job.CompletedAt,
		job.PausedAt,
		job.CancelledAt,
		job.ErrorMessage,
		domain.MetadataPtr(job.Metadata),
		job.ID,
	)

	return execRequireRows(result, execErr, fmt.Errorf("job not found: %s", job.ID))
}
```

- [ ] **Step 6: Verify build compiles**

Run: `cd /home/fsd42/dev/north-cloud && go build ./crawler/...`
Expected: Build succeeds

- [ ] **Step 7: Commit**

```bash
git add crawler/internal/database/job_repository.go
git commit -m "feat(#188): add type column to job repository SQL queries"
```

---

## Chunk 2: API Request Types + Handler Wiring

### Task 4: Add `Type` field to API request types and relax bindings

**Files:**
- Modify: `crawler/internal/api/types.go:25-44`

The `binding:"required"` on `SourceID` and `URL` must be removed now (not deferred) because leadership_scrape jobs don't need these fields. Manual validation in the handler replaces the binding.

- [ ] **Step 1: Update CreateJobRequest**

Replace lines 25-44:

```go
// CreateJobRequest represents a job creation request.
type CreateJobRequest struct {
	SourceID   string `json:"source_id"`
	SourceName string `json:"source_name"`
	URL        string `json:"url"`
	Type       string `json:"type"`

	// Interval-based scheduling (new)
	IntervalMinutes *int   `json:"interval_minutes"` // NULL = run once immediately
	IntervalType    string `json:"interval_type"`    // 'minutes', 'hours', 'days'
	ScheduleEnabled bool   `json:"schedule_enabled"`

	// Retry configuration (new)
	MaxRetries          *int `json:"max_retries"`           // Default: 3
	RetryBackoffSeconds *int `json:"retry_backoff_seconds"` // Default: 60

	// Legacy cron field (deprecated, maintained for backward compatibility)
	ScheduleTime string `json:"schedule_time"`

	// Metadata (new)
	Metadata map[string]any `json:"metadata"`
}
```

- [ ] **Step 2: Add Type to UpdateJobRequest**

Add after `URL` field (line ~50):

```go
Type       string `json:"type"`
```

- [ ] **Step 3: Commit**

```bash
git add crawler/internal/api/types.go
git commit -m "feat(#188): add Type to API request types, relax binding for leadership_scrape"
```

---

### Task 5: Wire Type field in CreateJob and UpdateJob handlers

**Files:**
- Modify: `crawler/internal/api/jobs_handler.go:148-229` (CreateJob)
- Modify: `crawler/internal/api/jobs_handler.go:232-306` (UpdateJob)
- Modify: `crawler/internal/api/jobs_handler.go:64-71` (allowedJobSortFields)

- [ ] **Step 1: Add type validation, defaults, and manual field validation in CreateJob**

In `CreateJob()`, after the `intervalType` default block (after line 172), add:

```go
// Default job type to "crawl"
jobType := domain.JobTypeCrawl
if req.Type != "" {
	if !domain.ValidJobType(req.Type) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job type: " + req.Type + ". Valid types: crawl, leadership_scrape",
		})
		return
	}
	jobType = req.Type
}

// Validate required fields per job type
if jobType == domain.JobTypeCrawl {
	if req.SourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_id is required for crawl jobs"})
		return
	}
	if req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required for crawl jobs"})
		return
	}
}

// Leadership scrape jobs use a fixed placeholder source_id (UNIQUE constraint
// on source_id means only one leadership_scrape job can exist — intentional,
// since the scraper processes all communities in a single run).
if jobType == domain.JobTypeLeadershipScrape {
	if req.SourceID == "" {
		req.SourceID = "leadership-scrape"
	}
	if req.URL == "" {
		req.URL = "leadership-scrape"
	}
}
```

In the `job := &domain.Job{...}` block (line 181-192), add `Type: jobType,` after `URL: req.URL,`.

- [ ] **Step 2: Wire Type in UpdateJob**

In `UpdateJob()`, after the URL update block (after line 262), add:

```go
if req.Type != "" {
	if !domain.ValidJobType(req.Type) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job type: " + req.Type,
		})
		return
	}
	job.Type = req.Type
}
```

- [ ] **Step 3: Add type to sort fields**

Add to `allowedJobSortFields` map:

```go
"type": "type",
```

- [ ] **Step 4: Verify build compiles**

Run: `cd /home/fsd42/dev/north-cloud && go build ./crawler/...`
Expected: Build succeeds

- [ ] **Step 5: Run existing tests**

Run: `cd /home/fsd42/dev/north-cloud && go test ./crawler/internal/api/ -v -count=1`
Expected: All existing tests PASS (the empty-body test still returns 400, now via manual validation instead of binding)

- [ ] **Step 6: Commit**

```bash
git add crawler/internal/api/jobs_handler.go
git commit -m "feat(#188): wire Type field in CreateJob and UpdateJob handlers"
```

---

## Chunk 3: Scheduler Dispatch + Scraper Integration

### Task 6: Add scraper runner to scheduler

**Files:**
- Create: `crawler/internal/scheduler/scraper_runner.go`
- Create: `crawler/internal/scheduler/scraper_runner_test.go`
- Modify: `crawler/internal/scheduler/options.go` (add WithScraperConfig)
- Modify: `crawler/internal/scheduler/interval_scheduler.go` (add scraperConfig field)

- [ ] **Step 1: Create the scraper runner**

Create `crawler/internal/scheduler/scraper_runner.go`:

```go
package scheduler

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/crawler/internal/scraper"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// ScraperConfig holds the config needed to run leadership scrape jobs.
type ScraperConfig struct {
	SourceManagerURL string
	JWTToken         string
}

// runLeadershipScrapeJob runs the leadership scraper for a scheduled job.
func runLeadershipScrapeJob(ctx context.Context, cfg ScraperConfig, logger infralogger.Logger) error {
	if cfg.SourceManagerURL == "" {
		return fmt.Errorf("leadership scrape: source-manager URL not configured")
	}

	s := scraper.New(scraper.Config{
		SourceManagerURL: cfg.SourceManagerURL,
		JWTToken:         cfg.JWTToken,
	}, logger)

	results, err := s.Run(ctx)
	if err != nil {
		return fmt.Errorf("leadership scrape: %w", err)
	}

	var totalPeople, totalOffices, totalErrors int
	for _, r := range results {
		totalPeople += r.PeopleAdded
		if r.OfficeUpdated {
			totalOffices++
		}
		if r.Error != "" {
			totalErrors++
		}
	}

	logger.Info("leadership scrape completed",
		infralogger.Int("communities_processed", len(results)),
		infralogger.Int("people_added", totalPeople),
		infralogger.Int("offices_updated", totalOffices),
		infralogger.Int("errors", totalErrors),
	)

	return nil
}
```

- [ ] **Step 2: Write unit test for missing URL**

Create `crawler/internal/scheduler/scraper_runner_test.go`:

```go
package scheduler

import (
	"context"
	"strings"
	"testing"
)

func TestRunLeadershipScrapeJob_MissingURL(t *testing.T) {
	err := runLeadershipScrapeJob(context.Background(), ScraperConfig{}, nil)
	if err == nil {
		t.Fatal("expected error for empty source-manager URL")
	}
	if !strings.Contains(err.Error(), "source-manager URL not configured") {
		t.Errorf("unexpected error: %s", err.Error())
	}
}
```

- [ ] **Step 3: Run test**

Run: `cd /home/fsd42/dev/north-cloud && go test ./crawler/internal/scheduler/ -v -run TestRunLeadershipScrapeJob_MissingURL`
Expected: PASS

- [ ] **Step 4: Add scraperConfig field to IntervalScheduler**

In `crawler/internal/scheduler/interval_scheduler.go`, add to the struct (after `logService logs.Service`, ~line 72):

```go
// Scraper config for leadership_scrape jobs
scraperConfig *ScraperConfig
```

- [ ] **Step 5: Add WithScraperConfig option**

In `crawler/internal/scheduler/options.go`, add:

```go
// WithScraperConfig sets the scraper configuration for leadership_scrape jobs.
func WithScraperConfig(cfg ScraperConfig) SchedulerOption {
	return func(s *IntervalScheduler) {
		s.scraperConfig = &cfg
	}
}
```

- [ ] **Step 6: Commit**

```bash
git add crawler/internal/scheduler/scraper_runner.go crawler/internal/scheduler/scraper_runner_test.go crawler/internal/scheduler/interval_scheduler.go crawler/internal/scheduler/options.go
git commit -m "feat(#188): add scraper runner and ScraperConfig option for scheduler"
```

---

### Task 7: Add job type dispatch in runJob

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go:634-708` (runJob method)

Refactor `runJob()` into a dispatcher that calls `runCrawlJob()` or `runLeadershipJob()` based on `job.Type`. The cleanup defer and panic recovery MUST remain in the outer `runJob()` — sub-methods must NOT duplicate them.

- [ ] **Step 1: Replace runJob with dispatcher + extracted runCrawlJob**

Replace the `runJob` method (lines 634-708) with:

```go
// runJob dispatches job execution by type.
func (s *IntervalScheduler) runJob(jobExec *JobExecution) {
	job := jobExec.Job
	execution := jobExec.Execution

	// Panic recovery registered first — runs last (LIFO) after cleanup defer.
	defer s.recoverFromPanic(job, execution)

	logWriter := s.startLogCapture(jobExec)

	defer func() {
		s.stopLogCapture(jobExec, logWriter)
		s.activeJobsMu.Lock()
		delete(s.activeJobs, job.ID)
		s.activeJobsMu.Unlock()
		s.metrics.DecrementRunning()
		s.releaseLock(job)
	}()

	// Dispatch by job type
	switch job.Type {
	case domain.JobTypeLeadershipScrape:
		s.runLeadershipJob(jobExec, logWriter)
	default: // "crawl" or empty (backward compat with pre-type jobs)
		s.runCrawlJob(jobExec, logWriter)
	}
}

// runCrawlJob executes a standard web crawl job.
func (s *IntervalScheduler) runCrawlJob(jobExec *JobExecution, logWriter logs.Writer) {
	job := jobExec.Job
	execution := jobExec.Execution

	// Create an isolated crawler for this job
	crawlerInstance, err := s.createJobCrawler(jobExec, logWriter)
	if err != nil {
		s.handleJobFailure(jobExec, err, nil)
		return
	}

	writeLog(logWriter, "info", "Starting job execution", job.ID, execution.ID, map[string]any{
		"source_id":     job.SourceID,
		"url":           job.URL,
		"retry_attempt": job.CurrentRetryCount,
	})

	s.logger.Info("Executing job",
		infralogger.String("job_id", job.ID),
		infralogger.String("source_id", job.SourceID),
		infralogger.String("url", job.URL),
		infralogger.Int("retry_attempt", job.CurrentRetryCount),
	)

	if job.SourceID == "" {
		writeLog(logWriter, "error", "Job missing required source_id", job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, errors.New("job missing required source_id"), nil)
		return
	}

	// Capture startTime AFTER crawler creation to match original timing semantics
	startTime := time.Now()
	writeLog(logWriter, "info", "Starting crawler", job.ID, execution.ID, map[string]any{
		"source_id": job.SourceID,
	})

	err = crawlerInstance.Start(jobExec.Context, job.SourceID)
	if err != nil {
		s.logCrawlerStartError(job, execution.ID, err, logWriter)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	writeLog(logWriter, "info", "Waiting for crawler to complete", job.ID, execution.ID, nil)

	err = crawlerInstance.Wait()
	if err != nil {
		writeLog(logWriter, "error", "Crawler failed: "+err.Error(), job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	jobSummary := crawlerInstance.GetJobLogger().BuildSummary()
	writeLog(logWriter, "info", "Job completed successfully", job.ID, execution.ID, map[string]any{
		"duration_ms":     time.Since(startTime).Milliseconds(),
		"pages_crawled":   jobSummary.PagesCrawled,
		"items_extracted": jobSummary.ItemsExtracted,
		"error_count":     jobSummary.ErrorsCount,
	})

	s.handleJobSuccess(jobExec, &startTime)
}
```

- [ ] **Step 2: Add runLeadershipJob method**

```go
// runLeadershipJob executes a leadership scrape job.
func (s *IntervalScheduler) runLeadershipJob(jobExec *JobExecution, logWriter logs.Writer) {
	job := jobExec.Job
	execution := jobExec.Execution
	startTime := time.Now()

	writeLog(logWriter, "info", "Starting leadership scrape job", job.ID, execution.ID, nil)

	s.logger.Info("Executing leadership scrape job",
		infralogger.String("job_id", job.ID),
	)

	if s.scraperConfig == nil {
		err := errors.New("leadership scrape: scraper not configured")
		writeLog(logWriter, "error", err.Error(), job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	err := runLeadershipScrapeJob(jobExec.Context, *s.scraperConfig, s.logger)
	if err != nil {
		writeLog(logWriter, "error", "Leadership scrape failed: "+err.Error(), job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	writeLog(logWriter, "info", "Leadership scrape completed successfully", job.ID, execution.ID, map[string]any{
		"duration_ms": time.Since(startTime).Milliseconds(),
	})

	s.handleLeadershipJobSuccess(jobExec, &startTime)
}
```

- [ ] **Step 3: Add handleLeadershipJobSuccess**

The existing `handleJobSuccess` references `jobExec.Crawler.GetJobLogger()` which won't exist for leadership jobs. Add a separate handler. Note: we use `calculateNextRun` (not `calculateAdaptiveOrFixedNextRun`) because leadership scrape jobs have no crawl-level content hashing — adaptive scheduling doesn't apply.

```go
// handleLeadershipJobSuccess handles successful leadership scrape completion.
// Uses calculateNextRun (not adaptive) because leadership scrape jobs have no
// content hash tracking — adaptive scheduling only applies to crawl jobs.
func (s *IntervalScheduler) handleLeadershipJobSuccess(jobExec *JobExecution, startTime *time.Time) {
	job := jobExec.Job
	execution := jobExec.Execution

	now := time.Now()
	durationMs := time.Since(*startTime).Milliseconds()

	// Update execution record (no crawler metrics for leadership jobs)
	execution.Status = string(StateCompleted)
	execution.CompletedAt = &now
	execution.DurationMs = &durationMs

	if err := s.executionRepo.Update(s.ctx, execution); err != nil {
		s.logger.Error("Failed to update execution",
			infralogger.String("execution_id", execution.ID),
			infralogger.Error(err),
		)
	}

	// Update job
	job.Status = string(StateCompleted)
	job.CompletedAt = &now
	job.CurrentRetryCount = 0
	job.ErrorMessage = nil

	// If recurring, schedule next run
	if job.IntervalMinutes != nil && job.ScheduleEnabled {
		job.Status = "scheduled"
		nextRun := s.calculateNextRun(job)
		job.NextRunAt = &nextRun
	}

	if err := s.repo.Update(s.ctx, job); err != nil {
		s.logger.Error("Failed to update job",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)
	}

	s.metrics.IncrementCompleted()
	s.metrics.IncrementTotalExecutions()

	s.logger.Info("Leadership scrape job completed successfully",
		infralogger.String("job_id", job.ID),
		infralogger.Int64("duration_ms", durationMs),
		infralogger.Any("next_run_at", job.NextRunAt),
	)

	// Publish SSE event
	s.publishJobCompleted(s.ctx, job, execution)
}
```

- [ ] **Step 4: Ensure `errors` and `domain` are imported**

In `interval_scheduler.go`, verify these imports exist (they should already):

```go
"errors"
"github.com/jonesrussell/north-cloud/crawler/internal/domain"
```

The `domain` import is new — add it if not present.

- [ ] **Step 5: Run all scheduler tests**

Run: `cd /home/fsd42/dev/north-cloud && go test ./crawler/internal/scheduler/ -v -count=1`
Expected: All existing tests PASS

- [ ] **Step 6: Commit**

```bash
git add crawler/internal/scheduler/interval_scheduler.go
git commit -m "feat(#188): add job type dispatch in scheduler runJob"
```

---

### Task 8: Wire ScraperConfig in bootstrap

**Files:**
- Modify: `crawler/internal/bootstrap/services.go:260-290` (createAndStartScheduler)

- [ ] **Step 1: Pass ScraperConfig when creating scheduler**

In `createAndStartScheduler()`, replace lines 274-279:

```go
// Create interval scheduler with default options
intervalScheduler := scheduler.NewIntervalScheduler(
	deps.Logger,
	db.JobRepo,
	db.ExecutionRepo,
	crawlerFactory,
)
```

with:

```go
// Build scraper config for leadership_scrape jobs
smCfg := deps.Config.GetSourceManagerConfig()
authCfg := deps.Config.GetAuthConfig()
scraperCfg := scheduler.ScraperConfig{
	SourceManagerURL: smCfg.URL,
	JWTToken:         authCfg.JWTSecret,
}

// Create interval scheduler with scraper config
intervalScheduler := scheduler.NewIntervalScheduler(
	deps.Logger,
	db.JobRepo,
	db.ExecutionRepo,
	crawlerFactory,
	scheduler.WithScraperConfig(scraperCfg),
)
```

- [ ] **Step 2: Verify build compiles**

Run: `cd /home/fsd42/dev/north-cloud && go build ./crawler/...`
Expected: Build succeeds

- [ ] **Step 3: Run full test suite**

Run: `cd /home/fsd42/dev/north-cloud && go test ./crawler/... -count=1`
Expected: All tests PASS

- [ ] **Step 4: Commit**

```bash
git add crawler/internal/bootstrap/services.go
git commit -m "feat(#188): wire ScraperConfig in bootstrap for leadership scrape jobs"
```

---

## Chunk 4: Verification

### Task 9: End-to-end verification

- [ ] **Step 1: Build**

```bash
cd /home/fsd42/dev/north-cloud && go build -o crawler/bin/crawler ./crawler/
```

- [ ] **Step 2: Create a leadership scrape job via API**

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "leadership_scrape",
    "interval_minutes": 360,
    "interval_type": "minutes",
    "schedule_enabled": true
  }'
```

Expected: 201 Created with `"type": "leadership_scrape"` in response

- [ ] **Step 3: Verify job listing shows type**

```bash
curl http://localhost:8080/api/v1/jobs | jq '.jobs[] | {id, type, status}'
```

Expected: Shows both `crawl` and `leadership_scrape` jobs

- [ ] **Step 4: Force-run the job and verify execution**

```bash
curl -X POST http://localhost:8080/api/v1/jobs/{JOB_ID}/force-run
```

Expected: Job runs, leadership scraper executes, results logged

---

## Summary of all files changed

| File | Action | Description |
|------|--------|-------------|
| `crawler/migrations/021_add_job_type.up.sql` | Create | ALTER TABLE ADD COLUMN type |
| `crawler/migrations/021_add_job_type.down.sql` | Create | Rollback migration |
| `crawler/internal/domain/job.go` | Modify | Add Type field + constants |
| `crawler/internal/domain/job_test.go` | Create | ValidJobType test |
| `crawler/internal/database/job_repository.go` | Modify | Add `type` to all SQL (insert, select, update) |
| `crawler/internal/api/types.go` | Modify | Add Type to request types, remove binding:"required" |
| `crawler/internal/api/jobs_handler.go` | Modify | Type validation, defaults, sort field |
| `crawler/internal/scheduler/scraper_runner.go` | Create | Leadership scrape runner function |
| `crawler/internal/scheduler/scraper_runner_test.go` | Create | Runner unit test |
| `crawler/internal/scheduler/options.go` | Modify | Add WithScraperConfig option |
| `crawler/internal/scheduler/interval_scheduler.go` | Modify | scraperConfig field, type dispatch, runCrawlJob/runLeadershipJob |
| `crawler/internal/bootstrap/services.go` | Modify | Wire ScraperConfig option |
