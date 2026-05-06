# Implementation Plan — Community Alert Pipeline

**Mission ID**: `01KQZC7A7SJJZ6EKHZ9JW3AZJG` (mid8: `01KQZC7A`)
**Mission Slug**: `community-alert-pipeline-01KQZC7A`
**Mission Type**: software-dev
**Date**: 2026-05-06
**Phase**: Plan (Phase 1)

## Branch Contract (deterministic)

Authoritative output of `spec-kitty agent mission setup-plan --json`:

- **Current branch**: `main`
- **Planning/base branch**: `main`
- **Merge target branch**: `main`
- **`branch_matches_target`**: `true`

The maintainer authorized `main` as the landing branch. No worktree is created at the plan phase; worktrees come at `/spec-kitty.implement`.

## Pointers

- Spec: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/spec.md`
- Research: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/research.md`
- Data Model: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/data-model.md`
- Contracts: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/contracts/`
- Quickstart: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/quickstart.md`
- Evidence: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/research/evidence-log.csv`

---

## Summary

Add a new `alert-crawler` Go service to the north-cloud monorepo that polls Manitoba Harm Reduction Network (`https://www.safersites.ca/drugalerts.rss`) every 30-60 minutes, normalizes drug-supply alerts into a generic `community_alert` envelope, persists each alert to a dedicated Elasticsearch index `community_alerts`, and emits lifecycle events (`created` / `updated` / `rescinded`) on Redis pub/sub channel `community_alerts:lifecycle`. Bypasses classifier and publisher routing (mirrors the `rfp-ingestor` pattern). The `scope` field uses a hierarchical controlled vocabulary sourced from `jonesrussell/indigenous-taxonomy` (this mission cuts a v1.1.0 additive bump of that package). Process model is oneshot binary + systemd timer (mirrors `signal-crawler`). First downstream consumer: Minoo community pages.

## Technical Context

**Language/Version**: Go 1.26+ (charter-mandated)
**Primary Dependencies**: stdlib `net/http` and `encoding/xml` (or `github.com/mmcdole/gofeed` for RSS resilience), `github.com/mattn/go-sqlite3` (CGO required), `github.com/jonesrussell/indigenous-taxonomy` (v1.1.0, additively extended in this mission), and the existing `infrastructure/{config,logger,redis}` packages
**Storage**: Elasticsearch (canonical alert documents in index `community_alerts`), Redis pub/sub (lifecycle events on channel `community_alerts:lifecycle`), SQLite at `data/state.db` (poll checkpoints + active-alert catalogue, volume-mounted across container runs)
**Testing**: `task test:alert-crawler` (unit, fast, default), `-tags integration` for tests that hit real ES/Redis/SQLite via the existing CI integration harness; `t.Helper()` in helpers; ≥80% coverage; build tags separate quick from slow suites
**Target Platform**: Docker container in north-cloud's existing Compose-orchestrated stack on Linux; production hosted on DigitalOcean VPS
**Project Type**: Single backend service (Go module) inside an established monorepo
**Performance Goals**: NFR-001 (95% ≤60min publish-to-consumer; 99% ≤120min); NFR-002 (99.5% ES read ≤2s); NFR-003 (99% live event delivery ≤5s); NFR-006 (zero spurious lifecycle events over 100 idempotent cycles)
**Constraints**: oneshot process model (no daemon), `restart: "no"` in Compose, manual wiring (no Uber FX), SQLite for local state (CGO permitted), classifier and publisher bypass (no routing rules), `*_classified_content` family explicitly NOT used (fresh `community_alerts` family), in-service ES mapping ownership (mirrors rfp-ingestor; deviates from literal charter reading; approved as single-writer greenfield index)
**Scale/Scope**: ≤20 alerts in steady state per source per day; ≤8 sources in v1+v2 outlook; one-shot poll cycles 30-60 min apart; cumulative alert document count ≤10K over 5 years per source

## Charter Check

