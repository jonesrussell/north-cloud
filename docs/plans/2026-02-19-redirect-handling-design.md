# Frontier Redirect Handling and Canonical URL Design

**Date**: 2026-02-19
**Status**: Approved

## Problem

Frontier URLs that encounter HTTP redirects are being treated as failed fetches. After retries they are marked **dead**, even when the failure is due to redirects (e.g. "too many redirects" or redirect chains), not true unreachability. Operators cannot distinguish redirect-related failures from real dead URLs (404, connection refused). Additionally, when a fetch succeeds after following redirects, the frontier row keeps the original request URL instead of the final (canonical) URL, which hurts deduplication and consistency.

## Approach Summary

1. **Follow redirects** in the frontier fetcher with a configurable hop limit; use the response’s final URL for canonicalization.
2. **Update the frontier row to the final URL** when a fetch succeeds after redirects (recompute `url`, `url_hash`, `host`); on unique constraint (another row already has that URL), fall back to marking the row fetched without changing URL.
3. **Distinguish redirect failures** via a canonical reason (`too_many_redirects`) in `last_error` so logs and dashboards can filter them without schema changes; retry/dead policy unchanged.

---

## Section 1 — Fetcher HTTP Client and Redirect Behavior

**Goal:** Follow redirects in the frontier fetcher with a bounded count and a single configuration surface.

- **Fetcher HTTP client:** Built in bootstrap from config: `FollowRedirects` (bool) and `MaxRedirects` (int, default 5). If `FollowRedirects` is true, use a custom `CheckRedirect` that returns a sentinel error after `MaxRedirects` hops so we control the limit and get a clear error. If false, set `CheckRedirect` to return `http.ErrUseLastResponse` so the client does not follow.
- **Config:** Fetcher config in bootstrap reads `FollowRedirects` and `MaxRedirects` from the main crawler config (existing env: `CRAWLER_FOLLOW_REDIRECTS`, `CRAWLER_MAX_REDIRECTS`).
- **Final URL:** After `client.Do(req)`, use `resp.Request.URL.String()` as the final URL when `statusCode` is 200 or 304 for the success path.

---

## Section 2 — Frontier Update to Final URL and "Final URL Already Exists"

**Goal:** On success after redirects, store the final URL in the frontier and handle unique `url_hash` conflicts.

- **New method:** `UpdateFetchedWithFinalURL(ctx, id, finalURL string, params FetchedParams) error`. Same fetch-state updates as `UpdateFetched`, plus set `url`, `url_hash`, `host` from normalized `finalURL` (using `frontier.NormalizeURL`, `frontier.URLHash`, `frontier.ExtractHost`).
- **Same URL:** If normalized final URL equals the row’s current URL (same hash), call existing `UpdateFetched(id, params)` only.
- **Different URL (redirect):** Call `UpdateFetchedWithFinalURL`. On PostgreSQL unique constraint violation (23505) for `url_hash`, catch and call `UpdateFetched(id, params)` without changing URL; return success. Repository owns this fallback so the worker stays simple.
- **Interface:** Extend `FrontierClaimer` with `UpdateFetchedWithFinalURL`; implement in `FrontierRepository` and in the bootstrap adapter.

---

## Section 3 — Redirect-Failure Reasons (2B)

**Goal:** Make redirect-related failures visible and distinct from real dead URLs in logs and dashboards.

- **Reason:** Use a single canonical reason `too_many_redirects` stored in `last_error` when the fetcher hits the redirect hop limit. Optional later: `redirect_loop` if we add loop detection.
- **Worker:** In `handleFetchError`, if the error is the redirect-limit sentinel (e.g. `errors.Is(fetchErr, errTooManyRedirects)`), call `UpdateFailed` with reason string `too_many_redirects` instead of raw error message. All other errors keep `fetchErr.Error()`.
- **Logging:** When recording a redirect failure, log with a structured field (e.g. `"reason": "too_many_redirects"`) plus url/id.
- **Dashboard:** Filter by `last_error` (e.g. equals or contains `too_many_redirects`). No API or schema change. Retry behavior unchanged.
- **Constants:** In fetcher package: `reasonTooManyRedirects = "too_many_redirects"`. Client’s `CheckRedirect` returns a sentinel error (e.g. `errTooManyRedirects`) so the worker can map to the canonical reason.

---

## Section 4 — Testing and Edge Cases

- **Fetcher unit tests:** Redirect following (301→302→200, assert final URL and update path); hop limit (302 chain, assert `UpdateFailed` with `too_many_redirects`); no redirect (200, assert `UpdateFetched` only); redirect disabled (302, no follow).
- **Repository unit tests:** `UpdateFetchedWithFinalURL` success (url/hash/host updated); unique violation (23505 → fallback to `UpdateFetched`, url unchanged); invalid final URL (error, no UPDATE).
- **Edge cases:** Same URL after redirects (normalized match → `UpdateFetched` only); conditional headers unchanged; concurrent redirects to same canonical URL (first wins, second gets 23505 fallback). Optional end-to-end test with real redirect for MVP.

---

## Out of Scope (Deferred)

- Redirect allowlists or same-domain-only policy (Section 3 from brainstorming).
- Never marking redirect-only failures as dead; they continue to exhaust retries unless we change policy later.
- `redirect_loop` distinct from `too_many_redirects` until we implement loop detection.

---

## References

- URL Frontier design: `docs/plans/2026-02-16-url-frontier-design.md`
- Frontier schema: `crawler/migrations/014_create_url_frontier.up.sql` (unique on `url_hash`)
- Normalization: `crawler/internal/frontier/normalize.go` (`NormalizeURL`, `URLHash`, `ExtractHost`)
- Fetcher worker: `crawler/internal/fetcher/worker.go`; bootstrap: `crawler/internal/bootstrap/services.go`
