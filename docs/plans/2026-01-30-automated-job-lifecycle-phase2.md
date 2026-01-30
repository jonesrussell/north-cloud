# Automated Job Lifecycle - Phase 2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the NoOpHandler with a JobService that automatically creates, updates, pauses, and deletes crawler jobs in response to source lifecycle events.

**Architecture:** The JobService implements the EventHandler interface. When source events arrive via Redis Streams, it creates/updates jobs in the crawler database. A ScheduleComputer determines crawl intervals based on source metadata (rate_limit, max_depth, priority). The existing IntervalScheduler picks up these jobs and executes them.

**Tech Stack:** Go 1.24+, PostgreSQL, Redis Streams, infrastructure/events types

---

## Prerequisites

- Phase 1 complete (event infrastructure, NoOpHandler, processed_events table)
- PR #62 merged or branch `refactor/scheduler` up to date

---

## Task 1: Add Auto-Managed Job Columns Migration

**Files:**
- Create: `crawler/migrations/009_add_auto_managed_jobs.up.sql`
- Create: `crawler/migrations/009_add_auto_managed_jobs.down.sql`

### Step 1: Create up migration

```sql
-- crawler/migrations/009_add_auto_managed_jobs.up.sql
-- Migration: Add columns for auto-managed job lifecycle
-- Description: Extends jobs table with priority, backoff, and auto-management tracking

BEGIN;

-- Add columns for auto-managed job lifecycle
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS auto_managed BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS priority INTEGER DEFAULT 50,
    ADD COLUMN IF NOT EXISTS failure_count INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_failure_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS backoff_until TIMESTAMPTZ;

-- Index for efficient due job queries with priority ordering
CREATE INDEX IF NOT EXISTS idx_jobs_due_priority
    ON jobs (next_run_at, priority DESC)
    WHERE status = 'pending' AND (backoff_until IS NULL OR backoff_until < NOW());

-- Index for finding jobs by source_id (used by JobService)
CREATE INDEX IF NOT EXISTS idx_jobs_source_id
    ON jobs (source_id);

-- Documentation
COMMENT ON COLUMN jobs.auto_managed IS 'True if job is managed by event-driven automation';
COMMENT ON COLUMN jobs.priority IS 'Numeric priority 0-100, higher = scheduled sooner';
COMMENT ON COLUMN jobs.failure_count IS 'Consecutive failure count for backoff calculation';
COMMENT ON COLUMN jobs.backoff_until IS 'Do not run until this time (failure backoff)';

COMMIT;
```

### Step 2: Create down migration

```sql
-- crawler/migrations/009_add_auto_managed_jobs.down.sql
-- Migration: Remove auto-managed job columns
-- Description: Rollback for 009_add_auto_managed_jobs.up.sql

BEGIN;

DROP INDEX IF EXISTS idx_jobs_due_priority;
DROP INDEX IF EXISTS idx_jobs_source_id;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS auto_managed,
    DROP COLUMN IF EXISTS priority,
    DROP COLUMN IF EXISTS failure_count,
    DROP COLUMN IF EXISTS last_failure_at,
    DROP COLUMN IF EXISTS backoff_until;

COMMIT;
```

### Step 3: Verify migration syntax

Run: `cd crawler && cat migrations/009_add_auto_managed_jobs.up.sql`
Expected: Valid SQL syntax displayed

### Step 4: Commit

```bash
git add crawler/migrations/009_add_auto_managed_jobs.up.sql crawler/migrations/009_add_auto_managed_jobs.down.sql
git commit -m "feat(crawler): add auto-managed job columns

Add priority, failure tracking, and backoff columns to support
event-driven job lifecycle. Include indexes for efficient queries.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 2: Update Job Domain Model

**Files:**
- Modify: `crawler/internal/domain/job.go`

### Step 1: Add new fields to Job struct

Add these fields to the Job struct after the existing fields:

```go
// Auto-managed job lifecycle
AutoManaged    bool       `db:"auto_managed"     json:"auto_managed"`
Priority       int        `db:"priority"         json:"priority"`
FailureCount   int        `db:"failure_count"    json:"failure_count"`
LastFailureAt  *time.Time `db:"last_failure_at"  json:"last_failure_at,omitempty"`
BackoffUntil   *time.Time `db:"backoff_until"    json:"backoff_until,omitempty"`
```

### Step 2: Verify the changes compile

Run: `cd crawler && go build ./...`
Expected: Build succeeds

### Step 3: Commit

```bash
git add crawler/internal/domain/job.go
git commit -m "feat(crawler): add auto-managed fields to Job domain model

Add AutoManaged, Priority, FailureCount, LastFailureAt, and
BackoffUntil fields to support event-driven job lifecycle.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 3: Extend JobRepository for Auto-Managed Jobs

**Files:**
- Modify: `crawler/internal/database/job_repository.go`
- Create: `crawler/internal/database/job_repository_automgmt_test.go`

### Step 1: Write failing tests for new repository methods

```go
// crawler/internal/database/job_repository_automgmt_test.go
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

	// This test verifies the interface - actual DB test would need integration setup
	var repo database.JobRepositoryInterface
	_ = repo // Interface check

	// For unit test, we verify the method exists by checking the type
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
```

### Step 2: Run test to verify it fails

Run: `cd crawler && go test ./internal/database/... -run TestJobRepository_FindBySourceID -v`
Expected: FAIL with "does not implement" or method not found

### Step 3: Add new methods to JobRepository

Add these methods to `crawler/internal/database/job_repository.go`:

```go
// FindBySourceID retrieves a job by its source ID.
// Returns nil, nil if no job exists for the source.
func (r *JobRepository) FindBySourceID(ctx context.Context, sourceID uuid.UUID) (*domain.Job, error) {
	var job domain.Job
	query := `
		SELECT id, source_id, source_name, url,
		       schedule_time, schedule_enabled,
		       interval_minutes, interval_type, next_run_at,
		       is_paused, max_retries, retry_backoff_seconds, current_retry_count,
		       lock_token, lock_acquired_at,
		       status, scheduler_version,
		       auto_managed, priority, failure_count, last_failure_at, backoff_until,
		       created_at, updated_at, started_at, completed_at,
		       paused_at, cancelled_at,
		       error_message, metadata
		FROM jobs
		WHERE source_id = $1
	`

	err := r.db.GetContext(ctx, &job, query, sourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("find job by source_id: %w", err)
	}

	return &job, nil
}

// UpsertAutoManaged creates or updates an auto-managed job.
// Uses source_id as the unique key for upsert.
func (r *JobRepository) UpsertAutoManaged(ctx context.Context, job *domain.Job) error {
	query := `
		INSERT INTO jobs (
			id, source_id, source_name, url,
			interval_minutes, interval_type, next_run_at,
			status, auto_managed, priority,
			failure_count, last_failure_at, backoff_until,
			schedule_enabled, is_paused,
			max_retries, retry_backoff_seconds
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (source_id) DO UPDATE SET
			source_name = EXCLUDED.source_name,
			url = EXCLUDED.url,
			interval_minutes = EXCLUDED.interval_minutes,
			interval_type = EXCLUDED.interval_type,
			next_run_at = COALESCE(jobs.next_run_at, EXCLUDED.next_run_at),
			status = CASE
				WHEN jobs.status IN ('running', 'scheduled') THEN jobs.status
				ELSE EXCLUDED.status
			END,
			priority = EXCLUDED.priority,
			failure_count = EXCLUDED.failure_count,
			last_failure_at = EXCLUDED.last_failure_at,
			backoff_until = EXCLUDED.backoff_until,
			updated_at = NOW()
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		job.ID,
		job.SourceID,
		job.SourceName,
		job.URL,
		job.IntervalMinutes,
		job.IntervalType,
		job.NextRunAt,
		job.Status,
		job.AutoManaged,
		job.Priority,
		job.FailureCount,
		job.LastFailureAt,
		job.BackoffUntil,
		job.ScheduleEnabled,
		job.IsPaused,
		job.MaxRetries,
		job.RetryBackoffSeconds,
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return fmt.Errorf("upsert auto-managed job: %w", err)
	}

	return nil
}

// DeleteBySourceID deletes a job by its source ID.
func (r *JobRepository) DeleteBySourceID(ctx context.Context, sourceID uuid.UUID) error {
	query := `DELETE FROM jobs WHERE source_id = $1`

	result, err := r.db.ExecContext(ctx, query, sourceID)
	if err != nil {
		return fmt.Errorf("delete job by source_id: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Not an error - job may not exist
		return nil
	}

	return nil
}

// UpdateStatusBySourceID updates a job's status by source ID.
func (r *JobRepository) UpdateStatusBySourceID(ctx context.Context, sourceID uuid.UUID, status string) error {
	query := `
		UPDATE jobs
		SET status = $1, updated_at = NOW()
		WHERE source_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, status, sourceID)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found for source_id: %s", sourceID)
	}

	return nil
}

// RecordProcessedEvent records an event as processed for idempotency.
func (r *JobRepository) RecordProcessedEvent(ctx context.Context, eventID uuid.UUID) error {
	query := `
		INSERT INTO processed_events (event_id, processed_at)
		VALUES ($1, NOW())
		ON CONFLICT (event_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("record processed event: %w", err)
	}

	return nil
}

// IsEventProcessed checks if an event has already been processed.
func (r *JobRepository) IsEventProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`

	err := r.db.GetContext(ctx, &exists, query, eventID)
	if err != nil {
		return false, fmt.Errorf("check event processed: %w", err)
	}

	return exists, nil
}
```

Also add the import for `uuid`:
```go
"github.com/google/uuid"
```

### Step 4: Run tests to verify they pass

Run: `cd crawler && go test ./internal/database/... -run TestJobRepository_ -v`
Expected: PASS

### Step 5: Commit

```bash
git add crawler/internal/database/job_repository.go crawler/internal/database/job_repository_automgmt_test.go
git commit -m "feat(crawler): extend JobRepository for auto-managed jobs

Add FindBySourceID, UpsertAutoManaged, DeleteBySourceID,
UpdateStatusBySourceID, RecordProcessedEvent, and IsEventProcessed
methods to support event-driven job lifecycle.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 4: Create ScheduleComputer

**Files:**
- Create: `crawler/internal/job/schedule_computer.go`
- Create: `crawler/internal/job/schedule_computer_test.go`

### Step 1: Write failing tests

```go
// crawler/internal/job/schedule_computer_test.go
package job_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/job"
)

func TestScheduleComputer_ComputeSchedule_NormalPriority(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit: 10,
		MaxDepth:  2,
		Priority:  "normal",
	})

	// Normal priority base: 60 minutes
	// Rate limit 10: no adjustment
	// Max depth 2: no adjustment
	// Expected: 60 minutes
	if output.IntervalMinutes != 60 {
		t.Errorf("expected 60 minutes, got %d", output.IntervalMinutes)
	}

	if output.NumericPriority != 50 {
		t.Errorf("expected priority 50, got %d", output.NumericPriority)
	}
}

func TestScheduleComputer_ComputeSchedule_HighPriority(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit: 10,
		MaxDepth:  2,
		Priority:  "high",
	})

	// High priority base: 30 minutes
	if output.IntervalMinutes != 30 {
		t.Errorf("expected 30 minutes, got %d", output.IntervalMinutes)
	}

	if output.NumericPriority != 75 {
		t.Errorf("expected priority 75, got %d", output.NumericPriority)
	}
}

func TestScheduleComputer_ComputeSchedule_LowRateLimit(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit: 3, // Low rate limit
		MaxDepth:  2,
		Priority:  "normal",
	})

	// Normal base: 60, low rate limit: +50% = 90 minutes
	if output.IntervalMinutes != 90 {
		t.Errorf("expected 90 minutes for low rate limit, got %d", output.IntervalMinutes)
	}
}

func TestScheduleComputer_ComputeSchedule_DeepCrawl(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit: 10,
		MaxDepth:  6, // Deep crawl
		Priority:  "normal",
	})

	// Normal base: 60, deep crawl: +50% = 90 minutes
	if output.IntervalMinutes != 90 {
		t.Errorf("expected 90 minutes for deep crawl, got %d", output.IntervalMinutes)
	}
}

