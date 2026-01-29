# Job Logging Pipeline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a production-grade job logging pipeline that routes all crawler logs through a unified JobLogger, with configurable verbosity, buffered SSE streaming, throttling, and rich job summaries.

**Architecture:** The crawler's ~45 `c.logger.*` calls are replaced with `jobLogger.*` calls. The JobLogger writes to: (1) in-memory ring buffer for SSE replay, (2) SSE publisher for live streaming, (3) MinIO archive buffer, and (4) optional stdout mirror. Token bucket throttling protects debug/trace modes.

**Tech Stack:** Go 1.24+, Vue.js 3 + TypeScript, SSE, MinIO, PostgreSQL

**Design Document:** `docs/plans/2026-01-28-job-logging-pipeline-design.md`

---

## Phase 1: Core Types and Interfaces

### Task 1.1: Create Verbosity Types

**Files:**
- Create: `crawler/internal/logs/verbosity.go`
- Test: `crawler/internal/logs/verbosity_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/verbosity_test.go
package logs

import (
	"testing"
)

func TestVerbosityParse(t *testing.T) {
	t.Helper()

	tests := []struct {
		input    string
		expected Verbosity
		valid    bool
	}{
		{"quiet", VerbosityQuiet, true},
		{"normal", VerbosityNormal, true},
		{"debug", VerbosityDebug, true},
		{"trace", VerbosityTrace, true},
		{"NORMAL", VerbosityNormal, true},
		{"invalid", "", false},
		{"", VerbosityNormal, true}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := ParseVerbosity(tt.input)
			if tt.valid {
				if err != nil {
					t.Errorf("ParseVerbosity(%q) unexpected error: %v", tt.input, err)
				}
				if v != tt.expected {
					t.Errorf("ParseVerbosity(%q) = %q, want %q", tt.input, v, tt.expected)
				}
			} else {
				if err == nil {
					t.Errorf("ParseVerbosity(%q) expected error, got nil", tt.input)
				}
			}
		})
	}
}

func TestVerbosityAllowsLevel(t *testing.T) {
	t.Helper()

	tests := []struct {
		verbosity Verbosity
		level     string
		allowed   bool
	}{
		{VerbosityQuiet, "info", true},
		{VerbosityQuiet, "debug", false},
		{VerbosityNormal, "info", true},
		{VerbosityNormal, "debug", false},
		{VerbosityDebug, "info", true},
		{VerbosityDebug, "debug", true},
		{VerbosityTrace, "debug", true},
	}

	for _, tt := range tests {
		name := string(tt.verbosity) + "_" + tt.level
		t.Run(name, func(t *testing.T) {
			if got := tt.verbosity.AllowsLevel(tt.level); got != tt.allowed {
				t.Errorf("%s.AllowsLevel(%q) = %v, want %v", tt.verbosity, tt.level, got, tt.allowed)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs/... -run TestVerbosity -v`
Expected: FAIL with "undefined: Verbosity"

**Step 3: Write minimal implementation**

```go
// crawler/internal/logs/verbosity.go
package logs

import (
	"errors"
	"strings"
)

// Verbosity defines the log detail level for job execution.
type Verbosity string

const (
	// VerbosityQuiet - Only scheduler + major crawler milestones (~5-10 lines/job)
	VerbosityQuiet Verbosity = "quiet"

	// VerbosityNormal - Pages discovered, crawled, items extracted, errors (~50-100 lines/job)
	VerbosityNormal Verbosity = "normal"

	// VerbosityDebug - Every URL visited, timing info, extraction details (~500-2000 lines/job)
	VerbosityDebug Verbosity = "debug"

	// VerbosityTrace - Extremely detailed, local debugging only (unlimited)
	VerbosityTrace Verbosity = "trace"
)

// ErrInvalidVerbosity is returned when parsing an unknown verbosity level.
var ErrInvalidVerbosity = errors.New("invalid verbosity level")

// ParseVerbosity parses a string into a Verbosity level.
// Empty string defaults to VerbosityNormal.
func ParseVerbosity(s string) (Verbosity, error) {
	if s == "" {
		return VerbosityNormal, nil
	}

	switch strings.ToLower(s) {
	case "quiet":
		return VerbosityQuiet, nil
	case "normal":
		return VerbosityNormal, nil
	case "debug":
		return VerbosityDebug, nil
	case "trace":
		return VerbosityTrace, nil
	default:
		return "", ErrInvalidVerbosity
	}
}

// AllowsLevel returns true if this verbosity level allows the given log level.
func (v Verbosity) AllowsLevel(level string) bool {
	switch level {
	case "info", "warn", "error":
		return true // Always allowed
	case "debug":
		return v == VerbosityDebug || v == VerbosityTrace
	case "trace":
		return v == VerbosityTrace
	default:
		return true
	}
}

// String returns the verbosity as a string.
func (v Verbosity) String() string {
	return string(v)
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs/... -run TestVerbosity -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/logs/verbosity.go crawler/internal/logs/verbosity_test.go
git commit -m "feat(logs): add verbosity levels for job logging"
```

---

### Task 1.2: Create Category Types

**Files:**
- Create: `crawler/internal/logs/category.go`
- Test: `crawler/internal/logs/category_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/category_test.go
package logs

import (
	"testing"
)

func TestCategoryString(t *testing.T) {
	t.Helper()

	tests := []struct {
		category Category
		expected string
	}{
		{CategoryLifecycle, "crawler.lifecycle"},
		{CategoryFetch, "crawler.fetch"},
		{CategoryExtract, "crawler.extract"},
		{CategoryError, "crawler.error"},
		{CategoryRateLimit, "crawler.rate_limit"},
		{CategoryQueue, "crawler.queue"},
		{CategoryMetrics, "crawler.metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.String(); got != tt.expected {
				t.Errorf("Category.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCategoryShortName(t *testing.T) {
	t.Helper()

	tests := []struct {
		category Category
		expected string
	}{
		{CategoryLifecycle, "lifecycle"},
		{CategoryFetch, "fetch"},
		{CategoryError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.ShortName(); got != tt.expected {
				t.Errorf("Category.ShortName() = %q, want %q", got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs/... -run TestCategory -v`
