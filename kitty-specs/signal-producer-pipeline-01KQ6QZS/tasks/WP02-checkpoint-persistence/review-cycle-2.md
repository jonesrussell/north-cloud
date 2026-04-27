---
affected_files: []
cycle_number: 2
mission_slug: signal-producer-pipeline-01KQ6QZS
reproduction_command:
reviewed_at: '2026-04-27T06:39:55Z'
reviewer_agent: unknown
verdict: rejected
wp_id: WP02
---

# Review Feedback — WP02 (cycle 1)

## Critical Issues (must fix)

- **Package does not compile.** `signal-producer/go.mod` is currently bare:
  ```
  module github.com/jonesrussell/north-cloud/signal-producer
  go 1.26
  ```
  No `require` block, no `replace` directive. `checkpoint.go` imports
  `github.com/jonesrussell/north-cloud/infrastructure/logger`, which means
  the package cannot build and the new tests cannot run. Reviewing-by-
  inspection only is unacceptable for a persistence primitive — we should
  be able to `go test` it now, not at WP05 integration.

  Amend the WP02 commit to also touch `signal-producer/go.mod` and
  `signal-producer/go.sum`:

  1. `require github.com/jonesrussell/north-cloud/infrastructure v0.0.0`
  2. `replace github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure`
     (matches the convention in `auth/go.mod` line 57 and every other
     in-repo Go service).
  3. `cd signal-producer && GOWORK=off go mod tidy` to populate go.sum
     (and pull whatever transitive deps `infrastructure/logger` needs).
  4. Verify `GOWORK=off go build ./internal/producer/...` and
     `GOWORK=off go test ./internal/producer/...` both succeed.

  This expands WP02's `owned_files` set to include `signal-producer/go.mod`
  and `signal-producer/go.sum`. That's intentional and the cleaner long-
  term fix; a Go package that doesn't compile is worse than a planning
  gap. Note this expansion in the WP02 task file when you re-plan.

## Should Fix

- None. The implementation itself is solid (see "verified" below).

## Nice to Have

- The atomic-write fault-injection technique (pre-create canonical path
  as a directory so `os.Rename` fails) is clever and cross-platform —
  worth a one-line comment in `checkpoint.go` near `os.Rename` that
  references the test, so future readers understand why we're rename-
  rather-than-overwrite-tolerant.
- Consider adding an `errors.Is(err, fs.ErrPermission)` branch in
  `LoadCheckpoint` to log a more specific message before wrapping. Not
  required for WP02; optional polish.

## What was verified (for the record)

- Diff scope: exactly the two files claimed (`git show 332c5299 --stat`).
- Constants present and named: `DefaultColdStartLookback = 24 * time.Hour`,
  `checkpointFileMode os.FileMode = 0o640`. No magic numbers.
- `Checkpoint` struct has `LastSuccessfulRun` + `LastBatchSize` with
  correct snake_case JSON tags. `ConsecutiveEmpty` correctly deferred
  to WP05 per spec.
- `LoadCheckpoint` signature, missing-file path, corrupt-file path
  (WARN + cold-start), invalid-values path, and wrapped-error path all
  match the spec.
- `SaveCheckpoint` + `writeAtomic` follow open-write-fsync-close-rename
  with cleanup on every error branch and explicit mode 0o640 in
  `os.OpenFile`.
- 8 distinct tests; all helpers use `t.Helper()`; `t.TempDir()`
  everywhere; Windows skips are appropriate for the mode-related tests.
- No `interface{}` and no `os.Getenv` in either file.
- `auth/go.mod` confirmed convention:
  `replace github.com/jonesrussell/north-cloud/infrastructure => ../infrastructure`.
