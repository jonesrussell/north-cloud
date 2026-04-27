# Mission Review — signal-producer-pipeline-01KQ6QZS

**Reviewer**: Senior mission reviewer (Opus 4.7, post-merge audit)
**Date**: 2026-04-27
**Mission**: `signal-producer-pipeline-01KQ6QZS` ("Signal Producer Pipeline")
**Mission type**: software-dev (6 work packages, single-lane sequential)
**Baseline**: `e8bda85f` (plan committed; first WP01 work begins at `26bff475`)
**HEAD reviewed**: `20d785e4` (squash merge to `main`)
**Branch (deleted)**: `kitty/mission-signal-producer-pipeline-01KQ6QZS`

---

## Critical Methodological Limitation (read first)

The agent sandbox has **no Go toolchain**. Neither WP implementers, WP reviewers, nor this post-merge reviewer were able to run `go build`, `go vet`, `go test`, `go mod tidy`, `go mod verify`, or `golangci-lint`. Every "test passes" claim in the review history and every assertion below about behavioral correctness is by **inspection only**. CI (GitHub Actions) is the first place real Go validation occurs.

This means:

- The 80% coverage NFR (NFR-003) is **inspected but not measured**.
- The integration test under `//go:build integration` (NFR-004) has never been run; it was added in WP05 but the run plan defers execution to a manual harness pointed at a live ES.
- `signal-producer/go.sum` was hand-copied from `auth/go.sum` (WP02 cycle-2) and topped up by maintenance commit `2214cf61` with the full transitive closure of `infrastructure/`. Hashes spot-checked here are byte-identical to `auth/go.sum` (zap, multierr, otel-1.41.0 confirmed), but `go mod tidy` was never executed to prove the closure is complete or minimal.

Treat this as the dominant deferred risk for the verdict.

---

## FR Coverage Matrix

Mapping is from `tasks.md` "Requirement Coverage" section. "Test verifies" lists the test function (or files) that asserts the FR; ADEQUATE means the named test would fail if the implementation were removed or contradicted.

