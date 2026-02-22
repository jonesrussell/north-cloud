# Feed Error Handling Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace blanket ERROR logging for all feed poll failures with a typed, severity-aware error model that auto-disables feeds after repeated failures.

**Architecture:** New `PollError` type in the crawler's feed package classifies HTTP status codes and network errors into categories with WARN or ERROR severity. The crawler tracks `last_error_type` in its `feed_state` table and calls source-manager to disable feeds when thresholds are reached. Source-manager owns disable state and cooldown filtering.

**Tech Stack:** Go 1.26, PostgreSQL, Gin, sqlmock (tests)

---

### Task 1: PollError Type and Classification

**Files:**
- Create: `crawler/internal/feed/poll_error.go`
- Test: `crawler/internal/feed/poll_error_test.go`

**Step 1: Write the failing test**

Create `crawler/internal/feed/poll_error_test.go`:

```go
package feed_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/feed"
)

func TestClassifyHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantType   feed.ErrorType
		wantLevel  feed.LogLevel
	}{
		{"429 rate limited", http.StatusTooManyRequests, feed.ErrTypeRateLimited, feed.LevelWarn},
		{"403 forbidden", http.StatusForbidden, feed.ErrTypeForbidden, feed.LevelWarn},
		{"404 not found", http.StatusNotFound, feed.ErrTypeNotFound, feed.LevelWarn},
		{"410 gone", http.StatusGone, feed.ErrTypeGone, feed.LevelWarn},
		{"500 server error", http.StatusInternalServerError, feed.ErrTypeUpstream, feed.LevelWarn},
		{"502 bad gateway", http.StatusBadGateway, feed.ErrTypeUpstream, feed.LevelWarn},
		{"503 service unavailable", http.StatusServiceUnavailable, feed.ErrTypeUpstream, feed.LevelWarn},
		{"599 max server error", 599, feed.ErrTypeUpstream, feed.LevelWarn},
		{"301 redirect", http.StatusMovedPermanently, feed.ErrTypeUnexpected, feed.LevelError},
		{"418 teapot", http.StatusTeapot, feed.ErrTypeUnexpected, feed.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pollErr := feed.ClassifyHTTPStatus(tt.statusCode, "https://example.com/feed")

			if pollErr.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", pollErr.Type, tt.wantType)
			}
			if pollErr.Level != tt.wantLevel {
				t.Errorf("Level = %d, want %d", pollErr.Level, tt.wantLevel)
			}
			if pollErr.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", pollErr.StatusCode, tt.statusCode)
			}
		})
	}
}

func TestPollError_Error(t *testing.T) {
	t.Parallel()

	pollErr := &feed.PollError{
		Type:       feed.ErrTypeNotFound,
		Level:      feed.LevelWarn,
		StatusCode: http.StatusNotFound,
		URL:        "https://example.com/feed",
		Cause:      fmt.Errorf("HTTP 404"),
	}

	msg := pollErr.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestPollError_Unwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("connection refused")
	pollErr := &feed.PollError{
		Type:  feed.ErrTypeNetwork,
		Level: feed.LevelWarn,
		Cause: cause,
	}

	if !errors.Is(pollErr, cause) {
		t.Error("expected Unwrap to return cause")
	}
}

func TestPollError_ErrorsAs(t *testing.T) {
	t.Parallel()

	pollErr := feed.ClassifyHTTPStatus(http.StatusNotFound, "https://example.com/feed")
	wrapped := fmt.Errorf("poll feed: %w", pollErr)

	var target *feed.PollError
	if !errors.As(wrapped, &target) {
		t.Fatal("expected errors.As to match PollError")
	}
	if target.Type != feed.ErrTypeNotFound {
		t.Errorf("Type = %q, want %q", target.Type, feed.ErrTypeNotFound)
	}
}

func TestClassifyNetworkError(t *testing.T) {
	t.Parallel()

	cause := errors.New("dial tcp: lookup example.com: no such host")
	pollErr := feed.ClassifyNetworkError(cause, "https://example.com/feed")

	if pollErr.Type != feed.ErrTypeNetwork {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeNetwork)
	}
	if pollErr.Level != feed.LevelWarn {
		t.Errorf("Level = %d, want %d", pollErr.Level, feed.LevelWarn)
	}
	if pollErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0", pollErr.StatusCode)
	}
}

func TestClassifyParseError(t *testing.T) {
	t.Parallel()

	cause := errors.New("invalid XML")
	pollErr := feed.ClassifyParseError(cause, "https://example.com/feed")

	if pollErr.Type != feed.ErrTypeParse {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeParse)
	}
	if pollErr.Level != feed.LevelWarn {
		t.Errorf("Level = %d, want %d", pollErr.Level, feed.LevelWarn)
	}
}

func TestDisableThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		errType   feed.ErrorType
		threshold int
		exists    bool
	}{
		{feed.ErrTypeNotFound, 3, true},
		{feed.ErrTypeGone, 1, true},
		{feed.ErrTypeForbidden, 5, true},
		{feed.ErrTypeUpstream, 10, true},
		{feed.ErrTypeNetwork, 10, true},
		{feed.ErrTypeParse, 5, true},
		{feed.ErrTypeRateLimited, 0, false},
		{feed.ErrTypeUnexpected, 0, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.errType), func(t *testing.T) {
			t.Parallel()

			threshold, ok := feed.DisableThreshold(tt.errType)
			if ok != tt.exists {
				t.Errorf("DisableThreshold(%q) exists = %v, want %v", tt.errType, ok, tt.exists)
			}
			if ok && threshold != tt.threshold {
				t.Errorf("DisableThreshold(%q) = %d, want %d", tt.errType, threshold, tt.threshold)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/feed/ -run TestClassifyHTTPStatus -v`
