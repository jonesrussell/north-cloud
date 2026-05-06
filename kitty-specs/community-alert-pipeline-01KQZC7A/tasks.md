# Tasks — Community Alert Pipeline

**Mission ID**: `01KQZC7A7SJJZ6EKHZ9JW3AZJG` (mid8: `01KQZC7A`)
**Mission Slug**: `community-alert-pipeline-01KQZC7A`
**Mission Type**: software-dev
**Date**: 2026-05-06
**Phase**: Tasks (Phase 2)

## Branch Contract

- **Current branch**: `main`
- **Planning/base branch**: `main`
- **Merge target**: `main`
- **`branch_matches_target`**: `true`

Worktrees are allocated per execution lane (computed by `finalize-tasks` from the WP graph) at `/spec-kitty.implement` time. WP files declare `owned_files` boundaries; agents must not modify files outside their owned set.

## Pointers

- Spec: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/spec.md`
- Plan: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/plan.md`
- Research: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/research.md`
- Data Model: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/data-model.md`
- Contracts: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/contracts/`
- Quickstart: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/quickstart.md`

## Overview

This mission decomposes into **25 work packages** spanning four phases:

- **Phase A** (4 WPs, sibling repo `/home/jones/dev/indigenous-taxonomy/`): release indigenous-taxonomy v1.1.0 with Treaty namespace, city regions, sample communities, `ParentRegion` walk function.
- **Phase B** (14 WPs, `/home/jones/dev/north-cloud/alert-crawler/`): scaffold the new alert-crawler service, develops in parallel with Phase A via `replace` directive.
- **Phase C** (5 WPs, `/home/jones/dev/north-cloud/`): reconciliation, deployment integration, documentation.
- **Phase D** (2 WPs, mission acceptance): NFR harness + pre-merge checklist + follow-on issue.

Total subtasks: ~110.

## Cross-Phase Parallelism

Phase A and Phase B execute **in parallel** because alert-crawler's `go.mod` carries a temporary `replace github.com/jonesrussell/indigenous-taxonomy => ../indigenous-taxonomy` directive throughout Phase B. The directive is removed by **WP19 (Pin taxonomy v1.1.0)**, which depends on **WP04 (Phase A merge + tag)**.

## Subtask Index

