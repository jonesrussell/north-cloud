# Audit: Dead Code Removal

**Mission**: `audit-dead-code-removal-01KQG57Z`
**Mission type**: software-dev
**Issues covered**: #697, #698, #699
**Target branch**: `main`
**Source**: 2026-04-30 code-smell + architecture audit

## Goal

Remove three dead-or-orphaned directories surfaced by the audit so the repo matches its stated scope and contributors stop tripping over scaffolds that look real but are not. No behavior changes to live services.

## Background

The audit identified three top-level directories with no production role:

- `dashboard-waaseyaa/` — PHP/Laravel rewrite of the active Vue 3 `dashboard/`. Not in `docker-compose.base.yml`, generic Waaseyaa scaffold README, no docs, no integration with any Go service.
- `ml-modules/` and `ml-framework/` — Sibling directories to the active `ml-sidecars/`. No README, no documented purpose, not referenced in classifier client code.
- `playwright-renderer/` — Parallel to the active `render-worker/`. No README, no CLAUDE.md, not in compose.

Each represents a half-finished refactor or experiment that was never concluded.

## Functional Requirements

| ID | Requirement | Priority |
| --- | --- | --- |
| FR-001 | Verify `dashboard-waaseyaa/` has no live consumers (search docker-compose, ops scripts, deploy ansible, README references) before deletion. | Required |
| FR-002 | Remove `dashboard-waaseyaa/` directory and any references in `Taskfile.yml`, `.github/workflows/*`, lint matrix, lefthook hooks, and root docs. | Required |
| FR-003 | Inspect `ml-modules/` and `ml-framework/` contents. If empty/orphaned, delete. If populated with intent, document and either fold into `ml-sidecars/` or open a follow-up mission. | Required |
| FR-004 | Inspect `playwright-renderer/`. If unused (no compose entry, no service consumes its output), delete. Otherwise document and add to compose. | Required |
| FR-005 | Update `README.md`, `ARCHITECTURE.md`, and any `docs/specs/*.md` that reference the removed directories. | Required |
| FR-006 | Update `.gitignore`, lint matrix entries, and CI pipelines so removed paths no longer cause spurious checks. | Required |
| FR-007 | Verify `task lint`, `task test`, `task ci`, and lefthook pre-push pass cleanly after each WP. | Required |

## Constraints

| ID | Constraint | Priority |
| --- | --- | --- |
| C-001 | No changes to live services (`dashboard/`, `render-worker/`, `ml-sidecars/`, classifier integration). | Required |
| C-002 | Each WP is one directory removal; do not bundle deletes into a single commit. Per-WP audit trail matters for revert if surprise consumer surfaces. | Required |
| C-003 | If a directory turns out to be referenced from somewhere unexpected (deploy scripts, ansible, monitoring), pause and record the finding rather than forcing the delete. | Required |
| C-004 | Update charter `directives.yaml` if any DIR-001 implication changes (it should not for these deletes; flag if it does). | Required |
| C-005 | Honor charter rule: force-push to main forbidden. | Required |

## Work Package Plan

1. **WP01 dashboard-waaseyaa removal** (#697): grep for references, confirm no consumers, remove directory, prune Taskfile/CI/lint matrix entries, update docs. Verify CI green.
2. **WP02 ml-modules and ml-framework triage** (#698): inspect contents, classify as orphan/keep, remove orphans, document the keepers (or fold into `ml-sidecars/`). Verify classifier still builds and integration tests pass.
3. **WP03 playwright-renderer removal** (#699): grep for references, confirm no consumers, remove directory, prune Taskfile/CI entries. Verify `render-worker/` integration unaffected.

## Success Criteria

- Three directories removed (or each populated one explicitly retained with a documented reason).
- `task lint`, `task test`, `task ci`, `task layers:check`, `task drift:check`, lefthook pre-push all green on `main`.
- Issues #697, #698, #699 closed with reference to the merge commit.
- README and ARCHITECTURE.md no longer mention removed services.
- Tracked file count drops measurably (audit reported 2,532; goal is to know the new baseline).

## Out of Scope

- Refactoring active dashboard, render-worker, or ml-sidecars code.
- Splitting handler god files (separate mission `audit-handler-god-file-split`).
- Decisions about non-pipeline services like `ircd`, `mcp-north-cloud`, `signal-*` (separate mission `audit-scope-boundary-decision`).
- Resolving the 89 unpushed commits + 150+ uncommitted file state on `main` (issue #707).
