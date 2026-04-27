# Research: Signal Producer Pipeline

**Mission**: `signal-producer-pipeline-01KQ6QZS`
**Date**: 2026-04-27

## Open Clarifications

None. The spec contains zero `[NEEDS CLARIFICATION]` markers. The five source issues plus the spec resolved every policy decision before planning. The decisions below are *engineering* choices made during planning.

## Engineering Decisions

### D1 — ES client: reuse `infrastructure/` wrapper if present, else `go-elasticsearch/v8` directly

**Decision**: First WP01 (scaffolding) task: survey `infrastructure/` and the most-used service (`classifier/`) for an ES client convention. If a wrapper exists in `infrastructure/elasticsearch/` (or similar) that other services use, reuse it. Otherwise instantiate `github.com/elastic/go-elasticsearch/v8` directly in `internal/producer/producer.go`.

**Rationale**: Re-implementing an ES client wrapper inside `signal-producer/` would violate the no-cross-service-imports rule indirectly (by re-creating the same patterns) and would diverge from how `classifier/` and `index-manager/` already query ES. Direct use of `go-elasticsearch/v8` is acceptable because it's a public Go module, not a service.

**Alternatives considered**:
- Build a brand-new abstraction in `infrastructure/elasticsearch/`: rejected — premature abstraction; only this service has the specific query shape.
- Copy code from `classifier/internal/elasticsearch/`: rejected — that's a cross-service import in spirit if not in form. Either it's worth promoting to `infrastructure/` (separate mission) or this service uses the upstream library.

### D2 — Retry helper: in-package, not in `infrastructure/`

**Decision**: Implement a small retry helper inside `internal/client/retry.go` (≤ 50 lines). Do not promote it to `infrastructure/`.

**Rationale**: Per the charter `infrastructure/` rule, shared code lives there. A retry helper used by exactly one service is not shared. Promoting it speculatively would violate the YAGNI principle and require coordinated changes across services if signatures evolve. If a second service needs the same helper later, the second-mover promotes it.

**Alternatives considered**:
- Use `github.com/cenkalti/backoff/v4` or similar: rejected — adds a dependency for ≤ 50 lines of behavior that we control fully.
- Promote to `infrastructure/retry`: rejected — premature.

### D3 — Checkpoint stays inside `internal/producer`, not its own package

**Decision**: Keep checkpoint code as `internal/producer/checkpoint.go` next to `producer.go`, matching what issues #595 and #592 specify. Two files in one package, single responsibility per file.

**Rationale**: Issue contracts. The 80% coverage NFR is per-package, not per-file; one package with two files is easier to test cohesively (the producer's run loop will use the same package's checkpoint helpers without an extra import). Splitting into `internal/checkpoint/` would force a third package without compensating clarity.

**Alternatives considered**:
- Separate `internal/checkpoint/` package: rejected — adds an import edge with no decoupling benefit.
- Package the checkpoint inside `cmd/main.go`: rejected — main shouldn't own persistence logic; not testable in isolation.

### D4 — Bootstrap shape: simple `cmd/main.go`, not the phased `internal/bootstrap/`

**Decision**: Wire dependencies directly in `cmd/main.go` like `auth/` and `search/` do. Do not introduce an `internal/bootstrap/` package.

**Rationale**: The phased `internal/bootstrap/` pattern (Profiling → Config → Logger → Database → Services → Server → Lifecycle) exists for HTTP services with long-running concerns. The signal producer is one-shot: it has Config, Logger, three internal packages, and exits. A phased bootstrap would be ceremonial. The root CLAUDE.md explicitly says "simple services (auth, search) use helper functions in main.go".

**Alternatives considered**:
- Phased bootstrap: rejected — the producer has no server lifecycle or background workers.
- Skip `cmd/` and put `main` at the top level: rejected — breaks the per-service convention everywhere else in the monorepo.

### D5 — Integration-test fixture strategy