| ID | Description | WP | Parallel |
|---|---|---|---|
| T001 | Create `schema/treaties.yaml` (11 treaty entries with member-region metadata) | WP01 | [P] | [D] |
| T002 | Extend `scripts/generate.py` to emit `treaties.go` from `treaties.yaml` | WP01 | | [D] |
| T003 | Generate `taxonomy/treaties.go` (`Treaty` type, 11 constants, `AllTreaties`, `IsValidTreaty`) | WP01 | | [D] |
| T004 | Add unit tests for Treaty namespace | WP01 | | [D] |
| T005 | Extend `schema/regions.yaml` with city children (winnipeg, toronto, ottawa, vancouver, calgary, saskatoon) | WP02 | [D] |
| T006 | Add sample First Nations community per active treaty (e.g., `canada:manitoba:sagkeeng-fn`) | WP02 | | [D] |
| T007 | Regenerate `taxonomy/regions.go` with new entries | WP02 | | [D] |
| T008 | Update region tests (count assertions; new constant existence) | WP02 | | [D] |
| T009 | Extend `scripts/generate.py` to emit `ParentRegion` lookup table from YAML `children:` | WP03 | | [D] |
| T010 | Generate `ParentRegion(Region) (Region, bool)` in `taxonomy/regions.go` | WP03 | | [D] |
| T011 | Unit tests covering hierarchy walks (leaf → province, province → canada, canada → no parent) | WP03 | | [D] |
| T012 | Audit `tests/` for hardcoded counter assertions (RR-004) | WP04 | | [D] |
| T013 | Update `taxonomy/version.go` and `go.mod` to v1.1.0 | WP04 | | [D] |
| T014 | Update `CHANGELOG.md` with additive changes | WP04 | | [D] |
| T015 | Open PR; tag `v1.1.0` after merge | WP04 | | [D] |
| T016 | Create `alert-crawler/` directory with `go.mod` (includes `replace` directive) | WP05 | | [D] |
| T017 | Create `main.go` skeleton with phase-order comments | WP05 | | [D] |
| T018 | Create `config.yml` (defaults blank where `SetDefaults` applies) | WP05 | | [D] |
| T019 | Create per-service `Taskfile.yml` (build, run, test, lint, vuln, clean) | WP05 | | [D] |
| T020 | Create multi-stage `Dockerfile` (Alpine, uid 1000 non-root user, CGO enabled) | WP05 | | [D] |
| T021 | Create `.layers` (L0 config/domain; L1 adapter/catalogue/elasticsearch/redis/severity/scope/observability; L2 runner) | WP05 | | [D] |
| T022 | Create `alert-crawler/CLAUDE.md` skeleton with placeholders for gotchas | WP05 | | [D] |
| T023 | Create `internal/domain/alert.go` (Alert envelope) | WP06 | | [D] |
| T024 | Create `internal/domain/hazard.go` (HarmReductionHazard, Substance) | WP06 | | [D] |
| T025 | Create `internal/domain/source.go` (AlertSource, AcquisitionStrategy enum) | WP06 | | [D] |
| T026 | Create `internal/domain/lifecycle_event.go` (LifecycleEvent, EventType enum) | WP06 | | [D] |
| T027 | Add unit tests for envelope JSON round-trip and schema conformance | WP06 | | [D] |
| T028 | Create `internal/config/config.go` (Config struct with env tags) | WP07 | | [D] |
| T029 | Create `internal/config/defaults.go` (`SetDefaults` function) | WP07 | | [D] |
| T030 | Wire to `infrastructure/config.LoadWithDefaults[Config]` | WP07 | | [D] |
| T031 | Add SetDefaults pitfall regression test (RR-007) — fields with SetDefaults must be absent from `config.yml` | WP07 | | [D] |
| T032 | Create `internal/adapter/rss/client.go` (HTTP fetch with `If-None-Match` and `If-Modified-Since`) | WP08 | [D] |
| T033 | Implement `ErrNotModified` on 304 path; ETag and Last-Modified return values | WP08 | | [D] |
| T034 | Configure User-Agent (`alert-crawler/1.0 (+https://northcloud.one)`) and HTTP timeouts | WP08 | | [D] |
| T035 | Unit tests: 304 path, 200 path, 5xx with backoff, network timeout | WP08 | | [D] |
| T036 | Create `internal/adapter/rss/parser.go` (RSS XML parse via `encoding/xml`) | WP09 | [D] |
| T037 | Implement `<description>` HTML extractor (substance composition, lab source, confirmation date) | WP09 | | [D] |
| T038 | Defensive parsing with `parse_quality` flag (clean/degraded); never auto-rescind on parse failure (TC-010) | WP09 | | [D] |
| T039 | Unit tests with golden RSS fixture (real safersites.ca sample anonymized to a fixture) | WP09 | | [D] |
| T040 | Create `internal/catalogue/schema.sql` (`poll_checkpoint`, `alert_catalogue` tables) | WP10 | [D] |
| T041 | Add `internal/catalogue/migrations/` with unique numeric prefixes | WP10 | | [D] |
| T042 | Create `internal/catalogue/store.go` (`LoadCheckpoint`, `SaveCheckpoint`, `LookupAlert`, `MarkSeen`, `RescindAbsent`) | WP10 | | [D] |
| T043 | Implement catalogue rebuild-from-ES path (idempotent recovery on startup) | WP10 | | [D] |
| T044 | Unit tests with in-memory SQLite | WP10 | | [D] |
| T045 | Create `internal/elasticsearch/mapping.go` (loads `community_alerts` mapping JSON from contracts/) | WP11 | [D] |
| T046 | Create `internal/elasticsearch/indexer.go` (raw HTTP; `Index`, `MarkRescinded`, `EnsureIndex`) | WP11 | | [D] |
| T047 | Implement idempotent `EnsureIndex` (HEAD → PUT on 404) | WP11 | | [D] |
| T048 | Unit tests with `httptest.Server` simulating ES responses | WP11 | | [D] |
| T049 | Create `internal/redis/publisher.go` (`LifecycleEvent` JSON serializer + `Publish`) | WP12 | [P] |
| T050 | Channel name from config (`community_alerts:lifecycle`); publish failures incremented as metrics, not rolled back | WP12 | |
| T051 | Wrap `infrastructure/redis` client; do not write a bespoke client | WP12 | |
| T052 | Unit tests with mock Redis client | WP12 | |
| T053 | Create `internal/severity/table.go` (substance presence → severity floor table) | WP13 | [P] |
| T054 | Create `internal/severity/infer.go` (`Infer(Hazard) Severity` function) | WP13 | |
| T055 | Default config table: `carfentanil` → `critical`, `nitazenes` → `high`, `medetomidine`+opioid → `high`, others → `medium` | WP13 | |
| T056 | Unit tests: highest-severity substance wins; empty hazard returns `medium` floor | WP13 | |
| T057 | Create `internal/scope/resolver.go` (imports `taxonomy.ParentRegion`) | WP14 | [P] |
| T058 | Implement default-scope expansion per source (e.g., `mhrn` defaults to `[treaty:1, canada:manitoba]`) | WP14 | |
| T059 | Implement location-name → city slug inference (e.g., "Winnipeg" → `canada:manitoba:winnipeg`) | WP14 | |
| T060 | Unit tests: hierarchy resolution; missing taxonomy token logs metric and continues | WP14 | |
| T061 | Create `internal/runner/runner.go` (poll cycle: load → fetch → parse → diff → resolve → infer → write ES → publish → save) | WP15 | |
| T062 | Implement rescission detection via `RescindAbsent` after fetch loop | WP15 | |
| T063 | Implement error classification (transient → backoff; structural → parse_failure metric) | WP15 | |
| T064 | Create `internal/observability/metrics.go` (structured-log emitter for the metric set in plan §4.5) | WP15 | |
| T065 | Implement consecutive_failures gauge per source; ≥6 emits operator-actionable signal | WP15 | |
| T066 | Unit tests with mocked dependencies; assert each per-poll metric is emitted | WP15 | |
| T067 | Wire all dependencies in `main.go` (config, logger, sqlite, ES, redis, catalogue, runner) | WP16 | |
| T068 | Add `signal.NotifyContext(SIGINT, SIGTERM)` and graceful shutdown | WP16 | |
| T069 | Implement phase order: flags → setup → buildSources → runner → exit; mirrors signal-crawler | WP16 | |
| T070 | Smoke test: `task run` boots, polls once against a httptest fixture, exits cleanly | WP16 | |
| T071 | Create `cmd/backfill/main.go` (one-shot subcommand) | WP17 | [P] |
| T072 | Reuse runner with `backfill_mode=true` flag; emit `created` for top-20 items | WP17 | |
| T073 | Idempotency: re-running backfill is a no-op (catalogue check) | WP17 | |
| T074 | Unit tests for backfill mode | WP17 | |
| T075 | Set up integration test harness (`//go:build integration`; real ES + Redis + SQLite via existing CI integration harness) | WP18 | |
| T076 | AS-01: drug supply alert reaches Treaty 1 page (synthetic feed → ES → consumer query) | WP18 | |
| T077 | AS-02: corrected alert supersedes earlier version (revision_history append + updated event) | WP18 | |
| T078 | AS-03: rescinded alert disappears within one poll cycle (feed-delta detection) | WP18 | |
| T079 | AS-04: subscriber recovery after downtime (active-set query from ES alone) | WP18 | |
| T080 | AS-05: source unreachable for extended period (consecutive_failures gauge; durable store unaffected) | WP18 | |
| T081 | AS-06: scope vocabulary lookup (Treaty 1 page surfaces a manitoba-scoped alert) | WP18 | |
| T082 | Wait for indigenous-taxonomy v1.1.0 tag (gates: WP04 merged) | WP19 | |
| T083 | Remove `replace` directive from `alert-crawler/go.mod` | WP19 | |
| T084 | Pin to `github.com/jonesrussell/indigenous-taxonomy v1.1.0` | WP19 | |
| T085 | Run `go mod tidy`; verify build succeeds without `replace` (CI gate) | WP19 | |
| T086 | Add `alert-crawler` service entry to `docker-compose.base.yml` (`restart: "no"`, named volume `alert-crawler-data:/app/data`, no health check) | WP20 | |
| T087 | Add `alert-crawler-data` named volume declaration | WP20 | |
| T088 | Verify service brings up cleanly via `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up alert-crawler` | WP20 | |
| T089 | Run `task ports:check`; regenerate ports SSOT for new compose entry | WP20 | |
| T090 | Add `alert-crawler` to oneshot skip list in `scripts/deploy.sh` | WP21 | [P] |
| T091 | Add `alert-crawler` to `GO_SERVICES` in `scripts/detect-changed-services.sh` | WP21 | |
| T092 | Add `vuln:alert-crawler` delegation in root `Taskfile.yml` | WP21 | |
| T093 | Verify CI workflow detects and tests alert-crawler on diff (manual: open dummy PR, observe CI) | WP21 | |
| T094 | Create `alert-crawler.timer.j2` (`OnCalendar=*-*-* *:30:00 UTC`, `Persistent=true`, `RandomizedDelaySec=120`) | WP22 | [P] |
| T095 | Create `alert-crawler.service.j2` (`Type=oneshot`, `ExecStartPre` image pull, `ExecStart` `docker compose ... run --rm alert-crawler`) | WP22 | |
| T096 | Add `nc_alert_crawler_schedule` default in `roles/north-cloud/defaults/main.yml` | WP22 | |
| T097 | Add data-dir creation task with `owner: "1000"` (RR PR-004 mitigation) | WP22 | |
| T098 | Add `docs/specs/community-alert-pipeline.md` (or pointer to mission spec) | WP23 | |
| T099 | Update `tools/drift-detector.sh` mapping for `alert-crawler/**` | WP23 | |
| T100 | Write `alert-crawler/CLAUDE.md` (config gotcha, mapping ownership, replace removal, rescission, RR PR-004) | WP23 | |
| T101 | Update repo-root `CLAUDE.md` orchestration table to include `alert-crawler/**` | WP23 | |
| T102 | NFR-001 latency harness: synthetic upstream publish; assert 95% ≤60min, 99% ≤120min | WP24 | |
| T103 | NFR-006 idempotency harness: 100-cycle replay; assert 0 spurious lifecycle events | WP24 | |
| T104 | NFR-009 blast-radius harness: synthetic alert-crawler crash; assert lead pipeline unaffected | WP24 | |
| T105 | First-deploy backfill rehearsal (D.3): run backfill against staging or sandbox; verify 20 `created` events | WP24 | |
| T106 | Run `task lint:force` clean; address any drift findings | WP25 | |
| T107 | Run `task test` and `task test:alert-crawler -- -tags integration` clean | WP25 | |
| T108 | Run `task drift:check`, `task ports:check`, `task layers:check` clean | WP25 | |
| T109 | Verify lefthook pre-commit and pre-push pass without `--no-verify` | WP25 | |
| T110 | Manual smoke test: end-to-end against staging (publish → consume) | WP25 | |
| T111 | File GitHub issue: "Migrate classifier and publisher to import indigenous-taxonomy directly" with R-004 context | WP25 | |