Expected: FAIL with "undefined: Category"

**Step 3: Write minimal implementation**

```go
// crawler/internal/logs/category.go
package logs

import "strings"

// Category defines the type of log event for filtering.
type Category string

const (
	// CategoryLifecycle - start, stop, shutdown, summary, heartbeat
	CategoryLifecycle Category = "crawler.lifecycle"

	// CategoryFetch - URL fetch attempts, retries, status codes
	CategoryFetch Category = "crawler.fetch"

	// CategoryExtract - content extraction, selectors, parse results
	CategoryExtract Category = "crawler.extract"

	// CategoryError - network errors, parse failures, unexpected conditions
	CategoryError Category = "crawler.error"

	// CategoryRateLimit - throttling, backoff, delays
	CategoryRateLimit Category = "crawler.rate_limit"

	// CategoryQueue - enqueue/dequeue events, depth changes
	CategoryQueue Category = "crawler.queue"

	// CategoryMetrics - timing, durations, counts, throttle warnings
	CategoryMetrics Category = "crawler.metrics"
)

// String returns the category as a string.
func (c Category) String() string {
	return string(c)
}

// ShortName returns the category without the "crawler." prefix.
func (c Category) ShortName() string {
	return strings.TrimPrefix(string(c), "crawler.")
}

// AllCategories returns all valid categories.
func AllCategories() []Category {
	return []Category{
		CategoryLifecycle,
		CategoryFetch,
		CategoryExtract,
		CategoryError,
		CategoryRateLimit,
		CategoryQueue,
		CategoryMetrics,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs/... -run TestCategory -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/logs/category.go crawler/internal/logs/category_test.go
git commit -m "feat(logs): add log categories for event filtering"
```

---

### Task 1.3: Create Field Helpers

**Files:**
- Create: `crawler/internal/logs/field.go`
- Test: `crawler/internal/logs/field_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/field_test.go
package logs

import (
	"errors"
	"testing"
	"time"
)

func TestFieldConstructors(t *testing.T) {
	t.Helper()

	t.Run("String", func(t *testing.T) {
		f := String("key", "value")
		if f.Key != "key" || f.Value != "value" {
			t.Errorf("String() = {%q, %v}, want {key, value}", f.Key, f.Value)
		}
	})

	t.Run("Int", func(t *testing.T) {
		f := Int("count", 42)
		if f.Key != "count" || f.Value != 42 {
			t.Errorf("Int() = {%q, %v}, want {count, 42}", f.Key, f.Value)
		}
	})

	t.Run("Int64", func(t *testing.T) {
		f := Int64("big", int64(1234567890))
		if f.Key != "big" || f.Value != int64(1234567890) {
			t.Errorf("Int64() = {%q, %v}, want {big, 1234567890}", f.Key, f.Value)
		}
	})

	t.Run("Duration", func(t *testing.T) {
		f := Duration("elapsed", 1500*time.Millisecond)
		if f.Key != "elapsed_ms" || f.Value != int64(1500) {
			t.Errorf("Duration() = {%q, %v}, want {elapsed_ms, 1500}", f.Key, f.Value)
		}
	})

	t.Run("URL", func(t *testing.T) {
		f := URL("https://example.com")
		if f.Key != "url" || f.Value != "https://example.com" {
			t.Errorf("URL() = {%q, %v}, want {url, https://example.com}", f.Key, f.Value)
		}
	})

	t.Run("Err", func(t *testing.T) {
		err := errors.New("something failed")
		f := Err(err)
		if f.Key != "error" || f.Value != "something failed" {
			t.Errorf("Err() = {%q, %v}, want {error, something failed}", f.Key, f.Value)
		}
	})

	t.Run("Err_nil", func(t *testing.T) {
		f := Err(nil)
		if f.Key != "error" || f.Value != "" {
			t.Errorf("Err(nil) = {%q, %v}, want {error, \"\"}", f.Key, f.Value)
		}
	})

	t.Run("Bool", func(t *testing.T) {
		f := Bool("enabled", true)
		if f.Key != "enabled" || f.Value != true {
			t.Errorf("Bool() = {%q, %v}, want {enabled, true}", f.Key, f.Value)
		}
	})
}

func TestFieldsToMap(t *testing.T) {
	t.Helper()

	fields := []Field{
		String("name", "test"),
		Int("count", 5),
		URL("https://example.com"),
	}

	m := FieldsToMap(fields)

	if m["name"] != "test" {
		t.Errorf("m[name] = %v, want test", m["name"])
	}
	if m["count"] != 5 {
		t.Errorf("m[count] = %v, want 5", m["count"])
	}
	if m["url"] != "https://example.com" {
		t.Errorf("m[url] = %v, want https://example.com", m["url"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs/... -run TestField -v`
Expected: FAIL with "undefined: Field"

**Step 3: Write minimal implementation**

```go
// crawler/internal/logs/field.go
package logs

import "time"

// Field represents a structured log field.
type Field struct {
	Key   string
	Value any
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a bool field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Duration creates a duration field (stored as milliseconds).
func Duration(key string, d time.Duration) Field {
	return Field{Key: key + "_ms", Value: d.Milliseconds()}
}

// URL creates a URL field.
func URL(url string) Field {
	return Field{Key: "url", Value: url}
}

// Err creates an error field.
func Err(err error) Field {
	if err == nil {
		return Field{Key: "error", Value: ""}
	}
	return Field{Key: "error", Value: err.Error()}
}

// Any creates a field with any value.
func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// FieldsToMap converts a slice of fields to a map.
func FieldsToMap(fields []Field) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value
	}
	return m
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs/... -run TestField -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/logs/field.go crawler/internal/logs/field_test.go
git commit -m "feat(logs): add structured field helpers for job logging"
```

---

### Task 1.4: Create Token Bucket Rate Limiter

