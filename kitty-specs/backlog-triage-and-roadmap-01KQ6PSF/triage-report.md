# Backlog Triage Report

**Mission**: `backlog-triage-and-roadmap-01KQ6PSF`
**Snapshot timestamp (UTC)**: `2026-04-27T05:44:07Z`
**Snapshot source**: [`snapshot.json`](./snapshot.json) — pinned output of `gh issue list --state open`.
**Total open issues at snapshot**: 13
**Milestone counts**: Lead Intelligence Integration (north-cloud) = 10; Phase 3: Signal Crawlers = 2; Developer Experience = 1.

This report classifies every open issue across the three named milestones, sequences the survivors by dependency, and recommends the next Spec Kitty missions. It is planning-only: no production code is touched (C-001), no GitHub state is mutated (C-002).

---

## 1. Per-Milestone Triage

### 1.1 Lead Intelligence Integration (north-cloud)

| #   | Title                                                                                              | Verdict      | Scope (one line)                                                                                                                |
| --- | -------------------------------------------------------------------------------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------------- |
| 669 | feat(validator): add sector_alignment coverage + accuracy metrics with earned promotion            | `deprecate`  | Already implemented by commit `dc4dac20`; nightly validator + thresholds shipped under `tools/validate-sector-alignment/`.       |
| 600 | ops: enrichment service deployment                                                                 | `keep`       | Systemd unit + Caddy config to run the enrichment HTTP server on port 8095 in production with health checks.                    |
| 599 | ops: signal producer deployment and scheduling                                                     | `keep`       | Systemd timer (every 15 min) for the signal-producer binary plus checkpoint volume and journald logging on the prod VPS.        |
| 598 | feat: implement enrichment callback client                                                         | `keep`       | HTTP client in `enrichment/internal/callback` that POSTs enrichment results back to Waaseyaa with retry and structured logging. |
| 597 | feat: implement company_intel, tech_stack, and hiring enrichers                                    | `keep`       | Three ES-backed enricher implementations satisfying the `Enricher` interface with confidence scoring per result quality.        |
| 596 | feat: implement enrichment service                                                                 | `keep`       | New `enrichment/` Go service (HTTP server on `/api/v1/enrich`) that fans out enrichers and returns 202 immediately.             |
| 595 | feat: implement checkpoint persistence for signal producer                                         | `keep`       | Atomic JSON checkpoint file at `/var/lib/signal-producer/checkpoint.json` with 24h default and 5m lookback buffer.              |
| 594 | feat: implement Waaseyaa HTTP client for signal posting                                            | `keep`       | HTTP client in `signal-producer/internal/client` that POSTs signal batches to Waaseyaa `/api/signals` with retry and auth.      |
| 593 | feat: implement ES hit to Waaseyaa signal mapper                                                   | `keep`       | Pure mapping package converting ES classified-content hits (rfp, need_signal) into the Waaseyaa Signal payload struct.          |
| 592 | feat: implement signal producer binary                                                             | `keep`       | New top-level `signal-producer/` binary that wires checkpoint + ES query + mapper + Waaseyaa client into a 15-min batch job.    |

### 1.2 Phase 3: Signal Crawlers

| #   | Title                                                              | Verdict | Scope (one line)                                                                                                       |
| --- | ------------------------------------------------------------------ | ------- | ---------------------------------------------------------------------------------------------------------------------- |
| 588 | Deploy signal-crawler to production with cron schedule             | `keep`  | Build signal-crawler on prod VPS, configure systemd timer (06:00 UTC daily), enable live mode once NorthOps endpoints exist. |
| 580 | Deploy warns about missing UMAMI env vars                          | `keep`  | Add `UMAMI_DB_PASSWORD` and `UMAMI_APP_SECRET` to the prod `.env` template (Ansible role) so deploy logs stop warning. |

### 1.3 Developer Experience

