# Tasks: Signal Producer Pipeline

**Mission**: `signal-producer-pipeline-01KQ6QZS`
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md) | **Data model**: [data-model.md](data-model.md) | **Contract**: [contracts/signals-post.yaml](contracts/signals-post.yaml)
**Branch contract**: planning base = `main`, merge target = `main`.

Six work packages along the layered design from `plan.md`. WP01 lays the
service scaffolding. WP02–WP04 are the three independent leaf packages
(checkpoint, mapper, client) and can land in parallel after WP01 — three
lanes. WP05 wires them into the producer main loop and binary. WP06 ships
the systemd unit + deploy pipeline.

## Subtask Index

| ID    | Description                                                                                              | WP    | Parallel |
| ----- | -------------------------------------------------------------------------------------------------------- | ----- | -------- |
| T001  | Create `signal-producer/` directory tree, `go.mod`, `Taskfile.yml`, `.layers`, `CLAUDE.md`, `config.yml.example` | WP01  |          | [D] |
| T002  | Add root `Taskfile.yml` delegations (`build`, `test`, `lint`, `vuln`)                                    | WP01  |          | [D] |
| T003  | Update `scripts/detect-changed-services.sh` `GO_SERVICES` array                                          | WP01  |          | [D] |
| T004  | Add `signal-producer/` section to `docs/specs/lead-pipeline.md` for drift acceptance                     | WP01  |          | [D] |
| T005  | Verify `task lint:signal-producer`, `task drift:check`, `task layers:check` all green                    | WP01  |          | [D] |
| T006  | Define `Checkpoint` struct in `internal/producer/checkpoint.go`                                          | WP02  | [D] |
| T007  | Implement `LoadCheckpoint` (file-missing → 24h fallback, corrupt → WARN + 24h fallback)                  | WP02  | [D] |
| T008  | Implement `SaveCheckpoint` (atomic write via tmp + rename, mode 0640)                                    | WP02  | [D] |
| T009  | Unit tests in `internal/producer/checkpoint_test.go` (load, default, save+reload, atomic-write fault)    | WP02  | [D] |
| T010  | Define `Signal` struct in `internal/mapper/mapper.go` (matches contracts/signals-post.yaml)              | WP03  | [D] |
| T011  | Implement `MapHit` for `content_type=rfp` (prefix `nc-rfp-`, GSIN sector, expires_at)                    | WP03  | [D] |
| T012  | Implement `MapHit` for `content_type=need_signal` (prefix `nc-sig-`)                                     | WP03  | [D] |
| T013  | Tolerance for missing optional fields (province, sector → empty string)                                  | WP03  | [D] |
| T014  | Unit tests in `internal/mapper/mapper_test.go` (both types, missing fields, prefix collision)            | WP03  | [D] |
| T015  | Define `WaaseyaaClient` interface + `IngestResult` struct in `internal/client/waaseyaa.go`               | WP04  | [P]      |
| T016  | Implement `PostSignals` (X-Api-Key header, JSON marshal, POST, response parse)                           | WP04  | [P]      |
| T017  | Implement retry helper in `internal/client/retry.go` (3 retries, 1s/5s/15s backoff, 5xx + net only)      | WP04  | [P]      |
| T018  | Wire context cancellation through retries                                                                 | WP04  | [P]      |
| T019  | Unit tests in `internal/client/waaseyaa_test.go` (success, 5xx retry, 4xx no-retry, ctx cancel, header)  | WP04  | [P]      |
| T020  | Implement ES query in `internal/producer/producer.go` (range + content_type + quality_score + sort)      | WP05  |          |
| T021  | Implement `Run(ctx)` main loop (load checkpoint, query ES, map, batch, post, advance, summary log)       | WP05  |          |
| T022  | Implement source-down detection (3 consecutive empty runs → WARN with `signal_producer.source_down`)     | WP05  |          |
| T023  | Wire `cmd/main.go` (config load, logger init, build deps, Run, exit code)                                | WP05  |          |
| T024  | Integration test: real ES + httptest.Server (in `internal/producer/integration_test.go`, build tag)      | WP05  |          |
| T025  | Unit tests for `producer.Run` with fakes for ES + client                                                 | WP05  |          |
| T026  | Write `signal-producer.service` (Type=oneshot, User=, StateDirectory=, EnvironmentFile=)                 | WP06  |          |
| T027  | Write `signal-producer.timer` (OnCalendar=*:0/15, Persistent=true)                                       | WP06  |          |
| T028  | Update `scripts/deploy.sh` and GH Actions workflow to install/enable the unit                             | WP06  |          |
| T029  | Update `docs/RUNBOOK.md` with source-down triage and force-rewind recipe                                 | WP06  |          |
| T030  | Add producer to deploy health-check skip-list and document first-run smoke test                          | WP06  |          |

`[P]` indicates parallel-safe across WPs (WP02/03/04 are three independent
parallel lanes after WP01). Subtasks within a single WP execute sequentially.

## Work Packages

### WP01 — Service Scaffolding

- **Goal**: Stand up the `signal-producer/` service skeleton so WP02–WP04 can land in parallel without stepping on each other or breaking CI.
- **Priority**: P0. Foundation for everything else.
- **Independent test**: `task lint:signal-producer`, `task drift:check`, `task layers:check` all pass on a fresh clone.
- **Estimated prompt size**: ~340 lines.
- **Execution mode**: `code_change`.
- **Dependencies**: none.

#### Included subtasks

- [x] T001 Create `signal-producer/` directory tree (WP01)
- [x] T002 Root `Taskfile.yml` delegations (WP01)
- [x] T003 Update `scripts/detect-changed-services.sh` (WP01)
- [x] T004 Spec section in `docs/specs/lead-pipeline.md` (WP01)
- [x] T005 Verify all gates green (WP01)

