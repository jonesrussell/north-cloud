# Implementation Plan: Signal Producer Pipeline

**Branch**: `main` (planning base) | **Date**: 2026-04-27 | **Spec**: [spec.md](spec.md)
**Mission**: `signal-producer-pipeline-01KQ6QZS` (mid8: `01KQ6QZS`)

## Summary

Build a new top-level Go service `signal-producer/` that runs every 15 minutes via systemd, queries Elasticsearch `*_classified_content` indexes for high-quality recent hits (rfp, need_signal), maps them to the Waaseyaa signal payload format, batches them in groups of 50, and POSTs to `${WAASEYAA_URL}/api/signals` with retry. Persists a checkpoint file between runs so signals are produced exactly once. Architectural shape: three internal packages (`producer` with main loop + checkpoint, `mapper`, `client`) under `signal-producer/cmd/main.go`, deployed as a host-level systemd unit (not a docker-compose service in this mission).

This is a greenfield service. No existing code is modified. No cross-service imports. The only allowed shared touch is `infrastructure/` (config and logger).

## Technical Context

| Aspect              | Value                                                                                                                                                                                                                                                                                          |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Language / runtime  | Go 1.26+ (matches existing services). One per-service module: `signal-producer/go.mod`. `GOWORK=off` per service.                                                                                                                                                                            |
| Service shape       | One-shot CLI binary. `Type=oneshot` systemd unit. NOT a daemon, NOT a docker-compose service.                                                                                                                                                                                                |
| Internal packages   | `internal/producer` (main loop + checkpoint load/save), `internal/mapper` (ES hit → Signal struct), `internal/client` (Waaseyaa HTTP POST + retry).                                                                                                                                            |
| Config              | `signal-producer/config.yml` + env overrides. Loaded via `infrastructure/config/` so the `os.Getenv` ban is honored. Env vars: `WAASEYAA_URL`, `WAASEYAA_API_KEY`, `ES_URL` (only allowed in `cmd/main.go`).                                                                                  |
| Logging             | `infrastructure/logger` (JSON output). C-003 of the spec explicitly overrides issue #592's `zap` mention. Structured fields per FR-014: `batch_size`, `ingested`, `skipped`, `errors`, `duration_ms`.                                                                                          |
| ES client           | The official `github.com/elastic/go-elasticsearch/v8` client (already in use by `classifier/`, `index-manager/`). Reuse the existing wrapper if one exists in `infrastructure/`; otherwise instantiate directly. Investigate during research.                                                  |
| HTTP client         | `net/http` standard library + a small retry helper. No new dependencies needed; if a retry helper already exists in `infrastructure/`, reuse it.                                                                                                                                              |
| Persistence         | A single JSON file at `/var/lib/signal-producer/checkpoint.json`. Atomic writes via temp + rename. Permissions 0640 (C-010).                                                                                                                                                                  |
| External contract   | `POST https://northops.ca/api/signals` with `X-Api-Key` header. Body `{"signals":[...]}`. Response `IngestResult{ingested, skipped, leads_created, leads_matched, unmatched}`. Treated as authoritative external contract owned by Waaseyaa (C-008).                                          |
| Deployment          | Host-level systemd unit on the existing North Cloud production VPS. Binary deployed via the standard `deploy.sh` GitHub Actions pipeline. Systemd timer fires every 15 minutes (`OnCalendar=*:0/15`, `Persistent=true`). Unit type `oneshot`.                                                  |
| Test approach       | Per the charter testing-tier clause: unit tests fast with fakes (mock HTTP server, in-memory ES round-trip stub), integration test against real ES via the CI integration harness. Build tags separate quick vs slow suites. ≥ 80% coverage per package.                                       |
| Security posture    | API key in `EnvironmentFile=` (root-owned, 0600) read by systemd; never logged. Checkpoint file 0640 owned by service user. Run as a dedicated `signal-producer` user, not root.                                                                                                              |
| Build / Taskfile    | `signal-producer/Taskfile.yml` exposes `build`, `test`, `lint`, `vuln`. Root `Taskfile.yml` adds delegations: `build:signal-producer`, `test:signal-producer`, `lint:signal-producer`, `vuln:signal-producer`. Add `signal-producer` to `GO_SERVICES` in `scripts/detect-changed-services.sh`. |
| Spec / drift        | Add a brief service section to `docs/specs/lead-pipeline.md` describing producer scope and the Waaseyaa contract dependency, so `task drift:check` accepts code changes under `signal-producer/`. Verify with `task drift:check`.                                                              |

## Charter Check

