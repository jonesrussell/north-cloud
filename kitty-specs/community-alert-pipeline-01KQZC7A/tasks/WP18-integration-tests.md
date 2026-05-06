---
work_package_id: WP18
title: Integration Tests (AS-01..AS-06)
dependencies:
- WP15
- WP16
- WP17
requirement_refs:
- FR-001
- FR-002
- FR-004
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
- FR-010
- NFR-001
- NFR-003
- NFR-004
- NFR-006
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T075
- T076
- T077
- T078
- T079
- T080
- T081
phase: B
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/integration/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/integration/harness.go
- alert-crawler/integration/fixture_feed.go
- alert-crawler/integration/subscriber.go
- alert-crawler/integration/as01_test.go
- alert-crawler/integration/as02_test.go
- alert-crawler/integration/as03_test.go
- alert-crawler/integration/as04_test.go
- alert-crawler/integration/as05_test.go
- alert-crawler/integration/as06_test.go
- alert-crawler/internal/runner/integration_test.go
priority: P1
tags: []
---

# WP18 — Integration Tests (AS-01..AS-06)

## Objective

End-to-end integration tests that exercise alert-crawler against real Elasticsearch, real Redis, and real SQLite (via the existing CI integration harness). Cover all six acceptance scenarios from spec §2.2. Build tag `//go:build integration` separates from unit tests.

## Context

- Spec §2.2 Acceptance Scenarios AS-01..AS-06
- Plan §Phased Build Sequence Phase B.14
- Charter: build tags separate quick from slow suites; integration tests run against real dependencies via the existing CI integration harness.

## Branch Strategy

Standard. Depends on WP15 (runner), WP16 (wiring), WP17 (backfill).

## Subtasks

### T075 — Set up integration test harness

**Purpose**: Bootstrap shared fixtures and helpers for integration tests.

**Steps**:
1. Create `alert-crawler/integration/` directory with:
   - `harness.go`: helpers to spin up clean ES + Redis state for each test (use the existing test container conventions from other NC services if present, or reuse `infrastructure/testing` if available).
   - `fixture_feed.go`: builders for synthetic RSS feeds the test can serve via `httptest.Server`.
   - `subscriber.go`: small Redis subscriber wrapper for collecting events during a test.
2. Add build tag at the top of every file: `//go:build integration`.
3. The harness must:
   - Connect to ES at the URL configured for integration (typically `http://localhost:9200` in CI, or `north-cloud-elasticsearch-1` if running inside compose).
   - Delete `community_alerts` between tests for isolation.
   - Connect to Redis and subscribe to `community_alerts:lifecycle`.
4. Provide a `WithIntegration(t *testing.T)` helper that skips on `testing.Short()` mode.

**Files**:
- `alert-crawler/integration/harness.go` (new, ~150 lines).
- `alert-crawler/integration/fixture_feed.go` (new, ~80 lines).
- `alert-crawler/integration/subscriber.go` (new, ~80 lines).

### T076 — AS-01: drug supply alert reaches Treaty 1 page

**Purpose**: End-to-end happy path. Synthetic feed publishes one alert; alert-crawler ingests; consumer queries ES filtered by `treaty:1`; alert is visible.

**Steps**:
1. Test in `alert-crawler/integration/as01_test.go`:
   - Start `httptest.Server` serving a synthetic RSS with 1 item (drug alert for Winnipeg).
   - Configure alert-crawler to use this URL with default scope `[treaty:1, canada:manitoba]`.
   - Run one poll cycle.
   - Query ES: `GET community_alerts/_search?q=scope:treaty:1` → assert 1 result.
   - Assert the document's `severity` matches the inferred value (e.g., `high` for fentanyl).
   - Assert a Redis `created` event was received within 5s of the poll completing.

**Files**:
- `alert-crawler/integration/as01_test.go` (new, ~120 lines).

### T077 — AS-02: corrected alert supersedes earlier version

**Purpose**: Update path. Same alert ID, content changes between polls; update event emitted; revision_history grows.

**Steps**:
1. Test in `alert-crawler/integration/as02_test.go`:
   - Round 1: serve a feed with severity `high`, fewer composition entries.
   - Run poll → 1 created event, ES doc has version 1.
   - Round 2: serve feed with same alert ID but severity `critical` and refined composition.
   - Run poll → 0 created events; 1 updated event; ES doc has 1 revision_history entry.

**Files**:
- `alert-crawler/integration/as02_test.go` (new, ~140 lines).

### T078 — AS-03: rescinded alert disappears within one poll cycle

**Purpose**: Feed-delta detection.

