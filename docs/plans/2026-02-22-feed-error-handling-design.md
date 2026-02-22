# Feed Error Handling — Design

**Date**: 2026-02-22
**Status**: Approved

## Problem

The crawler treats every non-200 HTTP response as a software error, logging all feed poll
failures at ERROR level. In production, ~95% of these are expected operational conditions
(403 blocked, 404 moved, 429 rate-limited, 5xx transient) that generate 300+ ERROR-level
log lines per day, drowning out real failures. The `consecutive_errors` counter in
`feed_state` is never read for any decision-making.

## Decision

Implement a principled error classification model with typed poll errors, severity-aware
logging, and auto-disable with per-type cooldowns. Feed disable state lives in
source-manager (source of truth for source config); cooldown filtering happens there too.

---

## 1. Typed PollError

New file: `crawler/internal/feed/poll_error.go`

```go
type ErrorType string

const (
    ErrTypeRateLimited ErrorType = "rate_limited"    // 429
    ErrTypeForbidden   ErrorType = "forbidden"        // 403
    ErrTypeNotFound    ErrorType = "not_found"        // 404
    ErrTypeGone        ErrorType = "gone"             // 410
    ErrTypeUpstream    ErrorType = "upstream_failure"  // 5xx
    ErrTypeNetwork     ErrorType = "network"           // DNS, timeout, connection reset
    ErrTypeParse       ErrorType = "parse_error"       // feed XML/HTML parse failure
    ErrTypeUnexpected  ErrorType = "unexpected"        // anything else
)

type LogLevel int

const (
    LevelWarn  LogLevel = iota
    LevelError
)

type PollError struct {
    Type       ErrorType
    Level      LogLevel
    StatusCode int    // 0 for non-HTTP errors
    URL        string
    Cause      error
}

func (e *PollError) Error() string { ... }
func (e *PollError) Unwrap() error { return e.Cause }
```

## 2. Status Code Classification

Replace the blanket `unexpected status` in `poller.go:109-116` with a decision tree:

```
HTTP 200       → success (existing)
HTTP 304       → not modified (existing)
HTTP 429       → PollError{Type: RateLimited,  Level: Warn}
HTTP 403       → PollError{Type: Forbidden,    Level: Warn}
HTTP 404       → PollError{Type: NotFound,     Level: Warn}
HTTP 410       → PollError{Type: Gone,         Level: Warn}
HTTP 500-599   → PollError{Type: Upstream,     Level: Warn}
Other          → PollError{Type: Unexpected,   Level: Error}
```

Network errors from `fetcher.Fetch()` (DNS, timeout, connection reset) are wrapped as
`PollError{Type: Network, Level: Warn}`.

Parse errors from `ParseFeed()` are wrapped as `PollError{Type: Parse, Level: Warn}`.

## 3. Severity Matrix

| Condition | Log Level | Auto-Disable | Cooldown |
|-----------|-----------|-------------|----------|
| 429 Rate Limited | WARN | Never | N/A (backoff only) |
| 403 Forbidden | WARN | After 5 consecutive | 24h |
| 404 Not Found | WARN | After 3 consecutive | 48h |
| 410 Gone | WARN | After 1 | 72h |
| 5xx Upstream | WARN | After 10 consecutive | 6h |
| DNS / timeout | WARN | After 10 consecutive | 12h |
| Parse error | WARN | After 5 consecutive | 24h |
| Unexpected status | ERROR | Never (needs human) | N/A |
| DB / Redis / internal | ERROR | Never (needs human) | N/A |

**Rule: auto-disable logic only applies to WARN-level PollErrors.** ERROR-level errors
signal unexpected conditions that need human attention — auto-disabling would hide them.

## 4. Logger Interface Update

Add `Warn` to the feed package's Logger interface:

```go
type Logger interface {
    Info(msg string, fields ...any)
    Warn(msg string, fields ...any)
    Error(msg string, fields ...any)
}
```

The existing `logAdapter` in `bootstrap/services.go` wraps `infralogger.Logger`, which
already supports `Warn` — just needs the adapter method added.

## 5. Severity-Aware recordError

`recordError` inspects the error type and dispatches to the correct log level:

```go
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
    } else {
        p.log.Error("feed poll failed",
            "source_id", sourceID,
            "error", err.Error(),
        )
    }

    p.feedState.UpdateError(ctx, sourceID, pollErr.Type, err.Error())
}
```

The `polling_loop.go:pollDueFeeds` method gets the same treatment — inspect `PollError.Level`
before choosing `log.Warn` vs `log.Error`.

## 6. Database Changes

### Crawler: migration 017

Add `last_error_type` to `feed_state`:

```sql
ALTER TABLE feed_state ADD COLUMN last_error_type VARCHAR(20);
```

`UpdateError` now stores both `last_error` (full message) and `last_error_type`
(the `ErrorType` string). `UpdateSuccess` clears both, plus resets `consecutive_errors`.

### Source-manager: migration 005

Add feed disable columns to `sources`:

```sql
ALTER TABLE sources ADD COLUMN feed_disabled_at    TIMESTAMP WITH TIME ZONE;
ALTER TABLE sources ADD COLUMN feed_disable_reason VARCHAR(20);
```

These columns are nullable. `NULL` means the feed is active.