Expected: FAIL — `feed.ClassifyHTTPStatus` undefined

**Step 3: Write minimal implementation**

Create `crawler/internal/feed/poll_error.go`:

```go
package feed

import "fmt"

// ErrorType classifies feed poll failures for severity-aware logging and auto-disable.
type ErrorType string

const (
	ErrTypeRateLimited ErrorType = "rate_limited"
	ErrTypeForbidden   ErrorType = "forbidden"
	ErrTypeNotFound    ErrorType = "not_found"
	ErrTypeGone        ErrorType = "gone"
	ErrTypeUpstream    ErrorType = "upstream_failure"
	ErrTypeNetwork     ErrorType = "network"
	ErrTypeParse       ErrorType = "parse_error"
	ErrTypeUnexpected  ErrorType = "unexpected"
)

// LogLevel determines whether a PollError is logged at WARN or ERROR.
type LogLevel int

const (
	LevelWarn  LogLevel = iota
	LevelError
)

// PollError represents a classified feed polling failure.
type PollError struct {
	Type       ErrorType
	Level      LogLevel
	StatusCode int
	URL        string
	Cause      error
}

func (e *PollError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("feed poll %s: HTTP %d for %s", e.Type, e.StatusCode, e.URL)
	}
	return fmt.Sprintf("feed poll %s: %s for %s", e.Type, e.Cause, e.URL)
}

func (e *PollError) Unwrap() error { return e.Cause }

// HTTP status code boundaries for classification.
const (
	statusForbidden       = 403
	statusNotFound        = 404
	statusGone            = 410
	statusTooManyRequests = 429
	statusServerErrorLow  = 500
	statusServerErrorHigh = 599
)

// ClassifyHTTPStatus creates a PollError from an HTTP status code.
func ClassifyHTTPStatus(statusCode int, url string) *PollError {
	cause := fmt.Errorf("HTTP %d", statusCode)

	switch {
	case statusCode == statusTooManyRequests:
		return &PollError{Type: ErrTypeRateLimited, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	case statusCode == statusForbidden:
		return &PollError{Type: ErrTypeForbidden, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	case statusCode == statusNotFound:
		return &PollError{Type: ErrTypeNotFound, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	case statusCode == statusGone:
		return &PollError{Type: ErrTypeGone, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	case statusCode >= statusServerErrorLow && statusCode <= statusServerErrorHigh:
		return &PollError{Type: ErrTypeUpstream, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	default:
		return &PollError{Type: ErrTypeUnexpected, Level: LevelError, StatusCode: statusCode, URL: url, Cause: cause}
	}
}

// ClassifyNetworkError creates a PollError for network-level failures (DNS, timeout, etc.).
func ClassifyNetworkError(cause error, url string) *PollError {
	return &PollError{Type: ErrTypeNetwork, Level: LevelWarn, URL: url, Cause: cause}
}

// ClassifyParseError creates a PollError for feed parsing failures.
func ClassifyParseError(cause error, url string) *PollError {
	return &PollError{Type: ErrTypeParse, Level: LevelWarn, URL: url, Cause: cause}
}

// disableThresholds maps error types to the number of consecutive failures before auto-disable.
// Types not in this map (RateLimited, Unexpected) are never auto-disabled.
var disableThresholds = map[ErrorType]int{
	ErrTypeNotFound:  3,
	ErrTypeGone:      1,
	ErrTypeForbidden: 5,
	ErrTypeUpstream:  10,
	ErrTypeNetwork:   10,
	ErrTypeParse:     5,
}

// DisableThreshold returns the auto-disable threshold for an error type.
// Returns (0, false) if the error type should never be auto-disabled.
func DisableThreshold(errType ErrorType) (int, bool) {
	threshold, ok := disableThresholds[errType]
	return threshold, ok
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/feed/ -run "TestClassify|TestPollError|TestDisableThreshold" -v`
Expected: All PASS

**Step 5: Lint**

Run: `cd crawler && golangci-lint run ./internal/feed/`
Expected: No errors

**Step 6: Commit**

```bash
git add crawler/internal/feed/poll_error.go crawler/internal/feed/poll_error_test.go
git commit -m "feat(crawler): add PollError type with HTTP status classification"
```

---

### Task 2: Add Warn to Logger Interface and logAdapter

**Files:**
- Modify: `crawler/internal/feed/poller.go:57-60` (Logger interface)
- Modify: `crawler/internal/bootstrap/services.go:561-573` (logAdapter)
- Modify: `crawler/internal/feed/poller_test.go:129-135` (mockLogger)

**Step 1: Write the failing test**

Add `Warn` to `mockLogger` in `crawler/internal/feed/poller_test.go:129-135`. Change:

```go
type mockLogger struct{}

func (m *mockLogger) Info(_ string, _ ...any) {
}

func (m *mockLogger) Error(_ string, _ ...any) {
}
```

To:

```go
type mockLogger struct {
	warnCalled  bool
	errorCalled bool
	lastMsg     string
}

func (m *mockLogger) Info(_ string, _ ...any) {}

func (m *mockLogger) Warn(msg string, _ ...any) {
	m.warnCalled = true
	m.lastMsg = msg
}

func (m *mockLogger) Error(msg string, _ ...any) {
	m.errorCalled = true
	m.lastMsg = msg
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/feed/ -v`
Expected: FAIL — `mockLogger` doesn't satisfy `feed.Logger` (missing `Warn`)

**Step 3: Update the Logger interface**

In `crawler/internal/feed/poller.go:57-60`, change:

```go
type Logger interface {
	Info(msg string, fields ...any)
	Error(msg string, fields ...any)
}
```

