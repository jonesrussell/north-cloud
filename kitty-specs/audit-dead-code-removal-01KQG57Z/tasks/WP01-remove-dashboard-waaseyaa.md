# WP01 — Remove `dashboard-waaseyaa/`

**Mission**: `audit-dead-code-removal-01KQG57Z`
**Issue**: #697
**Spec refs**: FR-001, FR-002, FR-005, FR-006, FR-007
**Dependencies**: none

## Goal

Delete the abandoned `dashboard-waaseyaa/` top-level directory and remove all repo-side references. The directory is the unmodified `waaseyaa/waaseyaa` CMS project skeleton (PHP 8.4, Laravel-flavored). It has no consumer in the North Cloud pipeline, no compose entry, no documented purpose, and was superseded by the active Vue 3 `dashboard/`.

## Pre-deletion verification (FR-001)

Grep performed against `origin/main` (commit `033d39d2`):

```
grep -rln 'dashboard-waaseyaa' . \
  --exclude-dir=dashboard-waaseyaa \
  --exclude-dir=.git \
  --exclude-dir=node_modules \
  --exclude-dir=vendor \
  --exclude-dir=.worktrees \
```

Results outside the target directory:

| File | Category | Action |
|---|---|---|
| `docs/superpowers/plans/2026-03-24-dashboard-waaseyaa-rewrite.md` | Historical SuperPowers planning doc for the abandoned rewrite | **Leave alone** — out of scope per DIR-004 (no while-I'm-here refactors). Possible follow-up cleanup. |
| `docs/superpowers/specs/2026-03-24-dashboard-waaseyaa-rewrite-design.md` | Historical SuperPowers design doc for the abandoned rewrite | Same as above. |
| `kitty-specs/audit-dead-code-removal-01KQG57Z/spec.md` | Mission A spec describing this deletion | **Leave alone** — expected reference. |
| `kitty-specs/audit-dead-code-removal-01KQG57Z/research.md` | Mission A Phase 0 research | **Leave alone** — expected reference. |
| `kitty-specs/audit-dead-code-removal-01KQG57Z/plan.md` | Mission A plan | **Leave alone** — expected reference. |

**Zero blocking matches** — no Go imports, no docker-compose entries, no deploy script references, no `.gitignore` entries, no `Taskfile.yml` tasks, no `.github/workflows/*` matrix entries, no `lefthook.yml` hooks, no `scripts/detect-changed-services.sh` membership, no `README.md`/`ARCHITECTURE.md` mentions.

Active `dashboard/` (Vue 3) is unrelated and untouched.

## Deletion (FR-002, FR-005, FR-006)

- `git rm -rf dashboard-waaseyaa/` — entire 614K / 96-file directory
- No `Taskfile.yml` edit needed (no entries reference the directory)
- No `.github/workflows/*` edit needed (no matrix entries)
- No `lefthook.yml` edit needed (no hooks scoped to it)
- No `.gitignore` edit needed (no patterns)
- No `README.md` edit needed (not listed)
- No `ARCHITECTURE.md` edit needed (not listed)
- No `scripts/detect-changed-services.sh` edit needed (not listed)

The deletion is purely the directory removal. Pruning steps from FR-002/005/006 are no-ops in this case because the audit's "no consumers" hypothesis was correct.

## Verification (FR-007)

After deletion, the following must be green:

- `task lint:changed` — Go lint scoped to changed packages (none changed)
- `task test:changed` — Go test scoped to changed services (none changed)
- `task drift:check` — spec drift detector (no spec changed)
- `task layers:check` — layer-checker (no Go imports changed)
- `task ports:check` — port SSOT (no compose changed)
- `lefthook` pre-commit + pre-push hooks (run automatically on commit/push)

`task ci` is not used in worktrees per CLAUDE.md ("missing Node deps for dashboard"). `task ci:changed` covers the Go-only delta.

## Acceptance

- `dashboard-waaseyaa/` no longer exists in the working tree.
- Every gate above is green on the WP01 branch and again on PR CI.
- PR diff matches the deletion: ~96 file removals, ~0 additions outside this WP01.md.
- Issue #697 closed via `Closes #697` in the PR body.

## Out of scope

- The two SuperPowers historical docs (`docs/superpowers/...`) about the failed rewrite. They reference the deleted directory but are documentation of past work, not live consumers. A separate cleanup mission can address them if desired.
- WP02 (`ml-modules`/`ml-framework` triage) and WP03 (`playwright-renderer` removal) — separate WP branches and PRs.