**Steps**:
1. Test in `alert-crawler/integration/as03_test.go`:
   - Round 1: serve feed with 2 alerts.
   - Run poll → 2 created events.
   - Round 2: serve feed with only 1 alert (the second is removed).
   - Run poll → 0 created events; 0 updated events; 1 rescinded event for the absent alert.
   - Query ES for `lifecycle_state == "rescinded"` → 1 result.
   - Query ES for `lifecycle_state == "active" AND expires_at > now` → 1 result.

**Files**:
- `alert-crawler/integration/as03_test.go` (new, ~140 lines).

### T079 — AS-04: subscriber recovery after downtime

**Purpose**: Subscribers can recover all currently-active alerts from ES alone (no Redis history).

**Steps**:
1. Test in `alert-crawler/integration/as04_test.go`:
   - Run 3 poll cycles (no subscriber connected) → 3 alerts in ES, 3 events emitted to no listener.
   - Connect a subscriber AFTER all 3 polls.
   - Subscriber's first action: query ES for `lifecycle_state == "active"` → 3 results retrieved in ≤2s (NFR-002).
   - Subscriber's second action: subscribe to Redis, run another poll, observe 1 new event arrive within 5s (NFR-003).

**Files**:
- `alert-crawler/integration/as04_test.go` (new, ~120 lines).

### T080 — AS-05: source unreachable for extended period

**Purpose**: `consecutive_failures` counter triggers operator-actionable signal at 6.

**Steps**:
1. Test in `alert-crawler/integration/as05_test.go`:
   - Run 6 poll cycles where the synthetic feed server returns 503.
   - After 6th cycle: assert `poll_checkpoint.consecutive_failures == 6`.
   - Assert a WARN-level log was emitted (capture via test logger or scan stderr).
   - ES contents from prior successful runs (if any) remain intact.
   - Run a 7th cycle that returns 200; assert counter resets to 0.

**Files**:
- `alert-crawler/integration/as05_test.go` (new, ~140 lines).

### T081 — AS-06: scope vocabulary lookup

**Purpose**: A community page configured for `treaty:1` correctly receives an alert with `[canada:manitoba, canada:manitoba:winnipeg]` (via hierarchy walk, since `treaty:1 ∋ canada:manitoba`... wait, actually the relationship is the OTHER direction).

**Re-verify mental model**: `treaty:1` is a parallel namespace, NOT a parent of `canada:manitoba`. So an alert scoped `[canada:manitoba]` does NOT automatically apply to a Treaty 1 community page UNLESS the alert's scope ALSO includes `treaty:1`.

This means alert-crawler's scope resolver (WP14) must explicitly add BOTH default scope tokens (`[treaty:1, canada:manitoba]`) for an MHRN alert. The page configurator's "applies to my community?" check resolves: page config = `treaty:1`; alert scope contains `treaty:1`; match.

The hierarchy walk (`ParentRegion`) is used for the geographic axis: an alert scoped `canada:manitoba:winnipeg` ALSO applies to a page configured for `canada:manitoba` (parent), because the page resolver walks UP from the alert's tokens.

Test:
1. Configure source defaults: `[treaty:1, canada:manitoba]`. Source emits an alert about Winnipeg.
2. Resolver expands to: `[treaty:1, canada:manitoba, canada:manitoba:winnipeg, canada]` (city + walk-up to canada).
3. Page configured for `treaty:1` queries ES with `term: scope:treaty:1` → match.
4. Page configured for `canada:manitoba` queries ES with `term: scope:canada:manitoba` → match.
5. Page configured for `canada:ontario` queries ES with `term: scope:canada:ontario` → no match.

**Files**:
- `alert-crawler/integration/as06_test.go` (new, ~140 lines).

## Definition of Done

- All 6 acceptance scenarios pass in CI's integration job.
- Tests are reliable (no flakes); use generous time tolerances for distributed timing.
- Coverage tracked separately for integration tests; ≥80% combined unit+integration coverage.
- The integration test build tag (`//go:build integration`) keeps unit-test runs fast.

## Risks

- **Test flakiness**: distributed timing scenarios (NFR-001 latency) can be flaky. Use generous tolerances (e.g., assert `<5s` not `<1s` for live event delivery).
- **State isolation**: tests must clean up `community_alerts` index and Redis state between runs.
- **CI harness availability**: relies on the existing NC integration harness providing real ES/Redis/SQLite. If absent, this WP is blocked.

## Reviewer Guidance

- Verify each AS-## from the spec maps to a test.
- Verify state cleanup between tests (no test depends on another's residue).
- Verify generous timing tolerances.
- Verify the integration build tag is honored (`task test:alert-crawler` should NOT run integration tests by default).

## Implementation Command

```bash
spec-kitty agent action implement WP18 --agent <name>
```

Depends on WP15, WP16, WP17.