To:

```go
type Logger interface {
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
}
```

**Step 4: Add Warn to logAdapter**

In `crawler/internal/bootstrap/services.go`, after the `Info` method (line 568), add:

```go
func (a *logAdapter) Warn(msg string, fields ...any) {
	a.log.Warn(msg, toInfraFields(fields)...)
}
```

Also update the comment on line 561 from:
```go
// logAdapter adapts infralogger.Logger to the feed.Logger and fetcher.WorkerLogger interfaces.
// Both interfaces have identical signatures: Info(msg, ...any) and Error(msg, ...any).
```
To:
```go
// logAdapter adapts infralogger.Logger to the feed.Logger and fetcher.WorkerLogger interfaces.
```

**Step 5: Run tests**

Run: `cd crawler && go test ./internal/feed/ -v`
Expected: All PASS

**Step 6: Lint**

Run: `cd crawler && golangci-lint run ./internal/feed/ ./internal/bootstrap/`
Expected: No errors

**Step 7: Commit**

```bash
git add crawler/internal/feed/poller.go crawler/internal/feed/poller_test.go crawler/internal/bootstrap/services.go
git commit -m "feat(crawler): add Warn to feed Logger interface and logAdapter"
```

---

### Task 3: Crawler Migration — Add last_error_type to feed_state

**Files:**
- Create: `crawler/migrations/017_add_feed_error_type.up.sql`
- Create: `crawler/migrations/017_add_feed_error_type.down.sql`
- Modify: `crawler/internal/domain/frontier.go:85-96` (FeedState struct)
- Modify: `crawler/internal/database/feed_state_repository.go:12-13,67-77` (columns + UpdateError)
- Modify: `crawler/internal/database/feed_state_repository_test.go:16-18,175-209` (column list + tests)
- Modify: `crawler/internal/feed/poller.go:30-34` (FeedStateStore interface)
- Modify: `crawler/internal/feed/poller_test.go:59-110` (mockFeedStateStore)

**Step 1: Create migration files**

Create `crawler/migrations/017_add_feed_error_type.up.sql`:
```sql
ALTER TABLE feed_state ADD COLUMN last_error_type VARCHAR(20);
```

Create `crawler/migrations/017_add_feed_error_type.down.sql`:
```sql
ALTER TABLE feed_state DROP COLUMN IF EXISTS last_error_type;
```

**Step 2: Update FeedState domain struct**

In `crawler/internal/domain/frontier.go:85-96`, add `LastErrorType` after `LastError`:

```go
type FeedState struct {
	SourceID          string     `db:"source_id"          json:"source_id"`
	FeedURL           string     `db:"feed_url"           json:"feed_url"`
	LastPolledAt      *time.Time `db:"last_polled_at"     json:"last_polled_at,omitempty"`
	LastETag          *string    `db:"last_etag"          json:"last_etag,omitempty"`
	LastModified      *string    `db:"last_modified"      json:"last_modified,omitempty"`
	LastItemCount     int        `db:"last_item_count"    json:"last_item_count"`
	ConsecutiveErrors int        `db:"consecutive_errors" json:"consecutive_errors"`
	LastError         *string    `db:"last_error"         json:"last_error,omitempty"`
	LastErrorType     *string    `db:"last_error_type"    json:"last_error_type,omitempty"`
	CreatedAt         time.Time  `db:"created_at"         json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"         json:"updated_at"`
}
```

**Step 3: Update repository SELECT columns**

In `crawler/internal/database/feed_state_repository.go:12-13`, change:

```go
const feedStateSelectColumns = `source_id, feed_url, last_polled_at, last_etag, last_modified,
	last_item_count, consecutive_errors, last_error, created_at, updated_at`
```

To:

```go
const feedStateSelectColumns = `source_id, feed_url, last_polled_at, last_etag, last_modified,
	last_item_count, consecutive_errors, last_error, last_error_type, created_at, updated_at`
```

**Step 4: Update UpdateSuccess to clear last_error_type**

In `crawler/internal/database/feed_state_repository.go:55-60`, change the query:

```go
	query := `
		UPDATE feed_state
		SET last_polled_at = NOW(), last_etag = $2, last_modified = $3,
			last_item_count = $4, consecutive_errors = 0, last_error = NULL, updated_at = NOW()
		WHERE source_id = $1
	`
```

To:

```go
	query := `
		UPDATE feed_state
		SET last_polled_at = NOW(), last_etag = $2, last_modified = $3,
			last_item_count = $4, consecutive_errors = 0, last_error = NULL,
			last_error_type = NULL, updated_at = NOW()
		WHERE source_id = $1
	`