func TestScheduleComputer_ComputeSchedule_WithFailures(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit:    10,
		MaxDepth:     2,
		Priority:     "normal",
		FailureCount: 2,
	})

	// Normal base: 60, 2 failures: 60 * 2^2 = 240 minutes
	if output.IntervalMinutes != 240 {
		t.Errorf("expected 240 minutes with 2 failures, got %d", output.IntervalMinutes)
	}
}

func TestScheduleComputer_ComputeSchedule_BackoffCapped(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()
	output := sc.ComputeSchedule(job.ScheduleInput{
		RateLimit:    10,
		MaxDepth:     2,
		Priority:     "normal",
		FailureCount: 10, // Many failures
	})

	// Backoff capped at 24 hours = 1440 minutes
	maxBackoff := 24 * 60
	if output.IntervalMinutes > maxBackoff {
		t.Errorf("expected backoff capped at %d, got %d", maxBackoff, output.IntervalMinutes)
	}
}

func TestScheduleComputer_ComputeInitialDelay(t *testing.T) {
	t.Helper()

	sc := job.NewScheduleComputer()

	tests := []struct {
		priority      string
		expectedDelay time.Duration
	}{
		{"critical", 0},
		{"high", 1 * time.Minute},
		{"normal", 5 * time.Minute},
		{"low", 10 * time.Minute},
	}

	for _, tt := range tests {
		output := sc.ComputeSchedule(job.ScheduleInput{
			RateLimit: 10,
			MaxDepth:  2,
			Priority:  tt.priority,
		})

		if output.InitialDelay != tt.expectedDelay {
			t.Errorf("priority %s: expected delay %v, got %v",
				tt.priority, tt.expectedDelay, output.InitialDelay)
		}
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd crawler && go test ./internal/job/... -v`
Expected: FAIL with "package not found" or undefined types

### Step 3: Write implementation

```go
// crawler/internal/job/schedule_computer.go
package job

import (
	"time"
)

// Priority levels for sources.
const (
	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

// Base intervals by priority (in minutes).
var priorityBaseIntervals = map[string]int{
	PriorityCritical: 15,  // 4x/hour
	PriorityHigh:     30,  // 2x/hour
	PriorityNormal:   60,  // 1x/hour
	PriorityLow:      180, // 3x/day
}

// Numeric priority values (higher = scheduled sooner).
var priorityNumericValues = map[string]int{
	PriorityCritical: 100,
	PriorityHigh:     75,
	PriorityNormal:   50,
	PriorityLow:      25,
}

// Initial delays by priority to stagger job starts.
var priorityInitialDelays = map[string]time.Duration{
	PriorityCritical: 0,
	PriorityHigh:     1 * time.Minute,
	PriorityNormal:   5 * time.Minute,
	PriorityLow:      10 * time.Minute,
}

// Maximum backoff interval (24 hours in minutes).
const maxBackoffMinutes = 24 * 60

// ScheduleComputer computes job schedules based on source metadata.
type ScheduleComputer struct{}

// ScheduleInput contains the source metadata used to compute a schedule.
type ScheduleInput struct {
	RateLimit    int    // requests per second allowed
	MaxDepth     int    // crawl depth
	Priority     string // low, normal, high, critical
	FailureCount int    // consecutive failures (for backoff)
}

// ScheduleOutput contains the computed schedule parameters.
type ScheduleOutput struct {
	IntervalMinutes int           // interval between runs
	IntervalType    string        // "minutes" or "hours"
	NumericPriority int           // 0-100, higher = sooner
	InitialDelay    time.Duration // delay before first run
}

// NewScheduleComputer creates a new schedule computer.
func NewScheduleComputer() *ScheduleComputer {
	return &ScheduleComputer{}
}

// ComputeSchedule calculates the optimal schedule for a source.
func (sc *ScheduleComputer) ComputeSchedule(input ScheduleInput) ScheduleOutput {
	priority := input.Priority
	if priority == "" {
		priority = PriorityNormal
	}

	// Start with base interval from priority
	baseInterval := priorityBaseIntervals[priority]
	if baseInterval == 0 {
		baseInterval = priorityBaseIntervals[PriorityNormal]
	}

	// Adjust for rate limit
	intervalMinutes := sc.adjustForRateLimit(baseInterval, input.RateLimit)

	// Adjust for max depth
	intervalMinutes = sc.adjustForDepth(intervalMinutes, input.MaxDepth)

	// Apply exponential backoff if there are failures
	intervalMinutes = sc.applyBackoff(intervalMinutes, input.FailureCount)

	// Determine interval type for readability
	intervalType := "minutes"
	if intervalMinutes >= 60 && intervalMinutes%60 == 0 {
		intervalType = "hours"
	}

	// Get numeric priority
	numericPriority := priorityNumericValues[priority]
	if numericPriority == 0 {
		numericPriority = priorityNumericValues[PriorityNormal]
	}

	// Get initial delay
	initialDelay := priorityInitialDelays[priority]

	return ScheduleOutput{
		IntervalMinutes: intervalMinutes,
		IntervalType:    intervalType,
		NumericPriority: numericPriority,
		InitialDelay:    initialDelay,
	}
}

// adjustForRateLimit adjusts interval based on rate limit.
// Lower rate limits need longer intervals to be polite.
func (sc *ScheduleComputer) adjustForRateLimit(baseInterval, rateLimit int) int {
	if rateLimit <= 0 {
		rateLimit = 10 // default
	}

	switch {
	case rateLimit <= 5:
		// Low rate limit: +50% interval
		return baseInterval * 3 / 2
	case rateLimit <= 10:
		// Normal: base interval
		return baseInterval
	case rateLimit <= 20:
		// Higher rate limit: -25% interval
		return baseInterval * 3 / 4
	default:
		// Very high rate limit: -50% interval
		return baseInterval / 2
	}
}

// adjustForDepth adjusts interval based on crawl depth.
// Deeper crawls take longer, so space them out more.
func (sc *ScheduleComputer) adjustForDepth(interval, maxDepth int) int {
	if maxDepth <= 0 {
		maxDepth = 1
	}

	switch {
	case maxDepth <= 2:
		// Shallow: base interval
		return interval
	case maxDepth <= 5:
		// Medium: +25%
		return interval * 5 / 4
	default:
		// Deep: +50%
		return interval * 3 / 2
	}
}

// applyBackoff applies exponential backoff based on failure count.
func (sc *ScheduleComputer) applyBackoff(interval, failureCount int) int {
	if failureCount <= 0 {
		return interval
	}

	// Exponential backoff: interval * 2^failures, capped at 24 hours
	backoffInterval := interval
	for i := 0; i < failureCount && backoffInterval < maxBackoffMinutes; i++ {
		backoffInterval *= 2
	}

	if backoffInterval > maxBackoffMinutes {
		backoffInterval = maxBackoffMinutes
	}

	return backoffInterval
}
```

### Step 4: Run tests to verify they pass

Run: `cd crawler && go test ./internal/job/... -v`
Expected: PASS (all 7 tests)

### Step 5: Commit

```bash
git add crawler/internal/job/schedule_computer.go crawler/internal/job/schedule_computer_test.go
git commit -m "feat(crawler): add ScheduleComputer for dynamic job scheduling

Compute crawl intervals based on source metadata:
- Priority: critical (15m), high (30m), normal (60m), low (180m)
- Rate limit adjustments: low (+50%), high (-25% to -50%)
- Depth adjustments: shallow (base), medium (+25%), deep (+50%)
- Exponential backoff on failures, capped at 24 hours

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 5: Create Source Client Interface

**Files:**
- Create: `crawler/internal/sources/client.go`
- Create: `crawler/internal/sources/client_test.go`

### Step 1: Write failing tests

```go
// crawler/internal/sources/client_test.go
package sources_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
)

func TestSourceClient_Interface(t *testing.T) {
	t.Helper()

	// Verify interface is defined
	var _ sources.Client = (*sources.HTTPClient)(nil)
}

func TestSource_HasRequiredFields(t *testing.T) {
	t.Helper()

	s := sources.Source{
		ID:        uuid.New(),
		Name:      "Test",
		URL:       "https://example.com",
		RateLimit: 10,
		MaxDepth:  2,
		Enabled:   true,
		Priority:  "normal",
	}

	if s.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if s.Name == "" {
		t.Error("Name should not be empty")
	}
}
```

### Step 2: Run test to verify it fails

Run: `cd crawler && go test ./internal/sources/... -run TestSourceClient -v`
Expected: FAIL with "undefined: sources.Client"

### Step 3: Write implementation

```go
// crawler/internal/sources/client.go
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Source represents a content source from source-manager.
type Source struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	RateLimit int       `json:"rate_limit"`
	MaxDepth  int       `json:"max_depth"`
	Enabled   bool      `json:"enabled"`
	Priority  string    `json:"priority"`
}

// Client defines the interface for fetching source data.
type Client interface {
	GetSource(ctx context.Context, sourceID uuid.UUID) (*Source, error)
}

// HTTPClient implements Client using HTTP requests to source-manager.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// Default timeouts for HTTP client.
const (
	defaultHTTPTimeout = 10 * time.Second
)

// NewHTTPClient creates a new HTTP client for source-manager.
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// GetSource fetches a source by ID from source-manager.
func (c *HTTPClient) GetSource(ctx context.Context, sourceID uuid.UUID) (*Source, error) {
	url := fmt.Sprintf("%s/api/v1/sources/%s", c.baseURL, sourceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch source: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Source not found is not an error
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var source Source
	if decodeErr := json.NewDecoder(resp.Body).Decode(&source); decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	return &source, nil
}

// NoOpClient is a client that always returns nil (for testing/disabled mode).
type NoOpClient struct{}

// NewNoOpClient creates a no-op client.
func NewNoOpClient() *NoOpClient {
	return &NoOpClient{}
}

// GetSource always returns nil for NoOpClient.
func (c *NoOpClient) GetSource(_ context.Context, _ uuid.UUID) (*Source, error) {
	return nil, nil
}
```

### Step 4: Run tests to verify they pass

Run: `cd crawler && go test ./internal/sources/... -v`
Expected: PASS

### Step 5: Commit

```bash
git add crawler/internal/sources/client.go crawler/internal/sources/client_test.go
git commit -m "feat(crawler): add Source client for fetching source data

Add Client interface and HTTPClient implementation for fetching
source metadata from source-manager API. Used by JobService when
processing SOURCE_UPDATED and SOURCE_ENABLED events.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 6: Create JobService

**Files:**
- Create: `crawler/internal/job/service.go`
- Create: `crawler/internal/job/service_test.go`

### Step 1: Write failing tests

```go
// crawler/internal/job/service_test.go
package job_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infraevents "github.com/north-cloud/infrastructure/events"
)

// MockJobRepository implements job.Repository for testing.
type MockJobRepository struct {
	Jobs            map[uuid.UUID]*domain.Job
	ProcessedEvents map[uuid.UUID]bool
	DeletedSources  []uuid.UUID
}

func NewMockJobRepository() *MockJobRepository {
	return &MockJobRepository{
		Jobs:            make(map[uuid.UUID]*domain.Job),
		ProcessedEvents: make(map[uuid.UUID]bool),
		DeletedSources:  make([]uuid.UUID, 0),
	}
}

func (m *MockJobRepository) FindBySourceID(_ context.Context, sourceID uuid.UUID) (*domain.Job, error) {
	return m.Jobs[sourceID], nil
}

func (m *MockJobRepository) UpsertAutoManaged(_ context.Context, j *domain.Job) error {
	sourceUUID, _ := uuid.Parse(j.SourceID)
	m.Jobs[sourceUUID] = j
	return nil
}

func (m *MockJobRepository) DeleteBySourceID(_ context.Context, sourceID uuid.UUID) error {
	delete(m.Jobs, sourceID)
	m.DeletedSources = append(m.DeletedSources, sourceID)
	return nil
}

func (m *MockJobRepository) UpdateStatusBySourceID(_ context.Context, sourceID uuid.UUID, status string) error {
	if j, ok := m.Jobs[sourceID]; ok {
		j.Status = status
	}
	return nil
}

func (m *MockJobRepository) RecordProcessedEvent(_ context.Context, eventID uuid.UUID) error {
	m.ProcessedEvents[eventID] = true
	return nil
}

func (m *MockJobRepository) IsEventProcessed(_ context.Context, eventID uuid.UUID) (bool, error) {
	return m.ProcessedEvents[eventID], nil
}

// MockSourceClient implements sources.Client for testing.
type MockSourceClient struct {
	Sources map[uuid.UUID]*sources.Source
}

func NewMockSourceClient() *MockSourceClient {
	return &MockSourceClient{
		Sources: make(map[uuid.UUID]*sources.Source),
	}
}

func (m *MockSourceClient) GetSource(_ context.Context, sourceID uuid.UUID) (*sources.Source, error) {
	return m.Sources[sourceID], nil
}

func TestJobService_HandleSourceCreated_CreatesJob(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	svc := job.NewService(repo, job.NewScheduleComputer(), sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceCreated,
		SourceID:  sourceID,
		Payload: infraevents.SourceCreatedPayload{
			Name:      "Test Source",
			URL:       "https://example.com",
			RateLimit: 10,
			MaxDepth:  2,
			Enabled:   true,
			Priority:  "normal",
		},
	}

	err := svc.HandleSourceCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify job was created
	createdJob := repo.Jobs[sourceID]
	if createdJob == nil {
		t.Fatal("expected job to be created")
	}

	if createdJob.SourceID != sourceID.String() {
		t.Errorf("expected source_id %s, got %s", sourceID, createdJob.SourceID)
	}

	if !createdJob.AutoManaged {
		t.Error("expected job to be auto_managed")
	}

	// Verify event was recorded as processed
	if !repo.ProcessedEvents[eventID] {
		t.Error("expected event to be recorded as processed")
	}
}

func TestJobService_HandleSourceCreated_SkipsDisabled(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	svc := job.NewService(repo, job.NewScheduleComputer(), sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceCreated,
		SourceID:  sourceID,
		Payload: infraevents.SourceCreatedPayload{
			Name:    "Test Source",
			URL:     "https://example.com",
			Enabled: false, // Disabled
		},
	}

	err := svc.HandleSourceCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no job was created
	if repo.Jobs[sourceID] != nil {
		t.Error("expected no job for disabled source")
	}

	// Event should still be recorded
	if !repo.ProcessedEvents[eventID] {
		t.Error("expected event to be recorded as processed")
	}
}

func TestJobService_HandleSourceDeleted_DeletesJob(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	svc := job.NewService(repo, job.NewScheduleComputer(), sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	// Pre-create a job
	repo.Jobs[sourceID] = &domain.Job{
		ID:       uuid.New().String(),
		SourceID: sourceID.String(),
	}

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceDeleted,
		SourceID:  sourceID,
		Payload: infraevents.SourceDeletedPayload{
			Name:           "Test Source",
			DeletionReason: "user_requested",
		},
	}

	err := svc.HandleSourceDeleted(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify job was deleted
	if repo.Jobs[sourceID] != nil {
		t.Error("expected job to be deleted")
	}

	if len(repo.DeletedSources) != 1 || repo.DeletedSources[0] != sourceID {
		t.Error("expected source to be in deleted list")
	}
}

func TestJobService_HandleSourceDisabled_PausesJob(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	svc := job.NewService(repo, job.NewScheduleComputer(), sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	// Pre-create a job
	repo.Jobs[sourceID] = &domain.Job{
		ID:       uuid.New().String(),
		SourceID: sourceID.String(),
		Status:   "pending",
	}

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceDisabled,
		SourceID:  sourceID,
		Payload: infraevents.SourceTogglePayload{
			Reason:    "maintenance",
			ToggledBy: "user",
		},
	}

	err := svc.HandleSourceDisabled(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify job was paused
	if repo.Jobs[sourceID].Status != "paused" {
		t.Errorf("expected status paused, got %s", repo.Jobs[sourceID].Status)
	}
}

func TestJobService_Idempotency_SkipsDuplicateEvents(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	svc := job.NewService(repo, job.NewScheduleComputer(), sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	// Mark event as already processed
	repo.ProcessedEvents[eventID] = true

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceCreated,
		SourceID:  sourceID,
		Payload: infraevents.SourceCreatedPayload{
			Name:    "Test Source",
			URL:     "https://example.com",
			Enabled: true,
		},
	}

	err := svc.HandleSourceCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no job was created (idempotent skip)
	if repo.Jobs[sourceID] != nil {
		t.Error("expected no job for duplicate event")
	}
}

func TestJobService_ImplementsEventHandler(t *testing.T) {
	t.Helper()

	// Verify JobService implements EventHandler interface
	var _ events.EventHandler = (*job.Service)(nil)
}
```

### Step 2: Run test to verify it fails

Run: `cd crawler && go test ./internal/job/... -run TestJobService -v`
Expected: FAIL with "undefined: job.Service"

### Step 3: Write implementation

```go
// crawler/internal/job/service.go
package job

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infraevents "github.com/north-cloud/infrastructure/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Repository defines the interface for job persistence.
type Repository interface {
	FindBySourceID(ctx context.Context, sourceID uuid.UUID) (*domain.Job, error)
	UpsertAutoManaged(ctx context.Context, job *domain.Job) error
	DeleteBySourceID(ctx context.Context, sourceID uuid.UUID) error
	UpdateStatusBySourceID(ctx context.Context, sourceID uuid.UUID, status string) error
	RecordProcessedEvent(ctx context.Context, eventID uuid.UUID) error
	IsEventProcessed(ctx context.Context, eventID uuid.UUID) (bool, error)
}

// Service handles job lifecycle based on source events.
type Service struct {
	repo             Repository
	scheduleComputer *ScheduleComputer
	sourceClient     sources.Client
	log              infralogger.Logger
}

// NewService creates a new job service.
func NewService(
	repo Repository,
	scheduleComputer *ScheduleComputer,
	sourceClient sources.Client,
	log infralogger.Logger,
) *Service {
	return &Service{
		repo:             repo,
		scheduleComputer: scheduleComputer,
		sourceClient:     sourceClient,
		log:              log,
	}
}

// HandleSourceCreated creates a job for a newly created source.
func (s *Service) HandleSourceCreated(ctx context.Context, event infraevents.SourceEvent) error {
	// Idempotency check
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		s.logDebug("Event already processed, skipping",
			infralogger.String("event_id", event.EventID.String()),
		)
		return nil
	}

	payload, ok := event.Payload.(infraevents.SourceCreatedPayload)
	if !ok {
		return fmt.Errorf("invalid payload type for SOURCE_CREATED")
	}

	// Skip disabled sources
	if !payload.Enabled {
		s.logInfo("Source created but disabled, skipping job creation",
			infralogger.String("source_id", event.SourceID.String()),
		)
		return s.repo.RecordProcessedEvent(ctx, event.EventID)
	}

	// Compute schedule based on source metadata
	schedule := s.scheduleComputer.ComputeSchedule(ScheduleInput{
		RateLimit: payload.RateLimit,
		MaxDepth:  payload.MaxDepth,
		Priority:  payload.Priority,
	})

	sourceName := payload.Name
	intervalMinutes := schedule.IntervalMinutes

	job := &domain.Job{
		ID:                  uuid.New().String(),
		SourceID:            event.SourceID.String(),
		SourceName:          &sourceName,
		URL:                 payload.URL,
		IntervalMinutes:     &intervalMinutes,
		IntervalType:        schedule.IntervalType,
		NextRunAt:           timePtr(time.Now().Add(schedule.InitialDelay)),
		Status:              "pending",
		AutoManaged:         true,
		Priority:            schedule.NumericPriority,
		ScheduleEnabled:     true,
		MaxRetries:          3,
		RetryBackoffSeconds: 60,
	}

	if upsertErr := s.repo.UpsertAutoManaged(ctx, job); upsertErr != nil {
		return fmt.Errorf("upsert job: %w", upsertErr)
	}

	// Record event as processed
	if recordErr := s.repo.RecordProcessedEvent(ctx, event.EventID); recordErr != nil {
		s.logWarn("Failed to record processed event",
			infralogger.String("event_id", event.EventID.String()),
			infralogger.Error(recordErr),
		)
	}

	s.logInfo("Created auto-managed job",
		infralogger.String("source_id", event.SourceID.String()),
		infralogger.String("source_name", payload.Name),
		infralogger.Int("interval_minutes", schedule.IntervalMinutes),
	)

	return nil
}

// HandleSourceUpdated updates a job when a source is updated.
func (s *Service) HandleSourceUpdated(ctx context.Context, event infraevents.SourceEvent) error {
	// Idempotency check
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		return nil
	}

	payload, ok := event.Payload.(infraevents.SourceUpdatedPayload)
	if !ok {
		return fmt.Errorf("invalid payload type for SOURCE_UPDATED")
	}

	// Check if schedule-affecting fields changed
	scheduleFields := map[string]bool{
		"rate_limit": true,
		"max_depth":  true,
		"priority":   true,
	}

	needsReschedule := false
	for _, field := range payload.ChangedFields {
		if scheduleFields[field] {
			needsReschedule = true
			break
		}
	}

	if !needsReschedule {
		// No schedule changes, just record and return
		return s.repo.RecordProcessedEvent(ctx, event.EventID)
	}

	// Fetch full source for recomputation
	source, sourceErr := s.sourceClient.GetSource(ctx, event.SourceID)
	if sourceErr != nil {
		return fmt.Errorf("fetch source: %w", sourceErr)
	}
	if source == nil {
		// Source not found, skip
		return s.repo.RecordProcessedEvent(ctx, event.EventID)
	}

	// Find existing job
	existingJob, findErr := s.repo.FindBySourceID(ctx, event.SourceID)
	if findErr != nil {
		return fmt.Errorf("find job: %w", findErr)
	}

	if existingJob == nil {
		// No job exists - if source is enabled, create one
		if source.Enabled {
			return s.HandleSourceCreated(ctx, infraevents.SourceEvent{
				EventID:   event.EventID,
				EventType: infraevents.SourceCreated,
				SourceID:  event.SourceID,
				Timestamp: event.Timestamp,
				Payload: infraevents.SourceCreatedPayload{
					Name:      source.Name,
					URL:       source.URL,
					RateLimit: source.RateLimit,
					MaxDepth:  source.MaxDepth,
					Enabled:   source.Enabled,
					Priority:  source.Priority,
				},
			})
		}
		return s.repo.RecordProcessedEvent(ctx, event.EventID)
	}

	// Recompute schedule
	schedule := s.scheduleComputer.ComputeSchedule(ScheduleInput{
		RateLimit:    source.RateLimit,
		MaxDepth:     source.MaxDepth,
		Priority:     source.Priority,
		FailureCount: existingJob.FailureCount,
	})

	intervalMinutes := schedule.IntervalMinutes
	existingJob.IntervalMinutes = &intervalMinutes
	existingJob.IntervalType = schedule.IntervalType
	existingJob.Priority = schedule.NumericPriority

	if upsertErr := s.repo.UpsertAutoManaged(ctx, existingJob); upsertErr != nil {
		return fmt.Errorf("update job schedule: %w", upsertErr)
	}

	s.logInfo("Updated job schedule",
		infralogger.String("source_id", event.SourceID.String()),
		infralogger.Int("new_interval", schedule.IntervalMinutes),
	)

	return s.repo.RecordProcessedEvent(ctx, event.EventID)
}

// HandleSourceDeleted deletes a job when a source is deleted.
func (s *Service) HandleSourceDeleted(ctx context.Context, event infraevents.SourceEvent) error {
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		return nil
	}

	if deleteErr := s.repo.DeleteBySourceID(ctx, event.SourceID); deleteErr != nil {
		return fmt.Errorf("delete job: %w", deleteErr)
	}

	s.logInfo("Deleted job for deleted source",
		infralogger.String("source_id", event.SourceID.String()),
	)

	return s.repo.RecordProcessedEvent(ctx, event.EventID)
}

// HandleSourceEnabled resumes a job when a source is enabled.
func (s *Service) HandleSourceEnabled(ctx context.Context, event infraevents.SourceEvent) error {
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		return nil
	}

	job, findErr := s.repo.FindBySourceID(ctx, event.SourceID)
	if findErr != nil {
		return fmt.Errorf("find job: %w", findErr)
	}

	// If no job exists, create one
	if job == nil {
		source, sourceErr := s.sourceClient.GetSource(ctx, event.SourceID)
		if sourceErr != nil {
			return fmt.Errorf("fetch source: %w", sourceErr)
		}
		if source == nil {
			return s.repo.RecordProcessedEvent(ctx, event.EventID)
		}

		return s.HandleSourceCreated(ctx, infraevents.SourceEvent{
			EventID:   event.EventID,
			EventType: infraevents.SourceCreated,
			SourceID:  event.SourceID,
			Timestamp: event.Timestamp,
			Payload: infraevents.SourceCreatedPayload{
				Name:      source.Name,
				URL:       source.URL,
				RateLimit: source.RateLimit,
				MaxDepth:  source.MaxDepth,
				Enabled:   true,
				Priority:  source.Priority,
			},
		})
	}

	// Resume existing job
	job.Status = "pending"
	job.NextRunAt = timePtr(time.Now())
	job.IsPaused = false

	if upsertErr := s.repo.UpsertAutoManaged(ctx, job); upsertErr != nil {
		return fmt.Errorf("resume job: %w", upsertErr)
	}

	s.logInfo("Resumed job for enabled source",
		infralogger.String("source_id", event.SourceID.String()),
	)

	return s.repo.RecordProcessedEvent(ctx, event.EventID)
}

