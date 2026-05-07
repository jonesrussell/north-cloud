---
work_package_id: WP24
title: NFR Validation Harness
dependencies:
- WP18
requirement_refs:
- FR-013
- NFR-001
- NFR-006
- NFR-009
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T102
- T103
- T104
- T105
phase: D
agent: "claude:opus:reviewer:reviewers2"
shell_pid: "529386"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/integration/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/integration/nfr_*
- alert-crawler/integration/backfill_rehearsal_test.go
priority: P1
tags: []
---

# WP24 — NFR Validation Harness

## Objective

Implement automated harnesses for NFR-001 (latency), NFR-006 (idempotency), and NFR-009 (blast-radius isolation), plus the first-deploy backfill rehearsal (D.3 in plan). Each harness produces a pass/fail report against thresholds in spec §4.

## Context

- Spec §4 NFR-001, NFR-006, NFR-009
- Plan §Phased Build Sequence Phase D.2/D.3, §NFR Traceability
- Build tag `//go:build integration` (extends WP18's harness).

## Branch Strategy

Standard. Depends on WP18 (reuses the integration harness).

## Subtasks

### T102 — NFR-001 latency harness

**Purpose**: Verify 95% of alerts visible within 60min, 99% within 120min from `issued_at` to ES indexing.

**Steps**:
1. Create `alert-crawler/integration/nfr_001_latency_test.go`.
2. The harness:
   - Spin up the synthetic httptest RSS server.
   - For each iteration (target 100):
     - Create an alert with `issued_at = now()` and a unique slug.
     - Serve it via the synthetic feed.
     - Run one poll cycle.
     - Record `time.Since(issued_at)` when ES `Index` is observed (use a mock ES client wrapper that timestamps writes).
   - Aggregate the 100 latencies. Assert:
     - 95th percentile ≤ 60min (in real-world conditions, well under 60s for unit-test scale; the spec target accommodates the worst-case poll cycle interval).
     - 99th percentile ≤ 120min.
3. The harness simulates real-world cadence by letting the runner sleep between cycles via configurable `time.Now()` injection. For unit-test runs, use compressed time.
4. The threshold check should pass with synthetic conditions (ms-scale latencies are well below the 60min target).

**Files**:
- `alert-crawler/integration/nfr_001_latency_test.go` (new, ~180 lines).

**Validation**:
- Test passes consistently in CI.
- Failure produces an actionable error message naming the percentile that exceeded threshold.

### T103 — NFR-006 idempotency harness

**Purpose**: 100-cycle replay produces 0 spurious lifecycle events.

**Steps**:
1. Create `alert-crawler/integration/nfr_006_idempotency_test.go`.
2. The harness:
   - Spin up a synthetic feed serving 5 unchanging items.
   - Run 100 poll cycles (one after another, no sleep needed in test).
   - Subscribe to Redis throughout.
   - Assert: 5 `created` events on the first cycle; 0 events on cycles 2–100.
   - Assert: ES has 5 documents with `_version` unchanged after cycle 1.
3. Tolerance: 0 spurious events. Any spurious event fails the test.

**Files**:
- `alert-crawler/integration/nfr_006_idempotency_test.go` (new, ~140 lines).

**Validation**:
- Strictly 0 spurious events.

### T104 — NFR-009 blast-radius harness

**Purpose**: A failing alert-crawler must NOT degrade the existing lead-pipeline.

**Steps**:
1. Create `alert-crawler/integration/nfr_009_blast_radius_test.go`.
2. The harness:
   - This test is more architectural than runtime: alert-crawler runs in its own oneshot process, so it cannot hold resources of the lead pipeline directly.
   - The blast-radius test verifies isolation by:
     - Synthetically crashing alert-crawler (panic mid-cycle).
     - Asserting that the SQLite catalogue is left in a recoverable state (alerts not yet processed are NOT marked seen; rescission has not run).
     - Asserting that the volume-mount setup still allows a fresh container start without manual intervention.
   - For the inter-service blast-radius (alert-crawler vs. lead pipeline / signal-crawler), the architectural answer is: separate process, separate volumes, separate ES index, separate Redis channel. So the test simply documents that no shared state exists.
3. Practical assertions:
   - SQLite WAL or rollback journal recovers cleanly after a crash.
   - Subsequent poll detects the crash-stranded entries via catalogue lookup.
   - No deadlocks or zombie processes.

**Files**:
- `alert-crawler/integration/nfr_009_blast_radius_test.go` (new, ~140 lines).

**Validation**:
- Crash-and-recover sequence completes within 2 cycles.
- No data loss in the alert catalogue.

### T105 — First-deploy backfill rehearsal (D.3)

**Purpose**: Run the backfill subcommand against a sandbox; verify 20 `created` events.

**Steps**:
1. Create `alert-crawler/integration/backfill_rehearsal_test.go`.
2. The harness:
   - Set up a synthetic feed with exactly 30 items (more than the backfill limit).
   - Run alert-crawler in backfill mode.
   - Subscribe to Redis throughout.
   - Assert: exactly 20 `created` events on the channel.
   - Assert: ES has 20 documents.
   - Run again (idempotent): assert 0 new events; ES still 20 docs.
   - Run normal poll mode: assert that the 21st-most-recent item (still in feed) gets a `created` event NOW (because backfill only covered the top 20).

**Files**:
- `alert-crawler/integration/backfill_rehearsal_test.go` (new, ~160 lines).

**Validation**:
- All assertions pass.
- Backfill is exactly idempotent.

## Definition of Done

- All four harnesses pass.
- CI integration job runs them as part of `task test:alert-crawler -- -tags integration`.
- Failure messages are actionable.

## Risks

- **Test flakiness**: NFR-001 percentile checks can be flaky in shared-CI environments. Mitigation: generous tolerances (use min/max bounds rather than exact thresholds where appropriate).
- **NFR-009 architectural rather than runtime**: confirm with reviewer that the test's level of assertion is meaningful given the inherent process isolation.

## Reviewer Guidance

- Verify each harness asserts the correct threshold from spec §4.
- Verify the harnesses use the existing WP18 integration framework.
- Verify backfill rehearsal asserts both first-run (20 created) and idempotent re-run (0 created).

## Implementation Command

```bash
spec-kitty agent action implement WP24 --agent <name>
```

Depends on WP18.

## Activity Log

- 2026-05-07T13:39:42Z – claude:sonnet:implementer:implementer – shell_pid=527594 – Started implementation via action command
- 2026-05-07T13:41:49Z – claude:sonnet:implementer:implementer – shell_pid=527594 – Ready for review: NFR-001/006/009 harnesses plus backfill rehearsal tests
- 2026-05-07T13:41:51Z – claude:opus:reviewer:reviewers2 – shell_pid=529386 – Started review via action command