```

**Step 5: Update UpdateError to accept and store errorType**

In `crawler/internal/database/feed_state_repository.go:67-77`, change:

```go
func (r *FeedStateRepository) UpdateError(ctx context.Context, sourceID, errMsg string) error {
	query := `
		UPDATE feed_state
		SET last_polled_at = NOW(), consecutive_errors = consecutive_errors + 1,
			last_error = $2, updated_at = NOW()
		WHERE source_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, sourceID, errMsg)
	return execRequireRows(result, err, fmt.Errorf("feed state not found: %s", sourceID))
}
```

To:

```go
func (r *FeedStateRepository) UpdateError(ctx context.Context, sourceID, errorType, errMsg string) error {
	query := `
		UPDATE feed_state
		SET last_polled_at = NOW(), consecutive_errors = consecutive_errors + 1,
			last_error = $3, last_error_type = $2, updated_at = NOW()
		WHERE source_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, sourceID, errorType, errMsg)
	return execRequireRows(result, err, fmt.Errorf("feed state not found: %s", sourceID))
}
```

**Step 6: Update FeedStateStore interface**

In `crawler/internal/feed/poller.go:30-34`, change:

```go
type FeedStateStore interface {
	GetOrCreate(ctx context.Context, sourceID, feedURL string) (*domain.FeedState, error)
	UpdateSuccess(ctx context.Context, sourceID string, result PollResult) error
	UpdateError(ctx context.Context, sourceID, errMsg string) error
}
```

To:

```go
type FeedStateStore interface {
	GetOrCreate(ctx context.Context, sourceID, feedURL string) (*domain.FeedState, error)
	UpdateSuccess(ctx context.Context, sourceID string, result PollResult) error
	UpdateError(ctx context.Context, sourceID, errorType, errMsg string) error
}
```

**Step 7: Update FeedStateRepoAdapter**

Find the `FeedStateRepoAdapter` in `crawler/internal/feed/` (likely in an adapter file) and update its `UpdateError` signature to pass through `errorType`. If it wraps `database.FeedStateRepository`, update the call.

**Step 8: Update mockFeedStateStore in tests**

In `crawler/internal/feed/poller_test.go:59-110`, change the `UpdateError` method and add `lastErrType`:

```go
type mockFeedStateStore struct {
	state           *domain.FeedState
	getOrCreateErr  error
	updateSuccErr   error
	updateErrErr    error
	successCalled   bool
	errorCalled     bool
	lastSuccResult  feed.PollResult
	lastErrType     string
	lastErrMsg      string
	lastErrSourceID string
}
```

And change `UpdateError`:

```go
func (m *mockFeedStateStore) UpdateError(
	_ context.Context,
	sourceID, errorType, errMsg string,
) error {
	m.errorCalled = true
	m.lastErrSourceID = sourceID
	m.lastErrType = errorType
	m.lastErrMsg = errMsg

	return m.updateErrErr
}
```

**Step 9: Update test column list**

In `crawler/internal/database/feed_state_repository_test.go:16-18`, change:

```go
var feedStateColumns = []string{
	"source_id", "feed_url", "last_polled_at", "last_etag", "last_modified",
	"last_item_count", "consecutive_errors", "last_error", "created_at", "updated_at",
}
```

To:

```go
var feedStateColumns = []string{
	"source_id", "feed_url", "last_polled_at", "last_etag", "last_modified",
	"last_item_count", "consecutive_errors", "last_error", "last_error_type",
	"created_at", "updated_at",
}
```

Update all `AddRow` calls in the test file to include a `nil` (or value) for `last_error_type` in the correct position (after `last_error`, before `created_at`).

**Step 10: Update repository UpdateError tests**

In `crawler/internal/database/feed_state_repository_test.go`, update the `TestFeedStateRepository_UpdateError` tests to pass three args:

```go
func TestFeedStateRepository_UpdateError(t *testing.T) {
	// ...
	mock.ExpectExec("UPDATE feed_state").
		WithArgs("source-uuid-1", "parse_error", "feed parse error: invalid XML").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateError(ctx, "source-uuid-1", "parse_error", "feed parse error: invalid XML")
	// ...
}
```

**Step 11: Update recordError call in poller.go**

In `crawler/internal/feed/poller.go:217`, change:

```go
if updateErr := p.feedState.UpdateError(ctx, sourceID, originalErr.Error()); updateErr != nil {
```

To (temporary — full rewrite comes in Task 4):

```go
if updateErr := p.feedState.UpdateError(ctx, sourceID, "", originalErr.Error()); updateErr != nil {
```

**Step 12: Run tests**

Run: `cd crawler && go test ./internal/feed/ ./internal/database/ -v`
Expected: All PASS

**Step 13: Lint**

Run: `cd crawler && golangci-lint run ./internal/feed/ ./internal/database/ ./internal/domain/ ./internal/bootstrap/`
Expected: No errors

**Step 14: Commit**

```bash
git add crawler/migrations/017_add_feed_error_type.up.sql crawler/migrations/017_add_feed_error_type.down.sql \
  crawler/internal/domain/frontier.go crawler/internal/database/feed_state_repository.go \
  crawler/internal/database/feed_state_repository_test.go crawler/internal/feed/poller.go \
  crawler/internal/feed/poller_test.go
git commit -m "feat(crawler): add last_error_type to feed_state schema and repository"
```

---

### Task 4: Severity-Aware Poller — Classify and Log

**Files:**
- Modify: `crawler/internal/feed/poller.go:89-119,210-223` (PollFeed + recordError)
- Modify: `crawler/internal/feed/polling_loop.go:59-67` (pollDueFeeds)
- Modify: `crawler/internal/feed/poller_test.go` (update existing tests + add new)

**Step 1: Write failing tests for classified errors**

Add to `crawler/internal/feed/poller_test.go`:

```go
func TestPollFeed_403_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusForbidden},
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T: %v", err, err)
	}
	if pollErr.Type != feed.ErrTypeForbidden {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeForbidden)
	}
	if pollErr.Level != feed.LevelWarn {
		t.Errorf("Level = %d, want %d (LevelWarn)", pollErr.Level, feed.LevelWarn)
	}
	if !stateStore.errorCalled {
		t.Error("expected UpdateError to be called")
	}
	if stateStore.lastErrType != string(feed.ErrTypeForbidden) {
		t.Errorf("lastErrType = %q, want %q", stateStore.lastErrType, feed.ErrTypeForbidden)
	}
}

func TestPollFeed_404_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusNotFound},
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T", err, err)
	}
	if pollErr.Type != feed.ErrTypeNotFound {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeNotFound)
	}
}

func TestPollFeed_429_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusTooManyRequests},
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T", err, err)
	}
	if pollErr.Type != feed.ErrTypeRateLimited {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeRateLimited)
	}
}

