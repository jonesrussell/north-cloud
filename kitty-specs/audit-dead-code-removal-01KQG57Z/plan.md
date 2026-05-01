# Implementation Plan: Audit — Dead Code Removal

**Branch**: `feat/mission-a-dead-code-removal` | **Date**: 2026-04-30 | **Spec**: [spec.md](./spec.md)
**Input**: `kitty-specs/audit-dead-code-removal-01KQG57Z/spec.md`

## Summary

Delete three top-level directories the 2026-04-30 code-smell audit identified as orphaned (no production consumers, no documentation, no compose entry):

- `dashboard-waaseyaa/` — abandoned PHP/Laravel rewrite of the active Vue 3 `dashboard/` (WP01, issue #697)
- `ml-modules/` and `ml-framework/` — sibling experiments to `ml-sidecars/`, no README, not referenced in classifier client code (WP02, issue #698)
- `playwright-renderer/` — parallel to active `render-worker/`, no compose entry, no consumer (WP03, issue #699)

Per-WP work is mechanical: grep repo-wide for references, confirm zero consumers, `rm -rf` the directory, prune entries in `Taskfile.yml`, `.github/workflows/*`, lint matrix, `lefthook.yml`, `.gitignore`, and root docs (`README.md`, `ARCHITECTURE.md`). Acceptance per WP: `task lint`, `task test`, `task ci`, lefthook pre-push all clean.

WP02 has a conditional branch encoded in spec FR-003: if `ml-modules/` or `ml-framework/` contents prove non-trivial, **do not delete** — fold into `ml-sidecars/` or document the keepers and open a follow-up mission. Decision is made during WP02 execution based on inspection.

## Technical Context

**Language/Version**: Go 1.26+ (target services unchanged), Vue 3 + TypeScript (active `dashboard/` unchanged), Python (active `ml-sidecars/` unchanged)
**Primary Dependencies**: N/A — mission removes existing directories; no new code introduced
**Storage**: N/A
**Testing**: Existing per-service suites (`task test:SERVICE`); full suite (`task test`); CI matrix (`task ci`); lefthook pre-push (`go-test`, `spec-drift`, `ports-ssot`, `layer-check`)
**Target Platform**: Repository structure (file-system layout); no runtime impact expected
**Project Type**: Maintenance — repository cleanup
**Performance Goals**: N/A
**Constraints**:
- No public API change for any active service (`crawler`, `classifier`, `publisher`, `render-worker`, `dashboard`, `ml-sidecars`, `source-manager`, etc.)
- CI gates (lint, test, drift, layers, ports) must remain green after each WP
- One PR per WP per DIR-004 (pending PR #709 merge)
- WP02 conditional: if `ml-modules`/`ml-framework` prove non-orphan, do not delete; document and defer
- No "while-I'm-here" refactoring of adjacent active code (DIR-004)

**Scale/Scope**: 3 top-level directory removals across 3 PRs; 3 WPs total, 1 PR per WP. Estimated diff size per PR: a few hundred deletion lines plus 5–20 lines pruned from `Taskfile.yml`, CI workflows, and root docs.

## Charter Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **DIR-001 (Respect risk boundaries)** ✓ — All three directories are out-of-scope for the content pipeline. `dashboard-waaseyaa` is a consuming-app frontend (Minoo/Waaseyaa territory per the charter). `ml-modules`/`ml-framework` and `playwright-renderer` are orphaned experiments adjacent to active sidecars. Removal aligns with the boundary, does not introduce new ones.
- **DIR-002 (Documentation synchronized)** ✓ — `README.md` and `ARCHITECTURE.md` updates are in scope per FR-005. Spec drift detector (`task drift:check`) must remain green.
- **DIR-003 (Modern Go discipline, pending PR #709)** ✓ — Mission removes code; does not add Go code. No language/framework/architecture rules apply directly. Severity-warn caveat from issue #710 does not affect this mission.
- **DIR-004 (Mission execution discipline, pending PR #709)** ✓ — One PR per WP. No abstractions invented. No adjacent refactors. Spec Kitty ceremony order honored (plan → tasks → tasks-packages → tasks-finalize → implement → review → accept).
- **Quality Gates** ✓ — `task lint`, `task test`, `task drift:check`, `task layers:check`, `task ports:check` all required green per WP. `lefthook pre-push` must pass without `--no-verify`.

No charter violations. No NEEDS CLARIFICATION marks.

## Project Structure

### Documentation (this feature)

```
kitty-specs/audit-dead-code-removal-01KQG57Z/
├── plan.md              # This file
├── research.md          # Phase 0: consumer-search method + audit evidence
├── spec.md              # Mission specification (from 2026-04-30 audit)
├── meta.json            # Mission metadata (target_branch=feat/mission-a-dead-code-removal)
├── status.events.jsonl  # Status event log
├── tasks/               # Phase 2 (NOT created by /spec-kitty.plan): WP01.md, WP02.md, WP03.md
├── checklists/
└── research/
```

`data-model.md`, `contracts/`, `quickstart.md` — **N/A for this mission**. There is no domain model to design (deletion only), no API contracts to publish (no service added or modified), no quickstart to write (operators do not interact with deletion).

### Source Code (repository root)

**Structure Decision**: No new source code is introduced. The mission removes three top-level directories:

```
# Removed by WP01 (#697)
dashboard-waaseyaa/      → deleted

# Removed (or folded into ml-sidecars/) by WP02 (#698)
ml-modules/              → deleted OR folded into ml-sidecars/
ml-framework/            → deleted OR folded into ml-sidecars/

# Removed by WP03 (#699)
playwright-renderer/     → deleted
```

Surrounding tracked files updated per WP (see WP execution prompts for exact greps):

- `Taskfile.yml` — remove per-service tasks for deleted dirs
- `.github/workflows/*.yml` — prune lint/test matrix entries
- `lefthook.yml` — remove hooks tied to deleted dirs (e.g., `dashboard-lint` if it scopes `dashboard-waaseyaa/`)
- `.gitignore` — remove patterns for deleted dirs (if any)
- `README.md` — update service inventory
- `ARCHITECTURE.md` — update layer diagrams or service inventories
- `docs/specs/*.md` — update if any spec references the removed directories
- `scripts/detect-changed-services.sh` — remove deleted dirs from service detection (per CLAUDE.md "New Go service checklist")

Active services (`crawler/`, `classifier/`, `publisher/`, `render-worker/`, `ml-sidecars/`, `dashboard/`, `source-manager/`, etc.) are NOT modified.

## Complexity Tracking

*No charter violations. Table empty.*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| _(none)_ | — | — |