// HandleSourceDisabled pauses a job when a source is disabled.
func (s *Service) HandleSourceDisabled(ctx context.Context, event infraevents.SourceEvent) error {
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		return nil
	}

	if updateErr := s.repo.UpdateStatusBySourceID(ctx, event.SourceID, "paused"); updateErr != nil {
		// Job might not exist, that's okay
		s.logDebug("Could not pause job (may not exist)",
			infralogger.String("source_id", event.SourceID.String()),
		)
	} else {
		s.logInfo("Paused job for disabled source",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}

	return s.repo.RecordProcessedEvent(ctx, event.EventID)
}

// Helper functions for nil-safe logging.
func (s *Service) logInfo(msg string, fields ...infralogger.Field) {
	if s.log != nil {
		s.log.Info(msg, fields...)
	}
}

func (s *Service) logDebug(msg string, fields ...infralogger.Field) {
	if s.log != nil {
		s.log.Debug(msg, fields...)
	}
}

func (s *Service) logWarn(msg string, fields ...infralogger.Field) {
	if s.log != nil {
		s.log.Warn(msg, fields...)
	}
}

// timePtr returns a pointer to the given time.
func timePtr(t time.Time) *time.Time {
	return &t
}
```

### Step 4: Run tests to verify they pass

Run: `cd crawler && go test ./internal/job/... -v`
Expected: PASS (all tests)

### Step 5: Run linter

Run: `cd crawler && golangci-lint run ./internal/job/...`
Expected: 0 issues

### Step 6: Commit

```bash
git add crawler/internal/job/service.go crawler/internal/job/service_test.go
git commit -m "feat(crawler): add JobService for event-driven job lifecycle