| FR     | Spec section                              | Owning WP | Implementation                                                                  | Test verifies                                                                              | Verdict   |
| ------ | ----------------------------------------- | --------- | ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | --------- |
| FR-001 | Standalone one-shot Go binary             | WP05      | `signal-producer/cmd/main.go` (`signal.NotifyContext` + `os.Exit(1)`)           | Inspection only (binary structure is exit-on-Run-error); no test directly forks the binary | ADEQUATE  |
| FR-002 | ES query (`*_classified_content`, gte, content_type ∈ {rfp,need_signal}, quality_score ≥ 40) | WP05 | `producer.buildQuery` in `producer.go`                                          | `TestBuildQuery_Shape`                                                                     | ADEQUATE  |
| FR-003 | Atomic checkpoint persistence             | WP02      | `Checkpoint` struct + `SaveCheckpoint`/`writeAtomic` in `checkpoint.go`         | `TestSaveLoadRoundtrip`, `TestSaveCheckpoint_AtomicOnFailure`, `TestSaveCheckpoint_FileMode` | ADEQUATE  |
| FR-004 | 24h cold-start fallback on missing/corrupt | WP02      | `coldStart()`, `LoadCheckpoint` missing/corrupt branches                        | `TestLoadCheckpoint_Missing_DefaultsToColdStart`, `TestLoadCheckpoint_Corrupt_FallsBackWithWarn` | ADEQUATE  |
| FR-005 | Checkpoint advances only after 2xx        | WP02/WP05 | `producer.deliverHits` saves checkpoint after `postAllBatches` returns nil      | `TestRun_FirstBatchSucceedsSecondFails` (first batch's signals NOT advanced — but test asserts error returned, not checkpoint state) | PARTIAL   |
| FR-006 | Signal field set                          | WP03      | `mapper.Signal` struct, `MapHit`, `applyRFPFields`, `applyNeedSignalFields`     | `TestMapHit_RFP_Complete`, `TestMapHit_NeedSignal_Complete`                                | ADEQUATE  |
| FR-007 | `nc-rfp-` / `nc-sig-` prefix distinction  | WP03      | `externalIDPrefixRFP` / `externalIDPrefixNeedSignal` constants                  | `TestMapHit_PrefixDistinction`                                                             | ADEQUATE  |
| FR-008 | Optional fields tolerated (province/sector) | WP03    | `optionalStringFromPath`, `stringFromPath` return `""`                          | `TestMapHit_RFP_MissingOptionalFields`, `TestMapHit_NeedSignal_NoSubobject`                | ADEQUATE  |
| FR-009 | POST `/api/signals` + `X-Api-Key` + JSON  | WP04      | `client.PostSignals` builds request with both headers                           | `TestPostSignals_Success` checks the headers                                               | ADEQUATE  |
| FR-010 | 3 retries 1s/5s/15s on 5xx; no retry 4xx  | WP04      | `defaultBackoffs`, `retry`, `isRetryable`                                       | `TestPostSignals_RetriesOn5xxThenSucceeds`, `TestPostSignals_NoRetryOn4xx`, `TestPostSignals_RetriesExhausted`, `TestRetry_ExhaustsBackoffs` | ADEQUATE  |
| FR-011 | Honor `context.Context` cancellation      | WP04      | `sleepCtx` select on `ctx.Done()`                                               | `TestPostSignals_ContextCancelDuringRetrySleep`, `TestRetry_ContextCancelDuringSleep`      | ADEQUATE  |
| FR-012 | Parse `IngestResult`                      | WP04      | `decodeIngestResult`, `IngestResult` struct                                     | `TestPostSignals_Success` asserts decoded fields                                           | ADEQUATE  |
| FR-013 | Batch in groups of 50 (configurable)      | WP05      | `defaultBatchSize=50`, `postAllBatches` chunking                                | `TestRun_TwoBatchesBothSucceed`, `TestRun_BatchExactlyAtLimit`                             | ADEQUATE  |
| FR-014 | Structured logs per query/batch/summary   | WP05      | `run_start`, `es_query`, `batch_post`, `run_summary` log lines                  | Inspection only (tests assert behavior, not log shape)                                     | PARTIAL   |
| FR-015 | YAML + env overrides                      | WP05      | `internal/config/config.go` + `infraconfig.Load`/`ApplyEnvOverrides`            | `internal/config/config_test.go` (142 LOC of cases)                                        | ADEQUATE  |
| FR-016 | Exit non-zero on batch failure            | WP05      | `cmd/main.go` `os.Exit(1)` when `run` returns error; producer returns wrapped error on post failure | Inspection only (cmd-level)                                                  | ADEQUATE  |
| FR-017 | systemd timer `OnCalendar=*:0/15`, `Persistent=true` | WP06 | `signal-producer.timer` (`OnCalendar=*:0/15`, `Persistent=true`, `AccuracySec=30s`) | None (config artifact)                                                                    | ADEQUATE  |
| FR-018 | Production install + checkpoint at `/var/lib/signal-producer/checkpoint.json` + journald | WP06 | `signal-producer.service` (`StateDirectory=signal-producer`, `User=signal-producer`, `StandardOutput=journal`) | None — install lives in companion `northcloud-ansible` repo (per WP06 cycle-2 fix)       | DEFERRED  |
| FR-019 | 3-consecutive-empty-runs alert            | WP05      | `Checkpoint.ConsecutiveEmpty`, `handleNoSignals` with `==` threshold equality   | `TestRun_SourceDownThreshold`, `TestRun_AllHitsUnmappable_FiresSourceDownAtThreshold`      | ADEQUATE  |
| FR-020 | `task signal-producer:{build,test,lint}`  | WP01      | `signal-producer/Taskfile.yml` + root `Taskfile.yml` delegations                | None (CI is the test)                                                                      | ADEQUATE  |

**Stats**: 17 ADEQUATE, 2 PARTIAL, 1 DEFERRED, 0 MISSING. **Zero verified by execution.**

---

## Drift Findings

### D-1 [NON-BLOCKING] — Drift detector not updated for `signal-producer/`

WP01's reviewer flagged that `tools/drift-detector.sh` does not map `signal-producer/**` to `docs/specs/lead-pipeline.md`. The current detector (line 29 and lines 128–132) only references `signal-crawler/`. No subsequent WP added a mapping. A spec drift between `signal-producer/` and `docs/specs/lead-pipeline.md` will not be caught by `task drift:check`.

- **Evidence**: `grep -n signal-producer tools/drift-detector.sh` returns zero results.
- **Severity**: low — `docs/specs/lead-pipeline.md` was updated in WP01 (T004) and lead-pipeline coverage of signal-producer is documented, so the spec is currently in sync. The risk is only future drift escaping detection.
- **Recommendation**: out-of-scope follow-up to add `["signal-producer/"]="docs/specs/lead-pipeline.md"` to the detector's mapping table.

### D-2 [NON-BLOCKING] — `docs/specs/lead-pipeline.md` listed as superpowers spec but root CLAUDE.md still points sprint-design specs only at `signal-crawler/`

Root CLAUDE.md "Where to Look" table maps `signal-crawler/**` → `docs/specs/lead-pipeline.md`. The `signal-producer/` row is not added. Inspection-level only; not enforced by automation. Operator-facing rather than functional.

### D-3 [NOTED] — Maintenance commit `2214cf61` between WP02 and WP03

After WP02 cycle-2 hand-copied a small set of `auth/go.sum` hashes, commit `2214cf61` ("chore(signal-producer): pre-populate full infra dep closure in go.mod") was merged as a *separate* PR (not a Spec Kitty WP). It pre-populates 46 indirect deps (gin, validator, sonic, prometheus, pyroscope, otel, zap, etc.) with hashes byte-identical to `auth/go.sum`. This expanded the WP02 owned-files surface mid-mission without going through WP review. The commit's intent (smooth WP03/WP04/WP05 integration) is sound and the file scope is purely additive, but the change was not captured in any WP's review record.

- **Why it's acceptable**: hashes are byte-identical to a verified peer (`auth/go.sum`); the alternative would have been WP05 doing a much riskier `go mod tidy` blind. Spot-checked zap, multierr, otel-1.41.0 hashes are identical to `auth/go.sum`.
- **Why it's a finding**: it's invisible to a reviewer reading only the `kitty-specs/signal-producer-pipeline-01KQ6QZS/tasks/` history. A future reviewer auditing Go-dep provenance would not trace `2214cf61` from the mission spec without external context.
- **Verdict**: ACCEPTABLE. Mention in the report; do not block.

### D-4 [PARTIAL DRIFT] — `runResult.skippedRcv` field is collected but not surfaced in `run_summary`

`producer.go` line ~298 increments `res.skippedRcv += ingest.Skipped`, but the `run_summary` log emission (lines 265–273) logs `skipped` (which is `res.skippedMap`, the mapper-side skip) and `ingested`. The receiver-side `skipped` count is discarded. FR-014 says "skipped" should be in the summary; the current behavior is unambiguous in the code (`res.skippedMap` only) but the log key `skipped` is overloaded — operators could reasonably assume it means receiver-side skipped given the IngestResult shape elsewhere in the same function.

- **Severity**: low. Cosmetic / log-schema hygiene. NFR-006 is honored either way.
- **Recommendation**: a follow-up to emit both `skipped_mapper` and `skipped_receiver` in `run_summary`, or to drop the unused `skippedRcv` accumulator.

---

## Risk Findings

### R-1 [HIGH, deferred] — No Go toolchain available pre-merge

Already documented in the limitation block above. The mission shipped without local `go build`, `go test`, or `golangci-lint` ever running on the merged tree. Every behavioral claim in this review is inspection-only. CI will be the first real validator. If CI fails, a hot-fix mission will be required.

### R-2 [MEDIUM] — Integration test (NFR-004) is build-tagged and unrun

`internal/producer/integration_test.go` carries `//go:build integration` (line 1) and reads `INTEGRATION_ES_URL`. The `Taskfile.yml` `test` and `test:cover` targets do not pass `-tags=integration`, so this test never runs in normal `task test:signal-producer`. It is also not in the deploy-time smoke. The "live ES round-trip" requirement is satisfied by *the existence of* the test, not by *running it*.

- **Severity**: medium for the spec's own claim ("integration tests for the producer end-to-end ... must run against a real ES instance per the charter"); the test exists but is not exercised in any committed CI invocation discoverable in the diff.
- **Recommendation**: out-of-scope follow-up to wire `task test:signal-producer:integration` into the deploy preflight or a nightly job pointed at the dev ES.

### R-3 [LOW] — `os.Getenv("LOG_LEVEL")` in `cmd/main.go`

Permitted by C-002 (`os.Getenv` allowed in `cmd/`), but bypasses `infraconfig`. If charter rules ever tighten, this is the single line that breaks first. Functional today.

### R-4 [LOW] — Cold-start `coldStart()` uses `time.Now().Add(-24h)` without a clock interface

`checkpoint.go` line 43–46 calls `time.Now()` directly. Tests that exercise the cold-start path therefore depend on wall-clock time. Acceptable for a one-shot binary, but makes deterministic time-travel tests harder.

### R-5 [LOW] — Indirect deps in `go.mod` may exceed real transitive closure

Maintenance commit `2214cf61` added every `auth/`-side indirect dep including `gin`, `validator`, `sonic`, etc. — modules `signal-producer` likely never imports. `go mod tidy` (when CI runs it) may strip these or — worse — the build may succeed but `govulncheck` may report false vulnerability hits in unused modules. Inspection-only; CI's `task vuln:signal-producer` is the deciding test.

### R-6 [LOW] — Backoff schedule is exact, not jittered

`defaultBackoffs = {1s, 5s, 15s}`. NFR-002's 25s budget cited "1+5+15s + jitter". No jitter is present in `retry.go`. Two simultaneously-failing 15-min timer fires would retry on identical schedules, but since the timer cadence is 15min and the run is single-instance, a thundering-herd cannot form against Waaseyaa from this producer alone.

### R-7 [INFO] — `infraconfig.GetConfigPath` and `os.Stat` inside `run()` perform IO before logger is built

`cmd/main.go` lines 51–60: config-file resolution happens before `infralogger.New`. A failed `os.Stat` non-NotExist case would propagate to `run()`'s error return and surface only via the `fmt.Fprintf(os.Stderr, ...)` fallback. Acceptable; documented in the comment block.

### R-8 [INFO] — `run_summary` emits `skipped` but the field name is overloaded — see D-4

---

## Silent Failure Candidates

| Candidate                                                                                          | Why it could fail silently                                                                    | Detection                                                                                                              |
| -------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `2214cf61` indirect deps drift from real closure                                                   | `go.sum` hashes are byte-identical to `auth/`, but if `signal-producer` actually needs a dep at a *different* version, sum mismatch yields cryptic build errors at first CI run | First CI run on `main`. If green, this is a non-issue.                                                                |
| `decodeHits` in `cmd/main.go` silently drops hits with non-string `_id`                            | `searchEnvelope` declares `ID string`. ES `_id` is always a string in practice, but a malformed shard response would yield `""` and the mapper would reject as missing required field | Production logs show `mapper: missing required field "_id"` — currently surfaces in mapper_error log line               |
| Integration test silently skipped                                                                  | `//go:build integration` tag means `go test ./...` ignores it. No CI step passes `-tags=integration`. NFR-004's "live ES round-trip" claim is technically met (test exists) but in practice unverified | Inspection of `Taskfile.yml` + GH Actions workflows. Confirmed: no `-tags=integration` invocation discoverable.       |
| `checkpoint.json` written 0640 by Go, but `StateDirectory=signal-producer` is `0750` (per service) | Group readability hinges on group ID matching. If Ansible role uses default `signal-producer:signal-producer` and journal accessor is in a different group, log triage scripts may fail | Manual verification on first deploy in `northcloud-ansible` mission                                                   |
| `runResult.skippedRcv` accumulator computed and never logged (D-4)                                 | Receiver-side skips are recorded per-batch (`batch_post.skipped`) but not summed into the run-level summary line; an operator scanning only `run_summary` lines in journald cannot see total receiver skips | Visual inspection of `producer.go` lines 297–273; `grep -n skippedRcv` on the file                                    |

---

## Security Notes

| Area                       | Finding                                                                                                                                                                                                              | Severity |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| API key handling           | `client.Client.apiKey` is set from config and emitted as `X-Api-Key` header only. No log call references `apiKey` (verified by `grep -n apiKey signal-producer/internal/client/`). C-010 honored.                    | OK       |
| Checkpoint file mode       | `checkpointFileMode os.FileMode = 0o640`, enforced explicitly in `os.OpenFile`. C-010 ("not world-readable") satisfied.                                                                                              | OK       |
| Systemd unit hardening     | `Type=oneshot`, `User=signal-producer`, `Group=signal-producer`, `StateDirectory=signal-producer`, `EnvironmentFile=/etc/signal-producer/env`, `TimeoutStartSec=10m`. No `NoNewPrivileges=true`, `ProtectSystem=`, `ProtectHome=`, `PrivateTmp=true` — common hardening directives are absent. | LOW      |
| Env file mode              | `EnvironmentFile=/etc/signal-producer/env` — file mode is set by Ansible, not declared here. Spec calls for 0600. Verification deferred to the `northcloud-ansible` PR.                                              | DEFERRED |
| ES query injection         | `buildQuery` builds the DSL from typed Go values (no string interpolation of operator input). Indexes come from validated config (`url.ParseRequestURI` checks). Safe.                                               | OK       |
| `nolint:gosec` in `LoadCheckpoint` | `os.ReadFile(path) //nolint:gosec` is justified ("operator-controlled path"); the path is sourced from config that has already been validated for non-empty.                                                  | OK       |
| TLS / HTTP(s)              | `client.New` validates `BaseURL` is non-empty but does not enforce `https://`. A misconfigured `WAASEYAA_URL=http://...` would leak the API key in cleartext.                                                        | LOW      |
| Response body always closed | `client.doOnce` line 190: `defer func() { _ = resp.Body.Close() }()` after the transport-error early return. Verified.                                                                                              | OK       |
| `fmt.Fprintf(os.Stderr, "signal-producer: %v\n", err)` in `main` | Pre-logger error path. Could leak config-load error text containing a portion of the API key only if the config loader were to embed the value in its error message. `infraconfig.Load[Config]` does not. | OK       |

---

## Cross-WP Integration Verification

| Boundary                                                                              | Verification                                                                                                                                    |
| ------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| WP02 → WP05: `Checkpoint.ConsecutiveEmpty` added cross-WP to file owned by WP02       | Field present (line 37 `checkpoint.go`), JSON tag `consecutive_empty`, comment documents zero-default backward-compat. Old files load cleanly.  |
| WP03 → WP05: `mapper.Signal` shape consumed by producer's `[]any` batch              | `deliverHits` calls `mapper.MapHit(hit)` and appends to `[]any` then wraps in `client.SignalBatch{Signals: signals}`. Shape compatible.         |
| WP04 → WP05: `client.WaaseyaaClient.PostSignals` called from `postAllBatches`         | Signature `(ctx, SignalBatch) (*IngestResult, error)` matches in both files. Sentinel `client.ErrClientResponse()` consumed via `errors.Is` in `logBatchError`. |
| WP05's `handleNoSignals` invocation from both empty-result and all-unmappable paths   | Verified: `producer.go` line 165 (`len(hits) == 0`) and line 245 (`len(signals) == 0` after mapping). No third "zero-signals" branch exists.    |
| WP06 deploy convention                                                                | `scripts/deploy.sh` contains *only* the skip-list entry (lines 580–588 in the diff). No `getent`, `useradd`, `install -m`, `daemon-reload`, `systemctl enable`. WP06 cycle-2 fix correctly applied. |

---

## Final Verdict

**PASS WITH NOTES**

### Rationale

The merged code accurately and completely realizes the spec at the level of FRs (17/20 ADEQUATE by inspection, 2 PARTIAL on log-shape only, 1 DEFERRED to the companion Ansible repo by design). Cross-WP integration boundaries are clean. The cycle-2 corrections to WP02 (go.mod replace + sum), WP05 (`handleNoSignals` shared helper for both zero-paths plus new `all_hits_unmappable` log code), and WP06 (Ansible-only install) are correctly applied in the merged tree.

The PASS WITH NOTES verdict is conditional on **CI being the first executor of `go build` / `go test` / `golangci-lint` / `govulncheck`** on this code. If CI green-lights the merge, the only outstanding items are:

1. **D-1**: drift-detector mapping for `signal-producer/` (low impact; spec is in sync today).
2. **D-4 / R-8**: `run_summary` does not emit receiver-side `skipped`, only mapper-side. Cosmetic.
3. **R-2**: integration test exists but is not invoked by any committed CI step. NFR-004 satisfied in letter, not in spirit.
4. **D-3**: maintenance commit `2214cf61` is acceptable (byte-identical hashes to `auth/go.sum` for spot-checked deps), but not traceable from the mission's WP review history. Document-only finding.
5. **Deployment realization**: FR-018 (production install) is intentionally deferred to a companion `northcloud-ansible` PR per the WP06 cycle-2 convention fix. The mission is "code-complete" but not "deploy-complete" until that PR lands and the first systemd timer fire is verified in journald.

If the first CI run on `main` is red on a Go-toolchain issue (most likely candidate: `go mod tidy` finding the indirect-dep set in `2214cf61` over- or under-shooting the real closure), the verdict reduces to **CONDITIONAL FAIL pending hotfix**.

---

## Open Items (non-blocking)

- [ ] Add `["signal-producer/"]="docs/specs/lead-pipeline.md"` mapping to `tools/drift-detector.sh` (D-1).
- [ ] Add `task signal-producer:test:integration` target with `-tags=integration` and wire to nightly or pre-deploy CI (R-2).
- [ ] Companion `northcloud-ansible` PR for first-deploy enablement (referenced in `signal-producer/deploy/README.md` per WP06 cycle-2 fix).
- [ ] Decide whether `run_summary` should split `skipped_mapper` and `skipped_receiver`, or whether the unused `runResult.skippedRcv` accumulator should be removed (D-4).
- [ ] After first real systemd timer fire, verify the journald summary line and the `/var/lib/signal-producer/checkpoint.json` mode (0640) per success-criterion 3 of the spec.
- [ ] Consider adding `NoNewPrivileges=true`, `ProtectSystem=strict`, `ProtectHome=true`, `PrivateTmp=true` to `signal-producer.service` for defense-in-depth (security note LOW).
- [ ] Consider enforcing `https://` on `WAASEYAA_URL` in `client.New` (security note LOW).

---

## Appendix: File-level audit map

- `signal-producer/internal/producer/checkpoint.go` — 150 LOC, WP02-owned, ConsecutiveEmpty added by WP05 (lines 31–38). Backward-compat verified by JSON zero-default semantics.
- `signal-producer/internal/producer/producer.go` — 420 LOC, WP05-owned. `handleNoSignals` invoked from both `len(hits)==0` (line 165) and `len(signals)==0` (line 245). No third unhandled zero-path.
- `signal-producer/internal/client/waaseyaa.go` — 211 LOC, WP04-owned. `defer resp.Body.Close()` confirmed line 190.
- `signal-producer/internal/client/retry.go` — 80 LOC, WP04-owned. Backoffs `{1s,5s,15s}`, ctx cancel during sleep returns `ctx.Err()`.
- `signal-producer/internal/mapper/mapper.go` — 243 LOC, WP03-owned. Prefix constants `nc-rfp-` / `nc-sig-` distinct; quality_score float64→int safe cast at line 147.
- `signal-producer/cmd/main.go` — 193 LOC, WP05-owned. `os.Getenv("LOG_LEVEL")` (line 67) and `infraconfig.GetConfigPath` (line 51) are the only env reads, both legal under C-002.
- `signal-producer/go.mod` — `replace ../infrastructure` line 66; full transitive closure pre-populated by `2214cf61`.
- `signal-producer/go.sum` — 148 lines; spot-checked zap, multierr, otel-1.41.0 byte-identical to `auth/go.sum`.
- `signal-producer/deploy/signal-producer.service` — `Type=oneshot`, `StateDirectory=signal-producer`, no manual `chown`. Compliant.
- `signal-producer/deploy/signal-producer.timer` — `OnCalendar=*:0/15`, `Persistent=true`, `AccuracySec=30s`. Compliant.
- `scripts/deploy.sh` — only the skip-list entry added (3-line diff, lines 580–588). WP06 cycle-2 fix correctly applied.
- `scripts/detect-changed-services.sh` — `signal-producer` present in `GO_SERVICES` (verified).
- Root `Taskfile.yml` — `lint:signal-producer{,:force}`, `vuln:signal-producer`, root delegations all wired.
- `tools/drift-detector.sh` — **no `signal-producer/` mapping** (D-1).