| Charter dimension     | Status                                                                                                                                                                                                                                              |
| --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Languages/Frameworks  | ✅ Go 1.26+; new top-level service consistent with existing pattern. No new framework introduced.                                                                                                                                                  |
| Service independence  | ✅ Producer imports only from `infrastructure/` (config, logger). No imports from `crawler/`, `classifier/`, etc.                                                                                                                                  |
| Testing requirements  | ✅ ≥ 80% coverage per internal package. Unit tier with fakes; one real-ES integration test in CI. Build tags split quick vs slow.                                                                                                                  |
| Quality gates         | ✅ All gates apply: `task lint:force`, `task test`, `task drift:check`, `task ports:check` (no new ports), `task layers:check` (single-service `.layers` file), lefthook hooks pass. No `--no-verify`.                                              |
| Logging               | ✅ `infrastructure/logger` mandated by spec C-003. Issue #592's `zap` mention is overridden.                                                                                                                                                       |
| Config                | ✅ `infrastructure/config/` for all env reads outside `cmd/main.go`. C-002 / C-004.                                                                                                                                                                |
| Review policy         | ✅ Solo maintainer + agent reviewer model. Each WP runs the implement-review loop. Deployment WP requires post-merge real-VPS verification.                                                                                                        |
| Performance targets   | ✅ NFR-001 (5 min p95 for 1000 signals) within charter expectations.                                                                                                                                                                              |
| Deployment            | ✅ Host-level systemd unit on existing VPS. Production is not a git repo — deploy via tarball from CI per existing pipeline. Health-check skip-list update: producer is `Type=oneshot` and not subject to docker-compose health checks.            |
| Risk boundaries       | ✅ NC owns the producer; Waaseyaa owns the receiver. C-008 makes that explicit. No reach into Minoo / Waaseyaa / consuming editorial apps.                                                                                                         |
| Bypass-eligibility    | ⚠ Mission is explicitly NOT bypass-eligible: new top-level service, deployment, schema-shaped wire format. Full Spec Kitty loop required. Triage report agreed.                                                                                    |

No charter conflicts. Charter Check **passes**.

## Phase 0 — Outline & Research

The spec contains zero `[NEEDS CLARIFICATION]` markers. The research phase resolves the *engineering* unknowns surfaced during planning, not policy unknowns. Output goes to `research.md`. Topics:

1. **ES client choice and reuse**. Does `infrastructure/` already expose an ES client wrapper, or do we instantiate `go-elasticsearch/v8` directly? Decision rationale, plus alternatives.
2. **Retry helper reuse**. Is there an existing retry helper under `infrastructure/` we can use, or do we write a small one in `internal/client`? Avoid reinventing the wheel; avoid adding it to `infrastructure/` if only one service needs it.
3. **Checkpoint package boundary**. Issues #595 puts checkpoint inside `internal/producer`; #592 directory structure agrees. Stick with the issues' structure (one package, two files: `producer.go`, `checkpoint.go`) rather than splitting into a separate `internal/checkpoint` package. Document the decision.
4. **Bootstrap shape for one-shot binaries**. The existing `internal/bootstrap/` pattern is for HTTP services. A one-shot binary needs config + logger but not a server lifecycle. Decision: simpler `main.go` like `auth/` and `search/` use, not the phased bootstrap pattern.
5. **Tests against real ES — fixture data shape**. Integration test seeds a couple of `*_classified_content` documents into a test index, runs the producer, captures the HTTP request to the test server, asserts the wire format. Document fixture schema decisions.
6. **Wire-format authority for the Waaseyaa contract**. Treat the contract from issue #594 as authoritative; capture it in `contracts/signals-post.yaml` (a small OpenAPI fragment) so the test server validates against the same schema the production code targets.
7. **Source-down alert delivery**. FR-019 requires "operator-actionable alert" but the spec is technology-agnostic. Decision: emit a structured WARN log with a stable code (e.g., `signal_producer.source_down`) and add a journald alert grep recipe to `docs/RUNBOOK.md`. Avoid building an alerting integration in this mission.

## Phase 1 — Design & Contracts

### Data model

Captured in `data-model.md`. The model has four logical types:

- **Checkpoint** — JSON file shape and atomic write contract.
- **ESHit** — the structurally relevant fields the mapper consumes (and therefore the integration test must produce).
- **Signal** — the Waaseyaa wire format.
- **IngestResult** — the Waaseyaa response.

State transitions (only one): a checkpoint advances from `t0` to `t1` only when *all* batches in a run delivered with HTTP 2xx. Partial success → no advance.

### Contracts

`contracts/signals-post.yaml` — a small OpenAPI 3 fragment for `POST /api/signals` that the test server uses for response shape and that the client unit tests assert against. Treats the Waaseyaa side as authoritative; this is a *consumer contract*.

### Quickstart