Implement EventHandler interface with full job lifecycle management:
- HandleSourceCreated: Create auto-managed job with computed schedule
- HandleSourceUpdated: Reschedule job when rate_limit/depth/priority change
- HandleSourceDeleted: Delete associated job
- HandleSourceEnabled: Resume or create job
- HandleSourceDisabled: Pause job

Includes idempotency via processed_events table.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 7: Wire Up JobService in httpd.go

**Files:**
- Modify: `crawler/cmd/httpd/httpd.go`
- Modify: `crawler/internal/config/config.go`

### Step 1: Add SourceManagerURL config

Add to `crawler/internal/config/config.go` RedisConfig section:

```go
// Add to RedisConfig struct or create new config section
type SourceManagerConfig struct {
	URL string `env:"SOURCE_MANAGER_URL" yaml:"url"`
}

// Add field to main Config struct
SourceManager *SourceManagerConfig `yaml:"source_manager"`

// Add to Interface
GetSourceManagerConfig() *SourceManagerConfig

// Add getter method
func (c *Config) GetSourceManagerConfig() *SourceManagerConfig {
	if c.SourceManager == nil {
		return &SourceManagerConfig{
			URL: "http://localhost:8050",
		}
	}
	return c.SourceManager
}

// Add to setDefaults
if cfg.SourceManager == nil {
	cfg.SourceManager = &SourceManagerConfig{
		URL: "http://localhost:8050",
	}
}
```