## 7. Auto-Disable Flow

After `feedState.UpdateError()` increments `consecutive_errors`, the poller checks thresholds:

```
1. Is the error a WARN-level PollError?  (no → skip, ERROR needs human attention)
2. Does consecutive_errors >= threshold for this ErrorType?  (no → skip)
3. Call source-manager API: PATCH /api/v1/sources/:id/feed-disable
   Body: { "reason": "not_found" }
4. Emit one WARN log: "feed disabled" with source_id, reason, consecutive_errors
```

The threshold map is a package-level constant in the feed package:

```go
var disableThresholds = map[ErrorType]int{
    ErrTypeNotFound:    3,
    ErrTypeGone:        1,
    ErrTypeForbidden:   5,
    ErrTypeUpstream:    10,
    ErrTypeNetwork:     10,
    ErrTypeParse:       5,
}
```

Types not in the map (RateLimited, Unexpected) are never auto-disabled.

## 8. Cooldown Filtering in Source-Manager

Source-manager owns the cooldown logic. When the crawler calls `ListSources()` to build
its `listDue` feed list, source-manager applies this filter:

```sql
WHERE feed_url IS NOT NULL
  AND feed_url != ''
  AND enabled = true
  AND (
      feed_disabled_at IS NULL                              -- active feeds
      OR feed_disabled_at + cooldown_interval <= NOW()      -- cooldown expired → retry
  )
```

**Cooldown durations by reason** (implemented as a SQL CASE or Go map):

| Reason | Cooldown |
|--------|----------|
| `not_found` | 48 hours |
| `gone` | 72 hours |
| `forbidden` | 24 hours |
| `upstream_failure` | 6 hours |
| `network` | 12 hours |
| `parse_error` | 24 hours |

This requires a new query variant or parameter on the existing `List`/`ListPaginated` endpoint.
The simplest approach: add a `feed_active` query parameter to `GET /api/v1/sources` that
applies the cooldown filter. The crawler's `buildListDueFunc` passes this parameter.

### Cooldown retry success

If a feed returns to 200 after cooldown retry, the crawler calls:
`PATCH /api/v1/sources/:id/feed-enable`
which clears `feed_disabled_at` and `feed_disable_reason`.

## 9. Manual Re-Enable

Two new source-manager endpoints:

| Method | Path | Description |
|--------|------|-------------|
| `PATCH` | `/api/v1/sources/:id/feed-disable` | Set `feed_disabled_at = NOW()`, `feed_disable_reason` |
| `PATCH` | `/api/v1/sources/:id/feed-enable` | Clear `feed_disabled_at`, `feed_disable_reason` |

Both require JWT. The dashboard can use these for manual control.

## 10. FeedStateStore Interface Changes

```go
type FeedStateStore interface {
    GetOrCreate(ctx context.Context, sourceID, feedURL string) (*domain.FeedState, error)
    UpdateSuccess(ctx context.Context, sourceID string, result PollResult) error
    UpdateError(ctx context.Context, sourceID, errorType, errMsg string) error
}
```

`UpdateError` gains an `errorType` parameter to populate `last_error_type`.

## 11. Source Disabler Interface

New interface in the feed package, implemented by the source-manager API client adapter:

```go
type SourceFeedDisabler interface {
    DisableFeed(ctx context.Context, sourceID, reason string) error
    EnableFeed(ctx context.Context, sourceID string) error
}
```

Injected into the Poller alongside the existing dependencies.

---

## Services Affected

| Service | Changes |
|---------|---------|
| **crawler** | PollError type, severity-aware logging, threshold checking, source-manager API calls |
| **source-manager** | Migration 005, feed-disable/enable endpoints, cooldown filter on ListSources |

No changes to classifier, publisher, or any other service.

## Files Changed (Crawler)

| File | Change |
|------|--------|
| `internal/feed/poll_error.go` | New: PollError type, ErrorType constants, thresholds |
| `internal/feed/poller.go` | Classify status codes, severity-aware recordError, threshold check |
| `internal/feed/polling_loop.go` | Severity-aware log in pollDueFeeds |
| `internal/domain/frontier.go` | Add `LastErrorType` field to FeedState |
| `internal/database/feed_state_repository.go` | UpdateError accepts errorType, new column in queries |
| `internal/bootstrap/services.go` | Add Warn to logAdapter, inject SourceFeedDisabler |
| `migrations/017_add_feed_error_type.up.sql` | Add last_error_type column |

## Files Changed (Source-Manager)

| File | Change |
|------|--------|
| `internal/models/source.go` | Add FeedDisabledAt, FeedDisableReason fields |
| `internal/repository/source.go` | Feed-disable/enable methods, cooldown filter query |
| `internal/handlers/source_handler.go` | Feed-disable/enable endpoints |
| `internal/api/api.go` | Register new routes |
| `migrations/005_add_feed_disable_fields.up.sql` | Add columns |

## Testing Strategy

- Unit tests for `PollError` classification (each status code maps correctly)
- Unit tests for threshold logic (each ErrorType triggers at correct count)
- Unit tests for `recordError` (Warn vs Error dispatch)
- Source-manager: test cooldown filter SQL logic
- Integration: verify disabled feeds are excluded from `ListSources` response
