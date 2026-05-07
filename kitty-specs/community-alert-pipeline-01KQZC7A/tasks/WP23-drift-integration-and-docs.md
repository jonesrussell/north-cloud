---
work_package_id: WP23
title: Drift Integration and Service Docs
dependencies:
- WP05
requirement_refs:
- C-008
- C-009
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T098
- T099
- T100
- T101
phase: C
agent: "claude:sonnet:implementer:implementer"
shell_pid: "525388"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: docs/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- docs/specs/community-alert-pipeline.md
- tools/drift-detector.sh
- alert-crawler/CLAUDE.md
- CLAUDE.md
priority: P1
tags: []
---

# WP23 — Drift Integration and Service Docs

## Objective

Add a `docs/specs/community-alert-pipeline.md` pointer to the mission spec, update the drift-detector to map `alert-crawler/**` to that spec, fill in the alert-crawler service-local CLAUDE.md gotcha sections with concrete content, and update the repo-root CLAUDE.md orchestration table.

## Context

- Plan §Phased Build Sequence Phase C.5–C.7
- Spec §5 C-008, C-009 (DIRECTIVE_003 decision documentation)
- Repo CLAUDE.md (orchestration table format)

## Branch Strategy

Standard. Parallel-safe with WP20, WP21, WP22.

## Subtasks

### T098 — Add `docs/specs/community-alert-pipeline.md`

**Purpose**: Pointer document so `task drift:check` has a concrete spec file to track.

**Steps**:
1. Create `docs/specs/community-alert-pipeline.md`:
   ```markdown
   # Community Alert Pipeline — Spec Pointer

   The authoritative specification for this subsystem lives in the Spec Kitty mission directory:

   - **Spec**: [`kitty-specs/community-alert-pipeline-01KQZC7A/spec.md`](../../kitty-specs/community-alert-pipeline-01KQZC7A/spec.md)
   - **Plan**: [`kitty-specs/community-alert-pipeline-01KQZC7A/plan.md`](../../kitty-specs/community-alert-pipeline-01KQZC7A/plan.md)
   - **Research**: [`kitty-specs/community-alert-pipeline-01KQZC7A/research.md`](../../kitty-specs/community-alert-pipeline-01KQZC7A/research.md)
   - **Data Model**: [`kitty-specs/community-alert-pipeline-01KQZC7A/data-model.md`](../../kitty-specs/community-alert-pipeline-01KQZC7A/data-model.md)
   - **Contracts**: [`kitty-specs/community-alert-pipeline-01KQZC7A/contracts/`](../../kitty-specs/community-alert-pipeline-01KQZC7A/contracts/)

   The `alert-crawler/` service is the implementation of this spec.

   ## Why this pointer exists

   `task drift:check` (per repo CLAUDE.md) maps source-code paths to spec documents. Because the authoritative spec lives in `kitty-specs/`, this pointer file makes the mapping explicit and discoverable for engineers grepping `docs/specs/`.

   ## Update policy

   This file is a pointer. Do NOT duplicate spec content here. Update the spec, and the pointer remains valid.
   ```

**Files**:
- `docs/specs/community-alert-pipeline.md` (new, ~25 lines).

### T099 — Update `tools/drift-detector.sh`

**Purpose**: Map `alert-crawler/**` to the spec file (or the kitty-specs mission). Per repo CLAUDE.md, `task drift:check` runs this script as a CI gate.

**Steps**:
1. Read `/home/jones/dev/north-cloud/tools/drift-detector.sh`. Find the spec-to-paths mapping (likely a bash dictionary or case statement).
2. Add an entry for alert-crawler:
   ```bash
   declare -A SPEC_PATHS=(
     # ... existing entries
     ["docs/specs/community-alert-pipeline.md"]="alert-crawler/**"
   )
   ```
3. Run `task drift:check` against the current branch state. It should be clean (no drift between alert-crawler/ files and the mission spec).

**Files**:
- `tools/drift-detector.sh` (modified, +~3 lines).

**Validation**:
- `task drift:check` exits 0.
- A no-op edit to `alert-crawler/main.go` flags drift relative to the spec (verify by simulation, or accept this as runtime-validated).

### T100 — Write `alert-crawler/CLAUDE.md` (gotchas filled in)

**Purpose**: Replace the WP05-stub TBD sections with concrete content.

