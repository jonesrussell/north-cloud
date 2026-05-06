---
work_package_id: WP15
title: Runner Orchestration and Observability
dependencies:
- WP06
- WP07
- WP08
- WP09
- WP10
- WP11
- WP12
- WP13
- WP14
requirement_refs:
- FR-001
- FR-006
- FR-007
- FR-008
- FR-015
- NFR-001
- NFR-005
- NFR-006
- NFR-008
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T061
- T062
- T063
- T064
- T065
- T066
phase: B
agent: "claude:sonnet:implementer:implementer"
shell_pid: "280028"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/runner/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/runner/runner.go
- alert-crawler/internal/runner/runner_test.go
- alert-crawler/internal/runner/mocks_test.go
- alert-crawler/internal/observability/**
priority: P1
tags: []
---

# WP15 — Runner Orchestration and Observability

## Objective

L2 orchestrator that drives one poll cycle end-to-end: load checkpoint → fetch → parse → diff catalogue → resolve scope → infer severity → write ES → publish Redis → save checkpoint. Includes the structured-metric emitter for observability per plan §4.5.

This is the integration WP — it consumes every L1 package built in WP08–WP14.

## Context

- Spec §3 FR-001, FR-006, FR-007, FR-008, FR-015, §4 NFR-001, NFR-005, NFR-006, NFR-008
- Plan §Component Design (Runner, Observability), §3 Architecture sequence diagram
- Data model: §State Machines (poll cycle sequence)
- TC-010 (parser-degraded handling), TC-011 (backfill), TC-014 (observability)

## Branch Strategy

Standard.

## Subtasks

### T061 — Create `internal/runner/runner.go`

**Purpose**: Main orchestrator type and `RunOnce(ctx)` method.

**Steps**:
1. Create `alert-crawler/internal/runner/runner.go`:
   ```go
   package runner

   import (
       "context"
       "errors"
       "fmt"
       "time"

       "github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/elasticsearch"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/observability"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/redis"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/scope"
       "github.com/jonesrussell/north-cloud/alert-crawler/internal/severity"
   )

   type Runner struct {
       fetch    *rss.Client
       store    *catalogue.Store
       indexer  *elasticsearch.Indexer
       pub      *redis.Publisher
       resolver *scope.Resolver
       sevTable severity.Table
       metrics  *observability.Metrics
       sources  []domain.AlertSource
       defaultExpiry time.Duration
   }

   func New(deps Dependencies) *Runner { /* construct */ }

   // Run iterates over all enabled sources sequentially, calling RunSource for each.
   // Returns nil only if every source completed without unrecoverable errors.
   func (r *Runner) Run(ctx context.Context) error {
       for _, src := range r.sources {
           if !src.Enabled {
               continue
           }
           if err := r.RunSource(ctx, src); err != nil {
               r.metrics.RecordSourceError(src.ID, err)
               // Continue to next source rather than aborting the whole cycle.
           }
       }
       return nil
   }

   // RunSource executes one poll cycle against one source.
   func (r *Runner) RunSource(ctx context.Context, src domain.AlertSource) error {
       /* Sequence per data-model.md poll cycle:
          1. Load checkpoint
          2. Fetch (with conditional GET)
          3. If 304: update last_polled_at; return.
          4. Parse feed → items
          5. For each item:
             - DeriveID
             - Lookup catalogue
             - Compute content hash
             - If new: write ES, mark seen, publish created event
             - If hash unchanged: idempotent (no event; mark seen)
             - If hash changed: update ES (revision_history append), mark seen, publish updated event
          6. RescindAbsent → for each absent ID: MarkRescinded ES + catalogue, publish rescinded event
          7. Save checkpoint (new ETag, last_polled_at, reset consecutive_failures)
       */
   }
   ```

**Files**:
- `alert-crawler/internal/runner/runner.go` (new, ~280 lines).

### T062 — Implement rescission detection

**Purpose**: After processing all items in the feed, mark any catalogue entries that were not seen this cycle as rescinded.

**Steps**:
1. Inside `RunSource`, after the per-item loop:
   ```go
   absentIDs, err := r.store.RescindAbsent(ctx, src.ID, pollStartedAt)
   if err != nil {
       return fmt.Errorf("RescindAbsent: %w", err)
   }
   for _, alertID := range absentIDs {
       now := time.Now().UTC()
       if err := r.indexer.MarkRescinded(ctx, alertID, now, "absent from upstream feed"); err != nil {
           r.metrics.RecordESWriteFailure(src.ID, "rescind")
           continue
       }
       if err := r.store.MarkRescinded(ctx, src.ID, alertID); err != nil {
           // ES is canonical; log but don't re-attempt
       }
       // Build rescinded event from current ES state (or recall last cached)
       event := domain.LifecycleEvent{
           EventType: domain.EventRescinded,
           EventAt:   now,
           AlertID:   alertID,
           // Note: full payload requires re-fetching the rescinded doc from ES
           // (or maintaining a recently-seen cache). Keep it simple for v1:
           // fetch via indexer.QueryActive-style by ID.
       }
       if err := r.pub.Publish(ctx, event); err != nil {
           r.metrics.RecordRedisPublishFailure(src.ID, "rescinded")
       }
       r.metrics.RecordRescinded(src.ID)
   }
   ```
2. Note: parser-degraded items (TC-010) are NEVER marked rescinded by this path. They appear in the catalogue with `is_active=true`, `last_seen_at=now()` from the same poll cycle (because the parser still ingested them with `parse_quality: degraded`).

**Files**:
- `alert-crawler/internal/runner/runner.go` (continued, ~50 lines added).

### T063 — Error classification

**Purpose**: Distinguish transient errors (retry-worthy via `consecutive_failures` increment + backoff at the systemd-timer level) from structural errors (parse failures; source-down — but no retry within the same cycle).

**Steps**:
1. Within `RunSource`:
   ```go
   if err != nil {
       switch {
       case errors.Is(err, rss.ErrNotModified):
           r.metrics.RecordPoll(src.ID, "not_modified", time.Since(pollStartedAt))
           // Update last_polled_at only; everything else stays.
           _ = r.store.SaveCheckpoint(ctx, /* updated checkpoint */)
           return nil
       case errors.Is(err, rss.ErrTransient):
           _ = r.store.IncrementConsecutiveFailures(ctx, src.ID, src.FeedURL)
           r.metrics.RecordPoll(src.ID, "error", time.Since(pollStartedAt))
           return err
       case errors.Is(err, rss.ErrStructural):
           // Don't increment consecutive_failures (it'd never reset).
           // Just record a metric.
           r.metrics.RecordPoll(src.ID, "error", time.Since(pollStartedAt))
           return err
       }
   }
   // On success: reset consecutive_failures.
   _ = r.store.ResetConsecutiveFailures(ctx, src.ID, src.FeedURL)
   ```

**Files**:
- `alert-crawler/internal/runner/runner.go` (continued, ~30 lines added).

### T064 — Create `internal/observability/metrics.go`

**Purpose**: Structured-log emitter matching the metric set in plan §4.5.

**Steps**:
1. Create `alert-crawler/internal/observability/metrics.go`:
   ```go
   package observability

   import (
       "time"

       infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
   )

   type Metrics struct {
       log infralogger.Logger
   }

   func New(log infralogger.Logger) *Metrics { return &Metrics{log: log} }

   func (m *Metrics) RecordPoll(sourceID, result string, duration time.Duration) {
       m.log.Info("alert_crawler.poll",
           infralogger.String("source_id", sourceID),
           infralogger.String("result", result), // ok|not_modified|error
           infralogger.Int64("duration_ms", duration.Milliseconds()),
       )
   }

   func (m *Metrics) RecordCreated(sourceID string, category string, severity string) {
       m.log.Info("alert_crawler.alert.created",
           infralogger.String("source_id", sourceID),
           infralogger.String("category", category),
           infralogger.String("severity", severity),
       )
   }

   // ... similarly for RecordUpdated, RecordRescinded, RecordParseFailure,
   // RecordESWriteFailure, RecordRedisPublishFailure.

   func (m *Metrics) RecordConsecutiveFailures(sourceID string, count int) {
       m.log.Info("alert_crawler.consecutive_failures",
           infralogger.String("source_id", sourceID),
           infralogger.Int("count", count),
       )
   }
   ```
2. Method signatures match plan §4.5 metric names (with `.` → `_` mapping if logger doesn't allow dots in keys).

**Files**:
- `alert-crawler/internal/observability/metrics.go` (new, ~120 lines).

### T065 — `consecutive_failures` ≥6 → operator-actionable signal

**Purpose**: NFR-005 explicitly: "six consecutive failures on a source SHALL surface an operator-actionable signal."

**Steps**:
1. After `IncrementConsecutiveFailures` returns, check the new count.
2. If `count >= 6`, emit a higher-severity log (`WARN` or `ERROR`):
   ```go
   if count >= 6 {
       m.log.Warn("alert_crawler.consecutive_failures.threshold_exceeded",
           infralogger.String("source_id", sourceID),
           infralogger.Int("count", count),
           infralogger.String("action", "investigate source connectivity"),
       )
   }
   ```
3. Production observability (Loki/Grafana) is configured to alert on `WARN`+ for `alert-crawler` service. (Config is operator-side; this WP just emits the right level.)

**Files**:
- `alert-crawler/internal/observability/metrics.go` (continued).

### T066 — Unit tests with mocked dependencies

**Purpose**: Cover every poll-cycle outcome (created, updated, rescinded, idempotent, errors).

**Steps**:
1. Create `alert-crawler/internal/runner/runner_test.go` with mock implementations of `rss.Client`, `catalogue.Store`, `elasticsearch.Indexer`, `redis.Publisher`, `scope.Resolver`, `severity.Table`, `observability.Metrics`.
2. Test cases:
   - **TestRunSource_NewAlert_PublishesCreated**: empty catalogue + 1 feed item → 1 ES write + 1 Redis `created` event.
   - **TestRunSource_UnchangedAlert_Idempotent**: catalogue has item with same content_hash → no ES write + no Redis event.
   - **TestRunSource_ChangedAlert_PublishesUpdated**: catalogue has item with different content_hash → 1 ES update + 1 Redis `updated` event.
   - **TestRunSource_AbsentAlert_PublishesRescinded**: catalogue has 2 items; feed has 1 → 1 RescindAbsent → 1 Redis `rescinded` event.
   - **TestRunSource_NotModified_NoOp**: 304 → no ES writes; checkpoint last_polled_at updated.
   - **TestRunSource_TransientError_IncrementsCounter**: 503 from feed → IncrementConsecutiveFailures called.
   - **TestRunSource_ConsecutiveFailuresAtThreshold_LogsWarn**: count goes from 5 to 6 → WARN log emitted.
   - **TestRunSource_ParseDegraded_NeverRescinds**: feed item with parse_quality=degraded; absent on next poll; assert it IS rescinded (degraded items still go through normal lifecycle).
   - Wait — re-read TC-010: "stale-parse-for-operator-review. Never auto-rescind on parse failure." Failed-parse items are SKIPPED entirely (never enter catalogue). Degraded items DO enter catalogue and are rescinded normally if absent from a future feed. Confirm this distinction in the test.
   - **TestRun_MultipleSourcesIsolatedErrors**: source A errors; source B succeeds → A's error doesn't abort B.
3. `t.Helper()` everywhere. Coverage ≥80%.

**Files**:
- `alert-crawler/internal/runner/runner_test.go` (new, ~400 lines).
- `alert-crawler/internal/observability/metrics_test.go` (new, ~80 lines).
- Mocks live in `alert-crawler/internal/runner/mocks_test.go` (avoid public mocks).

**Validation**:
- `task test:alert-crawler` passes.
- Coverage ≥80% on `internal/runner/` and `internal/observability/`.

## Definition of Done

- Runner orchestrates the full poll cycle.
- Rescission detection catches feed-deltas (TC-010 distinction respected).
- Error classification routes transient vs structural correctly.
- Metrics emitted per the plan §4.5 set.
- ≥6 consecutive failures emits operator-actionable WARN.
- Coverage ≥80%.

## Risks

- **Integration complexity**: this WP integrates 9 dependent packages. Be especially careful with the order of operations (ES write must precede Redis publish; checkpoint save must be last).
- **Rescission event payload**: emitting a `rescinded` event with the full Alert payload requires fetching the document. Decide whether to QueryActive-by-ID for the payload OR cache the most-recent state in the catalogue. Cleaner: fetch once at rescission time.

## Reviewer Guidance

- Verify the order of operations in `RunSource` matches the data-model sequence diagram.
- Verify TC-010 distinction is correct: parse-failure items are skipped (no catalogue entry); parse-degraded items are processed normally with a flag.
- Verify metrics are emitted at every interesting state change.
- Verify error isolation between sources.

## Implementation Command

```bash
spec-kitty agent action implement WP15 --agent <name>
```

Depends on WP06, WP07, WP08, WP09, WP10, WP11, WP12, WP13, WP14. This is the integration node — all earlier Phase B WPs must be complete.

## Activity Log

- 2026-05-06T23:38:55Z – claude:sonnet:implementer:implementer – shell_pid=280028 – Started implementation via action command
- 2026-05-06T23:53:47Z – claude:sonnet:implementer:implementer – shell_pid=280028 – Runner + observability complete
