---
affected_files: []
cycle_number: 1
mission_slug: signal-producer-pipeline-01KQ6QZS
reproduction_command:
reviewed_at: '2026-04-27T07:18:22Z'
reviewer_agent: unknown
verdict: rejected
wp_id: WP05
---

# WP05 Review — Cycle 1

**Verdict**: REJECT (one blocker, narrowly scoped)

## Blocker

### B1. All-hits-fail-mapping path does NOT increment ConsecutiveEmpty

In `signal-producer/internal/producer/producer.go`, function `deliverHits` (lines ~220–233), when every hit fails to map (`len(signals) == 0`), the code logs a `run_summary` and returns `nil` — but it does NOT increment `cp.ConsecutiveEmpty`, and does NOT save the checkpoint.

This contradicts the implementer's stated rationale in the commit message ("malformed docs are deterministic; failing systemd would spam") and breaks the source-down detection contract (FR-019, D7) from the operator's perspective:

- ES had hits, but none could be delivered, and the checkpoint did not advance.
- The next run will return the same hits (deterministically malformed), again deliver zero, again advance nothing.
- This is functionally indistinguishable from "ES is empty" for monitoring purposes — yet the source-down WARN will never fire because the counter never moves off zero.

The reviewer brief explicitly calls this out as a REJECT condition: *"ConsecutiveEmpty NOT incremented when all hits unmappable."*

**Fix** (small, scoped to this branch):

```go
if len(signals) == 0 {
    cp.ConsecutiveEmpty++
    if cp.ConsecutiveEmpty == sourceDownThreshold {
        p.log.Warn(
            "source appears down",
            infralogger.String("event", "source_down"),
            infralogger.String("code", sourceDownCode),
            infralogger.Int("consecutive_empty", cp.ConsecutiveEmpty),
            infralogger.Time("last_success_at", cp.LastSuccessfulRun),
        )
    }
    if saveErr := SaveCheckpoint(p.cfg.Checkpoint.File, cp); saveErr != nil {
        return fmt.Errorf("producer: save checkpoint after all-hits-unmappable: %w", saveErr)
    }
    p.log.Info("run_summary", /* ... */)
    return nil
}
```

A unit test belongs in `producer_test.go`: feed N hits where every mapper call returns an error, assert `ConsecutiveEmpty` advances by 1 on each Run and the WARN fires on the 3rd consecutive run.

## Follow-up (NOT a blocker — flag for maintainer / WP06)

### F1. Distinct log code for all-hits-unmappable

The reviewer brief suggests a separate WARN code (e.g. `signal_producer.all_hits_unmappable`) for this state, since it is operationally different from "ES truly empty" — it is a content-pipeline bug rather than a source outage. The current single `signal_producer.source_down` code conflates the two. Acceptable for now (the operator gets *some* signal), but worth filing as a follow-up issue so dashboards can split the two codes once volumes are observed.

## What was verified (and passed)

- **Diff scope**: `git show dc7feed9 --stat` shows exactly the 9 files documented in the deliverable.
- **`os.Getenv` scope**: only `cmd/main.go` (LOG_LEVEL, plus `infraconfig.GetConfigPath`) and `integration_test.go` (test override) — no leaks into library code.
- **No `interface{}`** anywhere in the new code (uses `any` consistently).
- **`os.Exit(1)` on Run error** in `cmd/main.go` line 43 — systemd will record failure (FR-016).
- **Checkpoint backward-compat**: `ConsecutiveEmpty` field added with json tag `consecutive_empty`, zero default — old files still load.
- **Source-down WARN fires at exact `==` threshold equality** (line 170): `if cp.ConsecutiveEmpty == sourceDownThreshold` — prevents repeat-spam, as designed.
- **NFR-006 log volume**: one `batch_post` INFO inside the per-batch loop, plus one `run_summary` outside — well under the 5/batch ceiling.
- **Checkpoint advancement guarded**: `cp.LastSuccessfulRun` is only assigned to `res.maxCrawledAt` after `postAllBatches` returns nil (line 245), so a partial-failure return aborts before the SaveCheckpoint call. FR-005 honored.
- **API key not logged**: scanned producer.go and main.go — no `APIKey` field in any log call.

## Concerns

- **`go test` not executed**: I was not able to run `go build` / `go test` in this review pass; recommend running `task test:signal-producer` and `golangci-lint run` before re-submitting after the fix.
- **All-hits-fail design** (per blocker B1) is the only substantive issue. Once fixed, the WP is in good shape for WP06 deploy.
- **WP06 risk**: WP06 will need to know about the `ConsecutiveEmpty` field for any dashboards / alerting it sets up; mention it in the WP06 hand-off.