**Files:**
- Create: `crawler/internal/logs/throttle.go`
- Test: `crawler/internal/logs/throttle_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/throttle_test.go
package logs

import (
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	t.Helper()

	t.Run("nil limiter always allows", func(t *testing.T) {
		var r *rateLimiter
		for i := 0; i < 100; i++ {
			if !r.Allow() {
				t.Errorf("nil rateLimiter.Allow() = false, want true")
			}
		}
	})

	t.Run("zero rate disables limiting", func(t *testing.T) {
		r := newRateLimiter(0)
		if r != nil {
			t.Errorf("newRateLimiter(0) should return nil")
		}
	})

	t.Run("respects rate limit", func(t *testing.T) {
		r := newRateLimiter(10) // 10 per second

		// Should allow first 10
		allowed := 0
		for i := 0; i < 15; i++ {
			if r.Allow() {
				allowed++
			}
		}

		if allowed != 10 {
			t.Errorf("allowed %d, want 10", allowed)
		}
	})

	t.Run("refills over time", func(t *testing.T) {
		r := newRateLimiter(10)

		// Exhaust tokens
		for i := 0; i < 10; i++ {
			r.Allow()
		}

		// Should be denied
		if r.Allow() {
			t.Error("should be denied after exhausting tokens")
		}

		// Wait for refill
		time.Sleep(150 * time.Millisecond)

		// Should allow again (1-2 tokens refilled)
		if !r.Allow() {
			t.Error("should allow after refill")
		}
	})
}

func TestRateLimiter_Stats(t *testing.T) {
	t.Helper()

	t.Run("nil limiter returns zeros", func(t *testing.T) {
		var r *rateLimiter
		tokens, max := r.Stats()
		if tokens != 0 || max != 0 {
			t.Errorf("nil Stats() = (%v, %v), want (0, 0)", tokens, max)
		}
	})

	t.Run("returns current state", func(t *testing.T) {
		r := newRateLimiter(50)
		_, max := r.Stats()
		if max != 50 {
			t.Errorf("Stats() max = %v, want 50", max)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs/... -run TestRateLimiter -v`
Expected: FAIL with "undefined: rateLimiter"

**Step 3: Write minimal implementation**

```go
// crawler/internal/logs/throttle.go
package logs

import (
	"sync"
	"time"
)

// rateLimiter implements a token bucket rate limiter.
type rateLimiter struct {
	maxPerSecond int
	tokens       float64
	lastRefill   time.Time
	mu           sync.Mutex
}

// newRateLimiter creates a new rate limiter.
// Returns nil if maxPerSecond <= 0 (disabled).
func newRateLimiter(maxPerSecond int) *rateLimiter {
	if maxPerSecond <= 0 {
		return nil
	}
	return &rateLimiter{
		maxPerSecond: maxPerSecond,
		tokens:       float64(maxPerSecond),
		lastRefill:   time.Now(),
	}
}

// Allow returns true if a log can be emitted, false if throttled.
func (r *rateLimiter) Allow() bool {
	if r == nil {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * float64(r.maxPerSecond)
	if r.tokens > float64(r.maxPerSecond) {
		r.tokens = float64(r.maxPerSecond)
	}
	r.lastRefill = now

	// Check if we have a token
	if r.tokens >= 1 {
		r.tokens--
		return true
	}

	return false
}

// Stats returns current throttler state for diagnostics.
func (r *rateLimiter) Stats() (tokens float64, maxPerSec int) {
	if r == nil {
		return 0, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tokens, r.maxPerSecond
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs/... -run TestRateLimiter -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/logs/throttle.go crawler/internal/logs/throttle_test.go
git commit -m "feat(logs): add token bucket rate limiter for log throttling"
```

---

### Task 1.5: Update LogEntry with Schema Version and Category

**Files:**
- Modify: `crawler/internal/logs/types.go`
- Test: `crawler/internal/logs/types_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/types_test.go
package logs

import (
	"encoding/json"
	"testing"
	"time"
)

func TestLogEntry_JSON(t *testing.T) {
	t.Helper()

	entry := LogEntry{
		SchemaVersion: CurrentSchemaVersion,
		Timestamp:     time.Date(2026, 1, 28, 12, 0, 0, 0, time.UTC),
		Level:         "info",
		Category:      string(CategoryFetch),
		Message:       "Page fetched",
		JobID:         "job-123",
		ExecID:        "exec-456",
		Fields: map[string]any{
			"url":    "https://example.com",
			"status": 200,
		},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded LogEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", decoded.SchemaVersion, CurrentSchemaVersion)
	}
	if decoded.Category != string(CategoryFetch) {
		t.Errorf("Category = %q, want %q", decoded.Category, CategoryFetch)
	}
	if decoded.Message != "Page fetched" {
		t.Errorf("Message = %q, want %q", decoded.Message, "Page fetched")
	}
}

func TestCurrentSchemaVersion(t *testing.T) {
	t.Helper()

	if CurrentSchemaVersion != 1 {
		t.Errorf("CurrentSchemaVersion = %d, want 1", CurrentSchemaVersion)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs/... -run TestLogEntry -v`
Expected: FAIL with "undefined: CurrentSchemaVersion" or field missing

**Step 3: Update implementation**

```go
// In crawler/internal/logs/types.go, update LogEntry struct:

// CurrentSchemaVersion is the current log entry schema version.
const CurrentSchemaVersion = 1

// LogEntry represents a single log line captured during job execution.
type LogEntry struct {
	SchemaVersion int            `json:"schema_version"`
	Timestamp     time.Time      `json:"timestamp"`
	Level         string         `json:"level"`
	Category      string         `json:"category"`
	Message       string         `json:"message"`
	JobID         string         `json:"job_id"`
	ExecID        string         `json:"execution_id"`
	Fields        map[string]any `json:"fields,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs/... -run TestLogEntry -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/logs/types.go crawler/internal/logs/types_test.go
git commit -m "feat(logs): add schema version and category to LogEntry"
```

---

### Task 1.6: Create JobLogger Interface

**Files:**
- Create: `crawler/internal/logs/job_logger.go`

**Step 1: Write the interface**

```go
// crawler/internal/logs/job_logger.go
package logs

import (
	"context"
)