### Step 2: Update setupEventConsumer to use JobService

Modify `crawler/cmd/httpd/httpd.go` setupEventConsumer function:

```go
// setupEventConsumer creates and starts the event consumer if Redis events are enabled.
// Returns nil if events are disabled or Redis is unavailable.
func setupEventConsumer(deps *CommandDeps, jobRepo *database.JobRepository) *crawlerintevents.Consumer {
	redisCfg := deps.Config.GetRedisConfig()
	if !redisCfg.Enabled {
		return nil
	}

	redisClient, err := infraredis.NewClient(infraredis.Config{
		Address:  redisCfg.Address,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
	if err != nil {
		deps.Logger.Warn("Redis not available, event consumer disabled",
			infralogger.Error(err),
		)
		return nil
	}

	// Create source client for fetching source data
	sourceManagerCfg := deps.Config.GetSourceManagerConfig()
	sourceClient := sources.NewHTTPClient(sourceManagerCfg.URL)

	// Create JobService as the event handler
	scheduleComputer := job.NewScheduleComputer()
	jobService := job.NewService(jobRepo, scheduleComputer, sourceClient, deps.Logger)

	consumer := crawlerintevents.NewConsumer(redisClient, "", jobService, deps.Logger)

	if startErr := consumer.Start(context.Background()); startErr != nil {
		deps.Logger.Error("Failed to start event consumer", infralogger.Error(startErr))
		return nil
	}

	deps.Logger.Info("Event consumer started with JobService handler")
	return consumer
}
```