## Work Packages

### Phase A — indigenous-taxonomy v1.1.0

## WP01 — Treaty namespace schema and Go bindings

**Goal**: Add an additive `Treaty` namespace to `jonesrussell/indigenous-taxonomy` with 11 numbered-treaty constants and a `taxonomy.Treaty` Go type. Foundation for sovereignty-aware scope resolution.

**Priority**: P1 (blocks Phase A merge → blocks WP19 → blocks final merge)

**Independent test**: `go test ./generated/go/taxonomy/... -run TestTreaty` passes; `taxonomy.IsValidTreaty("treaty:1")` returns `true`; `taxonomy.AllTreaties` has length 11.

**Estimated prompt size**: ~280 lines

**Subtasks**:

- [x] T001 Create `schema/treaties.yaml` (11 treaty entries with member-region metadata) (WP01)
- [x] T002 Extend `scripts/generate.py` to emit `treaties.go` from `treaties.yaml` (WP01)
- [x] T003 Generate `taxonomy/treaties.go` (`Treaty` type, 11 constants, `AllTreaties`, `IsValidTreaty`) (WP01)
- [x] T004 Add unit tests for Treaty namespace (WP01)

**Dependencies**: none.

**Risks**: RR-004 (existing tests assert hardcoded counts) — mitigated by audit step in WP04.

**Prompt**: `tasks/WP01-treaty-namespace-schema-and-go-bindings.md`

## WP02 — Region YAML extension (cities + sample communities)

**Goal**: Extend `schema/regions.yaml` with city children under existing province nodes, plus one First Nations community sample per active treaty area. Establishes the pattern for community-level scope tokens.

**Priority**: P1

**Independent test**: `taxonomy.IsValidRegion("canada:manitoba:winnipeg")` returns `true`; `taxonomy.AllRegions` includes all new constants.

**Estimated prompt size**: ~240 lines

**Subtasks**:

- [x] T005 Extend `schema/regions.yaml` with city children (winnipeg, toronto, ottawa, vancouver, calgary, saskatoon) (WP02)
- [x] T006 Add sample First Nations community per active treaty (e.g., `canada:manitoba:sagkeeng-fn`) (WP02)
- [x] T007 Regenerate `taxonomy/regions.go` with new entries (WP02)
- [x] T008 Update region tests (count assertions; new constant existence) (WP02)

