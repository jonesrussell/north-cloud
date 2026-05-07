---
affected_files: []
cycle_number: 1
mission_slug: community-alert-pipeline-01KQZC7A
reproduction_command:
reviewed_at: '2026-05-06T22:33:24Z'
reviewer_agent: unknown
verdict: rejected
wp_id: WP08
---

# WP08 Review Feedback — Changes Requested

**Reviewer**: claude:opus:reviewer:reviewer
**Verdict**: REJECT — fix lint violations and resubmit

## Summary

Implementation is structurally sound. All acceptance criteria for T032–T035
are met at the behavioral level: ETag/Last-Modified conditional GET works,
304 returns ErrNotModified, 5xx wraps ErrTransient, 4xx wraps ErrStructural,
default UA is correct, body is capped at 5 MB, functional options pattern
is clean, and 9 tests cover all required paths at 87.1% coverage.

**Gates:**
- `go test ./internal/adapter/rss/... -cover` → PASS (87.1% cov, ≥80% target met)
- `go vet ./internal/adapter/rss/...` → PASS
- `gofmt -l ./internal/adapter/rss/` → CLEAN
- `golangci-lint run ./internal/adapter/rss/...` → **FAIL: 4 issues**

The lint failures are blocking. Per project CLAUDE.md "Linting Prevention -
CRITICAL", magic numbers must be named constants, and per repo conventions
nolint directives need rationale.

## Required Fixes

All four issues are in `alert-crawler/internal/adapter/rss/client.go`:

### 1. line 86 — `httpNoBody` (gocritic)

```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.Source.FeedURL, nil)
```

Replace `nil` with `http.NoBody`:

```go
req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.Source.FeedURL, http.NoBody)
```

### 2. line 106 — `whyNoLint` (gocritic)

```go
defer resp.Body.Close() //nolint:errcheck
```

Add rationale to the directive:

```go
defer resp.Body.Close() //nolint:errcheck // best-effort cleanup; body already consumed or skipped on error
```

### 3. line 111 — `mnd` magic number 500

```go
case resp.StatusCode >= 500:
```

Define a named constant in the const block at the top of the file
(alongside `defaultUserAgent`, `defaultTimeout`, `maxFeedBytes`):

```go
const (
    httpServerErrorMin = 500
    httpClientErrorMin = 400
)
```

Then:

```go
case resp.StatusCode >= httpServerErrorMin:
    return nil, fmt.Errorf("upstream %d: %w", resp.StatusCode, ErrTransient)
case resp.StatusCode >= httpClientErrorMin:
    return nil, fmt.Errorf("upstream %d: %w", resp.StatusCode, ErrStructural)
```

### 4. line 113 — `mnd` magic number 400

Covered by fix #3 above.

## Verification After Fix

```
cd alert-crawler
GOWORK=off go test ./internal/adapter/rss/... -cover
GOWORK=off go vet ./internal/adapter/rss/...
gofmt -l ./internal/adapter/rss/
GOWORK=off golangci-lint run ./internal/adapter/rss/...
```

All four must pass cleanly. Coverage should remain ≥80%.

## Scope Notes (informational, not blockers)

- The CLAUDE.md edit (alert-crawler/CLAUDE.md gotchas) is technically
  WP23 territory per `owned_files`, but it's narrowly scoped to the
  T034 step ("Document UA in alert-crawler/CLAUDE.md") which is
  explicitly listed in the WP08 task body. Tolerable scope creep —
  WP23 will likely rewrite the whole file anyway.
- `errors.go` is correct and clean.
- Tests are well structured. Optional enhancement: a separate test
  asserting `If-Modified-Since` is sent (similar to TestFetch304's
  `If-None-Match` assertion) is already covered by
  `TestFetchLastModifiedHeader` — good.

## Downstream Notes

- WP09 (parser) consumes `FetchOutput.Body []byte` — contract is stable
  even after lint fixes. No rebase needed for WP09 once this lands.
- WP15 (runner) uses `errors.Is(err, rss.ErrNotModified)` per T033 step 3
  — that contract is unchanged.