The mission's plan-phase artifacts conform to `/home/jones/dev/north-cloud/.kittify/charter/charter.md`.

| Charter axis | Plan posture |
|---|---|
| Intent (frozen-except-backlog) | Net-new service authorized as a charter exception (AD-007 / C-011). Documented in spec; no further action required. |
| Languages / Frameworks | Go 1.26+. No new languages, frameworks, or scaffold-level dependencies. SQLite (already a permitted dep via signal-crawler precedent) and stdlib `encoding/xml` (or `gofeed` for resilience) are the only additions. |
| Testing | 80% coverage target. `t.Helper()` in helpers. Context-aware DB methods. Build tags `//go:build integration` separate quick unit tests from integration tests against real ES/Redis/SQLite via the existing CI integration harness. |
| Quality gates | `task lint:force`, `task test`, `task drift:check`, `task ports:check`, `task layers:check`, lefthook pre-commit and pre-push without `--no-verify`. Plan adds: `task vuln:alert-crawler`, `GO_SERVICES` entry in `scripts/detect-changed-services.sh`, oneshot skip-list entry in `scripts/deploy.sh`. |
| Review policy | Solo maintainer. Implement-review loop per WP. Mission-level review at end. |
| Performance targets | Inherits NFR-001..009 from spec. No new pipeline-wide targets. |
| Deployment constraints | Docker Compose + `deploy.sh`. Container `north-cloud-alert-crawler`. SQLite migrations use unique numeric prefixes. |

### Resolved charter tension: ES mapping ownership

The charter line "ES mapping changes go through `index-manager`" is interpreted as applying to **shared indices touched by multiple services** (the classifier-managed `*_classified_content` family in particular). Alert-crawler's `community_alerts` index is single-writer (alert-crawler is the only service that writes to it; consumers read but do not mutate the mapping). Therefore alert-crawler manages its own mapping in-service, mirroring the precedent set by `rfp-ingestor`. The `index-manager` service is not invoked.

This deviation is approved by the maintainer (2026-05-06) and recorded under DIRECTIVE_003.

## Locked decisions (Phase 0 outputs applied)

All ambiguities resolved. No `[NEEDS CLARIFICATION]` markers carry forward.

