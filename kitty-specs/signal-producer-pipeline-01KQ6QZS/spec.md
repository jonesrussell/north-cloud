# Specification: Signal Producer Pipeline

**Mission**: signal-producer-pipeline-01KQ6QZS
**Mission Type**: software-dev
**Status**: Draft
**Created**: 2026-04-27
**Target Branch**: main
**Source issues**: [#595](https://github.com/jonesrussell/north-cloud/issues/595), [#593](https://github.com/jonesrussell/north-cloud/issues/593), [#594](https://github.com/jonesrussell/north-cloud/issues/594), [#592](https://github.com/jonesrussell/north-cloud/issues/592), [#599](https://github.com/jonesrussell/north-cloud/issues/599)

---

## Overview

The Lead Intelligence pipeline depends on Waaseyaa receiving a steady feed of *classified content signals* — RFP postings and need-signal hits — extracted from North Cloud's content pipeline. Today, classified content lands in Elasticsearch and is never forwarded; consumers have to query ES directly, and the Waaseyaa lead workflow cannot observe new signals as they arrive.

This mission builds the **`signal-producer/`** service: a new top-level Go service that runs every 15 minutes, queries the `*_classified_content` ES indexes for high-quality recent hits, maps them into the Waaseyaa signal payload format, and POSTs them in batches to the existing `POST /api/signals` endpoint at `https://northops.ca`. It is *not* a long-running daemon. It is a one-shot binary scheduled by systemd, persisting a checkpoint between runs so signals are produced exactly once.

The mission delivers five layered subsystems in order: **checkpoint persistence** (the leaf), **ES-hit-to-signal mapper**, **Waaseyaa HTTP client**, the **umbrella binary** that wires them, and **production deployment** with the systemd timer. After this mission, Waaseyaa receives a fresh batch of signals every 15 minutes without human intervention; an alert fires if three consecutive runs deliver zero signals.

## User Scenarios

### Primary

The platform operator (you) deploys the signal-producer to the production VPS, enables the systemd timer, and walks away. Every 15 minutes the timer fires, the binary processes any classified content since the last successful run, posts to Waaseyaa, advances the checkpoint, and exits cleanly. The operator monitors via journald structured logs.

### Secondary

A Waaseyaa-side downstream consumer relies on the `/api/signals` endpoint receiving fresh, deduplicated, well-formed signals. They never speak to North Cloud directly; the producer is their only data path.

### Recovery scenarios

- The binary crashes mid-batch. On the next run, the checkpoint hasn't advanced past the failed batch, so the same signals are reprocessed. Waaseyaa-side dedup (via `external_id` with `nc-rfp-` / `nc-sig-` prefixes) prevents duplicates.
- Waaseyaa is down. The HTTP client retries 3× with exponential backoff (1s, 5s, 15s); if all retries fail, the run exits non-zero and the checkpoint is *not* advanced. The next timer fire reprocesses.
- The timer misses a fire (server reboot, clock skew). Systemd's `Persistent=true` runs a catch-up at next boot.
- The checkpoint file is missing or corrupt. The producer defaults to a 24-hour lookback and proceeds; a corrupt file is logged at WARN.

### Edge cases

- ES returns zero hits since checkpoint. The binary logs the empty result, advances no checkpoint, and exits 0. Three of these in a row trigger an operator alert (source-down indicator).
- ES indexing lag means a signal lands *after* its `crawled_at` timestamp has already passed the checkpoint. A 5-minute lookback buffer (subtract 5m from the checkpoint when querying) prevents misses.
- Waaseyaa returns 4xx (client error). No retry; the run fails fast and the checkpoint does not advance. This indicates a payload bug and must surface to the operator immediately.
- A batch contains an issue that cannot be mapped (missing required field). The mapper returns an error for that hit; the producer logs and skips it, includes it in the `skipped` count, and continues with the rest of the batch.
- Network partition splits a batch. The retry loop covers transient failures; a partial-success POST that returns non-200 is treated as failure and retried.

## Functional Requirements

| ID      | Requirement                                                                                                                                                                                                                                | Status   |
| ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------- |
| FR-001  | The system MUST provide a standalone Go binary at `signal-producer/cmd/main.go` that runs to completion and exits, suitable for invocation by a systemd timer (not a long-running daemon).                                                | Required |
| FR-002  | The binary MUST query Elasticsearch indexes matching `*_classified_content` for documents whose `crawled_at` is greater than or equal to (`checkpoint − 5 minutes`), filtered to `content_type ∈ {rfp, need_signal}` and `quality_score ≥ 40`. | Required |
| FR-003  | The system MUST persist a checkpoint at a configurable path (default `/var/lib/signal-producer/checkpoint.json`) containing the last successful run timestamp and last batch size, written atomically (write-temp-then-rename).            | Required |
| FR-004  | The system MUST default the checkpoint to 24 hours before "now" when the checkpoint file does not exist, and log this as a first-run signal.                                                                                                | Required |
| FR-005  | The system MUST advance the checkpoint only after a batch has been successfully delivered to Waaseyaa (HTTP 2xx response received). Partial-batch failures leave the checkpoint at the last successful run.                                | Required |
| FR-006  | The system MUST map ES hits to the Waaseyaa `Signal` payload format with `signal_type`, `external_id`, `source`, `source_url`, `label`, `strength`, `organization_name`, `sector`, `province`, optional `expires_at`, and `payload` fields. | Required |
| FR-007  | The mapper MUST prefix `external_id` with `nc-rfp-` for RFP hits and `nc-sig-` for need_signal hits to prevent collisions across signal types.                                                                                              | Required |
| FR-008  | The mapper MUST tolerate missing optional ES fields (province, sector) by emitting empty strings rather than failing the hit.                                                                                                              | Required |
| FR-009  | The HTTP client MUST POST to `${WAASEYAA_URL}/api/signals` with the `X-Api-Key` header set from `WAASEYAA_API_KEY` and a `Content-Type: application/json` body of shape `{"signals": [...]}`.                                              | Required |
| FR-010  | The HTTP client MUST retry transient failures (5xx and network errors) up to 3 times with exponential backoff at 1s, 5s, and 15s. It MUST NOT retry on 4xx responses.                                                                       | Required |
| FR-011  | The HTTP client MUST honor `context.Context` cancellation, stopping any in-flight retries when the context is done.                                                                                                                        | Required |
| FR-012  | The HTTP client MUST parse the response body into `IngestResult{Ingested, Skipped, LeadsCreated, LeadsMatched, Unmatched}` and return it to the producer for logging.                                                                      | Required |
| FR-013  | The producer MUST batch signals in groups of 50 (configurable) before each POST, processing the ES result set in a single run.                                                                                                              | Required |
| FR-014  | The producer MUST emit one structured log line per ES query, per batch POST, and per run summary, including fields `batch_size`, `ingested`, `skipped`, `errors`, and `duration_ms`.                                                       | Required |
| FR-015  | The system MUST be configurable via a `config.yml` file plus environment-variable overrides for `WAASEYAA_URL`, `WAASEYAA_API_KEY`, and `ES_URL`.                                                                                          | Required |
| FR-016  | The system MUST exit non-zero when any batch fails to deliver after all retries, so systemd records the unit as failed.                                                                                                                    | Required |
| FR-017  | A systemd timer MUST be deployed alongside the binary that fires every 15 minutes (`OnCalendar=*:0/15`), with `Persistent=true` to catch up on missed runs.                                                                                | Required |
| FR-018  | The deployment MUST place the binary on the existing North Cloud production VPS, store the checkpoint at `/var/lib/signal-producer/checkpoint.json` with the producer's runtime user as owner, and stream logs to journald.                | Required |
| FR-019  | The system MUST surface an operator-actionable alert when 3 consecutive runs return zero ingested signals (source-down detection).                                                                                                          | Required |
| FR-020  | The Taskfile MUST expose `task signal-producer:build`, `task signal-producer:test`, and `task signal-producer:lint` targets matching the existing per-service Taskfile pattern.                                                            | Required |

## Non-Functional Requirements

| ID       | Requirement                                                                                                                                       | Threshold                                                                                          | Status   |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | -------- |
| NFR-001  | A single 15-minute run must complete within the timer interval, including ES query, mapping, batch POSTs, and checkpoint advance.                | p95 wall-clock ≤ 5 minutes for batches up to 1000 signals.                                         | Required |
| NFR-002  | The HTTP client must not stall a run when Waaseyaa is unresponsive.                                                                              | Total per-batch retry budget ≤ 25 seconds (covers 1+5+15s + jitter).                              | Required |
| NFR-003  | Test coverage on the new packages must meet the charter target.                                                                                  | ≥ 80% per package on `internal/producer`, `internal/mapper`, `internal/client`.                    | Required |
| NFR-004  | Integration tests for the producer end-to-end (ES → Waaseyaa) must run against a real ES instance per the charter (no infrastructure mocks).     | At least one integration test exercises a live ES round-trip; HTTP target may be a test server.    | Required |
| NFR-005  | Checkpoint writes must survive crash mid-write.                                                                                                  | Atomic write-temp-then-rename; verified by a fault-injection test.                                | Required |
| NFR-006  | Structured log volume must remain operator-readable.                                                                                             | ≤ 5 INFO lines per batch + 1 summary line per run; ≤ 10 KB per typical run; no debug spam at INFO. | Required |

## Constraints

| ID    | Constraint                                                                                                                                                                                                                                       | Status   |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------- |
| C-001 | This mission MUST NOT modify the Waaseyaa receiver or any other service's source code. The producer is a new isolated service. The only allowed shared touch is `infrastructure/` if a genuinely cross-service concern emerges.                  | Required |
| C-002 | All Go code MUST conform to the project's golangci-lint configuration (`task lint:force` clean). No `os.Getenv` outside `cmd/` and `infrastructure/config/`. Cognitive complexity ≤ 20, funlen ≤ 100, line length ≤ 150.                          | Required |
| C-003 | Logging MUST use the project's `infrastructure/logger` package (JSON output), NOT `zap` directly. Issue #592's mention of `zap` is overridden by the charter and root CLAUDE.md.                                                                | Required |
| C-004 | Structured-config loading MUST go through `infrastructure/config/` so the `os.Getenv` ban is honored.                                                                                                                                          | Required |
| C-005 | Conventional commit messages; no `--no-verify`; no force-push to `main`; no `os.Getenv` outside the allowed locations.                                                                                                                          | Required |
| C-006 | Branch naming for any per-WP work follows `claude/{description}-{session-id}`. Worktree placement uses the standard `lanes.json` allocation.                                                                                                    | Required |
| C-007 | The mission MUST honor the existing dev/prod docker-compose split. The producer is a systemd-scheduled binary on the host, not a docker-compose service in this mission. (A future mission may containerize it; out of scope here.)             | Required |
| C-008 | The Waaseyaa `/api/signals` endpoint is treated as an external dependency owned by Waaseyaa. This mission MUST NOT modify it; it MUST treat the existing JSON contract as authoritative and verify behavior with a test HTTP server.            | Required |
| C-009 | `task drift:check` and `task layers:check` MUST pass before merge. Adding a new top-level service requires updating `GO_SERVICES` in `scripts/detect-changed-services.sh` and adding the service-name root delegation in `Taskfile.yml`.        | Required |
| C-010 | Checkpoint must NOT be world-readable. File permissions: 0640 (owner read/write, group read).                                                                                                                                                  | Required |

## Success Criteria

1. After deployment, Waaseyaa receives at least one fresh `signals` POST every 15 minutes during periods when classified content is being ingested.
2. Three consecutive timer fires that produce zero ingested signals trigger an operator alert (source-down detection works).
3. A maintainer can read journald logs and answer "how many signals went out, how many were rejected, how long did it take" within 30 seconds, without external tools.
4. The producer recovers cleanly from a Waaseyaa outage: the run that overlaps the outage exits non-zero, the checkpoint does not advance, and the next run reprocesses missed signals without duplicates on the Waaseyaa side.
5. A cold-start (no checkpoint file present) processes the last 24 hours of classified content in a single run within the timer interval.
6. Test coverage on each of the three internal packages is ≥ 80% as measured by `task test:cover` for `signal-producer/`.
7. The full test suite (`task test`) and full lint pass (`task lint:force`) remain green after the mission lands.

## Key Entities

- **Checkpoint** — `{LastSuccessfulRun: time.Time, LastBatchSize: int}`. Persisted as JSON at `/var/lib/signal-producer/checkpoint.json`. Loaded at start, saved after each successful batch.
- **ESHit** — A document returned from `*_classified_content`. Required fields used by the mapper: `_id`, `title`, `quality_score`, `url`, `crawled_at`, `content_type`. Type-specific subfields: `rfp.organization_name`, `rfp.province`, `rfp.categories`, `rfp.closing_date`; `need_signal.organization_name`, `need_signal.province`, `need_signal.sector`, `need_signal.signal_type`.
- **Signal** — The Waaseyaa wire format. Fields: `signal_type`, `external_id`, `source`, `source_url`, `label`, `strength`, `organization_name`, `sector`, `province`, `expires_at` (optional), `payload` (full ES hit).
- **IngestResult** — Waaseyaa's response: `{Ingested, Skipped, LeadsCreated, LeadsMatched, Unmatched}`. Used for run-summary logging.
- **Config** — Loaded from `config.yml` plus env overrides. Contains `waaseyaa.{url, api_key, batch_size, min_quality_score}`, `elasticsearch.{url, indexes}`, `schedule.lookback_buffer`, `checkpoint.file`.

## Assumptions

- The Waaseyaa `/api/signals` endpoint exists at `https://northops.ca/api/signals`, accepts `X-Api-Key` auth, and returns the documented `IngestResult` JSON shape. The mission proceeds against this contract; if Waaseyaa's contract differs at integration time, that's a Waaseyaa-side concern surfaced as a follow-up issue.
- Production Postgres, Elasticsearch, and Caddy are already running on the target VPS at the documented endpoints.
- The classifier is already populating `*_classified_content` indexes with `content_type` of `rfp` and `need_signal` for documents above quality score 40. (Verifiable independent of this mission.)
- The maintainer has SSH/Ansible access to the production VPS to install the systemd unit and binary; deployment via the existing GitHub Actions deploy.sh pipeline is preferred.
- The `WAASEYAA_API_KEY` value will be provisioned out-of-band (e.g., 1Password) before deployment; this mission does not generate it.
- ES indexing lag is bounded at < 5 minutes; if it drifts higher, the lookback buffer is widened in a follow-up.

## Out of Scope

- Building or modifying the Waaseyaa receiver, including any change to `/api/signals` semantics, dedup logic, or response schema (separate Waaseyaa-side mission).
- The enrichment service (separate `enrichment-service-rollout` mission, survivors #596 / #598 / #597 / #600).
- Signal-crawler deployment (#588 — a different existing service in `signal-crawler/`, not the producer).
- Containerization of the signal-producer (it ships as a host-level systemd unit; future mission may dockerize).
- Adding new ES indexes or modifying existing classifier output.
- Backfilling historical signals beyond the 24-hour cold-start window.
- Multi-region or HA deployment of the producer (single-VPS is intentional; the timer is idempotent).

## Risks and Mitigations

| Risk                                                                                              | Mitigation                                                                                                                                                                  |
| ------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Waaseyaa contract drifts before mission lands.                                                    | Treat `/api/signals` as authoritative external contract; integration test uses a recorded fixture. If real Waaseyaa diverges at deploy time, surface as a follow-up issue. |
| Checkpoint corruption silently skips a window of signals.                                         | Atomic file writes (NFR-005), defensive load (corrupt → 24h fallback + WARN log), Waaseyaa-side `external_id` dedup as final backstop.                                      |
| Timer overlap (a long run still in progress when the next fires).                                 | Systemd unit declares itself `Type=oneshot`; the timer respects unit state and does not start a second instance. Document this in the unit file.                            |
| Source-down false positives (legitimate quiet periods classified as outages).                    | The 3-consecutive-empty-runs threshold (FR-019) gives a 45-minute window; legitimate quiet periods longer than that are rare in this pipeline and worth flagging anyway.    |
| ES query inefficiency at 15-minute cadence.                                                       | The `crawled_at` range query is selective; ES indexes already include the field. NFR-001 budget (5min p95 for 1000 signals) is generous.                                    |
| Production deployment introduces a new attack surface (API key in env).                          | API key never logged; checkpoint file 0640; systemd `EnvironmentFile=` reads from a 0600 file owned by root with `User=` directive.                                         |

---

## Notes for Downstream Phases

- **/spec-kitty.plan**: produces the technical plan, including the layered package design (checkpoint → mapper → client → producer → deployment). Charter Check should examine the new top-level `signal-producer/` directory against `infrastructure/`-only sharing rule.
- **/spec-kitty.tasks**: expected to produce 5–6 work packages — likely one per layer plus one for deployment. Lanes should reflect dependency order; the three leaf packages (checkpoint, mapper, client) can land in parallel after a foundational scaffolding WP.
- **/spec-kitty.implement**: each WP runs the full implement-review loop. The deployment WP MUST verify the systemd unit and timer on the actual production VPS before approval.
- **/spec-kitty.review**: end-of-mission review verifies success criteria against journald output from the first three real timer fires.
