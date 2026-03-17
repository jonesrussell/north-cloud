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

## Governance Automation

- **SessionStart hook**: Runs `bin/check-milestones` to surface untriaged issues and stale milestones
- **PR template**: `.github/pull_request_template.md` enforces issue ref + milestone checklist
- **Drift detector**: `tools/drift-detector.sh` checks spec freshness against recent commits
