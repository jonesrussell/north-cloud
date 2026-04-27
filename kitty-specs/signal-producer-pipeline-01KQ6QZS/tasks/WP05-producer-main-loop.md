---
work_package_id: WP05
title: Producer Main Loop and Binary
dependencies:
- WP02
- WP03
- WP04
requirement_refs:
- FR-001
- FR-002
- FR-013
- FR-014
- FR-015
- FR-016
planning_base_branch: main
merge_target_branch: main
branch_strategy: 'Sequential WP after parallel leaves. Lane workspace from lanes.json. Final merge target: main.'
subtasks:
- T020
- T021
- T022
- T023
- T024
- T025
history:
- event: created
  at: '2026-04-27T05:55:00Z'
  by: /spec-kitty.tasks
authoritative_surface: signal-producer/
execution_mode: code_change
mission_id: 01KQ6QZSNBQF3VW515AKM0SQ46
mission_slug: signal-producer-pipeline-01KQ6QZS
owned_files:
- signal-producer/cmd/main.go
- signal-producer/internal/producer/producer.go
- signal-producer/internal/producer/producer_test.go
- signal-producer/internal/producer/integration_test.go
- signal-producer/internal/config/config.go
- signal-producer/internal/config/config_test.go
tags: []
---

# WP05 — Producer Main Loop and Binary

## Objective

Wire the three leaf packages (checkpoint, mapper, client) into the producer's `Run(ctx)` main loop and the `cmd/main.go` entry point. Add the integration test that runs end-to-end against a real ES instance and a `httptest.Server` standing in for Waaseyaa. After this WP merges, the binary works end-to-end locally.

## Context

Read first:
- [spec.md](../spec.md) FR-001, FR-002, FR-013–FR-016, NFR-001, NFR-004, NFR-006.
- [plan.md](../plan.md) "Execution Approach" — WP05 is the integration step.
- [data-model.md](../data-model.md) "Config" section.
- [research.md](../research.md) D1 (ES client), D4 (simple `cmd/main.go`), D5 (integration test fixture strategy), D7 (source-down via WARN log).
- [quickstart.md](../quickstart.md) — operator's view; the Run output should match what quickstart promises.
- WP02–WP04 outputs: read `signal-producer/internal/{producer,mapper,client}/` to understand the actual landed APIs.

Charter constraints: C-002, C-003, C-004 — config goes through `infrastructure/config/`; `os.Getenv` only in `cmd/main.go`.

## Branch Strategy

Planning base: `main`. Merge target: `main`. Lane workspace from `lanes.json`. Sequential — depends on WP02, WP03, WP04 all approved.

## Subtask Guidance

### T020 — ES query implementation

**Purpose**: Build the Elasticsearch range query and execute it against the configured indexes.

**Steps**:

1. Add to `signal-producer/internal/producer/producer.go`. Survey existing services first: `classifier/internal/elasticsearch/` and `index-manager/internal/elasticsearch/` show the conventional ES client wiring. Reuse `infrastructure/elasticsearch/` if such a wrapper exists; otherwise instantiate `github.com/elastic/go-elasticsearch/v8` directly (research D1).
2. Define an interface so the producer is testable with a fake:
   ```go
   type ESClient interface {
       Search(ctx context.Context, indexes []string, query map[string]any) ([]ESHit, error)
   }
   ```
3. Define `ESHit` as `map[string]any` (or a thin wrapper) — the mapper consumes a map.
4. Build the ES query in a helper `buildQuery(checkpoint time.Time, lookbackBuffer time.Duration, minQualityScore int) map[string]any` — exactly the JSON DSL from issue #592:
   ```json
   {
     "query": {
       "bool": {
         "must": [
           {"range": {"crawled_at": {"gte": "<checkpoint - lookback>"}}},
           {"terms": {"content_type": ["rfp", "need_signal"]}}
         ],
         "filter": [
           {"range": {"quality_score": {"gte": <minScore>}}}
         ]
       }
     },
     "sort": [{"crawled_at": "asc"}],
     "size": 100
   }
   ```
5. Iterate over multiple indexes (`*_classified_content` wildcard) — pass the wildcard pattern through to the ES client; ES handles expansion.