**Dependencies**: none.

**Risks**: same RR-004 audit applies.

**Prompt**: `tasks/WP02-region-yaml-extension.md`

## WP03 — ParentRegion walk function

**Goal**: Extend the code generator to emit a `ParentRegion(Region) (Region, bool)` lookup table from existing YAML `children:` data, enabling consumers to walk the region hierarchy upward.

**Priority**: P1

**Independent test**: `taxonomy.ParentRegion("canada:manitoba:winnipeg")` returns `("canada:manitoba", true)`; `taxonomy.ParentRegion("canada")` returns `("", false)`.

**Estimated prompt size**: ~220 lines

**Subtasks**:

- [x] T009 Extend `scripts/generate.py` to emit `ParentRegion` lookup table from YAML `children:` (WP03)
- [x] T010 Generate `ParentRegion(Region) (Region, bool)` in `taxonomy/regions.go` (WP03)
- [x] T011 Unit tests covering hierarchy walks (leaf → province, province → canada, canada → no parent) (WP03)

**Dependencies**: WP01, WP02 (the generator changes operate on the regions and treaties schemas; can race-condition the `regions.go` regeneration).

**Risks**: generator complexity; YAML cross-reference correctness.

**Prompt**: `tasks/WP03-parent-region-walk-function.md`

## WP04 — Version bump v1.1.0 + tag

**Goal**: Audit existing tests for hardcoded counter assertions, bump the package to v1.1.0, update the changelog, open the PR, and tag the release after merge.

**Priority**: P1 (gates Phase B merge via WP19)

**Independent test**: `git tag` shows `v1.1.0`; `go list -m github.com/jonesrussell/indigenous-taxonomy@v1.1.0` returns the new module.

**Estimated prompt size**: ~200 lines

**Subtasks**:

- [x] T012 Audit `tests/` for hardcoded counter assertions (RR-004) (WP04)
- [x] T013 Update `taxonomy/version.go` and `go.mod` to v1.1.0 (WP04)
- [x] T014 Update `CHANGELOG.md` with additive changes (WP04)
- [x] T015 Open PR; tag `v1.1.0` after merge (WP04)

**Dependencies**: WP01, WP02, WP03 (all schema and generator work must merge first).

**Risks**: RR-004 (counter assertions); release coordination.

**Prompt**: `tasks/WP04-version-bump-and-tag.md`

### Phase B — alert-crawler scaffolding (north-cloud monorepo)

## WP05 — Service scaffold

**Goal**: Create the `alert-crawler/` directory skeleton with `go.mod` (carrying `replace` directive), `main.go` skeleton, `config.yml`, `Taskfile.yml`, multi-stage `Dockerfile`, `.layers`, and `CLAUDE.md` placeholder. Establishes the operational surface mirroring `signal-crawler`.

**Priority**: P1 (blocks all subsequent Phase B WPs)

**Independent test**: `cd alert-crawler && task build` produces a binary; `docker build -f alert-crawler/Dockerfile alert-crawler/` succeeds.

**Estimated prompt size**: ~400 lines

**Subtasks**:

- [x] T016 Create `alert-crawler/` directory with `go.mod` (includes `replace` directive) (WP05)
- [x] T017 Create `main.go` skeleton with phase-order comments (WP05)
- [x] T018 Create `config.yml` (defaults blank where `SetDefaults` applies) (WP05)
- [x] T019 Create per-service `Taskfile.yml` (build, run, test, lint, vuln, clean) (WP05)
- [x] T020 Create multi-stage `Dockerfile` (Alpine, uid 1000 non-root user, CGO enabled) (WP05)
- [x] T021 Create `.layers` (L0 config/domain; L1 adapter/catalogue/elasticsearch/redis/severity/scope/observability; L2 runner) (WP05)
- [x] T022 Create `alert-crawler/CLAUDE.md` skeleton with placeholders for gotchas (WP05)

**Dependencies**: none.

**Risks**: RR-005 (first NC service to import indigenous-taxonomy directly; expose latent integration issues).

**Prompt**: `tasks/WP05-service-scaffold.md`

## WP06 — Domain types

**Goal**: Define the canonical `Alert`, `Hazard`, `Source`, and `LifecycleEvent` Go types matching `contracts/community-alert.schema.json` and `contracts/lifecycle-event.schema.json`.

**Priority**: P1 (foundational; blocks adapter/runner/publisher work)

**Independent test**: `go test ./internal/domain/...` passes; envelope JSON round-trip preserves all fields.

**Estimated prompt size**: ~280 lines

**Subtasks**:

- [x] T023 Create `internal/domain/alert.go` (Alert envelope) (WP06)
- [x] T024 Create `internal/domain/hazard.go` (HarmReductionHazard, Substance) (WP06)
- [x] T025 Create `internal/domain/source.go` (AlertSource, AcquisitionStrategy enum) (WP06)
- [x] T026 Create `internal/domain/lifecycle_event.go` (LifecycleEvent, EventType enum) (WP06)
- [x] T027 Add unit tests for envelope JSON round-trip and schema conformance (WP06)

**Dependencies**: WP05.

**Risks**: low; pure type definitions.

**Prompt**: `tasks/WP06-domain-types.md`

## WP07 — Configuration

**Goal**: Wire alert-crawler's configuration through `infrastructure/config.LoadWithDefaults[Config]` with env-tag binding. Include the SetDefaults pitfall regression test (RR-007).

**Priority**: P1

**Independent test**: `go test ./internal/config/...` passes; loading config with empty `config.yml` yields the SetDefaults-defined defaults; pitfall test fails the build if `config.yml` carries fields owned by SetDefaults.

**Estimated prompt size**: ~260 lines

**Subtasks**:

- [x] T028 Create `internal/config/config.go` (Config struct with env tags) (WP07)
- [x] T029 Create `internal/config/defaults.go` (`SetDefaults` function) (WP07)
- [x] T030 Wire to `infrastructure/config.LoadWithDefaults[Config]` (WP07)
- [x] T031 Add SetDefaults pitfall regression test (RR-007) — fields with SetDefaults must be absent from `config.yml` (WP07)

