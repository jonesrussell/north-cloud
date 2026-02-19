# Frontier Redirect Handling Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement bounded redirect following in the frontier fetcher, canonical URL updates when a fetch succeeds after redirects (with 23505 fallback), and a distinct `too_many_redirects` failure reason for logs/dashboards.

**Architecture:** Fetcher HTTP client gets configurable redirect following (custom CheckRedirect for hop limit); worker uses final URL on 200/304 to call either UpdateFetched or UpdateFetchedWithFinalURL; repository implements UpdateFetchedWithFinalURL with unique-constraint fallback to UpdateFetched. Redirect-limit errors map to canonical reason in last_error.

**Tech Stack:** Go 1.24+, crawler (fetcher, frontier, bootstrap), PostgreSQL (url_frontier), existing frontier.NormalizeURL/URLHash/ExtractHost.

**Design reference:** `docs/plans/2026-02-19-redirect-handling-design.md`

---

## Task 1: Fetcher config and redirect-limited HTTP client

**Files:**
- Modify: `crawler/internal/config/crawler/config.go` (FetcherConfig if present; else ensure FollowRedirects/MaxRedirects are available where fetcher config is built)
- Modify: `crawler/internal/bootstrap/services.go` (build fetcher HTTP client with redirect policy)
- Test: `crawler/internal/fetcher/worker_test.go` (later; client is injected)

**Step 1: Expose redirect config to fetcher bootstrap**

Fetcher config lives in `crawler/internal/fetcher/config.go` (type `Config` with `WorkerCount`, `RequestTimeout`, `MaxRetries`). Add `FollowRedirects bool` and `MaxRedirects int` to that struct with env tags (e.g. `FETCHER_FOLLOW_REDIRECTS`, `FETCHER_MAX_REDIRECTS`) and in `WithDefaults()` set default `MaxRedirects = 5`, `FollowRedirects = true`. In `crawler/internal/config/config.go`, when building/merging fetcher config from YAML or env, ensure these are passed through (main crawler config has `FollowRedirects`/`MaxRedirects` in `crawler/internal/config/crawler/config.go`; either merge from there or use fetcher’s own env).

**Step 2: Add redirect-limited transport helper**

In `crawler/internal/bootstrap/services.go`, where `httpClient := &http.Client{Timeout: fetcherCfg.RequestTimeout}` is built for the worker pool, replace with a client that:
- If `FollowRedirects` is false: set `CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }`.
- If true: set a custom `CheckRedirect` that, when `len(redirects) >= fetcherCfg.MaxRedirects`, returns a sentinel error (e.g. `errTooManyRedirects`). The sentinel must be defined in a package the fetcher can import (e.g. in `crawler/internal/fetcher` as `var ErrTooManyRedirects = errors.New("too many redirects")`) so the worker can use `errors.Is`. Bootstrap will need to use that same sentinel when building the client.

**Step 3: Implement redirect limit in CheckRedirect**

Create in fetcher package (e.g. in `worker.go` or a small `redirect.go` in fetcher):
- `var ErrTooManyRedirects = errors.New("too many redirects")`
- `func RedirectPolicy(maxHops int) func(*http.Request, []*http.Request) error` that returns `ErrTooManyRedirects` when `len(redirects) >= maxHops`.

In bootstrap, build client:
- `var checkRedirect func(*http.Request, []*http.Request) error`
- if !FollowRedirects: checkRedirect = useLastResponse
- else: checkRedirect = fetcher.RedirectPolicy(fetcherCfg.MaxRedirects)
- `httpClient := &http.Client{Timeout: ..., CheckRedirect: checkRedirect}`

**Step 4: Wire config into GetFetcherConfig**

Ensure `GetFetcherConfig()` (or equivalent) returns FollowRedirects and MaxRedirects. If the fetcher config struct is in config/crawler, add fields and map from env/yaml; then in bootstrap pass this client into `fetcher.NewWorkerPool` (worker pool must accept *http.Client or build it from config—see Task 2).

**Step 5: Commit**

```bash
git add crawler/internal/config/ crawler/internal/bootstrap/services.go crawler/internal/fetcher/
git commit -m "feat(crawler): add redirect-limited HTTP client for frontier fetcher"
```

---

## Task 2: Worker pool accepts HTTP client and uses final URL

**Files:**
- Modify: `crawler/internal/fetcher/worker.go` (accept client in config or constructor; use resp.Request.URL on success; call UpdateFetched vs UpdateFetchedWithFinalURL)
- Modify: `crawler/internal/bootstrap/services.go` (pass built httpClient into NewWorkerPool)

**Step 1: WorkerPoolConfig and NewWorkerPool**