func TestPollFeed_NetworkError_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{err: errors.New("dial tcp: no such host")}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T", err, err)
	}
	if pollErr.Type != feed.ErrTypeNetwork {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeNetwork)
	}
}

func TestPollFeed_ParseError_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: newOKResponse(t, "not valid xml at all"),
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T", err, err)
	}
	if pollErr.Type != feed.ErrTypeParse {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeParse)
	}
}
```

**Step 2: Run tests — verify they fail**

Run: `cd crawler && go test ./internal/feed/ -run "TestPollFeed_403|TestPollFeed_404|TestPollFeed_429|TestPollFeed_NetworkError_ReturnsPollError|TestPollFeed_ParseError_ReturnsPollError" -v`
Expected: FAIL — errors are not PollErrors yet

**Step 3: Rewrite PollFeed to classify errors**

Replace `crawler/internal/feed/poller.go:89-119` with:

```go
func (p *Poller) PollFeed(ctx context.Context, sourceID, feedURL string) error {
	state, err := p.feedState.GetOrCreate(ctx, sourceID, feedURL)
	if err != nil {
		return fmt.Errorf("poll feed get state: %w", err)
	}

	resp, fetchErr := p.fetcher.Fetch(ctx, feedURL, state.LastETag, state.LastModified)
	if fetchErr != nil {
		pollErr := ClassifyNetworkError(fetchErr, feedURL)
		p.recordError(ctx, sourceID, pollErr)
		return fmt.Errorf("poll feed fetch: %w", pollErr)
	}

	if resp.StatusCode == http.StatusNotModified {
		p.log.Info("feed not modified, skipping",
			"source_id", sourceID,
			"feed_url", feedURL,
		)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		pollErr := ClassifyHTTPStatus(resp.StatusCode, feedURL)
		p.recordError(ctx, sourceID, pollErr)
		return pollErr
	}

	return p.processResponse(ctx, sourceID, feedURL, resp)
}
```

**Step 4: Wrap parse errors**

In `crawler/internal/feed/poller.go:123-132` (processResponse), change:

```go
	items, parseErr := ParseFeed(ctx, resp.Body)
	if parseErr != nil {
		p.recordError(ctx, sourceID, parseErr)
		return fmt.Errorf("poll feed parse: %w", parseErr)
	}
```

To:

```go
	items, parseErr := ParseFeed(ctx, resp.Body)
	if parseErr != nil {
		pollErr := ClassifyParseError(parseErr, feedURL)
		p.recordError(ctx, sourceID, pollErr)
		return fmt.Errorf("poll feed parse: %w", pollErr)
	}
```

**Step 5: Rewrite recordError with severity dispatch**

Replace `crawler/internal/feed/poller.go:210-223` with:

```go
// recordError logs a feed polling error at the appropriate severity level
// and records it in the feed state store.
func (p *Poller) recordError(ctx context.Context, sourceID string, err error) {
	var pollErr *PollError
	if errors.As(err, &pollErr) {
		logFn := p.log.Warn
		if pollErr.Level == LevelError {
			logFn = p.log.Error
		}
		logFn("feed poll failed",
			"source_id", sourceID,
			"error_type", string(pollErr.Type),
			"status_code", pollErr.StatusCode,
			"error", pollErr.Error(),
		)

		if updateErr := p.feedState.UpdateError(ctx, sourceID, string(pollErr.Type), pollErr.Error()); updateErr != nil {
			p.log.Error("failed to record feed error",
				"source_id", sourceID,
				"error", updateErr.Error(),
			)
		}

		return
	}

	// Unknown error type — always ERROR level
	p.log.Error("feed poll failed",
		"source_id", sourceID,
		"error", err.Error(),
	)

	if updateErr := p.feedState.UpdateError(ctx, sourceID, string(ErrTypeUnexpected), err.Error()); updateErr != nil {
		p.log.Error("failed to record feed error",
			"source_id", sourceID,
			"error", updateErr.Error(),
		)
	}
}
```

Add `"errors"` to the imports in `poller.go`.

**Step 6: Update pollDueFeeds with severity-aware logging**

In `crawler/internal/feed/polling_loop.go:59-67`, change:

```go
	for i := range feeds {
		if pollErr := p.PollFeed(ctx, feeds[i].SourceID, feeds[i].FeedURL); pollErr != nil {
			p.log.Error("feed poll failed",
				"source_id", feeds[i].SourceID,
				"feed_url", feeds[i].FeedURL,
				"error", pollErr.Error(),
			)
		}
	}
```

To:

```go
	for i := range feeds {
		if pollErr := p.PollFeed(ctx, feeds[i].SourceID, feeds[i].FeedURL); pollErr != nil {
			// PollFeed already logged via recordError; only log here for
			// errors that bypass recordError (e.g., GetOrCreate failures).
			var classified *PollError
			if !errors.As(pollErr, &classified) {
				p.log.Error("feed poll failed",
					"source_id", feeds[i].SourceID,
					"feed_url", feeds[i].FeedURL,
					"error", pollErr.Error(),
				)
			}
		}
	}