### Step 3: Update Start function to pass jobRepo

Update the call to setupEventConsumer in the Start function:

```go
// Phase 4.5: Setup event consumer (if Redis events enabled)
eventConsumer := setupEventConsumer(deps, jsResult.JobRepo)
```

Note: JobsSchedulerResult needs to expose JobRepo. Add to the struct:
```go
JobRepo *database.JobRepository
```

And in setupJobsAndScheduler, add:
```go
return &JobsSchedulerResult{
	// ... existing fields
	JobRepo: jobRepo,
}, nil
```

### Step 4: Add imports

Add these imports to httpd.go:
```go
"github.com/jonesrussell/north-cloud/crawler/internal/job"
"github.com/jonesrussell/north-cloud/crawler/internal/sources"
```

### Step 5: Verify build

Run: `cd crawler && go build ./...`
Expected: Build succeeds

### Step 6: Run linter

Run: `cd crawler && golangci-lint run ./...`
Expected: 0 issues

### Step 7: Commit

```bash
git add crawler/cmd/httpd/httpd.go crawler/internal/config/config.go
git commit -m "feat(crawler): wire up JobService as event handler

Replace NoOpHandler with JobService for full event-driven job lifecycle.
Add SourceManagerConfig for fetching source metadata during events.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Task 8: Integration Test - Full Event Flow

**Files:**
- Modify: `docs/plans/phase1-testing.md` → `docs/plans/phase2-testing.md`

### Step 1: Create Phase 2 test documentation

```markdown
# Phase 2 Integration Test Results