**Decision**: The integration test (NFR-004) seeds two fixture documents — one rfp, one need_signal — into a uniquely-named test index using the existing test-utils pattern from `classifier/`. The producer is run with config pointing at that index; a `httptest.Server` captures the POST and asserts on the wire format. Fixtures use realistic but minimal field sets (only what the mapper consumes plus one unused field to verify field selection is intentional).

**Rationale**: Per the charter testing-tier clause, integration tests that cross service boundaries (here: real ES) must hit real infrastructure. The HTTP target may be a test server because the Waaseyaa side is owned by a different repo. Unique index name avoids parallel-test pollution.

**Alternatives considered**:
- Mock ES with `httptest.Server` returning canned responses: rejected — defeats the integration-test purpose; the real concern is the ES query DSL behavior under our document shape, which only a real ES instance proves.
- Use a docker-composed Waaseyaa for end-to-end: rejected — out of scope; Waaseyaa isn't in this repo.
- Skip integration test: rejected — NFR-004 is explicit.

### D6 — Waaseyaa contract: capture as `contracts/signals-post.yaml`

**Decision**: Write a small OpenAPI 3 fragment for `POST /api/signals` covering request body, headers, response body shape, and the documented status codes. The client unit test asserts payload conformance to this spec; the test HTTP server returns responses that match this spec.

**Rationale**: The contract crosses a repo boundary (Waaseyaa side). Pinning it to a file in this repo gives a single source of truth. If Waaseyaa diverges, the contract file is the diff — easy to inspect, easy to update.

**Alternatives considered**:
- Inline JSON examples in test code: rejected — duplicated, drift-prone.
- Pull contract from Waaseyaa repo at build time: rejected — couples build to an external repo, fragile.

### D7 — Source-down alerting via WARN log + journald grep, not an integration

**Decision**: When 3 consecutive runs return zero ingested signals, emit a single structured WARN log line with stable code `signal_producer.source_down`, including the consecutive-zero count and the time-since-last-success. Document the journald grep recipe in `docs/RUNBOOK.md` and a sample `journalctl -u signal-producer -p warning` invocation. No external alert integration in this mission.

**Rationale**: FR-019 requires "operator-actionable alert" — alertable does not mean integrated. A structured WARN with a stable code is operator-actionable: the operator sees it in journald and can also wire it to any alert pipeline they already run. Building a fresh alert integration here would expand scope beyond what's needed to verify the producer.

**Alternatives considered**:
- HTTP webhook to a Slack/PagerDuty endpoint: rejected — needs credentials, retry, idempotency. Out of scope.
- Email via local MTA: rejected — historically unreliable on this VPS.
- Skip the alert entirely: rejected — FR-019 is explicit.

### D8 — Service user and `StateDirectory=`

**Decision**: Run the binary as a dedicated `signal-producer` user (created in the deploy WP via systemd `User=` directive). Use systemd's `StateDirectory=signal-producer` to create `/var/lib/signal-producer/` with the correct ownership (0750) automatically. Checkpoint file gets 0640 via the producer's own write logic (umask is not enough).

**Rationale**: Avoids manual user-creation steps in deploy.sh. `StateDirectory=` is the canonical systemd idiom; respects security defaults.

**Alternatives considered**:
- Run as the existing `deployer` user: rejected — too privileged; mixes blast radius.
- Run as root: rejected — unnecessary.
- Use Ansible to pre-provision the user/dir: rejected — adds a cross-repo dependency for a one-time setup that systemd handles natively.

## References Consulted

- Issues #592, #593, #594, #595, #599 (snapshot.json from the triage mission).
- Root `CLAUDE.md` (bootstrap pattern, code conventions, troubleshooting).
- `ARCHITECTURE.md` (service boundaries, layer rules).
- `docs/specs/lead-pipeline.md` (where the producer's spec section will land for drift acceptance).
- `classifier/cmd/`, `classifier/internal/elasticsearch/`, `auth/cmd/`, `search/cmd/` (existing patterns for ES clients and main.go bootstrap shape).
- Charter `infrastructure_only` rule, testing-tier clause, review-policy clause.