**Steps**:
1. Edit `alert-crawler/CLAUDE.md`. Replace each TBD section:

   **`config.yml` overrides `SetDefaults` silently**:
   ```markdown
   `infrastructure/config.LoadWithDefaults[T]` merges YAML BEFORE applying SetDefaults. A non-empty YAML field silently shadows SetDefaults's value. As a result, a code-side default change has no effect if the YAML field carries an old value. Mitigation: keep `config.yml` mostly comments; only uncomment fields you intend to override per-environment. Unit test `internal/config/pitfall_test.go` enforces this.
   ```

   **ES mapping is in-service**:
   ```markdown
   The `community_alerts` ES index is single-writer (alert-crawler is the only producer). Per the resolved charter tension (kitty-specs/community-alert-pipeline-01KQZC7A/plan.md §1, approved 2026-05-06), the mapping lives in `internal/elasticsearch/mapping.json` and is applied via idempotent `EnsureIndex` (HEAD → PUT on 404) at startup. The `index-manager` service is NOT involved. Charter rule "ES mapping changes go through index-manager" applies to shared indices (`*_classified_content` family).
   ```

   **`go.mod` `replace` directive must be removed before merge**:
   ```markdown
   During Phase B (parallel dev with the indigenous-taxonomy v1.1.0 release), `go.mod` carries a `replace github.com/jonesrussell/indigenous-taxonomy => ../indigenous-taxonomy` directive. WP19 removes it and pins `v1.1.0`. CI build fails fast if `replace` is left in.
   ```

   **Rescission semantics**:
   ```markdown
   Parser-degraded items (`parse_quality: degraded`) are persisted as alerts with the flag visible to operators. They are NEVER auto-rescinded due to the parse failure. Rescission only happens when an alert is ABSENT from the upstream feed on a subsequent poll (catalogue diff in runner). This is per TC-010.

   Parser-failed items (`parse_quality: failed`) — required fields missing — are SKIPPED entirely and never enter the catalogue. Failed parses emit a `parse_failure_total` metric.
   ```

   **SQLite catalogue rebuild path**:
   ```markdown
   On startup, if `data/state.db` is missing or empty, alert-crawler can rebuild the active-alert catalogue from ES (`lifecycle_state == "active"` query). The rebuild is idempotent and does NOT emit `created` events for the rebuilt entries (they already exist downstream). Rebuild is triggered by passing `--rebuild` flag or auto-detected from row count == 0.
   ```

   **Docker volume ownership**:
   ```markdown
   Container runs as uid 1000 (`alert-crawler` user inside the image). Production data dir `/home/deployer/north-cloud/alert-crawler/data` MUST be owned by uid 1000, NOT `deploy_user` (uid 1001). Ansible task in `northcloud-ansible/roles/north-cloud/tasks/alert-crawler.yml` sets `owner: "1000"` explicitly. PR-004 mitigation.
   ```

   Add a new gotcha:
   **First-deploy backfill burst**:
   ```markdown
   Backfill subcommand emits up to 20 `created` lifecycle events on first deploy (TC-011). Downstream consumers must handle this burst (rate-limit consumer-side if needed). Out of mission scope; flagged as PR-002.
   ```

**Files**:
- `alert-crawler/CLAUDE.md` (rewritten, ~120 lines).

### T101 — Update repo-root `CLAUDE.md` orchestration table

**Purpose**: Add `alert-crawler/**` row to the orchestration table at the top of `/home/jones/dev/north-cloud/CLAUDE.md`.

**Steps**:
1. Edit `/home/jones/dev/north-cloud/CLAUDE.md`. Find the orchestration table.
2. Add a row:
   ```markdown
   | `alert-crawler/**` | `alert-crawler/CLAUDE.md` | `kitty-specs/community-alert-pipeline-01KQZC7A/spec.md` (canonical) — pointer at `docs/specs/community-alert-pipeline.md` |
   ```
3. Preserve alphabetical or logical ordering of existing rows.
4. If the repo-CLAUDE.md "Architecture" diagram or services list mentions all services explicitly, add alert-crawler there too.

**Files**:
- `CLAUDE.md` (root, modified, +~3 lines).

**Validation**:
- Repo CLAUDE.md correctly references the new service.
- Architectural diagrams (if any) include alert-crawler.

## Definition of Done

- `docs/specs/community-alert-pipeline.md` exists as a pointer.
- Drift-detector mapping includes alert-crawler.
- `task drift:check` clean.
- alert-crawler/CLAUDE.md gotchas are filled in (no TBD).
- Repo-root CLAUDE.md orchestration table includes alert-crawler.

## Risks

- **Drift mapping false-positives**: if the drift-detector flags benign changes (formatting, comments), tune the mapping. Out of v1 scope; track separately if it happens.
- **CLAUDE.md drift**: the mission's authoritative spec evolves; the pointer file does NOT duplicate content. Verify reviewers understand the indirection.

## Reviewer Guidance

- Verify the pointer file does NOT duplicate spec content.
- Verify drift-detector mapping is added.
- Verify CLAUDE.md gotchas are concrete (no remaining TBD).
- Verify repo-root CLAUDE.md table is consistent.

## Implementation Command

```bash
spec-kitty agent action implement WP23 --agent <name>
```

Depends on WP05.

## Activity Log

- 2026-05-07T13:37:41Z – claude:sonnet:implementer:implementer – shell_pid=525388 – Started implementation via action command
- 2026-05-07T13:39:25Z – claude:sonnet:implementer:implementer – shell_pid=525388 – Ready for review: pointer spec + drift mapping + alert-crawler/root CLAUDE updates; drift check clean