**Files**: `signal-producer/internal/producer/producer.go`.

**Validation**:

- [ ] Query JSON matches issue #592 exactly.
- [ ] Lookback buffer is applied (`checkpoint - 5m` reaches the ES query).
- [ ] ESClient is an interface (testable with fakes).

### T021 — `Run(ctx)` main loop

**Purpose**: Stitch the pipeline together. One `Run` call = one timer fire.

**Steps**:

1. Define:
   ```go
   type Producer struct {
       cfg        Config
       es         ESClient
       client     client.WaaseyaaClient
       checkpoint Checkpoint
       log        infralogger.Logger
       state      *runState  // for source-down detection (T022)
   }

   func (p *Producer) Run(ctx context.Context) error
   ```
2. Run flow (matches `quickstart.md` log narrative):
   1. Log `event=run_start` with `checkpoint_ts`, `lookback_buffer`.
   2. `LoadCheckpoint(p.cfg.Checkpoint.File, p.log)` — get the timestamp.
   3. Build query; call `p.es.Search(ctx, p.cfg.Elasticsearch.Indexes, query)`.
   4. Log `event=es_query` with `hits` count.
   5. If 0 hits, increment empty-run counter (T022), log `event=run_summary` with all zero counters, return nil.
   6. Map hits via `mapper.MapHit`. Mapping errors → log WARN, increment `skipped`, drop the hit; do NOT abort the run.
   7. Batch into groups of `cfg.Waaseyaa.BatchSize` (default 50).
   8. For each batch: `p.client.PostSignals(ctx, client.SignalBatch{Signals: batch})`. Log `event=batch_post`.
      - On non-retryable error (already wrapped errClient) → log ERROR, return error so systemd records failure (FR-016).
      - On retryable exhausted error → log ERROR, return error.
   9. After ALL batches succeed: `SaveCheckpoint(...)` advancing to the latest ES hit's `crawled_at`. (Decision: advance to `max(crawled_at)` of delivered hits, NOT to `time.Now()` — protects against lookback-buffer drift.)
   10. Reset empty-run counter.
   11. Log `event=run_summary` with totals.
3. Per FR-005, advance the checkpoint ONLY after every batch in the run delivered. Partial-success → no advance.
4. Per NFR-006, log volume cap: ≤ 5 INFO lines per batch + 1 summary per run. Do not log the request body or the full IngestResult more than once per batch.

**Files**: `signal-producer/internal/producer/producer.go` (extended).

**Validation**:

- [ ] Empty result set → returns nil; checkpoint NOT advanced.
- [ ] Successful run → checkpoint advances; counters logged correctly.
- [ ] Mid-run client error → returns error; checkpoint NOT advanced.
- [ ] Mapping failure for one hit → other hits in the batch still delivered.

### T022 — Source-down detection

**Purpose**: Implement the FR-019 + research D7 decision: 3 consecutive empty runs → single WARN with stable code.

**Steps**:

1. The empty-run counter must persist across runs since each `Run` is one process invocation. Store it in the checkpoint file.
2. Extend `Checkpoint`:
   ```go
   type Checkpoint struct {
       LastSuccessfulRun  time.Time `json:"last_successful_run"`
       LastBatchSize      int       `json:"last_batch_size"`
       ConsecutiveEmpty   int       `json:"consecutive_empty"`
   }
   ```
   This is a contract change with WP02; add the field with a sensible default (zero) so old checkpoint files still load.
