# GitHub Workflow Specification

## Versioning Model

North Cloud uses **milestone-based versioning**. Each milestone groups related issues and PRs into a coherent deliverable. Milestones may represent feature releases (e.g., `v0.6 — GIS + Community Indexing`), focused initiatives (e.g., `Drill Results Pipeline Production Readiness`), or ongoing tracks (e.g., `Tech Debt Cleanup`).

## Active Milestones

| Milestone | Purpose |
|-----------|---------|
| v0.6 — GIS + Community Indexing | Community/GIS data pipeline |
| M1: Smart Extraction | Intelligent content extraction |
| M2: Dynamic Crawling | Browser-rendered page crawling |
| Global Indigenous Source Onboarding | Indigenous news source coverage |
| Tech Debt Cleanup | Code quality and maintenance |
| Drill Results Extraction v1.0 | Mining drill result parsing |
| Source Manager DX | Source management tooling |
| Drill Results Pipeline Production Readiness | Production hardening for drill pipeline |

## Workflow Rules

### Rule 1: Every Issue Gets a Milestone
All open issues must be assigned to a milestone. Untriaged issues (no milestone) are surfaced by the `bin/check-milestones` SessionStart hook.

### Rule 2: Branch Names Reference Issues
Branch format: `claude/{description}-{session-id}` (automated sessions) or `feature/{description}` (manual). PR titles should reference the issue number.

### Rule 3: PRs Close Issues Explicitly
Every PR body must include `Closes #NNN` to auto-close the resolved issue on merge. Multiple issues can be closed: `Closes #1, Closes #2`.

### Rule 4: Milestones Have Due Dates
Active milestones should have due dates set. The `bin/check-milestones` hook flags milestones past their due date as stale.

### Rule 5: Commit Messages Follow Conventional Commits
Format: `type(scope): description` where type is `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `perf`. Scope is the service name or `deploy`/`context`/`deps` for cross-cutting changes.

### Rule 6: Stacked PR Merge Sequence

When a PR's base is another feature branch (not `main`), merging the parent does not auto-retarget dependent PRs. A dependent merged in that state squash-lands the commit onto the now-orphaned parent branch, so no Deploy workflow fires on `main`. The condition is recoverable via cherry-pick, but each occurrence costs an investigation and a re-landing PR.

When follow-on edits are identified on an open PR, pick one of three paths:

1. **Fold fixups into the open PR.** Best when the edit targets a file the parent PR is introducing. Push additional commits to the parent branch and leave a summary comment on the PR so reviewers know what the new commits contain.
2. **Stack a dependent PR, retarget before the parent merges.** Appropriate when the edit does not belong in the parent. Before squash-merging the parent, run `gh pr edit <dep> --base main` on every dependent PR. Then merge the parent, then merge dependents.
3. **Wait for the parent to merge, then branch from `main`.** Simplest sequence. Acceptable when the parent is imminent and the edit is not blocking.

**Worked example (2026-04-19 / PR #672).** PR #672 was open when two follow-on edits were identified. The edits targeted the file #672 was introducing. The fixups were folded into #672 directly as additional commits on the scaffold branch. Stacking separate PRs on that branch would have walked into the retarget footgun the moment #672 squash-merged. Merge commit `cec65bea`.

## Governance Automation

- **SessionStart hook**: Runs `bin/check-milestones` to surface untriaged issues and stale milestones
- **PR template**: `.github/pull_request_template.md` enforces issue ref + milestone checklist
- **Drift detector**: `tools/drift-detector.sh` checks spec freshness against recent commits