| #   | Title                                                          | Verdict | Scope (one line)                                                                                                                  |
| --- | -------------------------------------------------------------- | ------- | --------------------------------------------------------------------------------------------------------------------------------- |
| 646 | chore(infrastructure): clean up pre-existing lint violations   | `keep`  | Resolve 43 baseline `golangci-lint` issues across `infrastructure/` so pre-commit no longer requires `--no-verify` on edits.       |

No issues fell into an `other` (out-of-milestone) bucket; all 13 belong to one of the three named milestones.

---

## 2. Issue Detail Blocks

### #669 — feat(validator): add sector_alignment coverage + accuracy metrics with earned promotion

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `deprecate`
- **Scope**: Nightly validator + earned-promotion thresholds for the classifier `sector_alignment` component.
- **Justification**: Commit [`dc4dac20`](https://github.com/jonesrussell/north-cloud/commit/dc4dac20) (`feat(validator): add sector_alignment coverage and accuracy gate`) lands the GitHub Actions workflow (`.github/workflows/validate-sector-alignment.yml`), the validator binary (`tools/validate-sector-alignment/main.go` + tests), and the earned-promotion gate. The issue body's acceptance-criteria checkboxes are already marked done. No remaining work.
- **Follow-up**: Maintainer to close on GitHub citing `dc4dac20`.

### #600 — ops: enrichment service deployment

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `keep`
- **Scope**: Systemd unit + env wiring + health monitoring to run enrichment HTTP server on port 8095 in production.
- **Size**: S
- **Size rationale**: Systemd unit + env file + Caddy entry; no code change in this service tree, no schema.
- **Depends on**: #596, #597, #598 (binary and enrichers must exist before deployment)
- **Blocks**: (none in snapshot)
- **Bypass eligible**: `no` — production deploy + new public surface; Spec Kitty cycle gives the rollback safety the trivial path lacks.

### #599 — ops: signal producer deployment and scheduling

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `keep`
- **Scope**: Systemd timer (every 15 min) + checkpoint volume + journald logging for the new signal-producer binary.
- **Size**: S
- **Size rationale**: Ansible/systemd config only; no application code; mirrors signal-crawler pattern from #588.
- **Depends on**: #592 (binary) — and transitively #595 (checkpoint persistence path used by the timer's volume).
- **Blocks**: (none)
- **Bypass eligible**: `no` — production deploy with shared-secret env (`WAASEYAA_API_KEY`) and a new outbound integration; a real review pass is appropriate.

### #598 — feat: implement enrichment callback client

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `keep`
- **Scope**: HTTP client posting enrichment results to a Waaseyaa callback URL with retry/backoff and structured logging.
- **Size**: S
- **Size rationale**: Single package (`enrichment/internal/callback`); pure HTTP + retry; mockable end-to-end in unit tests.
- **Depends on**: (none — issue body confirms it can be developed against a mock server)
- **Blocks**: #600 (deployment needs the callback path wired)
- **Bypass eligible**: `no` — outbound HTTP with shared-secret auth header is security-relevant per C-006.

### #597 — feat: implement company_intel, tech_stack, and hiring enrichers

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `keep`
- **Scope**: Three ES-backed enrichers implementing the `Enricher` interface with structured outputs and confidence scoring.
- **Size**: M
- **Size rationale**: Three enrichers × ES query + confidence rule + tests; bounded to one service but multi-file.
- **Depends on**: #596 (defines the `Enricher` interface and registry)
- **Blocks**: #600 (deploy needs the enrichers compiled in)
- **Bypass eligible**: `no` — reads ES across multiple indexes and shapes downstream lead-pipeline confidence; exercise the spec loop.

### #596 — feat: implement enrichment service

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `keep`
- **Scope**: New `enrichment/` top-level Go service: HTTP server on `/api/v1/enrich`, fan-out goroutines, callback dispatch.
- **Size**: L
- **Size rationale**: New top-level service directory + bootstrap pattern + HTTP handler + concurrency model + Taskfile + Dockerfile.
- **Depends on**: (none — service skeleton can land before enrichers/callbacks; issue body lists only Waaseyaa as external)
- **Blocks**: #597, #598, #600
- **Bypass eligible**: `no` — new deployable service is exactly the case Spec Kitty exists for.

### #595 — feat: implement checkpoint persistence for signal producer

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `keep`
- **Scope**: Atomic JSON checkpoint file with 24h default + 5m lookback buffer for the signal-producer batch job.
- **Size**: S
- **Size rationale**: One file (`signal-producer/internal/producer/checkpoint.go`) + tests; no external dependencies.
- **Depends on**: (none)
- **Blocks**: #592 (binary's main loop reads/writes this checkpoint)
- **Bypass eligible**: `no` — atomic-file semantics + correctness here are load-bearing for downstream signal de-duplication; review is cheap insurance.

### #594 — feat: implement Waaseyaa HTTP client for signal posting

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `keep`
- **Scope**: HTTP client posting signal batches to Waaseyaa `/api/signals` with retry/backoff, API-key auth, and structured logs.
- **Size**: S
- **Size rationale**: Single package (`signal-producer/internal/client`); pure HTTP + retry; testable with httptest.
- **Depends on**: (none — issue body confirms mockable)
- **Blocks**: #592
- **Bypass eligible**: `no` — outbound HTTP with shared-secret auth is security-relevant.

### #593 — feat: implement ES hit to Waaseyaa signal mapper

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `keep`
- **Scope**: Pure mapping package converting ES classified-content hits (rfp, need_signal) into Waaseyaa `Signal` structs.
- **Size**: S
- **Size rationale**: Pure data mapping, no I/O; one package + table-driven tests.
- **Depends on**: (none — issue body explicitly states "pure data mapping")
- **Blocks**: #592
- **Bypass eligible**: `no` — defines the cross-service payload contract with Waaseyaa; spec cycle keeps the schema honest.

### #592 — feat: implement signal producer binary

- **Milestone**: Lead Intelligence Integration (north-cloud)
- **Verdict**: `keep`
- **Scope**: New top-level `signal-producer/` binary wiring checkpoint + ES query + mapper + Waaseyaa client into a 15-min batch job.
- **Size**: L
- **Size rationale**: New top-level service directory + cmd/main + bootstrap + ES query + Taskfile; integrates three sibling packages.
- **Depends on**: #593, #594, #595 (mapper, client, checkpoint packages it imports)
- **Blocks**: #599 (deploy needs the binary)
- **Bypass eligible**: `no` — new deployable batch service.

### #588 — Deploy signal-crawler to production with cron schedule

- **Milestone**: Phase 3: Signal Crawlers
- **Verdict**: `keep`
- **Scope**: Build signal-crawler on prod VPS, configure systemd timer (06:00 UTC daily), enable live mode once NorthOps endpoints land.
- **Size**: S
- **Size rationale**: Ansible/systemd configuration; the binary already exists and recent commits (`065d9069`, `f946b726`) wired most of the timer plumbing.
- **Depends on**: (none in this NC snapshot — externally blocked by `jonesrussell/northops-waaseyaa#92`, `#93`, which are out of scope for this triage)
- **Blocks**: (none)
- **Bypass eligible**: `no` — production deploy on the prod VPS with new outbound calls; review the deploy plan.
- **Note**: This is **not** a duplicate of #599 — #588 deploys `signal-crawler/` (existing service, Phase 3), while #599 deploys the new `signal-producer/` binary from #592. Different services, different cadences (daily vs every 15 min).

### #580 — Deploy warns about missing UMAMI env vars

- **Milestone**: Phase 3: Signal Crawlers
- **Verdict**: `keep`
- **Scope**: Add `UMAMI_DB_PASSWORD` and `UMAMI_APP_SECRET` to the prod `.env` template (Ansible north-cloud role) so deploy logs stop warning.
- **Size**: S
- **Size rationale**: Two values added to an Ansible vault-sourced `.env` template; no code change in this repo.
- **Depends on**: (none)
- **Blocks**: (none)
- **Bypass eligible**: `yes` — pure ops noise reduction in the sibling Ansible repo (`northcloud-ansible`); no NC code path, no schema, no API surface, no security regression (vars come from vault).
- **Note**: The fix lives in a sibling repository (`northcloud-ansible`). It still surfaces here because it shows up in NC deploy logs, but the implementation belongs in the Ansible role, not in this repo. See scope-violation handling under C-004 — the symptom is NC's, the fix is Ansible's; tracked here as `keep + bypass-eligible` so the maintainer can land the Ansible PR directly.

### #646 — chore(infrastructure): clean up pre-existing lint violations

- **Milestone**: Developer Experience
- **Verdict**: `keep`
- **Scope**: Resolve 43 baseline `golangci-lint` issues across `infrastructure/` (mnd, forbidigo, testpackage, gosec, nestif, exhaustive, gocognit, govet, nilnil) so pre-commit passes without `--no-verify`.
- **Size**: M
- **Size rationale**: 10 files across `infrastructure/`; mostly mechanical, but includes gosec G118 (real concurrency bug) and forbidigo `os.Getenv` removal (cross-cutting wiring change).
- **Depends on**: (none)
- **Blocks**: (none — but every infrastructure-touching PR currently pays a lint-bypass tax)
- **Bypass eligible**: `no` — gosec G118 is a real cancel-leak fix and the `os.Getenv` removal touches startup wiring. Per C-006 ("no security-relevant code path") this fails the bypass test. The mechanical mnd/testpackage subset *could* land as trivial chores, but the issue is filed as a single bundle, so treat the whole as Spec-Kitty-eligible.

---

## 3. Prioritized Survivor List

Topological sort of `depends_on` edges, with milestone-priority tiebreaker (Lead Intelligence > Phase 3 > Developer Experience) and size-ascending within each tier.

| Rank | #   | Title                                                                | Size | Bypass | Why this position                                                                  |
| ---- | --- | -------------------------------------------------------------------- | ---- | ------ | ---------------------------------------------------------------------------------- |
| 1    | 595 | checkpoint persistence for signal producer                           | S    | no     | No deps; small; blocks #592 — ship first to unblock the producer chain.            |
| 2    | 593 | ES hit to Waaseyaa signal mapper                                     | S    | no     | No deps; pure mapping; blocks #592.                                                |
| 3    | 594 | Waaseyaa HTTP client for signal posting                              | S    | no     | No deps; blocks #592.                                                              |
| 4    | 596 | enrichment service                                                   | L    | no     | No deps; defines `Enricher` interface that #597 and #598 import.                   |
| 5    | 598 | enrichment callback client                                           | S    | no     | No deps; pairs with #596 in the same service tree; blocks deploy #600.             |
| 6    | 592 | signal producer binary                                               | L    | no     | Depends on #593, #594, #595 (now all ranked above); umbrella binary for that chain. |
| 7    | 597 | company_intel, tech_stack, hiring enrichers                          | M    | no     | Depends on #596 (interface); blocks deploy #600.                                   |
| 8    | 599 | ops: signal producer deployment and scheduling                       | S    | no     | Depends on #592; deployment of the now-built binary.                               |
| 9    | 600 | ops: enrichment service deployment                                   | S    | no     | Depends on #596, #597, #598; deployment of the now-complete enrichment service.    |
| 10   | 588 | Deploy signal-crawler to production with cron schedule               | S    | no     | No NC-internal deps; externally blocked on northops-waaseyaa #92/#93.              |
| 11   | 580 | Deploy warns about missing UMAMI env vars                            | S    | yes    | Trivial Ansible env addition; can land any time independent of all other work.     |
| 12   | 646 | chore(infrastructure): clean up pre-existing lint violations         | M    | no     | Independent; lower priority than the lead-intelligence chain but unblocks future hygiene. |

The sort respects every `depends_on` edge: #592 (rank 6) follows #593/#594/#595 (ranks 1–3); #597 (rank 7) follows #596 (rank 4); #599 (rank 8) follows #592 (rank 6); #600 (rank 9) follows #596/#597/#598 (ranks 4, 5, 7).

---

## 4. Recommended Next Missions

Two missions are proposed. Together they cover the rank-1 survivor (FR-010) and the entire critical-path chain through rank 9. The lower-ranked items (#588, #580, #646) are intentionally left for ad-hoc handling by the maintainer — they are independent, small, and a third mission would add scheduling overhead without parallelism benefit. No proposed scope overlaps any active `claude/*` branch (only the current `claude/musing-benz-b38483` triage branch exists) or any open PR (none).

### Recommendation 1 — Signal Producer Pipeline

- **Friendly name**: Signal Producer Pipeline
- **Slug proposal**: `signal-producer-pipeline`
- **Survivor numbers (execution order)**: `[595, 593, 594, 592, 599]`
- **Scope paragraph**: Build and deploy the new `signal-producer/` batch service that pushes classified ES content to Waaseyaa every 15 minutes. Lands the three leaf packages (checkpoint persistence, ES-to-Signal mapper, Waaseyaa HTTP client), the umbrella binary that wires them, and the systemd timer that schedules it on the prod VPS. Out of scope: enrichment service (separate mission), signal-crawler deployment #588 (different existing service), and any change to the Waaseyaa receiver — this mission only produces signals against an already-deployed `/api/signals` endpoint.

### Recommendation 2 — Enrichment Service Rollout

- **Friendly name**: Enrichment Service Rollout
- **Slug proposal**: `enrichment-service-rollout`
- **Survivor numbers (execution order)**: `[596, 598, 597, 600]`
- **Scope paragraph**: Build and deploy the new `enrichment/` HTTP service that accepts enrichment requests from Waaseyaa, fans out three ES-backed enrichers (company_intel, tech_stack, hiring), and POSTs structured results back via callback. Lands the service skeleton, the callback client, the three enrichers, and the systemd unit on the prod VPS behind Caddy. Out of scope: signal-producer (separate mission), new enrichment types beyond the three named, and any Waaseyaa-side schema change — this mission targets the existing callback contract.

---

## 5. Deprecation Follow-Up

The maintainer should close the following issue on GitHub. Suggested close comment is provided so closure takes under a minute per issue. (Per C-002, this report performs no `gh issue close|edit|comment` action.)

| #   | Title                                                                                              | Rationale                                                                                                              | Suggested close comment                                                                                                                                                                                  |
| --- | -------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 669 | feat(validator): add sector_alignment coverage + accuracy metrics with earned promotion            | Implemented by `dc4dac20` (`feat(validator): add sector_alignment coverage and accuracy gate`); workflow + binary live. | "Closing — implemented in [`dc4dac20`](https://github.com/jonesrussell/north-cloud/commit/dc4dac20). The validator workflow at `.github/workflows/validate-sector-alignment.yml` and binary at `tools/validate-sector-alignment/` cover both metrics with earned-promotion gating." |

No issues received a `merge-into:#NNN` verdict (the deployment-cluster issues #588, #599, #600 looked superficially similar but cover three distinct services and remain independent `keep`s).

---

## 6. Notes for the Reviewer

- The two non–lead-intelligence survivors (#588, #580, #646) are intentionally left out of the recommended missions because they are small, independent, and the maintainer can land each as a single-PR chore without spinning up a full Spec Kitty cycle.
- #580 is the only `bypass_eligible: yes` item; it lives in a sibling repo (Ansible) but is filed here because the symptom appears in NC deploy logs.
- The `Lead Intelligence Integration` chain forms two clean parallel streams: signal-producer (#593, #594, #595 → #592 → #599) and enrichment (#596 → {#597, #598} → #600). The survivor sort interleaves them by topology + tiebreaker, but a maintainer with two agents could implement the two recommended missions concurrently.