// JobLogger provides structured logging for job execution.
// All methods are safe for concurrent use.
type JobLogger interface {
	// Core logging methods
	Info(category Category, msg string, fields ...Field)
	Warn(category Category, msg string, fields ...Field)
	Error(category Category, msg string, fields ...Field)
	Debug(category Category, msg string, fields ...Field)

	// Lifecycle events (always logged regardless of verbosity)
	JobStarted(sourceID, url string)
	JobCompleted(summary *JobSummary)
	JobFailed(err error)

	// Verbosity check (for expensive operations)
	IsDebugEnabled() bool
	IsTraceEnabled() bool

	// WithFields returns a scoped logger with pre-set fields.
	WithFields(fields ...Field) JobLogger

	// StartHeartbeat starts the heartbeat goroutine.
	StartHeartbeat(ctx context.Context)

	// BuildSummary returns the current metrics as a summary.
	BuildSummary() *JobSummary

	// Flush pending logs (called at job end)
	Flush() error
}

// JobSummary contains final statistics appended to job logs.
type JobSummary struct {
	// Core metrics
	PagesDiscovered int64 `json:"pages_discovered"`
	PagesCrawled    int64 `json:"pages_crawled"`
	ItemsExtracted  int64 `json:"items_extracted"`
	ErrorsCount     int64 `json:"errors_count"`

	// Duration breakdown
	Duration        int64 `json:"duration_ms"`
	CrawlDuration   int64 `json:"crawl_duration_ms,omitempty"`
	ExtractDuration int64 `json:"extract_duration_ms,omitempty"`
	BackoffDuration int64 `json:"backoff_duration_ms,omitempty"`

	// Network stats
	BytesFetched   int64 `json:"bytes_fetched,omitempty"`
	RequestsTotal  int64 `json:"requests_total,omitempty"`
	RequestsFailed int64 `json:"requests_failed,omitempty"`

	// Queue behavior
	QueueMaxDepth int64 `json:"queue_max_depth,omitempty"`
	QueueEnqueued int64 `json:"queue_enqueued,omitempty"`
	QueueDequeued int64 `json:"queue_dequeued,omitempty"`

	// Status code breakdown
	StatusCodes map[int]int64 `json:"status_codes,omitempty"`

	// Top errors (deduplicated, max 5)
	TopErrors []ErrorSummary `json:"top_errors,omitempty"`

	// Throttling stats
	LogsEmitted     int64   `json:"logs_emitted"`
	LogsThrottled   int64   `json:"logs_throttled,omitempty"`
	ThrottlePercent float64 `json:"throttle_percent,omitempty"`
}

// ErrorSummary summarizes a repeated error.
type ErrorSummary struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
	LastURL string `json:"last_url,omitempty"`
}
```

**Step 2: Verify it compiles**

Run: `cd crawler && go build ./internal/logs/...`
Expected: Success

**Step 3: Commit**

```bash
git add crawler/internal/logs/job_logger.go
git commit -m "feat(logs): add JobLogger interface and JobSummary types"
```

---

### Task 1.7: Create NoopJobLogger

**Files:**
- Create: `crawler/internal/logs/noop_logger.go`
- Test: `crawler/internal/logs/noop_logger_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/noop_logger_test.go
package logs

import (
	"context"
	"testing"
)