**Dependencies**: WP05 (scaffold), WP06 (domain types referenced by config).

**Risks**: RR-007 mitigated by T031.

**Prompt**: `tasks/WP07-configuration.md`

## WP08 — RSS HTTP client

**Goal**: Build the HTTP fetcher with ETag/Last-Modified conditional GET, configurable User-Agent, timeouts, and a `304 Not Modified` short-circuit path.

**Priority**: P1

**Independent test**: `go test ./internal/adapter/rss/... -run TestClient` passes against an `httptest.Server`; 304 path returns `ErrNotModified`; 5xx path returns wrapped error with `consecutive_failures` increment hint.

**Estimated prompt size**: ~280 lines

**Subtasks**:

- [x] T032 Create `internal/adapter/rss/client.go` (HTTP fetch with `If-None-Match` and `If-Modified-Since`) (WP08)
- [x] T033 Implement `ErrNotModified` on 304 path; ETag and Last-Modified return values (WP08)
- [x] T034 Configure User-Agent (`alert-crawler/1.0 (+https://northcloud.one)`) and HTTP timeouts (WP08)
- [x] T035 Unit tests: 304 path, 200 path, 5xx with backoff, network timeout (WP08)

**Dependencies**: WP05, WP06.

**Risks**: RR-001 (Cloudflare may eventually block) — out-of-scope mitigation.

**Prompt**: `tasks/WP08-rss-http-client.md`

## WP09 — RSS parser + degraded fallback

**Goal**: Parse RSS XML and extract structured fields from `<description>` HTML (substance composition, lab source, confirmation date). Defensive parsing with `parse_quality` flag — never auto-rescind on parse failure.

**Priority**: P1

**Independent test**: `go test ./internal/adapter/rss/... -run TestParser` passes against a golden RSS fixture; degraded items emit `parse_quality: degraded`.

**Estimated prompt size**: ~320 lines

**Subtasks**:

- [x] T036 Create `internal/adapter/rss/parser.go` (RSS XML parse via `encoding/xml`) (WP09)
- [x] T037 Implement `<description>` HTML extractor (substance composition, lab source, confirmation date) (WP09)
- [x] T038 Defensive parsing with `parse_quality` flag (clean/degraded); never auto-rescind on parse failure (TC-010) (WP09)
- [x] T039 Unit tests with golden RSS fixture (real safersites.ca sample anonymized to a fixture) (WP09)

**Dependencies**: WP05, WP06.

**Risks**: RR-002 (NationBuilder template change) — mitigated by `parse_quality` flag.

**Prompt**: `tasks/WP09-rss-parser-and-degraded-fallback.md`

## WP10 — SQLite catalogue

**Goal**: Local state store for poll checkpoints (ETag, Last-Modified, consecutive_failures per source) and the active-alert catalogue used for rescission detection. Idempotent rebuild from ES on startup.

**Priority**: P1

**Independent test**: `go test ./internal/catalogue/...` passes against in-memory SQLite; rebuild path repopulates from a synthetic ES snapshot.

**Estimated prompt size**: ~340 lines

**Subtasks**:

- [x] T040 Create `internal/catalogue/schema.sql` (`poll_checkpoint`, `alert_catalogue` tables) (WP10)
- [x] T041 Add `internal/catalogue/migrations/` with unique numeric prefixes (WP10)
- [x] T042 Create `internal/catalogue/store.go` (`LoadCheckpoint`, `SaveCheckpoint`, `LookupAlert`, `MarkSeen`, `RescindAbsent`) (WP10)
- [x] T043 Implement catalogue rebuild-from-ES path (idempotent recovery on startup) (WP10)
- [x] T044 Unit tests with in-memory SQLite (WP10)

**Dependencies**: WP05, WP06.

**Risks**: migration prefix collisions (deploy.sh fails fast on duplicates).

**Prompt**: `tasks/WP10-sqlite-catalogue.md`

## WP11 — Elasticsearch indexer

**Goal**: Single-doc indexing into `community_alerts` with idempotent `EnsureIndex` (HEAD then PUT on 404). Mapping owned in-service per the resolved charter tension. Raw `net/http`; no ES SDK.

**Priority**: P1

**Independent test**: `go test ./internal/elasticsearch/...` passes against `httptest.Server` simulating ES; `EnsureIndex` is idempotent.

**Estimated prompt size**: ~320 lines

**Subtasks**:

- [x] T045 Create `internal/elasticsearch/mapping.go` (loads `community_alerts` mapping JSON from contracts/) (WP11)
- [x] T046 Create `internal/elasticsearch/indexer.go` (raw HTTP; `Index`, `MarkRescinded`, `EnsureIndex`) (WP11)
- [x] T047 Implement idempotent `EnsureIndex` (HEAD → PUT on 404) (WP11)
- [x] T048 Unit tests with `httptest.Server` simulating ES responses (WP11)

**Dependencies**: WP05, WP06.

**Risks**: ES mapping ownership tension (resolved by maintainer 2026-05-06).

**Prompt**: `tasks/WP11-elasticsearch-indexer.md`

## WP12 — Redis publisher

**Goal**: Publish `LifecycleEvent` JSON payloads to Redis pub/sub channel `community_alerts:lifecycle`. Wraps `infrastructure/redis`. Failures are observable but non-fatal (ES is canonical).

**Priority**: P1

**Independent test**: `go test ./internal/redis/...` passes against mock Redis; payload conforms to `contracts/lifecycle-event.schema.json`.

**Estimated prompt size**: ~240 lines

**Subtasks**:

- [ ] T049 Create `internal/redis/publisher.go` (`LifecycleEvent` JSON serializer + `Publish`) (WP12)
- [ ] T050 Channel name from config (`community_alerts:lifecycle`); publish failures incremented as metrics, not rolled back (WP12)
- [ ] T051 Wrap `infrastructure/redis` client; do not write a bespoke client (WP12)
- [ ] T052 Unit tests with mock Redis client (WP12)