`quickstart.md` — operator guide for: building locally, running with a fixture ES, smoke-testing against a mock Waaseyaa, deploying via the CI pipeline, reading journald, and triaging the source-down alert.

## Execution Approach

The mission breaks naturally into 6 work packages along the layered design and respecting the dependency graph:

1. **Service scaffolding** — `signal-producer/` directory, `go.mod`, `Taskfile.yml`, root `Taskfile.yml` delegations, `.layers`, `config.yml.example`, empty `cmd/main.go`, CLAUDE.md, `GO_SERVICES` update. Foundation for parallel work below.
2. **`internal/producer` (checkpoint)** — Just the checkpoint code (`Checkpoint`, `LoadCheckpoint`, `SaveCheckpoint`, atomic write). Defers the main loop to WP05. Independent of WP03/WP04.
3. **`internal/mapper`** — `MapHit(hit) (Signal, error)` for both rfp and need_signal types, plus `external_id` prefix logic. Independent of WP02/WP04.
4. **`internal/client`** — `WaaseyaaClient` with retry, context cancellation, `IngestResult` parsing. Test against `httptest.Server`. Independent of WP02/WP03.
5. **Producer main loop + binary** — Wires checkpoint, ES query, mapper, batch dispatcher, client. `cmd/main.go` resolves config, builds the dependency graph, runs once, exits. Integration test against real ES + test HTTP server. Depends on WP01–WP04.
6. **Deployment** — systemd unit + timer files, `EnvironmentFile=` template, deploy-pipeline updates (tarball includes binary, health-check skip-list, migration prefix not needed). Update `docs/specs/lead-pipeline.md` for drift acceptance and `docs/RUNBOOK.md` for source-down triage. Depends on WP05.

### Parallelism

WP02, WP03, and WP04 are independent of each other and can land in parallel (three lanes). WP01 must precede them all. WP05 must wait for WP02–WP04. WP06 must wait for WP05.

```
WP01 (scaffolding)
  ├── WP02 (checkpoint)        ┐
  ├── WP03 (mapper)            ├── WP05 (producer + binary) ─── WP06 (deploy)
  └── WP04 (HTTP client)       ┘
```

## Risks (Premortem)

| Risk                                                                                                         | Likelihood | Impact | Mitigation                                                                                                                                                            |
| ------------------------------------------------------------------------------------------------------------ | ---------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Waaseyaa contract differs at first prod run (header name, response shape, status code semantics).            | Medium     | High   | `contracts/signals-post.yaml` captures the documented shape; an integration test asserts client behavior against it. If real Waaseyaa diverges → follow-up issue.    |
| ES client wrapper conventions differ subtly between services and we pick the wrong one.                      | Medium     | Med    | Research D1 surveys existing usage in `classifier/`, `index-manager/`, `rfp-ingestor/`. Match the most-used pattern.                                                 |
| Checkpoint file ownership wrong on first deploy → permission denied → run fails silently in journald.       | Medium     | Med    | Deploy WP creates `/var/lib/signal-producer/` via systemd `StateDirectory=signal-producer`, which gets correct ownership automatically.                              |
| Systemd timer fires while previous run still active (slow ES, slow Waaseyaa).                                | Low        | Low    | `Type=oneshot` + default systemd behavior prevents overlap. Documented in unit file.                                                                                 |
| Integration test flaky against real ES.                                                                      | Medium     | Med    | Test seeds fixtures into a uniquely-named index, asserts on document IDs (not match counts), and uses `index-manager`-style isolation per test.                      |
| Adding a new service requires Taskfile / CI / drift / layer infrastructure changes that get missed.          | Medium     | Med    | WP01 (scaffolding) bundles all these touchpoints in one place. Reviewer guidance for WP01 explicitly checks each.                                                    |
| Deploy pipeline doesn't pick up systemd unit changes (rsync ownership, stale systemd cache).                 | Medium     | High   | Deploy WP includes `systemctl daemon-reload`, `systemctl enable --now`, and post-deploy health verification (`systemctl is-active`, journald sanity check).         |
| Production VPS lacks `/var/lib/signal-producer/` parent directory or the dedicated user.                     | Medium     | Med    | systemd `StateDirectory=` + `User=` + `Group=` create them as a side effect. Verify in deploy WP via `id signal-producer` and `ls -la /var/lib/signal-producer/`.    |

## Stop Point

Phase 1 complete. Artifacts: `plan.md`, `research.md`, `data-model.md`, `contracts/signals-post.yaml`, `quickstart.md`. No bulk-edit map (not a bulk edit).

Next command: `/spec-kitty.tasks`. Branch contract for that command: planning base = `main`, merge target = `main`, matches=true.