Add `HTTPClient *http.Client` to `WorkerPoolConfig`. In `NewWorkerPool`, use `cfg.HTTPClient` if non-nil; otherwise keep existing `&http.Client{Timeout: cfg.RequestTimeout}` for backward compatibility. In bootstrap, pass the redirect-aware `httpClient` in the config.

**Step 2: fetchPage and final URL**

In `fetchPage`, after `resp, doErr := wp.httpClient.Do(req)`, no change to error path. On success, return body, statusCode, and the final URL: change signature to return `(body []byte, statusCode int, finalURL string, err error)` and set `finalURL = resp.Request.URL.String()`.

**Step 3: ProcessURL success path**

In `ProcessURL`, when `handleStatusCode` is used for 200/304, we need to pass `finalURL`. So `handleStatusCode` should accept `finalURL string` and:
- If normalized finalURL equals normalized claimed URL (use frontier.NormalizeURL for both and compare), call `wp.frontier.UpdateFetched(ctx, furl.ID, params)`.
- Else call `wp.frontier.UpdateFetchedWithFinalURL(ctx, furl.ID, finalURL, params)`.

To avoid circular dependency, the worker can normalize using the frontier package (it already depends on domain). Add import for `crawler/internal/frontier` and compare normalized URLs. FetchedParams is already in fetcher; the new method is on FrontierClaimer.

**Step 4: Update FrontierClaimer interface and handleFetchError**

In `worker.go`, add to FrontierClaimer:
- `UpdateFetchedWithFinalURL(ctx context.Context, id, finalURL string, params FetchedParams) error`

In `handleFetchError`: if `errors.Is(fetchErr, ErrTooManyRedirects)`, call `UpdateFailed(ctx, furl.ID, reasonTooManyRedirects, wp.maxRetries)` where `reasonTooManyRedirects = "too_many_redirects"`. Else call `UpdateFailed(ctx, furl.ID, fetchErr.Error(), wp.maxRetries)`.

**Step 5: Adjust handleStatusCode and handleSuccess**

Ensure handleSuccess (and handleNotModified) receive finalURL and call either UpdateFetched or UpdateFetchedWithFinalURL as above. handleStatusCode already receives body and statusCode; add finalURL parameter and pass it through to handleSuccess/handleNotModified.

**Step 6: Run tests**

Run: `cd crawler && go test ./internal/fetcher/... -v`
Fix any compile errors and update tests that mock FrontierClaimer (add UpdateFetchedWithFinalURL to mock) and that call fetchPage/ProcessURL (adjust for new return value or finalURL).

**Step 7: Commit**

```bash
git add crawler/internal/fetcher/worker.go
git commit -m "feat(crawler): use final URL on success and too_many_redirects reason on limit"
```

---

## Task 3: Frontier repository UpdateFetchedWithFinalURL and 23505 fallback

**Files:**
- Modify: `crawler/internal/database/frontier_repository.go` (add UpdateFetchedWithFinalURL, detect 23505, fallback to UpdateFetched)
- Modify: `crawler/internal/bootstrap/fetcher_adapters.go` (implement UpdateFetchedWithFinalURL on adapter)
- Test: `crawler/internal/database/frontier_repository_test.go`

**Step 1: Write failing repository test**