## Date: YYYY-MM-DD

## Test: Full Job Lifecycle via Events

### Prerequisites

1. Redis running
2. PostgreSQL running with migrations applied
3. source-manager running
4. crawler running with `REDIS_EVENTS_ENABLED=true`

### Test 1: Source Created → Job Created

1. Create a source:
```bash
curl -X POST http://localhost:8050/api/v1/sources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Phase 2 Test Source",
    "url": "https://example.com",
    "rate_limit": 10,
    "max_depth": 2,
    "enabled": true,
    "priority": "high"
  }'
```

2. Verify job created in crawler:
```bash
curl http://localhost:8060/api/v1/jobs | jq '.[] | select(.source_name == "Phase 2 Test Source")'
```

Expected:
- [ ] Job exists with auto_managed=true
- [ ] Interval computed based on priority/rate_limit
- [ ] Status is "pending"

### Test 2: Source Updated → Job Rescheduled

1. Update the source:
```bash
curl -X PUT http://localhost:8050/api/v1/sources/{id} \
  -H "Content-Type: application/json" \
  -d '{
    "rate_limit": 5
  }'
```

2. Verify job interval changed:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id}
```

Expected:
- [ ] Interval increased (lower rate limit = longer interval)

### Test 3: Source Disabled → Job Paused

1. Disable the source:
```bash
curl -X POST http://localhost:8050/api/v1/sources/{id}/disable
```

2. Verify job paused:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id}
```

Expected:
- [ ] Status is "paused"

### Test 4: Source Enabled → Job Resumed

1. Enable the source:
```bash
curl -X POST http://localhost:8050/api/v1/sources/{id}/enable
```

2. Verify job resumed:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id}
```

Expected:
- [ ] Status is "pending"
- [ ] next_run_at is near current time

### Test 5: Source Deleted → Job Deleted

1. Delete the source:
```bash
curl -X DELETE http://localhost:8050/api/v1/sources/{id}
```

2. Verify job deleted:
```bash
curl http://localhost:8060/api/v1/jobs | jq '.[] | select(.source_name == "Phase 2 Test Source")'
```

Expected:
- [ ] Job no longer exists

### Test 6: Idempotency

1. Note event count in processed_events table
2. Restart crawler
3. Wait for event replay
4. Verify no duplicate jobs created

Expected:
- [ ] Same number of jobs before and after restart

## Notes

...
```

### Step 2: Commit

```bash
git add docs/plans/phase2-testing.md
git commit -m "docs: add Phase 2 integration test documentation

Document full job lifecycle testing via events:
- Source created → Job created
- Source updated → Job rescheduled
- Source disabled → Job paused
- Source enabled → Job resumed
- Source deleted → Job deleted
- Idempotency verification

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Phase 2 Complete - Summary

### Files Created
- `crawler/migrations/009_add_auto_managed_jobs.up.sql`
- `crawler/migrations/009_add_auto_managed_jobs.down.sql`
- `crawler/internal/job/schedule_computer.go`
- `crawler/internal/job/schedule_computer_test.go`
- `crawler/internal/job/service.go`
- `crawler/internal/job/service_test.go`
- `crawler/internal/sources/client.go`
- `crawler/internal/sources/client_test.go`
- `crawler/internal/database/job_repository_automgmt_test.go`
- `docs/plans/phase2-testing.md`

### Files Modified
- `crawler/internal/domain/job.go` - Added auto-managed fields
- `crawler/internal/database/job_repository.go` - Added auto-managed methods
- `crawler/internal/config/config.go` - Added SourceManagerConfig
- `crawler/cmd/httpd/httpd.go` - Wired up JobService

### Success Criteria
- [ ] Sources automatically get jobs created when enabled
- [ ] Source updates propagate to job schedules
- [ ] Disabled sources pause their jobs
- [ ] Deleted sources delete their jobs
- [ ] Idempotency prevents duplicate job creation
- [ ] All existing tests pass
- [ ] Linter passes with 0 issues

---

**Next Phase:** Phase 3 will migrate existing sources to auto-managed jobs and deprecate manual job creation.
