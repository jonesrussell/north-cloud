# Research: Audit — Dead Code Removal

**Phase**: 0 (Outline & Research)
**Mission**: `audit-dead-code-removal-01KQG57Z`
**Date**: 2026-04-30

## Purpose

There are no `NEEDS CLARIFICATION` marks in `plan.md`. This document instead records the consumer-search method each WP will execute and the pre-existing audit evidence that justifies treating each directory as orphaned. The actual greps run during WP execution (FR-001/003/004 require per-WP verification before deletion).

## Decision Log

### Decision: delete `dashboard-waaseyaa/` (WP01)

- **Decision**: Remove the directory entirely. Update `Taskfile.yml`, `.github/workflows/*`, `lefthook.yml`, `.gitignore`, `README.md`, `ARCHITECTURE.md` and any `docs/specs/*.md` referencing it.
- **Rationale**: 2026-04-30 audit found a generic Waaseyaa scaffold README, no docs, no integration with any Go service, no entry in `docker-compose.base.yml`. Active dashboard is the Vue 3 app under `dashboard/`. `dashboard-waaseyaa` is an abandoned PHP/Laravel rewrite. Charter DIR-001 explicitly puts consuming-app frontends out of scope for North Cloud.
- **Alternatives considered**: (a) Keep and document — rejected, no documented purpose, no active development. (b) Move to its own repo — rejected, abandoned content with no consumer signals; nobody to inherit. (c) Leave untouched — rejected by audit; treats unmaintained code as live, raises ongoing review and lint cost.

### Decision: triage `ml-modules/` and `ml-framework/` (WP02)

- **Decision**: Inspect contents during WP02. If empty or trivially scaffolded → delete. If populated with intent → fold into `ml-sidecars/` or document and open a follow-up mission. Defer the binary delete-vs-keep call to the implementer based on inspection.
- **Rationale**: 2026-04-30 audit found no README, no documented purpose, not referenced in classifier client code. But these are sibling directories to the active `ml-sidecars/`, which suggests either (a) abandoned predecessors or (b) unfinished extractions. Cannot decide without looking inside.
- **Alternatives considered**: (a) Delete unconditionally — rejected, risks losing in-progress work without inspection. (b) Keep both indefinitely — rejected, undocumented top-level dirs are exactly the audit's target. (c) Open a separate mission for each before any action — rejected as overhead; the single conditional branch in WP02 captures the decision space without ceremony.

### Decision: delete `playwright-renderer/` (WP03)

- **Decision**: Remove the directory entirely. Update `Taskfile.yml`, `.github/workflows/*`, `README.md`, `ARCHITECTURE.md`. Confirm `render-worker/` integration unaffected.
- **Rationale**: 2026-04-30 audit found no README, no `CLAUDE.md`, no compose entry. Active rendering is `render-worker/`. `playwright-renderer/` is a parallel experiment with no production role.
- **Alternatives considered**: (a) Fold into `render-worker/` — rejected, audit found no shared interface or recoverable code. (b) Keep as a future toolbox — rejected, charter forbids speculative scaffolds (DIR-001 risk-boundary clause).

## Consumer-Search Method (per WP)

Each WP's first action is to verify zero consumers before deletion. The grep template (run from repo root):

```bash
DIR="<dashboard-waaseyaa | ml-modules | ml-framework | playwright-renderer>"
grep -rln "$DIR" \
  --exclude-dir="$DIR" \
  --exclude-dir=.git \
  --exclude-dir=node_modules \
  --exclude-dir=vendor \
  --exclude-dir=.worktrees \
  --include="*.go" \
  --include="*.yml" --include="*.yaml" \
  --include="*.md" \
  --include="*.json" \
  --include="*.sh" \
  --include="Taskfile*" \
  --include="Dockerfile*" \
  .
```

Acceptable matches (do not block deletion, but do require pruning in the same WP):

- `Taskfile.yml` task entries naming the directory
- `.github/workflows/*.yml` lint/test matrix members
- `lefthook.yml` hook entries
- `.gitignore` patterns
- `README.md` / `ARCHITECTURE.md` / `docs/specs/*.md` mentions
- `scripts/detect-changed-services.sh` `GO_SERVICES` membership
- `kitty-specs/**` historical references (informational, no action needed)

Blocking matches (stop the WP; escalate to maintainer):

- `import` / `require` / `use` statements in active service code (Go, TypeScript, Python)
- `docker-compose*.yml` `depends_on`, `build.context`, or `volumes` references
- Production deploy scripts (`scripts/deploy.sh`, ansible playbooks, GitHub Actions deploy workflows) referencing the directory
- Migration files or SQL referencing the directory
- Any reference inside the directory's sibling `*-CLAUDE.md` orchestration map (root `CLAUDE.md` table)

If a blocking match is found, the WP halts and the maintainer decides whether the consumer must be migrated first or the deletion deferred to a later mission.

## Pre-Existing Evidence (from spec background)

The 2026-04-30 audit already collected the following evidence per directory; WP execution should verify but need not re-derive:

| Directory | Evidence |
|---|---|
| `dashboard-waaseyaa/` | Generic Waaseyaa scaffold README, no docs, not in `docker-compose.base.yml`, no Go service consumes it |
| `ml-modules/` | Sibling to active `ml-sidecars/`, no README, no documented purpose, not referenced in classifier client code |
| `ml-framework/` | Same as `ml-modules/` |
| `playwright-renderer/` | Parallel to active `render-worker/`, no README, no `CLAUDE.md`, not in compose |

## Risks and Open Questions

| Risk | Mitigation |
|---|---|
| Hidden runtime dependency not surfaced by static grep (e.g., dynamic Docker reference, environment-driven config) | Run `task lint`, `task test`, `task ci` after each deletion. CI must remain green. lefthook pre-push runs spec-drift and layer-check. |
| Production deploy scripts reference deleted dir but are not in repo (e.g., separate ansible repo) | Out of scope for this mission; flag in PR description if any such reference is suspected. |
| `dashboard-waaseyaa` ever reactivated as the dashboard rewrite | If maintainer flags this during review, halt WP01 and re-evaluate. Current audit signal is "dead". |
| Removing `lefthook.yml` `dashboard-lint` hook breaks pre-commit for unrelated changes | Verify hook scope before deletion; if it lints the active `dashboard/` too, just remove `dashboard-waaseyaa/` from its file globs rather than deleting the whole hook. |
| WP02's "fold into ml-sidecars/" branch escalates scope beyond a deletion mission | If folding requires more than mechanical move, halt and open a follow-up mission rather than overrunning WP02. |

## Out of Scope (research-level)

- Investigating why these directories were created in the first place (historical interest only).
- Reviving any directory's contents into a new package.
- Decisions about other non-pipeline services (`ircd`, `mcp-north-cloud`, `signal-*`, etc.) — handled by mission `audit-scope-boundary-decision-01KQG5DX`.
- Splitting handler god-files — handled by mission `audit-handler-god-file-split-01KQG5B6`.

## Phase 1 Artifacts: N/A

`data-model.md`, `contracts/`, `quickstart.md` are not produced for this mission. There is no domain model to design (no entity introduced or modified), no API contracts to publish (no endpoint added), and no quickstart applicable (no operator-facing workflow). The plan section "Project Structure → Source Code" enumerates the file-system changes in lieu of a domain model.