**Dependencies**: WP05, WP06.

**Risks**: Redis password handling on prod (CLAUDE.md gotcha).

**Prompt**: `tasks/WP12-redis-publisher.md`

## WP13 — Severity inference

**Goal**: Configurable heuristic mapping table that infers `severity` from observed substances. Highest-severity substance wins. Operator-tunable in `config.yml`.

**Priority**: P2

**Independent test**: `go test ./internal/severity/...` passes; `Infer({substances: ["fentanyl", "carfentanil"]}) → "critical"`.

**Estimated prompt size**: ~240 lines

**Subtasks**:

- [ ] T053 Create `internal/severity/table.go` (substance presence → severity floor table) (WP13)
- [ ] T054 Create `internal/severity/infer.go` (`Infer(Hazard) Severity` function) (WP13)
- [ ] T055 Default config table: `carfentanil` → `critical`, `nitazenes` → `high`, `medetomidine`+opioid → `high`, others → `medium` (WP13)
- [ ] T056 Unit tests: highest-severity substance wins; empty hazard returns `medium` floor (WP13)

**Dependencies**: WP05, WP06, WP07.

**Risks**: PR-003 — heuristic table may drift from issuing-org practice; configurable mitigates.

**Prompt**: `tasks/WP13-severity-inference.md`

## WP14 — Scope resolver

**Goal**: Resolve `scope` tokens from a combination of source-default tokens and content-derived tokens (location-name → city slug). Imports `taxonomy.ParentRegion` for hierarchy walks.

**Priority**: P2

**Independent test**: `go test ./internal/scope/...` passes; resolver expands `[treaty:1]` to include all member province/city slugs via ParentRegion walk.

**Estimated prompt size**: ~260 lines

**Subtasks**:

- [ ] T057 Create `internal/scope/resolver.go` (imports `taxonomy.ParentRegion`) (WP14)
- [ ] T058 Implement default-scope expansion per source (e.g., `mhrn` defaults to `[treaty:1, canada:manitoba]`) (WP14)
- [ ] T059 Implement location-name → city slug inference (e.g., "Winnipeg" → `canada:manitoba:winnipeg`) (WP14)
- [ ] T060 Unit tests: hierarchy resolution; missing taxonomy token logs metric and continues (WP14)

**Dependencies**: WP05, WP06, WP07. (Functionally also depends on the `taxonomy.ParentRegion` function from Phase A; this dependency is satisfied at runtime via the `replace` directive in `go.mod` and is not a sequencing dependency.)

**Risks**: RR-005 (latent indigenous-taxonomy integration issues).

**Prompt**: `tasks/WP14-scope-resolver.md`

## WP15 — Runner orchestration + observability

**Goal**: The L2 orchestrator that drives one poll cycle: load checkpoint → fetch → parse → diff catalogue → resolve scope → infer severity → write ES → publish Redis → save checkpoint. Includes observability metric emission.

**Priority**: P1 (integrates everything below it)

**Independent test**: `go test ./internal/runner/...` passes with mocked dependencies; each per-poll metric is emitted.

**Estimated prompt size**: ~480 lines

**Subtasks**:

- [ ] T061 Create `internal/runner/runner.go` (poll cycle orchestration) (WP15)
- [ ] T062 Implement rescission detection via `RescindAbsent` after fetch loop (WP15)
- [ ] T063 Implement error classification (transient → backoff; structural → parse_failure metric) (WP15)
- [ ] T064 Create `internal/observability/metrics.go` (structured-log emitter for the metric set in plan §4.5) (WP15)
- [ ] T065 Implement consecutive_failures gauge per source; ≥6 emits operator-actionable signal (WP15)
- [ ] T066 Unit tests with mocked dependencies; assert each per-poll metric is emitted (WP15)

**Dependencies**: WP06, WP07, WP08, WP09, WP10, WP11, WP12, WP13, WP14.

**Risks**: integration complexity; many moving parts.

**Prompt**: `tasks/WP15-runner-orchestration-and-observability.md`

## WP16 — Wiring in main.go

**Goal**: Phase-order wiring in `main.go` (flags → setup → buildSources → runner → exit). Mirrors signal-crawler. Adds `signal.NotifyContext` for graceful shutdown.

**Priority**: P1

**Independent test**: `task run` boots, polls once against an `httptest` fixture, exits cleanly. No goroutine leaks.

**Estimated prompt size**: ~240 lines

**Subtasks**:

- [ ] T067 Wire all dependencies in `main.go` (config, logger, sqlite, ES, redis, catalogue, runner) (WP16)
- [ ] T068 Add `signal.NotifyContext(SIGINT, SIGTERM)` and graceful shutdown (WP16)
- [ ] T069 Implement phase order: flags → setup → buildSources → runner → exit; mirrors signal-crawler (WP16)
- [ ] T070 Smoke test: `task run` boots, polls once against a httptest fixture, exits cleanly (WP16)

**Dependencies**: WP15 and all preceding Phase B WPs.

**Risks**: low.

**Prompt**: `tasks/WP16-wiring-main.md`

## WP17 — Backfill subcommand

**Goal**: One-shot `cmd/backfill/main.go` that pulls the 20 most recent feed items and emits them as `created` events. Idempotent re-runs.

**Priority**: P2

**Independent test**: `go test ./cmd/backfill/...` passes; first run emits 20 events; second run emits 0.

**Estimated prompt size**: ~220 lines

**Subtasks**:

- [ ] T071 Create `cmd/backfill/main.go` (one-shot subcommand) (WP17)
- [ ] T072 Reuse runner with `backfill_mode=true` flag; emit `created` for top-20 items (WP17)
- [ ] T073 Idempotency: re-running backfill is a no-op (catalogue check) (WP17)
- [ ] T074 Unit tests for backfill mode (WP17)

**Dependencies**: WP15, WP16.