| ID | Decision |
|---|---|
| TC-001 | Acquisition: RSS 2.0 feed at `https://www.safersites.ca/drugalerts.rss` with ETag conditional GETs. HTML scraping deferred. |
| TC-002 | Stable alert ID format: `${source_id}:${url_path_slug}` derived from RSS `<link>` (e.g., `safersites:20260505fentanyl`). |
| TC-003 | Process model: oneshot binary, systemd timer (per `signal-crawler` topology). Manual wiring in `main.go`; no Uber FX. |
| TC-004 | State store: SQLite at `data/state.db` (volume-mounted). Two tables: `poll_checkpoint`, `alert_catalogue`. CGO required. |
| TC-005 | Persistence: ES `community_alerts` index (canonical) + Redis pub/sub channel `community_alerts:lifecycle` (live events). ES write is synchronous; Redis publish is post-write side effect. |
| TC-006 | ES client: raw `net/http.Client` (no Go ES SDK). Mapping owned in `alert-crawler/internal/elasticsearch/mapping.go`. Idempotent `EnsureIndex` HEAD→PUT on startup. |
| TC-007 | Scope vocabulary: direct import of `github.com/jonesrussell/indigenous-taxonomy`. Mission cuts v1.1.0 of that package (additive: Treaty namespace, city regions, sample communities, `ParentRegion` walk function). |
| TC-008 | Sequencing (Q-6): **Plan B**. Alert-crawler develops in parallel via `replace github.com/jonesrussell/indigenous-taxonomy => ../indigenous-taxonomy` directive in `go.mod`. Final WP removes the directive and pins the released v1.1.0 tag. |
| TC-009 | Expiry policy (Q-1): per-source override; default `720h` (30 days from `issued_at`) when upstream omits. |
| TC-010 | Parser-degraded items (Q-2): stale-parse-for-operator-review. `parse_quality: degraded` flag on the document. **Never** auto-rescind on parse failure. Operator decides. |
| TC-011 | First-deploy backfill (Q-3): backfill the 20 most recent feed items so consumers see a populated state immediately. Backfilled items are emitted as `created` lifecycle events. |
| TC-012 | Severity inference (Q-4): configurable heuristic mapping table in `config.yml` (substance presence → severity floor, e.g., `carfentanil` → `critical`, `nitazenes` → `high`, `medetomidine`+opioid → `high`). v1 default ships conservative; operator-tunable. |
| TC-013 | Channel naming (Q-5): `community_alerts:lifecycle`. Single channel, event-typed payloads. |
| TC-014 | Observability: structured metrics from day 1 (do not adopt rfp-ingestor's logs-only posture). Per-poll metric set documented below. |
| TC-015 | Dependency rule: alert-crawler imports only `infrastructure/*` and external Go modules. No imports from peer services. Layer model: L0 `config`/`domain`, L1 `adapter`/`catalogue`/`elasticsearch`/`redis`/`severity`/`scope`/`observability`, L2 `runner`. |

### Cross-repo coordination

Two repos are touched by this mission:

1. `/home/jones/dev/north-cloud` — primary; contains alert-crawler service, infrastructure additions, deployment integration.
2. `/home/jones/dev/indigenous-taxonomy` — sibling; v1.1.0 release with additive vocabulary and `ParentRegion` walk fn.

The Ansible role updates live in `/home/jones/dev/northcloud-ansible/roles/north-cloud/`. These are not part of the same mission but are coordinated via the deployment WP.

## Architecture Overview

```
                                ┌──────────────────────────┐
                                │  safersites.ca/drugalerts │  upstream RSS feed
                                │   (RSS 2.0, ETag-cached)  │
                                └────────────┬──────────────┘
                                             │ HTTPS GET (If-None-Match)
                                             │
                              ┌──────────────▼─────────────┐
                              │      alert-crawler         │  oneshot Go binary
                              │  (north-cloud monorepo)    │  systemd timer
                              ├────────────────────────────┤
                              │  acquire (RSS)             │
                              │  parse + normalize         │
                              │  scope-resolve (taxonomy)  │
                              │  catalogue diff            │
                              │  severity infer (heuristic)│
                              │  write ES                  │
                              │  publish Redis             │
                              └─────┬──────┬──────────┬────┘
                                    │      │          │
                            ┌───────▼──┐  ┌▼─────┐  ┌─▼──────────────┐
                            │ ES index │  │ Redis│  │ SQLite local   │
                            │community │  │ chan │  │ state          │
                            │ _alerts  │  │ comm │  │ (checkpoints,  │
                            │          │  │ unity│  │ catalogue)     │
                            │  CANON   │  │ alerts│  │                │
                            │          │  │ :life│  │ volume-mounted │
                            │          │  │ cycle│  │                │
                            └────┬─────┘  └──┬───┘  └────────────────┘
                                 │           │
                                 │           │
              ┌──────────────────┴───────────┴──────────────────┐
              │             Downstream consumers                │
              │   Minoo (current); SMS / push (future missions)│
              │  Page-load query → ES; live updates ← Redis    │
              └─────────────────────────────────────────────────┘
```

## Project Structure

### Documentation (this feature)

```
kitty-specs/community-alert-pipeline-01KQZC7A/
├── spec.md
├── plan.md                  # this file
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── community-alert.schema.json
│   ├── lifecycle-event.schema.json
│   ├── redis-channels.md
│   └── es-index-mapping.json
├── checklists/
│   └── requirements.md
├── research/
│   ├── evidence-log.csv
│   └── source-register.csv
└── tasks/                   # populated by /spec-kitty.tasks (NOT this command)
```

### Source Code (repository root)

```
/home/jones/dev/north-cloud/alert-crawler/
├── CLAUDE.md            # service-local notes (gotchas, conventions)
├── Dockerfile           # multi-stage Alpine + uid 1000 user
├── Taskfile.yml         # build/run/test/lint/vuln per service
├── config.yml           # default config (leave SetDefaults fields blank)
├── .layers              # 3-tier model
├── go.mod / go.sum      # carries `replace` directive during Phase A+B
├── main.go              # entry point: flags → setup → buildSources → runner → exit
├── cmd/
│   └── backfill/        # one-shot subcommand for first-deploy backfill (Phase C)
├── data/
│   └── .gitkeep
└── internal/
    ├── config/          # L0  Config + SetDefaults; uses infrastructure/config
    ├── domain/          # L0  Alert envelope, Hazard, Source, LifecycleEvent types
    ├── adapter/         # L1  RSS feed client + parser; pluggable per AcquisitionStrategy
    │   └── rss/
    ├── catalogue/       # L1  SQLite poll-checkpoint + active-alert state
    ├── elasticsearch/   # L1  Indexer (raw HTTP) + mapping + EnsureIndex
    ├── redis/           # L1  Lifecycle event publisher
    ├── severity/        # L1  Heuristic severity inference (config-driven table)
    ├── scope/           # L1  Taxonomy resolver (uses indigenous-taxonomy)
    ├── observability/   # L1  Metrics + structured logging glue
    └── runner/          # L2  Poll cycle orchestration; emits to ES + Redis

/home/jones/dev/north-cloud/.layers entry for alert-crawler:
  L0: config, domain
  L1: adapter, adapter/rss, catalogue, elasticsearch, redis, severity, scope, observability
  L2: runner
```

External directories touched (separately scoped, coordinated by this mission):

```
/home/jones/dev/indigenous-taxonomy/                    # Phase A — v1.1.0 release
├── schema/
│   ├── treaties.yaml         # NEW
│   └── regions.yaml          # extended (city + community children)
├── scripts/generate.py       # extended (emit ParentRegion lookup)
└── generated/go/taxonomy/
    ├── treaties.go           # NEW
    ├── regions.go            # regenerated with ParentRegion fn
    └── version.go            # bumped to v1.1.0

/home/jones/dev/northcloud-ansible/roles/north-cloud/   # Phase C
├── templates/
│   ├── alert-crawler.timer.j2     # NEW
│   └── alert-crawler.service.j2   # NEW
└── defaults/main.yml               # NEW: nc_alert_crawler_schedule
```

**Structure Decision**: Single backend service inside the existing Go monorepo. Layout mirrors `signal-crawler` (oneshot topology) with the internal package model adapted for ES + Redis (cf. `rfp-ingestor`). The mission also extends a sibling Go module (`indigenous-taxonomy`) and updates an external Ansible repo; both are coordinated via WP sequencing rather than absorbed into this directory.

## Component Design (informative)

### Acquisition (`internal/adapter/rss`)

- `Client.Fetch(ctx, source)` issues `GET feed_url` with `If-None-Match: <stored ETag>` and `If-Modified-Since: <stored Last-Modified>` (when present).
- On `304`: returns sentinel `ErrNotModified`. No further work for this source this cycle.
- On `200`: parses RSS (stdlib `encoding/xml` plus a minimal feed struct, OR `gofeed` if resilience demands; choose during scaffolding).
- Returns `[]FeedItem` and the new `ETag` / `Last-Modified` for checkpoint update.
- Error policy: transient errors increment `consecutive_failures`; non-transient errors (e.g., feed format break) fall through with a parse-failure metric.

### Catalogue (`internal/catalogue`)

- SQLite store with two tables (see data-model.md §6 and §7).
- `LoadCheckpoint(source_id) (PollCheckpoint, error)`
- `SaveCheckpoint(PollCheckpoint) error`
- `LookupAlert(alert_id) (CatalogEntry, error)`
- `MarkSeen(alert_id, content_hash) error`
- `RescindAbsent(source_id, poll_started_at) ([]alert_id, error)` — returns alert IDs whose `last_seen_at < poll_started_at` and `is_active = true`.
- Catalogue is rebuildable from ES on startup if missing (idempotent recovery; queries `lifecycle_state == active`).

### Elasticsearch indexer (`internal/elasticsearch`)

- `Indexer.EnsureIndex(ctx)` — HEAD `/{index}`; if `404`, PUT mapping (see `contracts/es-index-mapping.json`).
- `Indexer.Index(ctx, alert)` — single doc PUT, deterministic `_id = alert.id`.
- `Indexer.MarkRescinded(ctx, alert_id, ts)` — partial update via `POST /{index}/_update/{id}` setting `lifecycle_state = rescinded`, `rescinded_at`, plus appended `revision_history` entry.
- No bulk path in v1 (alert volume is low). Retain a `BulkIndex` constructor for the backfill subcommand only.

### Redis publisher (`internal/redis`)

- `Publisher.Publish(ctx, event LifecycleEvent)` writes a JSON-encoded payload to channel `community_alerts:lifecycle` via the existing `infrastructure/redis` client.
- Publish failures are logged and incremented as a metric; **do not** roll back the ES write. ES is canonical.
- Ordering: best-effort (Redis pub/sub does not guarantee ordering). Consumers MUST handle re-fetching the canonical state from ES if event order is ambiguous.

### Observability (`internal/observability`)

Per-poll structured metrics emitted at `INFO` level (consumed by Loki/Grafana via existing log pipeline; no new Prometheus exporter for v1):

| Metric | Type | Labels |
|---|---|---|
| `alert_crawler.poll.duration_ms` | gauge | `source_id`, `result=ok|not_modified|error` |
| `alert_crawler.poll.feed_items_total` | counter | `source_id` |
| `alert_crawler.alert.created_total` | counter | `source_id`, `category`, `severity` |
| `alert_crawler.alert.updated_total` | counter | `source_id`, `category`, `severity` |
| `alert_crawler.alert.rescinded_total` | counter | `source_id` |
| `alert_crawler.parse.failure_total` | counter | `source_id`, `failure_kind` |
| `alert_crawler.consecutive_failures` | gauge | `source_id` |
| `alert_crawler.es.write_failure_total` | counter | `source_id`, `op=create|update|rescind` |
| `alert_crawler.redis.publish_failure_total` | counter | `source_id`, `event_type` |

A v1.5 follow-up may add a Prometheus `/metrics` endpoint if operations demands it. Out of scope here.

## Phased Build Sequence

Mission decomposes into four phases. **Phases A and B execute in parallel.** Phase C reconciles. Phase D is acceptance verification.

### Phase A — Indigenous-taxonomy v1.1.0 (sibling repo)

Lives in `/home/jones/dev/indigenous-taxonomy`. Released as a tagged version pre-merge of the alert-crawler PR.

- **A.1**: Add `schema/treaties.yaml` with 11 treaty entries (`treaty:1` … `treaty:11`), plus member-region metadata.
- **A.2**: Extend `schema/regions.yaml` with city children under existing province nodes (`canada:manitoba:winnipeg`, `canada:ontario:toronto`, `canada:ontario:ottawa`, `canada:british-columbia:vancouver`, `canada:alberta:calgary`, `canada:saskatchewan:saskatoon`) and one sample First Nations community per active treaty area to establish the pattern.
- **A.3**: Extend `scripts/generate.py` to emit a `ParentRegion(Region) (Region, bool)` lookup table from the existing YAML `children:` keys.
- **A.4**: Generate `taxonomy/treaties.go` with `Treaty` type alias, 11 constants, `AllTreaties []Treaty`, `IsValidTreaty(string) bool`.
- **A.5**: Update `taxonomy/version.go` to `v1.1.0`. Update `go.mod`. Audit `tests/` for hardcoded counter assertions (RR-004).
- **A.6**: Open PR; tag `v1.1.0` after merge.

### Phase B — alert-crawler scaffolding (north-cloud monorepo)

Develops in parallel with Phase A. Pins taxonomy via `replace` directive until A.6 lands.

- **B.1**: Service scaffold. `alert-crawler/{main.go, config.yml, Taskfile.yml, Dockerfile, .layers, go.mod, go.sum}`. Add `replace github.com/jonesrussell/indigenous-taxonomy => ../indigenous-taxonomy` to `go.mod` for parallel development. Layer file. Empty internal package skeletons.
- **B.2**: Domain types. `internal/domain/{alert.go, hazard.go, source.go, lifecycle_event.go}`. Implements the envelope per spec §3 / data-model §1-§4.
- **B.3**: Config. `internal/config/{config.go, defaults.go}`. Uses `infrastructure/config.LoadWithDefaults[T]`. Pitfall protection: unit test that asserts `SetDefaults`-only fields are absent from `config.yml` (mitigates RR-007).
- **B.4**: Acquisition (RSS adapter). `internal/adapter/rss/{client.go, parser.go}`. Includes ETag conditional GET, RSS XML parse, `<description>` HTML-to-fields extractor. Defensive: any unparseable item is recorded as `parse_quality: degraded` with the raw `<description>` preserved.
- **B.5**: Catalogue. `internal/catalogue/{store.go, schema.sql, migrations/}`. SQLite-backed. Migrations use unique numeric prefixes (NC convention). `t.Helper()` everywhere.
- **B.6**: Elasticsearch indexer. `internal/elasticsearch/{indexer.go, mapping.go}`. Raw `net/http.Client`. `EnsureIndex` HEAD→PUT. Single-doc `Index`. Partial-update `MarkRescinded`.
- **B.7**: Redis publisher. `internal/redis/publisher.go`. Wraps `infrastructure/redis`. Serializes `LifecycleEvent` to JSON. Channel `community_alerts:lifecycle`.
- **B.8**: Severity inference. `internal/severity/{table.go, infer.go}`. Config-driven mapping (substance presence → severity floor). Default table in `config.yml` is conservative (`carfentanil` → `critical`, etc.).
- **B.9**: Scope resolver. `internal/scope/resolver.go`. Imports `indigenous-taxonomy/generated/go/taxonomy`. Default-scope expansion: per-source default tokens + tokens inferred from upstream content (location names → city slugs).
- **B.10**: Runner. `internal/runner/runner.go`. Orchestrates one poll cycle: load checkpoint → fetch → parse → diff catalogue → infer scope/severity → write ES → publish Redis → save checkpoint. L2 only; depends on L0/L1.
- **B.11**: Observability glue. `internal/observability/metrics.go`. Structured-log metric emission per the table above.
- **B.12**: Wiring. `main.go` phase order: flags → setup (config, logger, sqlite, ES, redis) → buildSources → runner → exit. Mirrors signal-crawler's pattern.
- **B.13**: Unit tests. ≥80% coverage. `t.Helper()` in test helpers. Build tag `//go:build !integration` for unit tests.
- **B.14**: Integration tests. `//go:build integration`. Real ES, real Redis, real SQLite. Cover AS-01..AS-06 acceptance scenarios. Run via the existing CI integration harness.
- **B.15**: Backfill subcommand. `cmd/backfill/main.go`. One-shot mode that pulls the 20 most recent items and emits them as `created` events. Idempotent.

### Phase C — Reconciliation & deployment integration

Begins after **A.6** lands and Phase B is feature-complete.

- **C.1**: Pin taxonomy. Remove `replace` directive from `alert-crawler/go.mod`. Pin to `github.com/jonesrussell/indigenous-taxonomy v1.1.0`. `go mod tidy`.
- **C.2**: Docker Compose entry. Add `alert-crawler` service to `docker-compose.base.yml` with `restart: "no"`, named volume `alert-crawler-data:/app/data`, no health check (oneshot pattern).
- **C.3**: Deploy integration. Add to oneshot skip list in `scripts/deploy.sh`. Add to `GO_SERVICES` in `scripts/detect-changed-services.sh`. Add `vuln:alert-crawler` delegation to root `Taskfile.yml`.
- **C.4**: Ansible role. Add `roles/north-cloud/templates/alert-crawler.timer.j2` (`OnCalendar=*-*-* *:30:00 UTC`, `Persistent=true`, `RandomizedDelaySec=120`) and `alert-crawler.service.j2` (`Type=oneshot`, `ExecStartPre` image pull, `ExecStart` `docker compose ... run --rm alert-crawler`). Default cadence variable `nc_alert_crawler_schedule` in `roles/north-cloud/defaults/main.yml`. Owner `1000:1000` on data dir (RR caveat: container uid 1000 vs host deploy_user uid 1001).
- **C.5**: Drift integration. `task drift:check` mapping for `docs/specs/community-alert-pipeline.md` (or pointer to mission spec). `task ports:check` regenerated for new compose entry.
- **C.6**: Service CLAUDE.md. `alert-crawler/CLAUDE.md` documenting gotchas: the `config.yml`-overrides-`SetDefaults` pitfall, the in-service ES mapping ownership rationale, the `replace` directive that **must** be removed before merge, the rescission semantics on parser-degraded items, the SQLite catalogue rebuild path.
- **C.7**: Repo-root CLAUDE.md update. Add `alert-crawler/**` to the orchestration table.

### Phase D — Acceptance & merge readiness

- **D.1**: Acceptance scenario verification. Each AS-01..AS-06 mapped to an integration test with explicit acceptance criteria.
- **D.2**: NFR validation harness. Latency (NFR-001), idempotency (NFR-006), blast-radius isolation (NFR-009) verified via the integration suite.
- **D.3**: First-deploy backfill rehearsal. Run backfill subcommand against staging or a mocked-ES sandbox; verify 20 items result in 20 `created` events.
- **D.4**: Pre-merge checklist (per spec C-008): `task lint:force` clean, `task test` clean, `task drift:check` clean, `task ports:check` clean, `task layers:check` clean, lefthook hooks pass.
- **D.5**: Post-merge follow-on issue: file the deferred "Migrate classifier and publisher to import indigenous-taxonomy" backlog item.

## NFR Traceability

| NFR | Verification approach |
|---|---|
| NFR-001 (latency 95%/60min, 99%/120min) | D.2 latency harness: synthetic upstream publish; assert end-to-end visibility in ES under thresholds |
| NFR-002 (durable read 99.5%/2s) | Inherited from existing ES SLO; no extra mission work |
| NFR-003 (live event delivery 99%/5s) | Integration test: subscribe; publish; assert receipt within 5s |
| NFR-004 (subscriber recovery) | Integration test: simulate disconnect, reconnect, query ES, assert full active set retrieved |
| NFR-005 (source-unreachable handling) | Unit test: 6th consecutive failure emits operator-actionable signal; other sources unaffected |
| NFR-006 (idempotent ingestion) | Integration test: replay 100 cycles of identical content; assert 0 lifecycle events after the first `created` |
| NFR-007 (≥80% coverage) | `task test:alert-crawler -- -coverpkg=./internal/...` enforced in CI |
| NFR-008 (per-poll metrics) | Unit test: each poll cycle emits the metric set documented above; missing metric fails the test |
| NFR-009 (blast-radius isolation) | D.2: synthetic alert-crawler crash loop; assert lead pipeline cycle time and throughput within baseline |

## Risks (rollup of research RR-001..007 + plan-phase additions)

| ID | Risk | Mitigation |
|---|---|---|
| RR-001 | Cloudflare may block RSS feed access | Monitor; document Playwright fallback; deferred until needed |
| RR-002 | NationBuilder template change breaks `<description>` parser | Defensive parsing; `parse_quality: degraded` flag; never auto-rescind on parse failure (TC-010) |
| RR-003 | RSS pagination behaviour unconfirmed; backfill window may be 20 items only | Plan Phase D.3 verifies `?page=N`; v1 contract is "rolling-20 historical window" if pagination unsupported |
| RR-004 | Taxonomy package tests may have hardcoded counter assertions | Phase A.5 audit |
| RR-005 | Aspirational `indigenous-taxonomy` import mandate is unenforced; alert-crawler is the first NC service to import — exposing latent integration issues | Plan Phase B.1 includes a small import-spike before the rest of B proceeds |
| RR-006 | Deriving `expires_at` server-side may diverge from issuing-org intent | TC-009 makes per-source override configurable; v1.5 may revisit |
| RR-007 | `config.yml`-overrides-`SetDefaults` pitfall regresses defaults silently | B.3 includes a unit test asserting `SetDefaults`-only fields are absent from `config.yml` |
| PR-001 | Coordination drift between Phase A (sibling repo) and Phase B (`replace` directive) | Phase C.1 reconciliation WP gates merge; CI in north-cloud will fail to build if `replace` is left in `go.mod` post-pin |
| PR-002 | First-deploy backfill emits 20 `created` events in a burst, potentially overwhelming Minoo on first connect | Backfill subcommand is opt-in; D.3 rehearses; consumer side rate-limits if needed (out of mission scope) |
| PR-003 | Severity heuristic table drifts from issuing-org practice | Heuristic table is config-driven (TC-012); document review cadence in service CLAUDE.md |
| PR-004 | SQLite volume permissions (uid 1000 vs deploy_user uid 1001) cause container startup failures | Phase C.4 Ansible task explicitly sets `owner: "1000"` on data dir |

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| In-service ES mapping ownership (deviates from literal "ES mapping changes go through index-manager") | `community_alerts` is single-writer; alert-crawler is the only producer; no shared-mapping coordination cost. Mirrors precedent set by `rfp-ingestor`. | Routing through `index-manager` adds an inter-service dependency for zero mapping-coordination benefit; introduces a deploy-order coupling that does not exist in the rfp-ingestor reference. |
| Cross-repo coordination via `replace` directive (Phase A + Phase B parallel) | Serializing taxonomy → alert-crawler doubles wall-clock time on the critical path. | Strict sequential (taxonomy v1.1.0 first, then alert-crawler) was rejected because the parallelism is genuine and Phase C.1 reconciliation cleanly removes the `replace` before merge. |
| Charter exception for net-new service | Issuing organization (Manitoba Harm Reduction Network) has stated their primary distribution channel suppresses these alerts; a strategic addition closes an active distribution gap. | Filing as a backlog item first was rejected: backlog triage already complete (per Phase 3 milestone status); the maintainer authorized the exception and the spec/plan flow through specify→plan→tasks→implement→review preserves discipline. |

## Open items deferred to `/spec-kitty.tasks`

The plan above sketches the WP outline (B.1..B.15, C.1..C.7, etc.). The `/spec-kitty.tasks` step will:

- Size each WP for an implement-review loop iteration (target: 1-3 hours of work per WP)
- Define explicit WP acceptance criteria, dependencies, and lane assignments
- Decompose any WP that is too large for a single implement-review cycle
- Identify any WP that is genuinely parallelizable across lanes (worktrees)
- Surface any WP that blocks others and must be a critical-path entry

No additional research is required for tasks generation; this plan is grounded in research findings.

## Branch contract restated (per workflow requirement)

Authoritative output of `spec-kitty agent mission setup-plan --json` (re-asserted):

- **Current branch**: `main`
- **Planning/base branch**: `main`
- **Merge target**: `main`
- **`branch_matches_target`**: `true`

Completed mission changes will merge into `main`. Worktrees for implementation are created at `/spec-kitty.implement` time, one per lane.

## Next command

`/spec-kitty.tasks` (user must invoke; do not auto-run).

This plan command stops here per workflow discipline. Tasks generation will produce work-package files under `kitty-specs/community-alert-pipeline-01KQZC7A/tasks/` and a `tasks-manifest.yaml` mapping WPs to lanes.
