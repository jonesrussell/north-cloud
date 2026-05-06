---
work_package_id: WP08
title: RSS HTTP Client (with ETag Conditional GET)
dependencies:
- WP05
- WP06
requirement_refs:
- FR-001
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T032
- T033
- T034
- T035
phase: B
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/adapter/rss/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/adapter/rss/client.go
- alert-crawler/internal/adapter/rss/client_test.go
- alert-crawler/internal/adapter/rss/errors.go
priority: P1
tags: []
---

# WP08 — RSS HTTP Client (with ETag Conditional GET)

## Objective

Build the HTTP fetcher for RSS feeds. Supports ETag and Last-Modified conditional GETs (304 short-circuit), configurable User-Agent and timeouts, transient-vs-structural error classification. Returns raw feed bytes plus updated cache headers; parsing is WP09's concern.

## Context

- Spec §3 FR-001
- Plan §Component Design (Acquisition)
- Research R-001: feed at `https://www.safersites.ca/drugalerts.rss`, 4h server-side cache, ETag present, no Last-Modified
- TC-001 (acquisition is RSS-only for v1)

## Branch Strategy

Standard. Depends on WP05, WP06.

## Subtasks

### T032 — Create `internal/adapter/rss/client.go`

**Purpose**: HTTP fetcher with conditional GET semantics.

**Steps**:
1. Create `alert-crawler/internal/adapter/rss/client.go`:
   ```go
   package rss

   import (
       "context"
       "errors"
       "fmt"
       "io"
       "net/http"
       "time"

       "github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
   )

   type Client struct {
       httpClient *http.Client
       userAgent  string
   }

   type FetchInput struct {
       Source       domain.AlertSource
       LastETag     string
       LastModified string
   }

   type FetchOutput struct {
       Body         []byte
       ETag         string
       LastModified string
       StatusCode   int
   }

   func New(opts ...Option) *Client {
       c := &Client{
           httpClient: &http.Client{Timeout: 30 * time.Second},
           userAgent:  "alert-crawler/1.0 (+https://northcloud.one)",
       }
       for _, o := range opts {
           o(c)
       }
       return c
   }

   func (c *Client) Fetch(ctx context.Context, in FetchInput) (*FetchOutput, error) {
       req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.Source.FeedURL, nil)
       if err != nil {
           return nil, fmt.Errorf("build request: %w", err)
       }
       req.Header.Set("User-Agent", c.userAgent)
       req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml")
       if in.LastETag != "" {
           req.Header.Set("If-None-Match", in.LastETag)
       }
       if in.LastModified != "" {
           req.Header.Set("If-Modified-Since", in.LastModified)
       }

       resp, err := c.httpClient.Do(req)
       if err != nil {
           return nil, classifyTransport(err)
       }
       defer resp.Body.Close()

       if resp.StatusCode == http.StatusNotModified {
           return nil, ErrNotModified
       }
       if resp.StatusCode >= 500 {
           return nil, fmt.Errorf("upstream 5xx: %d: %w", resp.StatusCode, ErrTransient)
       }
       if resp.StatusCode >= 400 {
           return nil, fmt.Errorf("upstream 4xx: %d: %w", resp.StatusCode, ErrStructural)
       }

       body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes))
       if err != nil {
           return nil, fmt.Errorf("read body: %w", err)
       }
       return &FetchOutput{
           Body:         body,
           ETag:         resp.Header.Get("ETag"),
           LastModified: resp.Header.Get("Last-Modified"),
           StatusCode:   resp.StatusCode,
       }, nil
   }

   const maxFeedBytes = 5 * 1024 * 1024 // 5MB safety cap
   ```

2. Define `Option` functional options for `WithUserAgent`, `WithTimeout`, `WithHTTPClient`.

**Files**:
- `alert-crawler/internal/adapter/rss/client.go` (new, ~120 lines).

### T033 — `ErrNotModified` and ETag/Last-Modified return values

**Purpose**: Semantic 304 path; no body to parse.