**Risks**: PR-002 (backfill burst overwhelming consumers); v1 mitigation: opt-in.

**Prompt**: `tasks/WP17-backfill-subcommand.md`

## WP18 — Integration tests (AS-01..AS-06)

**Goal**: End-to-end integration tests against real ES, Redis, and SQLite for each acceptance scenario. Build tag `//go:build integration` separates from unit tests.

**Priority**: P1

**Independent test**: `task test:alert-crawler -- -tags integration` passes; CI integration job is green.

**Estimated prompt size**: ~480 lines

**Subtasks**:

- [ ] T075 Set up integration test harness (`//go:build integration`; real ES + Redis + SQLite via existing CI integration harness) (WP18)
- [ ] T076 AS-01: drug supply alert reaches Treaty 1 page (synthetic feed → ES → consumer query) (WP18)
- [ ] T077 AS-02: corrected alert supersedes earlier version (revision_history append + updated event) (WP18)
- [ ] T078 AS-03: rescinded alert disappears within one poll cycle (feed-delta detection) (WP18)
- [ ] T079 AS-04: subscriber recovery after downtime (active-set query from ES alone) (WP18)
- [ ] T080 AS-05: source unreachable for extended period (consecutive_failures gauge; durable store unaffected) (WP18)
- [ ] T081 AS-06: scope vocabulary lookup (Treaty 1 page surfaces a manitoba-scoped alert) (WP18)

**Dependencies**: WP15, WP16, WP17.

**Risks**: flaky integration tests; mitigate with explicit timeouts and retry-once.

**Prompt**: `tasks/WP18-integration-tests.md`

### Phase C — Reconciliation & deployment integration

## WP19 — Pin taxonomy v1.1.0 (remove replace directive)

**Goal**: Remove the `replace` directive from `alert-crawler/go.mod`, pin to released `v1.1.0`. Gates final merge.

**Priority**: P1

**Independent test**: `go mod tidy` succeeds without `replace`; CI build passes.

**Estimated prompt size**: ~180 lines

**Subtasks**:

- [ ] T082 Wait for indigenous-taxonomy v1.1.0 tag (gates: WP04 merged) (WP19)
- [ ] T083 Remove `replace` directive from `alert-crawler/go.mod` (WP19)
- [ ] T084 Pin to `github.com/jonesrussell/indigenous-taxonomy v1.1.0` (WP19)
- [ ] T085 Run `go mod tidy`; verify build succeeds without `replace` (CI gate) (WP19)

**Dependencies**: WP04, WP18.

**Risks**: PR-001 (coordination drift); CI fails fast if `replace` is left in.

**Prompt**: `tasks/WP19-pin-taxonomy.md`

## WP20 — Docker Compose entry

**Goal**: Add the alert-crawler service to `docker-compose.base.yml` following the oneshot pattern. Verify clean bring-up.

**Priority**: P1

**Independent test**: `docker compose ... up alert-crawler` exits cleanly; `task ports:check` clean.

**Estimated prompt size**: ~220 lines

**Subtasks**:

- [ ] T086 Add `alert-crawler` service entry to `docker-compose.base.yml` (`restart: "no"`, named volume, no health check) (WP20)
- [ ] T087 Add `alert-crawler-data` named volume declaration (WP20)
- [ ] T088 Verify service brings up cleanly via `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up alert-crawler` (WP20)
- [ ] T089 Run `task ports:check`; regenerate ports SSOT for new compose entry (WP20)

**Dependencies**: WP05.

**Risks**: low.

**Prompt**: `tasks/WP20-docker-compose-entry.md`

## WP21 — Deploy integration

**Goal**: Hook alert-crawler into north-cloud's deploy and CI surfaces (oneshot skip list, GO_SERVICES, vuln target).

**Priority**: P1

**Independent test**: opening a dummy PR that touches `alert-crawler/` files triggers CI for alert-crawler.

**Estimated prompt size**: ~220 lines

**Subtasks**:

- [ ] T090 Add `alert-crawler` to oneshot skip list in `scripts/deploy.sh` (WP21)
- [ ] T091 Add `alert-crawler` to `GO_SERVICES` in `scripts/detect-changed-services.sh` (WP21)
- [ ] T092 Add `vuln:alert-crawler` delegation in root `Taskfile.yml` (WP21)
- [ ] T093 Verify CI workflow detects and tests alert-crawler on diff (manual: open dummy PR, observe CI) (WP21)

**Dependencies**: WP05.

**Risks**: missing one of the three integration points → CI silently skips.

**Prompt**: `tasks/WP21-deploy-integration.md`

## WP22 — Ansible role

**Goal**: Add `alert-crawler.timer.j2` and `alert-crawler.service.j2` to the existing `north-cloud` Ansible role. Parameterize cadence; fix uid mismatch on data dir.

**Priority**: P1

**Independent test**: `ansible-playbook --check` clean; rendered unit files validate via `systemd-analyze verify`.

**Estimated prompt size**: ~280 lines

**Subtasks**:

- [ ] T094 Create `alert-crawler.timer.j2` (`OnCalendar=*-*-* *:30:00 UTC`, `Persistent=true`, `RandomizedDelaySec=120`) (WP22)
- [ ] T095 Create `alert-crawler.service.j2` (`Type=oneshot`, `ExecStartPre` image pull, `ExecStart` `docker compose ... run --rm alert-crawler`) (WP22)
- [ ] T096 Add `nc_alert_crawler_schedule` default in `roles/north-cloud/defaults/main.yml` (WP22)
- [ ] T097 Add data-dir creation task with `owner: "1000"` (RR PR-004 mitigation) (WP22)

**Dependencies**: WP05, WP20.

**Risks**: PR-004 (volume ownership) — mitigated by T097.

**Prompt**: `tasks/WP22-ansible-role.md`

## WP23 — Drift integration + service docs

**Goal**: Add a NC docs spec pointer for alert-crawler, update drift-detector mappings, write the service CLAUDE.md, update repo-root CLAUDE.md.