### WP02 — Checkpoint Persistence

- **Goal**: Persistent checkpoint with safe load and atomic save behavior.
- **Priority**: P0. Independent leaf package.
- **Independent test**: `task test:signal-producer` exercises the package; coverage ≥ 80%.
- **Estimated prompt size**: ~280 lines.
- **Execution mode**: `code_change`.
- **Dependencies**: WP01.

#### Included subtasks

- [x] T006 Define `Checkpoint` struct (WP02)
- [x] T007 Implement `LoadCheckpoint` (WP02)
- [x] T008 Implement `SaveCheckpoint` (WP02)
- [x] T009 Unit tests (WP02)

### WP03 — ES-Hit-to-Signal Mapper

- **Goal**: Pure function that maps an ES hit to a `Signal` for both `rfp` and `need_signal` content types.
- **Priority**: P0. Independent leaf package.
- **Independent test**: Unit tests pass; coverage ≥ 80%.
- **Estimated prompt size**: ~330 lines.
- **Execution mode**: `code_change`.
- **Dependencies**: WP01.

#### Included subtasks

- [x] T010 Define `Signal` struct (WP03)
- [x] T011 Map `rfp` hits (WP03)
- [x] T012 Map `need_signal` hits (WP03)
- [x] T013 Tolerance for missing optional fields (WP03)
- [x] T014 Unit tests (WP03)

### WP04 — Waaseyaa HTTP Client

- **Goal**: HTTP client that POSTs signal batches with retry, backoff, and context cancellation.
- **Priority**: P0. Independent leaf package.
- **Independent test**: Unit tests against `httptest.Server` cover success, 5xx retry, 4xx no-retry, ctx cancel; coverage ≥ 80%.
- **Estimated prompt size**: ~350 lines.
- **Execution mode**: `code_change`.
- **Dependencies**: WP01.

#### Included subtasks

- [ ] T015 Define `WaaseyaaClient` + `IngestResult` (WP04)
- [ ] T016 Implement `PostSignals` (WP04)
- [ ] T017 Retry helper (WP04)
- [ ] T018 Context cancellation (WP04)
- [ ] T019 Unit tests (WP04)

### WP05 — Producer Main Loop and Binary

- **Goal**: Wire checkpoint, ES query, mapper, batch dispatch, and client into a single `Run(ctx)` function and the `cmd/main.go` entry point. Add the integration test.
- **Priority**: P0. Final wiring.
- **Independent test**: Integration test (real ES + `httptest.Server`) exercises end-to-end; coverage ≥ 80%; `task test:signal-producer` green.
- **Estimated prompt size**: ~440 lines.
- **Execution mode**: `code_change`.
- **Dependencies**: WP02, WP03, WP04 (all three leaf packages).

#### Included subtasks

- [ ] T020 ES query implementation (WP05)
- [ ] T021 `Run(ctx)` main loop (WP05)
- [ ] T022 Source-down detection (WP05)
- [ ] T023 `cmd/main.go` wiring (WP05)
- [ ] T024 Integration test (WP05)
- [ ] T025 Unit tests with fakes (WP05)

### WP06 — Production Deployment

- **Goal**: Ship the binary to the production VPS via the existing CI pipeline; install systemd unit and timer; verify first run via journald.
- **Priority**: P0. Mission's outcome only realizes after deploy.
- **Independent test**: After deploy, `systemctl is-active signal-producer.timer` returns "active" and journald shows a successful first-run summary line within 15 minutes.
- **Estimated prompt size**: ~370 lines.
- **Execution mode**: `code_change`.
- **Dependencies**: WP05.

#### Included subtasks

- [ ] T026 `signal-producer.service` unit (WP06)
- [ ] T027 `signal-producer.timer` unit (WP06)
- [ ] T028 Deploy script + GH Actions updates (WP06)
- [ ] T029 RUNBOOK source-down + force-rewind (WP06)
- [ ] T030 Health-check skip-list + smoke test (WP06)

## Dependency Graph

```
WP01 (scaffolding)
  ├── WP02 (checkpoint)        ┐
  ├── WP03 (mapper)            ├── WP05 (producer + binary) ─── WP06 (deploy)
  └── WP04 (HTTP client)       ┘
```

## Parallelization Strategy

After WP01 lands and is approved, **WP02, WP03, and WP04 fire in parallel**
(three lanes). Each is fully independent — different package, different test
file, no shared state. Once all three approve, **WP05** integrates them.
**WP06** is sequential after WP05 since it depends on a working binary.

## Requirement Coverage

- **FRs**: FR-001..FR-002 → WP05; FR-003..FR-005 → WP02; FR-006..FR-008 → WP03; FR-009..FR-012 → WP04; FR-013..FR-016 → WP05; FR-017..FR-019 → WP06; FR-020 → WP01.
- **NFRs**: NFR-001..NFR-002 → WP05/WP04; NFR-003 spread across WP02/WP03/WP04/WP05; NFR-004 → WP05; NFR-005 → WP02; NFR-006 → WP05.
- **Constraints**: C-001..C-010 enforced across all WPs; reviewer guidance per WP cites the relevant constraints.

## MVP Scope

There is no smaller deliverable than the full mission. Each WP is necessary:
without WP02 there is no run-to-run idempotence; without WP03/WP04 there is no
output; without WP05 nothing is wired; without WP06 nothing runs in
production. WP05 is the implementation tipping point — once it merges with all
three leaf packages, the binary works end-to-end against a live test fixture.

## Next Command

After `finalize-tasks` writes `lanes.json`, the mission is ready for
`/spec-kitty.implement` (or for the implement-review skill to drive
end-to-end with parallel dispatch on WP02/WP03/WP04).