Add test `TestFrontierRepository_UpdateFetchedWithFinalURL_Success`: one row with id A, url U1. Call UpdateFetchedWithFinalURL(ctx, A, finalURL, params) where finalURL normalizes to U2 (different from U1). Assert row A has url=U2, url_hash=hash(U2), host=host(U2), status=fetched, fetch_count incremented, content_hash/etag/last_modified set.

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/database/ -run TestFrontierRepository_UpdateFetchedWithFinalURL -v`
Expected: FAIL (method or function missing).

**Step 3: Implement UpdateFetchedWithFinalURL (success path)**

In frontier_repository.go, add:
- Normalize finalURL with frontier.NormalizeURL, compute urlHash with frontier.URLHash, host with frontier.ExtractHost. If any error, return it.
- UPDATE url_frontier SET url=$1, url_hash=$2, host=$3, status='fetched', last_fetched_at=NOW(), fetch_count=fetch_count+1, content_hash=$4, etag=$5, last_modified=$6, retry_count=0, updated_at=NOW() WHERE id=$7.
- Use execRequireRows. If error is PostgreSQL 23505 (unique_violation), fall back: call UpdateFetched(ctx, id, params) and return nil.

**Step 4: Detect 23505 and fallback**

Use `var errUniqueViolation *pq.Error` or `errors.Is`/`pq.Error` type assertion; code 23505. On that, run UpdateFetched(ctx, id, params) and return nil.

**Step 5: Add test for 23505 fallback**

Test: two rows, id A (url U1) and id B (url U2, url_hash H2). Update A with finalURL that normalizes to U2 (hash H2). Assert: no error; row A has status=fetched, url still U1 (unchanged); row B unchanged.

**Step 6: Implement adapter**

In fetcher_adapters.go, add `UpdateFetchedWithFinalURL(ctx, id, finalURL string, params fetcher.FetchedParams) error` that calls `a.repo.UpdateFetchedWithFinalURL(ctx, id, finalURL, database.FetchedParams{...})`. Extend database.FetchedParams if needed (already has ContentHash, ETag, LastModified).

**Step 7: Run repository tests**

Run: `cd crawler && go test ./internal/database/ -v -run Frontier`
Expected: PASS.

**Step 8: Commit**

```bash
git add crawler/internal/database/frontier_repository.go crawler/internal/database/frontier_repository_test.go crawler/internal/bootstrap/fetcher_adapters.go
git commit -m "feat(crawler): UpdateFetchedWithFinalURL with 23505 fallback to UpdateFetched"
```

---

## Task 4: Fetcher unit tests (redirect follow, hop limit, same URL, no follow)

**Files:**
- Modify: `crawler/internal/fetcher/worker_test.go` (add tests for redirect paths and too_many_redirects)
- Optional: `crawler/internal/fetcher/redirect_test.go` if you extract RedirectPolicy to a separate func

**Step 1: Test redirect following and final URL**

Add test: mock server 301 → 302 → 200 with distinct URLs. WorkerPool with FollowRedirects true, MaxRedirects 5, mock frontier. Claim one URL (the first in chain). ProcessURL. Assert mock frontier received UpdateFetchedWithFinalURL with final URL = last URL in chain, or UpdateFetched if normalized same. May require building WorkerPool with a real redirect-following client in test.

**Step 2: Test hop limit**

Mock server that 302 redirects to self (or chain of 6). WorkerPool with MaxRedirects 5. ProcessURL. Assert UpdateFailed was called with reason "too_many_redirects" (or lastError containing it); UpdateFetched/UpdateFetchedWithFinalURL not called.

**Step 3: Test no redirect (200)**

Mock server returns 200. Assert UpdateFetched called, UpdateFetchedWithFinalURL not called (or called with same URL so fallback is equivalent).

**Step 4: Test redirect disabled**

With FollowRedirects false, mock returns 302. Assert we do not follow; handleStatusCode or handleFetchError gets non-2xx path; UpdateFailed called with error message (not necessarily too_many_redirects).

**Step 5: Run fetcher tests**

Run: `cd crawler && go test ./internal/fetcher/... -v`
Expected: PASS.

**Step 6: Commit**

```bash
git add crawler/internal/fetcher/worker_test.go
git commit -m "test(crawler): fetcher redirect follow, hop limit, and reason tests"
```

---

## Task 5: Lint and integration check

**Files:**
- Modify: any files that need nolint or small fixes

**Step 1: Lint**

Run: `task lint:crawler` or `cd crawler && golangci-lint run`
Fix any issues (e.g. unused imports, err check).

**Step 2: Full crawler tests**

Run: `task test:crawler` or `cd crawler && go test ./...`
Expected: PASS.

**Step 3: Commit if needed**

```bash
git add -A && git status
# If changes: git commit -m "chore(crawler): lint and test fixes for redirect handling"
```

---

## Task 6: Docs and design reference

**Files:**
- Modify: `crawler/CLAUDE.md` or `crawler/README.md` (optional: one line on redirect behavior and too_many_redirects in frontier)
- Design doc already at `docs/plans/2026-02-19-redirect-handling-design.md`

**Step 1: Add one-line note**

In crawler docs, add under Frontier or Fetcher: "Frontier fetcher follows redirects (configurable via CRAWLER_FOLLOW_REDIRECTS / CRAWLER_MAX_REDIRECTS). On success, the frontier row is updated to the final URL when different; redirect limit failures are stored as last_error=too_many_redirects."

**Step 2: Commit**

```bash
git add crawler/CLAUDE.md
git commit -m "docs(crawler): note redirect handling and too_many_redirects in frontier"
```

---

## Execution summary

- **Task 1:** Config + redirect-limited client and sentinel error.
- **Task 2:** Worker uses final URL and UpdateFetchedWithFinalURL / UpdateFetched; handleFetchError maps ErrTooManyRedirects to reason.
- **Task 3:** Repository UpdateFetchedWithFinalURL with 23505 fallback; adapter.
- **Task 4:** Fetcher unit tests for redirect paths.
- **Task 5:** Lint and full tests.
- **Task 6:** Short doc note.

After saving this plan, offer execution choice per writing-plans skill.