**Steps**:
1. Create `alert-crawler/internal/adapter/rss/errors.go`:
   ```go
   package rss

   import "errors"

   // ErrNotModified is returned when the upstream responds 304 Not Modified.
   // Callers must treat this as a no-op cycle: do not parse, do not modify
   // the catalogue, but DO update poll_checkpoint.last_polled_at.
   var ErrNotModified = errors.New("rss: not modified")

   // ErrTransient indicates a retry-worthy failure (network, 5xx).
   var ErrTransient = errors.New("rss: transient error")

   // ErrStructural indicates a non-retryable failure (4xx, malformed feed format).
   var ErrStructural = errors.New("rss: structural error")
   ```
2. The `Fetch` function from T032 already returns `ErrNotModified` and wraps `ErrTransient`/`ErrStructural` for 5xx/4xx responses.
3. The runner (WP15) uses `errors.Is(err, rss.ErrNotModified)` to short-circuit gracefully.

**Files**:
- `alert-crawler/internal/adapter/rss/errors.go` (new, ~20 lines).

### T034 — User-Agent and timeout configuration

**Purpose**: Polite, configurable client. The User-Agent is publicly recognizable.

**Steps**:
1. Already wired via `Option` functional options in T032.
2. Default UA: `alert-crawler/1.0 (+https://northcloud.one)`.
3. Default timeout: 30s (sufficient for the 50KB feed; conservative enough not to hang the poll cycle).
4. Document UA in `alert-crawler/CLAUDE.md` so reviewers can match it against safersites.ca server logs if debugging access.

**Files**:
- `alert-crawler/internal/adapter/rss/client.go` (already done in T032).
- `alert-crawler/CLAUDE.md` (modify the gotchas section to mention UA).

### T035 — Unit tests with `httptest.Server`

**Purpose**: Exhaustive coverage of the fetch path.

**Steps**:
1. Create `alert-crawler/internal/adapter/rss/client_test.go` with cases:
   - **TestFetch200**: server returns 200 + body + ETag + Last-Modified; `Fetch` returns `FetchOutput` with body and headers.
   - **TestFetch304**: server returns 304; client must have sent `If-None-Match: <stored ETag>`; `Fetch` returns `ErrNotModified`.
   - **TestFetch5xxIsTransient**: server returns 503; `Fetch` returns wrapped `ErrTransient`.
   - **TestFetch4xxIsStructural**: server returns 404; `Fetch` returns wrapped `ErrStructural`.
   - **TestFetchTimeout**: server hangs longer than client timeout; `Fetch` returns a context-deadline error.
   - **TestFetchUserAgent**: server captures the request; assert `User-Agent` header is the expected value.
   - **TestFetchBodySizeCap**: server returns 6MB body; client must cap at 5MB and not OOM.
2. Use `httptest.NewServer` and `httptest.NewTLSServer` (the latter exercises TLS path; not strictly required but a nice belt-and-suspenders).
3. Use `t.Helper()`.

**Files**:
- `alert-crawler/internal/adapter/rss/client_test.go` (new, ~200 lines).

**Validation**:
- `task test:alert-crawler` passes for the rss package.
- Coverage ≥80%.

## Definition of Done

- All four subtasks complete.
- Client correctly handles 304, 200, 5xx, 4xx, timeout, oversized body.
- Unit tests cover every code path.
- Lint and vet clean.

## Risks

- **RR-001**: Cloudflare may eventually block the alert-crawler container. Mitigation deferred to post-mission. The User-Agent identifies the service if outreach is needed.
- **Feed format change**: not this WP's concern; WP09 handles parser-degraded fallback.

## Reviewer Guidance

- Verify `If-None-Match` is sent when `LastETag` is non-empty.
- Verify the body is limited to a sane size cap.
- Verify error classifications are correct (transient vs structural).
- Verify the User-Agent string follows convention (`{name}/{version} (+{url})`).

## Implementation Command

```bash
spec-kitty agent action implement WP08 --agent <name>
```

Depends on WP05, WP06. Parallel-safe with WP09–WP14.