```

Add `"errors"` to imports in `polling_loop.go`.

**Step 7: Update TestPollFeed_UnexpectedStatus**

The existing `TestPollFeed_UnexpectedStatus` uses status 500, which is now a typed PollError.
Rename it or verify it still passes (it should — status 500 still returns an error, just
now a PollError instead of a plain fmt.Errorf).

**Step 8: Run all tests**

Run: `cd crawler && go test ./internal/feed/ -v`
Expected: All PASS

**Step 9: Lint**

Run: `cd crawler && golangci-lint run ./internal/feed/`
Expected: No errors

**Step 10: Commit**

```bash
git add crawler/internal/feed/poller.go crawler/internal/feed/polling_loop.go crawler/internal/feed/poller_test.go
git commit -m "feat(crawler): severity-aware feed error classification and logging"
```

---

### Task 5: Source-Manager — Feed Disable Schema and Endpoints

**Files:**
- Create: `source-manager/migrations/005_add_feed_disable_fields.up.sql`
- Create: `source-manager/migrations/005_add_feed_disable_fields.down.sql`
- Modify: `source-manager/internal/models/source.go:11-26` (Source struct)
- Modify: `source-manager/internal/repository/source.go` (all queries + new methods)
- Modify: `source-manager/internal/handlers/source.go` (new endpoints)
- Modify: `source-manager/internal/api/router.go:131-138` (new routes)

**Step 1: Create migration files**

Create `source-manager/migrations/005_add_feed_disable_fields.up.sql`:
```sql
ALTER TABLE sources ADD COLUMN feed_disabled_at    TIMESTAMP WITH TIME ZONE;
ALTER TABLE sources ADD COLUMN feed_disable_reason VARCHAR(20);

CREATE INDEX idx_sources_feed_disabled_at ON sources(feed_disabled_at)
    WHERE feed_disabled_at IS NOT NULL;
```

Create `source-manager/migrations/005_add_feed_disable_fields.down.sql`:
```sql
DROP INDEX IF EXISTS idx_sources_feed_disabled_at;
ALTER TABLE sources DROP COLUMN IF EXISTS feed_disable_reason;
ALTER TABLE sources DROP COLUMN IF EXISTS feed_disabled_at;
```

**Step 2: Add fields to Source model**

In `source-manager/internal/models/source.go:11-26`, add after `FeedPollIntervalMinutes`:

```go
	FeedDisabledAt    *time.Time `db:"feed_disabled_at"           json:"feed_disabled_at,omitempty"`
	FeedDisableReason *string    `db:"feed_disable_reason"        json:"feed_disable_reason,omitempty"`
```

**Step 3: Update all repository queries**

Every SELECT, INSERT, and UPDATE in `source-manager/internal/repository/source.go` must
include the two new columns. Update:
- `Create()` INSERT: add `feed_disabled_at`, `feed_disable_reason` to column list and values
- `GetByID()` SELECT: add the two columns
- `ListPaginated()` SELECT: add the two columns
- `List()` SELECT: add the two columns
- `Update()` SET: add the two columns
- `UpsertSource()`: add the two columns
- `scanSourceRow()`: scan the two new fields

**Step 4: Add DisableFeed and EnableFeed repository methods**

Add to `source-manager/internal/repository/source.go`:

```go
// DisableFeed marks a source's feed as disabled with a reason.
func (r *SourceRepository) DisableFeed(ctx context.Context, id, reason string) error {
	query := `
		UPDATE sources
		SET feed_disabled_at = NOW(), feed_disable_reason = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, reason)
	if err != nil {
		return fmt.Errorf("disable feed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("source not found")
	}
	return nil
}

// EnableFeed clears a source's feed disabled state.
func (r *SourceRepository) EnableFeed(ctx context.Context, id string) error {
	query := `
		UPDATE sources
		SET feed_disabled_at = NULL, feed_disable_reason = NULL, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("enable feed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("source not found")
	}
	return nil
}
```

**Step 5: Add handler endpoints**

Add to `source-manager/internal/handlers/source.go`:

```go
// FeedDisableRequest is the request body for disabling a feed.
type FeedDisableRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// DisableFeed marks a source's feed as disabled.
func (h *SourceHandler) DisableFeed(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source ID"})
		return
	}

	var req FeedDisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.repo.DisableFeed(c.Request.Context(), id, req.Reason); err != nil {
		h.logger.Error("Failed to disable feed",
			infralogger.String("source_id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable feed"})
		return
	}

	h.logger.Info("Feed disabled",
		infralogger.String("source_id", id),
		infralogger.String("reason", req.Reason),
	)

	c.JSON(http.StatusOK, gin.H{"message": "Feed disabled", "source_id": id, "reason": req.Reason})
}

// EnableFeed clears a source's feed disabled state.
func (h *SourceHandler) EnableFeed(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source ID"})
		return
	}

	if err := h.repo.EnableFeed(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to enable feed",
			infralogger.String("source_id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable feed"})
		return
	}

	h.logger.Info("Feed enabled", infralogger.String("source_id", id))

	c.JSON(http.StatusOK, gin.H{"message": "Feed enabled", "source_id": id})
}
```

**Step 6: Register routes**

In `source-manager/internal/api/router.go:131-138`, add after the existing `sources.DELETE` line:

```go
	sources.PATCH("/:id/feed-disable", sourceHandler.DisableFeed)
	sources.PATCH("/:id/feed-enable", sourceHandler.EnableFeed)
```

**Step 7: Run tests**

Run: `cd source-manager && go test ./... -v`
Expected: All PASS

**Step 8: Lint**

Run: `cd source-manager && golangci-lint run`
Expected: No errors

**Step 9: Commit**

```bash
git add source-manager/migrations/ source-manager/internal/models/source.go \
  source-manager/internal/repository/source.go source-manager/internal/handlers/source.go \
  source-manager/internal/api/router.go