3. In `Run`:
   - On 0 hits: `cp.ConsecutiveEmpty++`. If `cp.ConsecutiveEmpty == 3`, log a single WARN line with code `signal_producer.source_down`, fields `consecutive_empty: 3`, `last_success_at: ...`. (Don't repeat the WARN on subsequent empty runs unless the counter rolls over — only at the threshold transition.)
   - On any successful delivery: `cp.ConsecutiveEmpty = 0`.
4. Save the checkpoint EVEN on empty-run paths so the counter persists. (This is a slight change to FR-005 wording: "advance" the timestamp only on success, but we must persist the counter regardless. Document this distinction in a comment in `Run`.)

**Files**: `signal-producer/internal/producer/producer.go` (extended); update `checkpoint.go` if WP02 didn't add the field (coordinate with WP02 reviewer; if WP02 already shipped without it, this WP adds it as a non-breaking field with `json:"omitempty"` semantics).

**Validation**:

- [ ] Three runs in a row with zero hits emit exactly one WARN with code `signal_producer.source_down`.
- [ ] Fourth empty run does NOT re-emit the WARN.
- [ ] First successful delivery resets the counter.
- [ ] Counter persists across process restarts (survives in checkpoint file).

### T023 — `cmd/main.go` wiring

**Purpose**: Build the dependency graph at process start and run once. Per research D4 — simple, no phased bootstrap.

**Steps**:

1. Create `signal-producer/cmd/main.go`. Package: `main`.
2. Flow:
   ```go
   func main() {
       ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
       defer cancel()

       cfg, err := config.Load(...)  // via internal/config/, which uses infrastructure/config/
       if err != nil { fail(err) }

       log := infralogger.New(...)

       esClient := newESClient(cfg, log)         // internal helper, instantiates go-elasticsearch
       waaseyaaClient := client.New(cfg.Waaseyaa.URL, cfg.Waaseyaa.APIKey, log)

       p := producer.New(cfg, esClient, waaseyaaClient, log)

       if err := p.Run(ctx); err != nil {
           log.Error("run failed", infralogger.Error(err))
           os.Exit(1)
       }
   }
   ```
3. `os.Getenv` only here in `cmd/main.go` (allowed). Anything else uses `infrastructure/config/`.
4. Create `signal-producer/internal/config/config.go` exposing a `Load` function that returns the validated Config struct. Use `infrastructure/config/` for env parsing. Validation rules from `data-model.md` Config section: URL parse, batch_size in [1, 500], min_quality_score in [0, 100], duration parse, checkpoint parent dir writable.
5. Create `signal-producer/internal/config/config_test.go`: positive case + each validation error case.

**Files**: `signal-producer/cmd/main.go`, `signal-producer/internal/config/config.go`, `signal-producer/internal/config/config_test.go`.

**Validation**:

- [ ] `task build:signal-producer` produces a runnable binary.
- [ ] `./signal-producer` with valid config + reachable ES + mock Waaseyaa runs end-to-end.
- [ ] `./signal-producer` with invalid `WAASEYAA_API_KEY=""` exits non-zero with a clear error message before any network call.
- [ ] `os.Getenv` appears only in `cmd/main.go` (search the diff).

### T024 — Integration test

**Purpose**: NFR-004. End-to-end exercise against real ES + an `httptest.Server` standing in for Waaseyaa.

**Steps**:

1. Create `signal-producer/internal/producer/integration_test.go` with build tag `//go:build integration`. The CI integration harness runs tests with `-tags=integration`.
2. Test setup helpers:
   - `setupTestES(t)` — connects to the dev ES (`http://localhost:9200` by default); creates a uniquely-named index `signal-producer-it-<timestamp>`; cleans up via `t.Cleanup`.
   - `seedFixtures(t, idx)` — index two docs: one rfp, one need_signal, both with `quality_score >= 40` and `crawled_at` within the last hour.
   - `startMockWaaseyaa(t)` — `httptest.Server` that captures requests and returns a 200 with `IngestResult{Ingested: 2}` for the test.
3. Test:
   - `TestProducer_EndToEnd`: build `Config` pointing at the test index and the mock server URL; run `producer.Run(ctx)`; assert the mock received exactly one POST with both signals (per FR-013 batch-of-50 default; both fit in one batch); assert the response was parsed; assert the checkpoint file advanced; assert log lines include `event=run_summary` with `total_signals: 2`.
4. Use `t.Helper()` everywhere.
5. Do NOT mock ES. The "real ES" requirement (NFR-004) is the whole point.

**Files**: `signal-producer/internal/producer/integration_test.go`.

**Validation**:

- [ ] `go test -tags=integration ./signal-producer/internal/producer/...` runs and passes against a running dev ES.
- [ ] Test cleans up its index (`t.Cleanup` deletes it).
- [ ] Mock server received exactly the expected payload shape.
- [ ] Checkpoint file written and parses correctly post-run.

### T025 — Unit tests for `Run` with fakes

**Purpose**: Cover Run's branching logic without needing real ES.

**Steps**:

1. Create `signal-producer/internal/producer/producer_test.go`. Use a `fakeESClient` (implements ESClient interface) and a `fakeWaaseyaa` (implements `client.WaaseyaaClient`).
2. Tests:
   - `TestRun_HappyPath`: 5 hits, 1 batch, 1 successful POST → checkpoint advances, source-down counter resets.
   - `TestRun_EmptyResult`: 0 hits → counter increments, no POST, checkpoint timestamp unchanged.
   - `TestRun_SourceDownThreshold`: simulate 3 consecutive empty runs (load checkpoint with `ConsecutiveEmpty=2`, run with 0 hits) → exactly one WARN with the source_down code.
   - `TestRun_BatchExactlyAtLimit`: hits == batch size → one POST.
   - `TestRun_TwoBatchesBothSucceed`: hits == batch size + 1 → two POSTs, checkpoint advances after both.
   - `TestRun_FirstBatchSucceedsSecondFails`: first POST returns success, second returns errClient → Run returns error, checkpoint NOT advanced (FR-005).
   - `TestRun_MapperErrorOnOneHit`: 5 hits, one is malformed → 4 delivered, 1 skipped; Run still succeeds; counters logged.
   - `TestRun_ContextCancelled`: cancel ctx before Run starts → returns ctx error before any ES call.

**Files**: `signal-producer/internal/producer/producer_test.go`.

**Validation**:

- [ ] All eight tests pass.
- [ ] Coverage on `producer.go` ≥ 80%.
- [ ] Test helpers use `t.Helper()`.

## Definition of Done

- [ ] All six subtasks complete with their validation checklists ticked.
- [ ] `task lint:signal-producer` and `task test:signal-producer` green.
- [ ] Coverage ≥ 80% on `producer.go` and `config.go`.
- [ ] Integration test passes against a running dev ES.
- [ ] `os.Getenv` appears only in `cmd/main.go`.
- [ ] `task drift:check` and `task layers:check` green.

## Reviewer Guidance

1. **Checkpoint advances correctly**: Find the `SaveCheckpoint` call in `Run`. It MUST be after the last batch's success, with the timestamp being `max(crawled_at)` of delivered hits (not `time.Now()`). Reject if it advances on partial success.
2. **`os.Getenv` only in `cmd/`**: `grep -rn 'os\.Getenv' signal-producer/` should show matches only in `cmd/main.go`. Anything else violates C-002.
3. **No `interface{}`**: search the diff for `interface{}` (use `any` per C-002).
4. **Source-down WARN fires once**: walk `TestRun_SourceDownThreshold` to confirm the WARN is emitted at the threshold transition, not on every empty run.
5. **Integration test is real**: confirm the test connects to ES, not a mock; confirm the index is uniquely named; confirm `t.Cleanup` deletes it. Reject if the test mocks ES.
6. **NFR-006 log volume**: count INFO log calls per batch in `Run`. ≤ 5. Reject if there's `log.Info` inside a tight loop.
7. **Exit code on failure**: `cmd/main.go` MUST `os.Exit(1)` on `Run` error so systemd records the unit as failed (FR-016).

## Risks and Mitigations

| Risk                                                                                            | Mitigation                                                                                                                                                      |
| ----------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ES client choice (wrapper vs direct) drifts from existing services.                             | Survey `classifier/` and `index-manager/` first. Match their pattern. Reviewer verifies.                                                                        |
| Checkpoint schema growth (T022 adds `ConsecutiveEmpty`) breaks WP02's tests.                   | Add the field with zero-value default. Re-run WP02's tests after the merge to prove backward compatibility.                                                     |
| Batching bug skips signals.                                                                     | `TestRun_BatchExactlyAtLimit` and `TestRun_TwoBatchesBothSucceed` cover the boundary cases.                                                                    |
| `cmd/main.go` reads env vars for fields the config struct already covers.                       | All env reads happen in `internal/config.Load`; `cmd/main.go` only calls `Load`. (One exception: `os.Getenv("CONFIG_PATH")` to find the config file is OK.) |

## Implementation Command

```bash
spec-kitty agent action implement WP05 --agent <agent-name> --mission signal-producer-pipeline-01KQ6QZS
```
