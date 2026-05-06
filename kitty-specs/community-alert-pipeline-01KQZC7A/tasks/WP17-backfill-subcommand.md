---
work_package_id: WP17
title: Backfill Subcommand
dependencies:
- WP15
- WP16
requirement_refs:
- FR-006
planning_base_branch: main
merge_target_branch: main
branch_strategy: lane-worktree-from-main
subtasks:
- T071
- T072
- T073
- T074
phase: B
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/cmd/backfill/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/cmd/backfill/**
- alert-crawler/internal/runner/backfill.go
- alert-crawler/internal/runner/backfill_test.go
priority: P2
tags: []
---

# WP17 — Backfill Subcommand

## Objective

Implement first-deploy backfill (TC-011): pull the 20 most recent feed items, persist them to ES, and emit `created` lifecycle events. Idempotent re-runs (catalogue prevents double-emission). Implementable as a runner mode with a CLI flag (`--backfill`); a separate `cmd/backfill/main.go` binary is also provided for clarity.

## Context

- Spec §3 FR-006 (idempotent re-fetch)
- Plan §TC-011 (first-deploy backfill = top-20)
- Research Q-3 (resolved: backfill-on-first-deploy)
- Risk PR-002 (burst on first connect)

## Branch Strategy

Standard. Depends on WP15 (runner) and WP16 (main wiring).

## Subtasks

### T071 — Create `cmd/backfill/main.go`

**Purpose**: Optional standalone binary that runs backfill mode without `--backfill` flag confusion.

**Steps**:
1. Create `alert-crawler/cmd/backfill/main.go`:
   ```go
   package main

   import (
       "context"
       "flag"
       "os"
       "os/signal"
       "syscall"

       /* ... imports same as main.go ... */
   )

   func main() {
       configPath := flag.String("config", "/etc/alert-crawler/config.yml", "path to config.yml")
       flag.Parse()

       ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
       defer stop()

       /* same setup as main.go's main() */

       if err := r.Backfill(ctx); err != nil {
           log.Error("backfill failed", infralogger.Error(err))
           os.Exit(1)
       }
   }
   ```
2. Alternative: do NOT create a separate binary; instead, use `--backfill` flag on the main binary (WP16). Discuss with reviewer; either is acceptable. Default approach in this WP: flag-based on the main binary, with `cmd/backfill/main.go` only if reviewer prefers a separate ENTRYPOINT.

**Files**:
- `alert-crawler/cmd/backfill/main.go` (new IF the separate-binary approach is chosen, ~80 lines). Otherwise, omit and rely on the `--backfill` flag.

### T072 — `Runner.Backfill(ctx)` method

**Purpose**: A second mode of the runner that emits `created` events for the most recent N items regardless of catalogue state, then bootstraps the catalogue.

**Steps**:
1. Create `alert-crawler/internal/runner/backfill.go`:
   ```go
   package runner

   import (
       "context"
       "fmt"
       "time"

       "github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
   )

   const backfillLimit = 20

   func (r *Runner) Backfill(ctx context.Context) error {
       for _, src := range r.sources {
           if !src.Enabled {
               continue
           }
           if err := r.backfillSource(ctx, src); err != nil {
               r.metrics.RecordSourceError(src.ID, err)
           }
       }
       return nil
   }

   func (r *Runner) backfillSource(ctx context.Context, src domain.AlertSource) error {
       /*
         1. Fetch (no conditional GET — force a fresh body)
         2. Parse → items[:backfillLimit]
         3. For each item:
            - DeriveID
            - Lookup catalogue
            - If already exists with same hash: idempotent (no event, no write)
            - If missing: write ES, mark seen, publish created event
         4. Save checkpoint (set ETag from response so subsequent normal poll can use 304)
       */
   }
   ```
2. Notably, backfill does NOT do the rescission step (no catalogue state to compare against).

**Files**:
- `alert-crawler/internal/runner/backfill.go` (new, ~120 lines).

### T073 — Idempotency via catalogue check

**Purpose**: Re-running backfill is a no-op if all 20 items are already in the catalogue.

**Steps**:
1. Already structurally enforced in T072 (LookupAlert returns existing entry → skip).
2. Verify the implementation:
   - First run on empty catalogue: 20 ES writes, 20 Redis events.
   - Second run on populated catalogue: 0 ES writes, 0 Redis events.
3. Document in CLAUDE.md: "Backfill is safe to re-run; it never double-emits."

**Files**:
- `alert-crawler/internal/runner/backfill.go` (already).

### T074 — Unit tests

**Purpose**: Cover backfill mode.

**Steps**:
1. Create `alert-crawler/internal/runner/backfill_test.go`:
   - **TestBackfill_EmptyCatalogue_Writes20**: feed has 20 items; assert 20 writes + 20 events.
   - **TestBackfill_FullCatalogue_NoOp**: feed has 20 items; catalogue has all 20 with same hash; assert 0 writes + 0 events.
   - **TestBackfill_PartialCatalogue_OnlyWritesNew**: feed has 20; catalogue has 10 (oldest); assert 10 writes + 10 events.
   - **TestBackfill_HonorsLimit**: feed has 30 items; assert only top 20 are processed.
2. `t.Helper()`. Coverage ≥80%.

**Files**:
- `alert-crawler/internal/runner/backfill_test.go` (new, ~200 lines).

**Validation**:
- All tests pass.
- Coverage ≥80%.

## Definition of Done

- Backfill processes top-20 items from each enabled source.
- Idempotent re-runs.
- Coverage ≥80%.
- Either via `--backfill` flag on main binary OR a separate `cmd/backfill/main.go` (decision documented in CLAUDE.md).

## Risks

- **PR-002**: 20-item burst on first connect. Document for downstream consumer (Minoo) operator: expect a burst of `created` events at first deploy.
- **Re-running backfill on a partially-populated catalogue is intentional**: catches alerts that would have been missed if the regular poll happened before backfill landed. Verify reviewer is OK with this semantics.

## Reviewer Guidance

- Verify the limit is exactly 20 (matches TC-011).
- Verify idempotency.
- Verify that the rescission path is NOT exercised in backfill mode (only in normal `Run` mode).

## Implementation Command

```bash
spec-kitty agent action implement WP17 --agent <name>
```

Depends on WP15, WP16.