git commit -m "feat(source-manager): add feed disable/enable endpoints with cooldown schema"
```

---

### Task 6: Source-Manager — Cooldown Filtering on ListSources

**Files:**
- Modify: `source-manager/internal/repository/source.go` (ListFilter + queries)
- Modify: `source-manager/internal/handlers/source.go:157-202` (parseListQuery)

**Step 1: Add FeedActive filter**

In `source-manager/internal/repository/source.go:134-142`, add `FeedActive` to `ListFilter`:

```go
type ListFilter struct {
	Limit      int
	Offset     int
	SortBy     string
	SortOrder  string
	Search     string
	Enabled    *bool
	FeedActive *bool  // nil = all, true = feeds that are active or past cooldown
}
```

**Step 2: Update buildListWhere with cooldown logic**

In the `buildListWhere` function, add a clause for `FeedActive`:

```go
	if filter.FeedActive != nil && *filter.FeedActive {
		// Include sources where:
		// 1. feed is not disabled (feed_disabled_at IS NULL), OR
		// 2. cooldown has expired (based on reason-specific durations)
		clauses = append(clauses, `(
			feed_disabled_at IS NULL
			OR feed_disabled_at + (CASE feed_disable_reason
				WHEN 'not_found'        THEN INTERVAL '48 hours'
				WHEN 'gone'             THEN INTERVAL '72 hours'
				WHEN 'forbidden'        THEN INTERVAL '24 hours'
				WHEN 'upstream_failure'  THEN INTERVAL '6 hours'
				WHEN 'network'          THEN INTERVAL '12 hours'
				WHEN 'parse_error'      THEN INTERVAL '24 hours'
				ELSE INTERVAL '24 hours'
			END) <= NOW()
		)`)
	}
```

This clause uses no positional parameters, so `pos` stays unchanged.

**Step 3: Update parseListQuery to accept feed_active**

In `source-manager/internal/handlers/source.go`, after the `enabled` parsing block, add:

```go
	var feedActive *bool
	if v := c.Query("feed_active"); v == "true" {
		t := true
		feedActive = &t
	}
```

And add `FeedActive: feedActive` to the returned `ListFilter`.

**Step 4: Run tests**

Run: `cd source-manager && go test ./... -v`
Expected: All PASS

**Step 5: Lint**

Run: `cd source-manager && golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add source-manager/internal/repository/source.go source-manager/internal/handlers/source.go
git commit -m "feat(source-manager): add cooldown filtering on ListSources with feed_active param"
```

---

### Task 7: Crawler — Auto-Disable and Source-Manager Integration

**Files:**
- Create: `crawler/internal/feed/disabler.go` (SourceFeedDisabler interface)
- Modify: `crawler/internal/feed/poller.go` (add disabler, threshold check)
- Modify: `crawler/internal/sources/apiclient/client.go` (DisableFeed/EnableFeed methods)
- Modify: `crawler/internal/bootstrap/services.go` (inject disabler, pass feed_active param)
- Create: `crawler/internal/feed/disabler_test.go`

**Step 1: Create SourceFeedDisabler interface**

Create `crawler/internal/feed/disabler.go`:

```go
package feed

import "context"

// SourceFeedDisabler manages feed disable/enable state via the source-manager API.
type SourceFeedDisabler interface {
	DisableFeed(ctx context.Context, sourceID, reason string) error
	EnableFeed(ctx context.Context, sourceID string) error
}
```

**Step 2: Add Poller field and constructor parameter**

In `crawler/internal/feed/poller.go`, add `disabler` to the struct:

```go
type Poller struct {
	fetcher   HTTPFetcher
	feedState FeedStateStore
	frontier  FrontierSubmitter
	disabler  SourceFeedDisabler
	log       Logger
}
```

Update `NewPoller`:

```go
func NewPoller(
	fetcher HTTPFetcher,
	feedState FeedStateStore,
	frontierSubmitter FrontierSubmitter,
	disabler SourceFeedDisabler,
	log Logger,
) *Poller {
	return &Poller{
		fetcher:   fetcher,
		feedState: feedState,
		frontier:  frontierSubmitter,
		disabler:  disabler,
		log:       log,
	}
}
```

**Step 3: Add threshold check to recordError**

After the `feedState.UpdateError` call in `recordError`, add threshold checking.
Only for WARN-level PollErrors:

```go
		// Check auto-disable threshold (WARN-level only — ERROR needs human attention)
		if pollErr.Level == LevelWarn && p.disabler != nil {
			p.checkDisableThreshold(ctx, sourceID, pollErr.Type)
		}
```

Add a new method:

```go
// checkDisableThreshold checks if a feed should be auto-disabled based on consecutive errors.
func (p *Poller) checkDisableThreshold(ctx context.Context, sourceID string, errType ErrorType) {
	threshold, shouldDisable := DisableThreshold(errType)
	if !shouldDisable {
		return
	}

	state, err := p.feedState.GetOrCreate(ctx, sourceID, "")
	if err != nil {
		return
	}

	if state.ConsecutiveErrors < threshold {
		return
	}

	if disableErr := p.disabler.DisableFeed(ctx, sourceID, string(errType)); disableErr != nil {
		p.log.Error("failed to disable feed",
			"source_id", sourceID,
			"error", disableErr.Error(),
		)
		return
	}

	p.log.Warn("feed disabled",
		"source_id", sourceID,
		"reason", string(errType),
		"consecutive_errors", state.ConsecutiveErrors,
	)
}
```

**Step 4: Add re-enable on success**

In `processResponse` (after `UpdateSuccess`), add:

```go
	// Re-enable feed if it was disabled and is now succeeding (cooldown retry)
	if p.disabler != nil {
		if enableErr := p.disabler.EnableFeed(ctx, sourceID); enableErr != nil {
			p.log.Warn("failed to re-enable feed",
				"source_id", sourceID,
				"error", enableErr.Error(),
			)
		}
	}
