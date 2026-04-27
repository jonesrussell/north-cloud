---
work_package_id: WP04
title: Waaseyaa HTTP Client
dependencies:
- WP01
requirement_refs:
- FR-009
- FR-010
- FR-011
- FR-012
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T015
- T016
- T017
- T018
- T019
agent: "claude:opus-4.7:reviewer:reviewer"
shell_pid: "26240"
history:
- event: created
  at: '2026-04-27T05:55:00Z'
  by: /spec-kitty.tasks
authoritative_surface: signal-producer/internal/client/
execution_mode: code_change
mission_id: 01KQ6QZSNBQF3VW515AKM0SQ46
mission_slug: signal-producer-pipeline-01KQ6QZS
owned_files:
- signal-producer/internal/client/waaseyaa.go
- signal-producer/internal/client/retry.go
- signal-producer/internal/client/waaseyaa_test.go
- signal-producer/internal/client/retry_test.go
tags: []
---

# WP04 — Waaseyaa HTTP Client

## Objective

Implement the HTTP client that POSTs signal batches to Waaseyaa with retry, exponential backoff, and context cancellation. Parses the `IngestResult` response. Independent of the mapper and the producer main loop.

## Context

Read first:
- [spec.md](../spec.md) FR-009, FR-010, FR-011, FR-012, NFR-002 (retry budget ≤ 25s).
- [contracts/signals-post.yaml](../contracts/signals-post.yaml) — request/response shape, status code semantics.
- [data-model.md](../data-model.md) "IngestResult" section.
- [research.md](../research.md) D2 (in-package retry helper, no new dependencies).

Charter constraints: C-002 (golangci-lint clean), C-003 (`infrastructure/logger`), C-004 (no `os.Getenv`).

## Branch Strategy

Planning base: `main`. Merge target: `main`. Lane workspace from `lanes.json`. Independent leaf WP — runs in parallel with WP02 and WP03 after WP01 lands.

## Subtask Guidance

### T015 — Define `WaaseyaaClient` interface and types

**Purpose**: Establish the public API shape and the IngestResult struct.

**Steps**:

1. Create `signal-producer/internal/client/waaseyaa.go`. Package: `client`.
2. Avoid importing the mapper package — that creates a dependency cycle risk if mapper grows. Instead, accept a `[]any` or define a tiny `Signal` interface here. **Decision**: define a generic body type:
   ```go
   // SignalBatch is a stand-in payload type. The producer marshals
   // mapper.Signal values into a generic JSON shape that satisfies the
   // contract; the client treats the body as opaque.
   type SignalBatch struct {
       Signals []any `json:"signals"`
   }
   ```
   This keeps `internal/client` free of an import on `internal/mapper`. The producer (WP05) will pass mapped signals into a `SignalBatch{Signals: signals}` literal at the call site.
3. Define:
   ```go
   type IngestResult struct {
       Ingested     int `json:"ingested"`
       Skipped      int `json:"skipped"`
       LeadsCreated int `json:"leads_created"`
       LeadsMatched int `json:"leads_matched"`
       Unmatched    int `json:"unmatched"`
   }

   type WaaseyaaClient interface {
       PostSignals(ctx context.Context, batch SignalBatch) (*IngestResult, error)
   }

   type httpClient struct {
       baseURL    string
       apiKey     string
       httpClient *http.Client
       logger     infralogger.Logger
   }

   func New(baseURL, apiKey string, log infralogger.Logger) WaaseyaaClient
   ```
4. Constants for the endpoint path (`signalsEndpoint = "/api/signals"`), header name (`headerAPIKey = "X-Api-Key"`), and HTTP timeout (`requestTimeout = 30 * time.Second`).

**Files**: `signal-producer/internal/client/waaseyaa.go`.

**Validation**:

- [ ] Interface defined; `New` constructor returns it.
- [ ] No import on `internal/mapper`.
- [ ] No `interface{}` (use `any`).

### T016 — Implement `PostSignals`

**Purpose**: One round-trip POST, JSON marshal, JSON unmarshal, status-code-driven error classification.

**Steps**:

1. Build the request URL: `baseURL + signalsEndpoint`. Use `url.JoinPath` to handle trailing-slash variations.
2. Marshal `batch` to JSON.
3. Build `http.NewRequestWithContext(ctx, http.MethodPost, ...)` with the JSON body. Set headers: `X-Api-Key`, `Content-Type: application/json`.
4. Wrap the call in the retry helper from T017. The retry helper takes a function returning `(*http.Response, error)` and decides whether to retry based on the result.
5. Read and unmarshal the response body into `IngestResult`. Close the body in a deferred call.
6. Status code handling (drives retry decision in T017):
   - 2xx → success, parse `IngestResult`, return.
   - 4xx → wrap as `errClient` (a sentinel that the retry helper recognizes as non-retryable).
   - 5xx → wrap as `errServer` (retryable).
   - Network/transport error → retryable.
7. On non-2xx, attempt to read the body for error context but do not error if the body is unreadable.
8. Log per-attempt at INFO with: URL, batch size, response status, duration. Use `infrastructure/logger`.

**Files**: `signal-producer/internal/client/waaseyaa.go` (extended).

**Validation**:

- [ ] Successful POST returns parsed `IngestResult`.
- [ ] 4xx returns an error wrapping `errClient`.
- [ ] 5xx returns an error wrapping `errServer`.
- [ ] Headers include `X-Api-Key` and `Content-Type: application/json`.
- [ ] Response body is closed (use `defer resp.Body.Close()`).

### T017 — Retry helper in `retry.go`

**Purpose**: Small in-package helper, ≤ 50 lines. Deliberately not promoted to `infrastructure/` (research D2).

**Steps**:

1. Create `signal-producer/internal/client/retry.go`. Package: `client`.
2. Define:
   ```go
   var defaultBackoffs = []time.Duration{
       1 * time.Second,
       5 * time.Second,
       15 * time.Second,
   }

   var (
       errClient = errors.New("client error: non-retryable 4xx")
       errServer = errors.New("server error: retryable 5xx")
   )

   // doWithRetry attempts fn up to len(backoffs)+1 times. It honors ctx
   // cancellation between attempts. It does not retry on errClient.
   func doWithRetry(ctx context.Context, log infralogger.Logger, backoffs []time.Duration, fn func() error) error
   ```
3. Implementation:
   - Try `fn()` once.
   - If error and `errors.Is(err, errClient)` or `ctx.Err() != nil` → return immediately.
   - Otherwise, for each backoff:
     - `select { case <-time.After(backoff): case <-ctx.Done(): return ctx.Err() }`.
     - Try `fn()` again.
     - Same termination rules.
   - After all backoffs exhausted, return the last error wrapped: `fmt.Errorf("retry exhausted after %d attempts: %w", attempts, err)`.
4. Total retry budget MUST be ≤ 25 seconds (NFR-002): 1 + 5 + 15 = 21s of sleeps, plus per-attempt request time bounded by `requestTimeout`. Document this in a comment.

**Files**: `signal-producer/internal/client/retry.go`.

**Validation**:

- [ ] First attempt with success returns nil immediately.
- [ ] `errClient` short-circuits retries.
- [ ] Context cancellation breaks out of `time.After` immediately.
- [ ] All four call sites (initial + 3 retries) bounded; no infinite loop possible.

### T018 — Wire context cancellation through the full call

**Purpose**: Ensure cancelling the context stops in-flight HTTP and all subsequent retries.

**Steps**:

1. The HTTP request from T016 already takes `ctx` via `http.NewRequestWithContext`; that handles in-flight cancellation.
2. The retry helper from T017 already breaks out of backoff sleep on `ctx.Done()`.
3. Add a final defensive check before each retry attempt: `if err := ctx.Err(); err != nil { return err }`.

**Files**: `signal-producer/internal/client/waaseyaa.go` and `retry.go` (verify wiring; no new file).

**Validation**:

- [ ] Cancelling the context during a long retry sleep returns within ~10ms (the select on ctx.Done()).
- [ ] Cancelling during an in-flight HTTP request returns the wrapped `context.Canceled` error.

### T019 — Unit tests

**Purpose**: ≥ 80% coverage per NFR-003. Use `httptest.Server` to simulate Waaseyaa.

**Steps**:

1. Create `signal-producer/internal/client/waaseyaa_test.go` and `retry_test.go`. Test helpers start with `t.Helper()`.
2. `waaseyaa_test.go` tests:
   - `TestPostSignals_Success`: mock returns 200 + IngestResult JSON → client returns parsed struct.
   - `TestPostSignals_RetriesOn500`: mock returns 500 twice then 200 → client succeeds on 3rd try; assert request count is exactly 3.
   - `TestPostSignals_NoRetryOn400`: mock returns 400 → client errors immediately; request count is exactly 1.
   - `TestPostSignals_RetriesExhausted`: mock always returns 503 → client errors after 4 attempts (initial + 3 retries) wrapping `errServer`.
   - `TestPostSignals_ContextCancelledDuringRetry`: mock sleeps long; cancel ctx; client returns ctx.Canceled within bounded time.
   - `TestPostSignals_HeadersSent`: assert `X-Api-Key` and `Content-Type` arrive at the test server.
   - `TestPostSignals_BodyShape`: assert request body is `{"signals":[...]}`.
3. `retry_test.go` tests:
   - `TestDoWithRetry_FirstAttemptSucceeds`: fn returns nil first → 1 invocation.
   - `TestDoWithRetry_RetryableThenSuccess`: fn fails twice, succeeds → 3 invocations.
   - `TestDoWithRetry_NonRetryableShortCircuits`: fn returns errClient → 1 invocation.
   - `TestDoWithRetry_ExhaustsBackoffs`: fn always fails → 4 invocations, returns last error wrapped.
   - `TestDoWithRetry_ContextCancelDuringSleep`: cancel during the 5-second backoff sleep; fn invocation count stops; returns ctx error.
4. Use short backoffs in tests (e.g., `[]time.Duration{10*time.Millisecond, 20*time.Millisecond, 30*time.Millisecond}`) so tests run fast.

**Files**: `signal-producer/internal/client/waaseyaa_test.go`, `signal-producer/internal/client/retry_test.go`.

**Validation**:

- [ ] `task test:signal-producer` passes; runtime ≤ 5s for the client tests.
- [ ] Coverage ≥ 80% on both files.
- [ ] Headers asserted at the test server side (FR-009).

## Definition of Done

- [ ] All five subtasks complete with their validation checklists ticked.
- [ ] `task lint:signal-producer` and `task test:signal-producer` green.
- [ ] Coverage ≥ 80% on `waaseyaa.go` and `retry.go`.
- [ ] No files modified outside `owned_files`.
- [ ] No import on `internal/mapper` (cross-package ordering hygiene).

## Reviewer Guidance

1. **No mapper import**: `grep -n 'internal/mapper' signal-producer/internal/client/` should return nothing. Reject if present.
2. **Retry budget ≤ 25s**: confirm `defaultBackoffs` sums to 21s, plus the 30s `requestTimeout` per attempt is enough headroom for NFR-002.
3. **4xx genuinely doesn't retry**: walk the test that asserts a single request count on 400.
4. **Context cancellation is real**: the cancel-during-retry test must finish within 100ms of the cancel; if it takes 5 seconds, the select isn't wired correctly.
5. **Test backoffs are short**: tests use millisecond backoffs, not the production 1/5/15s.
6. **Header check happens at the server side**: assertions in the `httptest` handler, not on the client side. Reject if only client-side because that doesn't prove the wire.

## Risks and Mitigations

| Risk                                                                          | Mitigation                                                                                                                |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Retry helper grows generic enough to belong in `infrastructure/`.            | Defer until a second consumer needs it. Comment in code references the in-mission research D2 decision.                 |
| Test relies on `time.Sleep` being precise, becomes flaky.                    | Use short, generous backoffs in tests; assert relative ordering and call counts, not wall-clock thresholds.              |
| `errClient`/`errServer` sentinels miss a status-code edge case.              | Treat 4xx as `errClient`, 5xx as `errServer`, all other (network, dial errors) as transient retryable. Tested.           |

## Implementation Command

```bash
spec-kitty agent action implement WP04 --agent <agent-name> --mission signal-producer-pipeline-01KQ6QZS
```

## Activity Log

- 2026-04-27T06:59:23Z – claude:opus-4.7:implementer:implementer – shell_pid=12784 – Started implementation via action command
- 2026-04-27T07:03:04Z – claude:opus-4.7:implementer:implementer – shell_pid=12784 – Waaseyaa HTTP client ready for review
- 2026-04-27T07:03:32Z – claude:opus-4.7:reviewer:reviewer – shell_pid=26240 – Started review via action command