**Priority**: P1

**Independent test**: `task drift:check` clean; CLAUDE.md orchestration table includes `alert-crawler/**`.

**Estimated prompt size**: ~260 lines

**Subtasks**:

- [ ] T098 Add `docs/specs/community-alert-pipeline.md` (or pointer to mission spec) (WP23)
- [ ] T099 Update `tools/drift-detector.sh` mapping for `alert-crawler/**` (WP23)
- [ ] T100 Write `alert-crawler/CLAUDE.md` (config gotcha, mapping ownership, replace removal, rescission, RR PR-004) (WP23)
- [ ] T101 Update repo-root `CLAUDE.md` orchestration table to include `alert-crawler/**` (WP23)

**Dependencies**: WP05.

**Risks**: drift-detector mapping easy to miss (CI catches).

**Prompt**: `tasks/WP23-drift-integration-and-docs.md`

### Phase D — Acceptance & merge readiness

## WP24 — NFR validation harness

**Goal**: Implement automated harnesses for NFR-001 (latency), NFR-006 (idempotency), NFR-009 (blast-radius). Plus the first-deploy backfill rehearsal.

**Priority**: P1

**Independent test**: each harness produces a pass/fail report against thresholds in spec §4.

**Estimated prompt size**: ~300 lines

**Subtasks**:

- [ ] T102 NFR-001 latency harness: synthetic upstream publish; assert 95% ≤60min, 99% ≤120min (WP24)
- [ ] T103 NFR-006 idempotency harness: 100-cycle replay; assert 0 spurious lifecycle events (WP24)
- [ ] T104 NFR-009 blast-radius harness: synthetic alert-crawler crash; assert lead pipeline unaffected (WP24)
- [ ] T105 First-deploy backfill rehearsal (D.3): run backfill against staging or sandbox; verify 20 `created` events (WP24)

**Dependencies**: WP18 (integration test harness reused).

**Risks**: test flakiness in distributed timing scenarios — mitigate with generous tolerance windows.

**Prompt**: `tasks/WP24-nfr-validation-harness.md`

## WP25 — Pre-merge checklist + post-merge follow-on issue

**Goal**: Final lint/test/drift validation, manual smoke test against staging, file the deferred classifier/publisher migration backlog item.

**Priority**: P1

**Independent test**: all CI gates green; staging end-to-end smoke confirms publish → consume; GitHub issue filed and linked from research.md.

**Estimated prompt size**: ~280 lines

**Subtasks**:

- [ ] T106 Run `task lint:force` clean; address any drift findings (WP25)
- [ ] T107 Run `task test` and `task test:alert-crawler -- -tags integration` clean (WP25)
- [ ] T108 Run `task drift:check`, `task ports:check`, `task layers:check` clean (WP25)
- [ ] T109 Verify lefthook pre-commit and pre-push pass without `--no-verify` (WP25)
- [ ] T110 Manual smoke test: end-to-end against staging (publish → consume) (WP25)
- [ ] T111 File GitHub issue: "Migrate classifier and publisher to import indigenous-taxonomy directly" with R-004 context (WP25)

**Dependencies**: WP19, WP20, WP21, WP22, WP23, WP24.

**Risks**: late-breaking lint or drift failures.

**Prompt**: `tasks/WP25-pre-merge-and-followon.md`

## Parallelization Highlights

- **Phase A and Phase B run in parallel.** WP01-WP04 (taxonomy) and WP05-WP18 (alert-crawler) progress independently while the `replace` directive bridges them.
- **Within Phase A**: WP01, WP02 are parallel (different YAML files). WP03 depends on both. WP04 depends on WP01, WP02, WP03.
- **Within Phase B**:
  - WP05 (scaffold) is the foundation; nothing else can start until it lands.
  - WP06 (domain), WP07 (config) are parallel after WP05.
  - WP08, WP09, WP10, WP11, WP12, WP13, WP14 are parallel after WP06+WP07 (each touches a different package).
  - WP15 (runner) integrates everything; depends on all of WP06..WP14.
  - WP16, WP17 are parallel after WP15.
  - WP18 (integration tests) depends on WP16+WP17.
- **Phase C** WPs WP20, WP21, WP22, WP23 are parallel after WP05.
- **WP19** (pin taxonomy) gates final merge; depends on WP04 + Phase B feature-complete.
- **Phase D** WPs WP24, WP25 are sequential.

## MVP Scope

**MVP = WP01–WP05, plus a thin slice of WP15 sufficient to ingest one alert and persist it to ES.**

This bypasses Redis publish, scope resolution, severity inference, and integration tests, demonstrating the architectural skeleton end-to-end. Not the full feature, but the smallest demonstration that the wiring is correct and the contracts hold.

For the production-ready feature, all 25 WPs are required.

## Implementation Path Recommendation

1. **Lane A** (one agent): WP01 → WP02 → WP03 → WP04 (Phase A, sequential).
2. **Lane B** (one or more agents): WP05 (foundation, must complete first), then parallel: { WP06 + WP07 } → { WP08, WP09, WP10, WP11, WP12, WP13, WP14 in parallel } → WP15 → { WP16, WP17 } → WP18.
3. **Lane C** (one agent, after WP05): WP20, WP21, WP22, WP23 in parallel.
4. **Reconciliation** (after WP04 lands and Lane B is feature-complete): WP19.
5. **Acceptance** (final): WP24 → WP25.

`finalize-tasks` will compute the actual lane assignments from the dependency graph.

## Notes for the Implement-Review Loop

- Every WP includes its own unit tests; no separate "tests" WP. The 80% coverage gate applies per package.
- Integration tests (WP18) cover acceptance scenarios end-to-end; they use the existing CI integration harness with real ES + Redis + SQLite.
- WP25's manual smoke test is the only step that cannot be automated.
- Phase A (sibling repo) WPs operate on `/home/jones/dev/indigenous-taxonomy/` — agents must respect cross-repo boundaries explicitly declared in `owned_files`.