```

**Step 5: Add DisableFeed/EnableFeed to apiclient**

In `crawler/internal/sources/apiclient/client.go`, add:

```go
// DisableFeed disables a source's feed via the source-manager API.
func (c *Client) DisableFeed(ctx context.Context, sourceID, reason string) error {
	disableURL, err := url.JoinPath(c.baseURL, sourceID, "feed-disable")
	if err != nil {
		return fmt.Errorf("construct disable URL: %w", err)
	}

	body, marshalErr := json.Marshal(map[string]string{"reason": reason})
	if marshalErr != nil {
		return fmt.Errorf("marshal disable request: %w", marshalErr)
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPatch, disableURL, bytes.NewReader(body))
	if reqErr != nil {
		return fmt.Errorf("create disable request: %w", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")

	var response map[string]any
	return c.doRequest(req, &response)
}

// EnableFeed clears a source's feed disabled state via the source-manager API.
func (c *Client) EnableFeed(ctx context.Context, sourceID string) error {
	enableURL, err := url.JoinPath(c.baseURL, sourceID, "feed-enable")
	if err != nil {
		return fmt.Errorf("construct enable URL: %w", err)
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPatch, enableURL, http.NoBody)
	if reqErr != nil {
		return fmt.Errorf("create enable request: %w", reqErr)
	}

	var response map[string]any
	return c.doRequest(req, &response)
}
```

**Step 6: Update bootstrap wiring**

In `crawler/internal/bootstrap/services.go`, update the `createFeedPoller` function.
Create a disabler adapter and pass it to `NewPoller`:

```go
	disablerAdapter := &feedDisablerAdapter{client: apiClient, log: deps.Logger}
	poller = feed.NewPoller(httpFetcher, feedStateAdapter, frontierAdapter, disablerAdapter, logAdapter)
```

Add the adapter type:

```go
// feedDisablerAdapter adapts apiclient.Client to the feed.SourceFeedDisabler interface.
type feedDisablerAdapter struct {
	client *apiclient.Client
	log    infralogger.Logger
}

func (a *feedDisablerAdapter) DisableFeed(ctx context.Context, sourceID, reason string) error {
	return a.client.DisableFeed(ctx, sourceID, reason)
}

func (a *feedDisablerAdapter) EnableFeed(ctx context.Context, sourceID string) error {
	return a.client.EnableFeed(ctx, sourceID)
}
```

Also update `buildListDueFunc` to pass `?feed_active=true` when calling the source-manager API:

In the `ListSources` call, update the URL to include the query parameter. This may require
updating `apiclient.ListSources()` to accept query params, or adding a `ListActiveFeedSources`
method.

**Step 7: Update test mocks**

Update all callers of `feed.NewPoller` in tests. The mock disabler can be:

```go
type mockDisabler struct {
	disableCalled bool
	enableCalled  bool
	lastReason    string
}

func (m *mockDisabler) DisableFeed(_ context.Context, _ string, reason string) error {
	m.disableCalled = true
	m.lastReason = reason
	return nil
}

func (m *mockDisabler) EnableFeed(_ context.Context, _ string) error {
	m.enableCalled = true
	return nil
}
```

Update `newTestPoller` to accept and pass the disabler (or pass `nil` for tests that don't
need it).

**Step 8: Write threshold check tests**

Add tests that verify:
- After 3 consecutive 404 errors, `DisableFeed` is called
- After 1 consecutive 410, `DisableFeed` is called
- 429 never triggers `DisableFeed`
- ERROR-level errors never trigger `DisableFeed`

**Step 9: Run all tests**

Run: `cd crawler && go test ./... -v`
Expected: All PASS

**Step 10: Lint**

Run: `cd crawler && golangci-lint run`
Expected: No errors

**Step 11: Commit**

```bash
git add crawler/internal/feed/disabler.go crawler/internal/feed/poller.go \
  crawler/internal/feed/poller_test.go crawler/internal/sources/apiclient/client.go \
  crawler/internal/bootstrap/services.go
git commit -m "feat(crawler): auto-disable feeds after consecutive failures via source-manager"
```

---

### Task 8: Run Migrations on Production

**Step 1: Run crawler migration**

```bash
ssh jones@northcloud.biz
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml run --rm crawler \
  go run cmd/migrate/main.go up
```

**Step 2: Run source-manager migration**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml run --rm source-manager \
  go run cmd/migrate/main.go up
```

**Step 3: Verify**

```bash
# Verify crawler feed_state has new column
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler_db \
  -c "\d feed_state"

# Verify source-manager sources has new columns
docker exec -it north-cloud-postgres-source-manager psql -U postgres -d source_manager_db \
  -c "\d sources"
```

**Step 4: Deploy updated services**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d --build crawler source-manager
```

**Step 5: Verify logs show new behavior**

```bash
docker logs north-cloud-crawler --since 5m 2>&1 | grep "feed poll"
```

Expected: `WARN` level for 403/404/429/5xx, `ERROR` only for unexpected conditions.

**Step 6: Commit** (nothing to commit — deployment step)

---

### Task 9: Fix Known Bad Sources

**Step 1: Disable permanently broken feeds**

Using the MCP tool or direct API calls:

```bash
# Business Insider — malformed URL with comma
# CTV News — RSS URL returns 404 (moved)
# These will auto-disable after the new code runs, but clean them up proactively
```

Use `mcp__North_Cloud__Local___update_source` or the source-manager API to either:
- Fix the Business Insider feed URL (remove the comma)
- Update the CTV News feed URL to the new location
- Or set their feed_url to NULL if they should not be polled

**Step 2: Verify error counts drop**

After 1 hour, check:

```bash
ssh jones@northcloud.biz "docker logs north-cloud-crawler --since 1h 2>&1 | grep -c 'level.*error'"
```

Expected: Significant reduction from ~300/day to near zero ERROR-level feed poll messages.