func TestNoopJobLogger(t *testing.T) {
	t.Helper()

	logger := NoopJobLogger()

	// Should not panic
	logger.Info(CategoryFetch, "test")
	logger.Warn(CategoryError, "test")
	logger.Error(CategoryError, "test")
	logger.Debug(CategoryQueue, "test")
	logger.JobStarted("source", "url")
	logger.JobCompleted(&JobSummary{})
	logger.JobFailed(nil)
	logger.StartHeartbeat(context.Background())

	if logger.IsDebugEnabled() {
		t.Error("NoopJobLogger.IsDebugEnabled() should return false")
	}

	if logger.IsTraceEnabled() {
		t.Error("NoopJobLogger.IsTraceEnabled() should return false")
	}

	scoped := logger.WithFields(String("key", "value"))
	if scoped == nil {
		t.Error("NoopJobLogger.WithFields() should return non-nil")
	}

	summary := logger.BuildSummary()
	if summary == nil {
		t.Error("NoopJobLogger.BuildSummary() should return non-nil")
	}

	if err := logger.Flush(); err != nil {
		t.Errorf("NoopJobLogger.Flush() unexpected error: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs/... -run TestNoopJobLogger -v`
Expected: FAIL with "undefined: NoopJobLogger"

**Step 3: Write minimal implementation**

```go
// crawler/internal/logs/noop_logger.go
package logs

import "context"

// noopJobLogger is a no-op implementation of JobLogger.
type noopJobLogger struct{}

// NoopJobLogger returns a JobLogger that does nothing.
// Useful as a fallback when no logger is configured.
func NoopJobLogger() JobLogger {
	return &noopJobLogger{}
}

func (n *noopJobLogger) Info(_ Category, _ string, _ ...Field)  {}
func (n *noopJobLogger) Warn(_ Category, _ string, _ ...Field)  {}
func (n *noopJobLogger) Error(_ Category, _ string, _ ...Field) {}
func (n *noopJobLogger) Debug(_ Category, _ string, _ ...Field) {}
func (n *noopJobLogger) JobStarted(_, _ string)                 {}
func (n *noopJobLogger) JobCompleted(_ *JobSummary)             {}
func (n *noopJobLogger) JobFailed(_ error)                      {}
func (n *noopJobLogger) StartHeartbeat(_ context.Context)       {}
func (n *noopJobLogger) IsDebugEnabled() bool                   { return false }
func (n *noopJobLogger) IsTraceEnabled() bool                   { return false }
func (n *noopJobLogger) WithFields(_ ...Field) JobLogger        { return n }
func (n *noopJobLogger) BuildSummary() *JobSummary              { return &JobSummary{} }
func (n *noopJobLogger) Flush() error                           { return nil }

// Ensure noopJobLogger implements JobLogger
var _ JobLogger = (*noopJobLogger)(nil)
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs/... -run TestNoopJobLogger -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/logs/noop_logger.go crawler/internal/logs/noop_logger_test.go
git commit -m "feat(logs): add NoopJobLogger for fallback scenarios"
```

---

### Task 1.8: Run All Phase 1 Tests

**Step 1: Run all logs package tests**

Run: `cd crawler && go test ./internal/logs/... -v`
Expected: All tests pass

**Step 2: Run linter**

Run: `cd crawler && golangci-lint run ./internal/logs/...`
Expected: No errors

**Step 3: Commit phase completion marker**

```bash
git commit --allow-empty -m "chore(logs): complete Phase 1 - core types and interfaces"
```

---

## Phase 2: JobLogger Implementation

### Task 2.1: Create Log Metrics Collector

**Files:**
- Create: `crawler/internal/logs/metrics.go`
- Test: `crawler/internal/logs/metrics_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/metrics_test.go
package logs

import (
	"testing"
)

func TestLogMetrics_Counters(t *testing.T) {
	t.Helper()

	m := newLogMetrics()

	m.IncrementPagesDiscovered()
	m.IncrementPagesDiscovered()
	m.IncrementPagesCrawled()
	m.IncrementItemsExtracted()
	m.IncrementErrors()
	m.IncrementLogsEmitted()
	m.IncrementThrottled()

	if got := m.pagesDiscovered.Load(); got != 2 {
		t.Errorf("pagesDiscovered = %d, want 2", got)
	}
	if got := m.pagesCrawled.Load(); got != 1 {
		t.Errorf("pagesCrawled = %d, want 1", got)
	}
	if got := m.itemsExtracted.Load(); got != 1 {
		t.Errorf("itemsExtracted = %d, want 1", got)
	}
	if got := m.errorsCount.Load(); got != 1 {
		t.Errorf("errorsCount = %d, want 1", got)
	}
}

func TestLogMetrics_StatusCodes(t *testing.T) {
	t.Helper()

	m := newLogMetrics()

	m.RecordStatusCode(200)
	m.RecordStatusCode(200)
	m.RecordStatusCode(404)

	summary := m.BuildSummary()

	if summary.StatusCodes[200] != 2 {
		t.Errorf("StatusCodes[200] = %d, want 2", summary.StatusCodes[200])
	}
	if summary.StatusCodes[404] != 1 {
		t.Errorf("StatusCodes[404] = %d, want 1", summary.StatusCodes[404])
	}
}

func TestLogMetrics_TopErrors(t *testing.T) {
	t.Helper()

	m := newLogMetrics()

	m.RecordError("timeout", "https://a.com")
	m.RecordError("timeout", "https://b.com")
	m.RecordError("connection refused", "https://c.com")

	summary := m.BuildSummary()

	if len(summary.TopErrors) != 2 {
		t.Fatalf("TopErrors length = %d, want 2", len(summary.TopErrors))
	}

	// Timeout should be first (count=2)
	if summary.TopErrors[0].Message != "timeout" || summary.TopErrors[0].Count != 2 {
		t.Errorf("TopErrors[0] = %+v, want timeout with count 2", summary.TopErrors[0])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs/... -run TestLogMetrics -v`
Expected: FAIL with "undefined: newLogMetrics"

**Step 3: Write minimal implementation**

```go
// crawler/internal/logs/metrics.go
package logs

import (
	"sort"
	"sync"
	"sync/atomic"
)

// logMetrics collects metrics during job execution.
type logMetrics struct {
	pagesDiscovered atomic.Int64
	pagesCrawled    atomic.Int64
	itemsExtracted  atomic.Int64
	errorsCount     atomic.Int64
	bytesReceived   atomic.Int64
	requestsTotal   atomic.Int64
	requestsFailed  atomic.Int64
	logsEmitted     atomic.Int64
	logsThrottled   atomic.Int64
	queueDepth      atomic.Int64
	queueMaxDepth   atomic.Int64
	queueEnqueued   atomic.Int64
	queueDequeued   atomic.Int64

	statusCodes sync.Map // map[int]*atomic.Int64
	errorCounts sync.Map // map[string]*errorTracker
}

type errorTracker struct {
	count   atomic.Int64
	lastURL atomic.Value // string
}

func newLogMetrics() *logMetrics {
	return &logMetrics{}
}

func (m *logMetrics) IncrementPagesDiscovered() { m.pagesDiscovered.Add(1) }
func (m *logMetrics) IncrementPagesCrawled()    { m.pagesCrawled.Add(1) }
func (m *logMetrics) IncrementItemsExtracted()  { m.itemsExtracted.Add(1) }
func (m *logMetrics) IncrementErrors()          { m.errorsCount.Add(1) }
func (m *logMetrics) IncrementLogsEmitted()     { m.logsEmitted.Add(1) }
func (m *logMetrics) IncrementThrottled()       { m.logsThrottled.Add(1) }

func (m *logMetrics) RecordStatusCode(code int) {
	counter, _ := m.statusCodes.LoadOrStore(code, &atomic.Int64{})
	counter.(*atomic.Int64).Add(1)
}

func (m *logMetrics) RecordError(msg, url string) {
	tracker, _ := m.errorCounts.LoadOrStore(msg, &errorTracker{})
	t := tracker.(*errorTracker)
	t.count.Add(1)
	t.lastURL.Store(url)
}

func (m *logMetrics) BuildSummary() *JobSummary {
	summary := &JobSummary{
		PagesDiscovered: m.pagesDiscovered.Load(),
		PagesCrawled:    m.pagesCrawled.Load(),
		ItemsExtracted:  m.itemsExtracted.Load(),
		ErrorsCount:     m.errorsCount.Load(),
		BytesFetched:    m.bytesReceived.Load(),
		RequestsTotal:   m.requestsTotal.Load(),
		RequestsFailed:  m.requestsFailed.Load(),
		LogsEmitted:     m.logsEmitted.Load(),
		LogsThrottled:   m.logsThrottled.Load(),
		QueueMaxDepth:   m.queueMaxDepth.Load(),
		QueueEnqueued:   m.queueEnqueued.Load(),
		QueueDequeued:   m.queueDequeued.Load(),
		StatusCodes:     make(map[int]int64),
	}

	// Collect status codes
	m.statusCodes.Range(func(key, value any) bool {
		summary.StatusCodes[key.(int)] = value.(*atomic.Int64).Load()
		return true
	})

	// Collect top 5 errors
	const maxTopErrors = 5
	summary.TopErrors = m.getTopErrors(maxTopErrors)

	// Calculate throttle percent
	total := summary.LogsEmitted + summary.LogsThrottled
	if total > 0 {
		summary.ThrottlePercent = float64(summary.LogsThrottled) / float64(total) * 100
	}

	return summary
}

func (m *logMetrics) getTopErrors(n int) []ErrorSummary {
	var errors []ErrorSummary

	m.errorCounts.Range(func(key, value any) bool {
		t := value.(*errorTracker)
		lastURL, _ := t.lastURL.Load().(string)
		errors = append(errors, ErrorSummary{
			Message: key.(string),
			Count:   int(t.count.Load()),
			LastURL: lastURL,
		})
		return true
	})

	sort.Slice(errors, func(i, j int) bool {
		return errors[i].Count > errors[j].Count
	})

	if len(errors) > n {
		errors = errors[:n]
	}

	return errors
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs/... -run TestLogMetrics -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/logs/metrics.go crawler/internal/logs/metrics_test.go
git commit -m "feat(logs): add metrics collector for job summary"
```

---

### Task 2.2: Create JobLogger Implementation

**Files:**
- Create: `crawler/internal/logs/job_logger_impl.go`
- Test: `crawler/internal/logs/job_logger_impl_test.go`

**Step 1: Write the failing test**

```go
// crawler/internal/logs/job_logger_impl_test.go
package logs

import (
	"sync/atomic"
	"testing"
	"time"
)

// mockBuffer implements Buffer for testing
type mockBuffer struct {
	entries []LogEntry
}

func (b *mockBuffer) Write(entry LogEntry)    { b.entries = append(b.entries, entry) }
func (b *mockBuffer) ReadAll() []LogEntry     { return b.entries }
func (b *mockBuffer) ReadLast(n int) []LogEntry {
	if n > len(b.entries) {
		n = len(b.entries)
	}
	return b.entries[len(b.entries)-n:]
}
func (b *mockBuffer) Size() int               { return len(b.entries) }
func (b *mockBuffer) Bytes() []byte           { return nil }
func (b *mockBuffer) LineCount() int          { return len(b.entries) }
func (b *mockBuffer) Clear()                  { b.entries = nil }

// mockPublisher implements Publisher for testing
type mockPublisher struct {
	published []LogEntry
}

func (p *mockPublisher) PublishLogLine(_ any, entry LogEntry) {
	p.published = append(p.published, entry)
}
func (p *mockPublisher) PublishLogArchived(_ any, _ *LogMetadata) {}

func TestJobLoggerImpl_Info(t *testing.T) {
	t.Helper()

	buf := &mockBuffer{}
	pub := &mockPublisher{}

	logger := NewJobLogger(
		"job-123",
		"exec-456",
		VerbosityNormal,
		buf,
		pub,
		nil, // no stdout
		0,   // no throttling
	)

	logger.Info(CategoryFetch, "Page fetched", URL("https://example.com"), Int("status", 200))

	if len(buf.entries) != 1 {
		t.Fatalf("buffer has %d entries, want 1", len(buf.entries))
	}

	entry := buf.entries[0]
	if entry.Level != "info" {
		t.Errorf("Level = %q, want info", entry.Level)
	}
	if entry.Category != string(CategoryFetch) {
		t.Errorf("Category = %q, want %s", entry.Category, CategoryFetch)
	}
	if entry.Message != "Page fetched" {
		t.Errorf("Message = %q, want 'Page fetched'", entry.Message)
	}
	if entry.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", entry.SchemaVersion, CurrentSchemaVersion)
	}
}

func TestJobLoggerImpl_Debug_Verbosity(t *testing.T) {
	t.Helper()

	t.Run("normal verbosity skips debug", func(t *testing.T) {
		buf := &mockBuffer{}
		logger := NewJobLogger("job", "exec", VerbosityNormal, buf, nil, nil, 0)
		logger.Debug(CategoryQueue, "test")
		if len(buf.entries) != 0 {
			t.Error("debug log should be skipped at normal verbosity")
		}
	})

	t.Run("debug verbosity allows debug", func(t *testing.T) {
		buf := &mockBuffer{}
		logger := NewJobLogger("job", "exec", VerbosityDebug, buf, nil, nil, 0)
		logger.Debug(CategoryQueue, "test")
		if len(buf.entries) != 1 {
			t.Error("debug log should be allowed at debug verbosity")
		}
	})
}

func TestJobLoggerImpl_Throttling(t *testing.T) {
	t.Helper()

	buf := &mockBuffer{}
	logger := NewJobLogger("job", "exec", VerbosityDebug, buf, nil, nil, 5) // 5 per sec

	// Should allow first 5
	for i := 0; i < 10; i++ {
		logger.Debug(CategoryQueue, "test")
	}

	if len(buf.entries) != 5 {
		t.Errorf("buffer has %d entries, want 5 (throttled)", len(buf.entries))
	}

	summary := logger.BuildSummary()
	if summary.LogsThrottled != 5 {
		t.Errorf("LogsThrottled = %d, want 5", summary.LogsThrottled)
	}
}

func TestJobLoggerImpl_WithFields(t *testing.T) {
	t.Helper()

	buf := &mockBuffer{}
	logger := NewJobLogger("job", "exec", VerbosityNormal, buf, nil, nil, 0)

	scoped := logger.WithFields(URL("https://example.com"), Int("depth", 2))
	scoped.Info(CategoryQueue, "Link enqueued")

	if len(buf.entries) != 1 {
		t.Fatalf("buffer has %d entries, want 1", len(buf.entries))
	}

	fields := buf.entries[0].Fields
	if fields["url"] != "https://example.com" {
		t.Errorf("url field = %v, want https://example.com", fields["url"])
	}
	if fields["depth"] != 2 {
		t.Errorf("depth field = %v, want 2", fields["depth"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/logs/... -run TestJobLoggerImpl -v`
Expected: FAIL with "undefined: NewJobLogger"

**Step 3: Write minimal implementation**

```go
// crawler/internal/logs/job_logger_impl.go
package logs

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Heartbeat interval for long-running jobs.
const HeartbeatInterval = 15 * time.Second

// MaxLogsPerJob is the hard ceiling to prevent pathological cases.
const MaxLogsPerJob = 50000

// jobLoggerImpl implements JobLogger.
type jobLoggerImpl struct {
	jobID       string
	executionID string
	verbosity   Verbosity
	startTime   time.Time

	// Output targets
	buffer    Buffer
	publisher Publisher
	stdout    infralogger.Logger

	// Throttling
	throttler        *rateLimiter
	throttledCount   atomic.Int64
	lastThrottleLog  time.Time
	throttleLogMu    sync.Mutex

	// Metrics
	metrics *logMetrics

	// Log cap
	truncatedEmitted atomic.Bool

	// Scoped fields (for WithFields)
	scopedFields []Field

	mu sync.Mutex
}

// NewJobLogger creates a new job logger.
func NewJobLogger(
	jobID, executionID string,
	verbosity Verbosity,
	buffer Buffer,
	publisher Publisher,
	stdout infralogger.Logger,
	maxLogsPerSec int,
) JobLogger {
	return &jobLoggerImpl{
		jobID:       jobID,
		executionID: executionID,
		verbosity:   verbosity,
		startTime:   time.Now(),
		buffer:      buffer,
		publisher:   publisher,
		stdout:      stdout,
		throttler:   newRateLimiter(maxLogsPerSec),
		metrics:     newLogMetrics(),
	}
}

func (l *jobLoggerImpl) Info(category Category, msg string, fields ...Field) {
	l.log("info", category, msg, fields)
}

func (l *jobLoggerImpl) Warn(category Category, msg string, fields ...Field) {
	l.log("warn", category, msg, fields)
}

func (l *jobLoggerImpl) Error(category Category, msg string, fields ...Field) {
	l.log("error", category, msg, fields)
	l.metrics.IncrementErrors()
}

func (l *jobLoggerImpl) Debug(category Category, msg string, fields ...Field) {
	if !l.verbosity.AllowsLevel("debug") {
		return
	}

	// Apply throttling
	if l.throttler != nil && !l.throttler.Allow() {
		l.throttledCount.Add(1)
		l.metrics.IncrementThrottled()
		l.maybeLogThrottleWarning()
		return
	}

	l.log("debug", category, msg, fields)
}

func (l *jobLoggerImpl) log(level string, category Category, msg string, fields []Field) {
	// Check hard cap
	if l.metrics.logsEmitted.Load() >= MaxLogsPerJob {
		l.emitTruncatedOnce()
		return
	}

	// Merge scoped fields with call-site fields
	allFields := l.mergeFields(fields)

	entry := LogEntry{
		SchemaVersion: CurrentSchemaVersion,
		Timestamp:     time.Now(),
		Level:         level,
		Category:      category.String(),
		Message:       msg,
		JobID:         l.jobID,
		ExecID:        l.executionID,
		Fields:        FieldsToMap(allFields),
	}

	// Write to buffer
	if l.buffer != nil {
		l.buffer.Write(entry)
	}

	// Publish to SSE
	if l.publisher != nil {
		l.publisher.PublishLogLine(context.Background(), entry)
	}

	// Mirror to stdout
	if l.stdout != nil {
		l.logToStdout(level, category, msg, allFields)
	}

	l.metrics.IncrementLogsEmitted()

	// Auto-collect metrics from category
	l.collectMetricsFromLog(category, msg, allFields)
}

func (l *jobLoggerImpl) mergeFields(fields []Field) []Field {
	if len(l.scopedFields) == 0 {
		return fields
	}
	result := make([]Field, 0, len(l.scopedFields)+len(fields))
	result = append(result, l.scopedFields...)
	result = append(result, fields...)
	return result
}

func (l *jobLoggerImpl) collectMetricsFromLog(category Category, msg string, fields []Field) {
	switch category {
	case CategoryQueue:
		if msg == "Link enqueued" || msg == "Link queued" {
			l.metrics.IncrementPagesDiscovered()
		}
	case CategoryFetch:
		if msg == "Page fetched" || msg == "Response received" {
			l.metrics.IncrementPagesCrawled()
			for _, f := range fields {
				if f.Key == "status" {
					if status, ok := f.Value.(int); ok {
						l.metrics.RecordStatusCode(status)
					}
				}
			}
		}
	case CategoryExtract:
		if msg == "Content extracted" {
			l.metrics.IncrementItemsExtracted()
		}
	}
}

func (l *jobLoggerImpl) logToStdout(level string, category Category, msg string, fields []Field) {
	// Convert to infrastructure logger format
	infraFields := make([]infralogger.Field, 0, len(fields)+3)
	infraFields = append(infraFields,
		infralogger.String("job_id", l.jobID),
		infralogger.String("category", category.String()),
	)
	for _, f := range fields {
		infraFields = append(infraFields, infralogger.Any(f.Key, f.Value))
	}

	switch level {
	case "debug":
		l.stdout.Debug(msg, infraFields...)
	case "info":
		l.stdout.Info(msg, infraFields...)
	case "warn":
		l.stdout.Warn(msg, infraFields...)
	case "error":
		l.stdout.Error(msg, infraFields...)
	}
}

func (l *jobLoggerImpl) maybeLogThrottleWarning() {
	l.throttleLogMu.Lock()
	defer l.throttleLogMu.Unlock()

	const throttleWarnInterval = 10 * time.Second
	if time.Since(l.lastThrottleLog) < throttleWarnInterval {
		return
	}

	l.lastThrottleLog = time.Now()
	count := l.throttledCount.Load()

	// Emit via Info to bypass throttling
	l.log("warn", CategoryMetrics, "Logs being throttled",
		[]Field{
			Int64("throttled_count", count),
		},
	)
}

func (l *jobLoggerImpl) emitTruncatedOnce() {
	if l.truncatedEmitted.CompareAndSwap(false, true) {
		entry := LogEntry{
			SchemaVersion: CurrentSchemaVersion,
			Timestamp:     time.Now(),
			Level:         "warn",
			Category:      CategoryLifecycle.String(),
			Message:       "Log output truncated",
			JobID:         l.jobID,
			ExecID:        l.executionID,
			Fields: map[string]any{
				"logs_emitted": MaxLogsPerJob,
				"reason":       "max_logs_per_job reached",
			},
		}

		if l.buffer != nil {
			l.buffer.Write(entry)
		}
		if l.publisher != nil {
			l.publisher.PublishLogLine(context.Background(), entry)
		}
	}
}

func (l *jobLoggerImpl) JobStarted(sourceID, url string) {
	l.log("info", CategoryLifecycle, "Job started", []Field{
		String("source_id", sourceID),
		URL(url),
	})
}

func (l *jobLoggerImpl) JobCompleted(summary *JobSummary) {
	summary.LogsThrottled = l.throttledCount.Load()
	summary.LogsEmitted = l.metrics.logsEmitted.Load()
	summary.Duration = time.Since(l.startTime).Milliseconds()

	l.log("info", CategoryLifecycle, "Job completed", []Field{
		Int64("pages_discovered", summary.PagesDiscovered),
		Int64("pages_crawled", summary.PagesCrawled),
		Int64("items_extracted", summary.ItemsExtracted),
		Int64("errors_count", summary.ErrorsCount),
		Int64("duration_ms", summary.Duration),
		Any("status_codes", summary.StatusCodes),
		Any("top_errors", summary.TopErrors),
	})
}

func (l *jobLoggerImpl) JobFailed(err error) {
	summary := l.BuildSummary()
	l.log("error", CategoryLifecycle, "Job failed", []Field{
		Err(err),
		Int64("pages_crawled", summary.PagesCrawled),
		Int64("errors_count", summary.ErrorsCount),
		Int64("duration_ms", time.Since(l.startTime).Milliseconds()),
	})
}

func (l *jobLoggerImpl) IsDebugEnabled() bool {
	return l.verbosity == VerbosityDebug || l.verbosity == VerbosityTrace
}

func (l *jobLoggerImpl) IsTraceEnabled() bool {
	return l.verbosity == VerbosityTrace
}

func (l *jobLoggerImpl) WithFields(fields ...Field) JobLogger {
	return &scopedJobLogger{
		parent: l,
		fields: fields,
	}
}

func (l *jobLoggerImpl) StartHeartbeat(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				l.emitHeartbeat()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (l *jobLoggerImpl) emitHeartbeat() {
	summary := l.metrics.BuildSummary()
	l.Info(CategoryLifecycle, "Heartbeat",
		Int64("pages_crawled", summary.PagesCrawled),
		Int64("items_extracted", summary.ItemsExtracted),
		Int64("errors_count", summary.ErrorsCount),
		Int64("queue_depth", l.metrics.queueDepth.Load()),
		Int64("elapsed_ms", time.Since(l.startTime).Milliseconds()),
	)
}

func (l *jobLoggerImpl) BuildSummary() *JobSummary {
	return l.metrics.BuildSummary()
}

func (l *jobLoggerImpl) Flush() error {
	// No buffered writes to flush in current implementation
	return nil
}

// Ensure jobLoggerImpl implements JobLogger
var _ JobLogger = (*jobLoggerImpl)(nil)

// scopedJobLogger wraps a parent logger with pre-set fields.
type scopedJobLogger struct {
	parent *jobLoggerImpl
	fields []Field
}

func (s *scopedJobLogger) Info(category Category, msg string, fields ...Field) {
	s.parent.log("info", category, msg, s.merge(fields))
}

func (s *scopedJobLogger) Warn(category Category, msg string, fields ...Field) {
	s.parent.log("warn", category, msg, s.merge(fields))
}

func (s *scopedJobLogger) Error(category Category, msg string, fields ...Field) {
	s.parent.log("error", category, msg, s.merge(fields))
	s.parent.metrics.IncrementErrors()
}

func (s *scopedJobLogger) Debug(category Category, msg string, fields ...Field) {
	if !s.parent.verbosity.AllowsLevel("debug") {
		return
	}
	if s.parent.throttler != nil && !s.parent.throttler.Allow() {
		s.parent.throttledCount.Add(1)
		s.parent.metrics.IncrementThrottled()
		s.parent.maybeLogThrottleWarning()
		return
	}
	s.parent.log("debug", category, msg, s.merge(fields))
}

func (s *scopedJobLogger) merge(fields []Field) []Field {
	result := make([]Field, 0, len(s.fields)+len(fields))
	result = append(result, s.fields...)
	result = append(result, fields...)
	return result
}

func (s *scopedJobLogger) JobStarted(sourceID, url string)       { s.parent.JobStarted(sourceID, url) }
func (s *scopedJobLogger) JobCompleted(summary *JobSummary)      { s.parent.JobCompleted(summary) }
func (s *scopedJobLogger) JobFailed(err error)                   { s.parent.JobFailed(err) }
func (s *scopedJobLogger) IsDebugEnabled() bool                  { return s.parent.IsDebugEnabled() }
func (s *scopedJobLogger) IsTraceEnabled() bool                  { return s.parent.IsTraceEnabled() }
func (s *scopedJobLogger) WithFields(fields ...Field) JobLogger  { return &scopedJobLogger{parent: s.parent, fields: append(s.fields, fields...)} }
func (s *scopedJobLogger) StartHeartbeat(ctx context.Context)    { s.parent.StartHeartbeat(ctx) }
func (s *scopedJobLogger) BuildSummary() *JobSummary             { return s.parent.BuildSummary() }
func (s *scopedJobLogger) Flush() error                          { return s.parent.Flush() }

var _ JobLogger = (*scopedJobLogger)(nil)
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/logs/... -run TestJobLoggerImpl -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/logs/job_logger_impl.go crawler/internal/logs/job_logger_impl_test.go
git commit -m "feat(logs): implement JobLogger with throttling and scoped fields"
```

---

## Continue in Next Session

This plan continues with:
- **Phase 2 (continued)**: Ring buffer ReadLast, config updates
- **Phase 3**: Crawler instrumentation (converting ~45 c.logger calls)
- **Phase 4**: SSE replay buffer
- **Phase 5**: Frontend updates

Each task follows the same TDD pattern: write failing test, run to verify fail, implement, run to verify pass, commit.

---

**Plan complete and saved to `docs/plans/2026-01-28-job-logging-pipeline-implementation.md`.**

**Two execution options:**

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach?**
