---
work_package_id: WP21
title: Deploy Integration
dependencies:
- WP05
requirement_refs:
- C-007
- C-008
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T090
- T091
- T092
- T093
phase: C
agent: "claude:opus:reviewer:reviewers2"
shell_pid: "523397"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: scripts/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- scripts/deploy.sh
- scripts/detect-changed-services.sh
- Taskfile.yml
priority: P1
tags: []
---

# WP21 — Deploy Integration

## Objective

Hook alert-crawler into north-cloud's deploy and CI surfaces: oneshot skip list in `scripts/deploy.sh`, `GO_SERVICES` in `scripts/detect-changed-services.sh`, and `vuln:alert-crawler` delegation in the root `Taskfile.yml`. Repo CLAUDE.md notes these three integration points are the "new Go service checklist" — missing any of them silently breaks CI.

## Context

- Plan §Phased Build Sequence Phase C.3
- Repo CLAUDE.md ("New Go service checklist")
- Spec §5 C-007, C-008

## Branch Strategy

Standard. Parallel-safe with WP20, WP22, WP23.

## Subtasks

### T090 — Add `alert-crawler` to oneshot skip list in `scripts/deploy.sh`

**Purpose**: Tell `deploy.sh` not to wait for a health check on alert-crawler (it's oneshot).

**Steps**:
1. Read `/home/jones/dev/north-cloud/scripts/deploy.sh`. Find the section that lists oneshot services (likely a bash array or a case statement; signal-crawler is in the list per research R-002).
2. Add `alert-crawler` to the skip list following the existing pattern. Example:
   ```bash
   # Skip health checks for these oneshot services:
   ONESHOT_SKIP_SERVICES=("signal-crawler" "rfp-ingestor" "alert-crawler")
   ```
3. If the script uses individual `case` clauses, add the case for alert-crawler matching the signal-crawler pattern.

**Files**:
- `scripts/deploy.sh` (modified).

**Validation**:
- A grep for `alert-crawler` in deploy.sh returns at least one hit.
- Running deploy.sh in dry-run mode (if available) does NOT attempt a health check on alert-crawler.

### T091 — Add `alert-crawler` to `GO_SERVICES`

**Purpose**: CI's changed-service detector must include alert-crawler so PRs touching its files trigger its tests.

**Steps**:
1. Read `/home/jones/dev/north-cloud/scripts/detect-changed-services.sh`. Find the `GO_SERVICES` array.
2. Add `alert-crawler` to the list:
   ```bash
   GO_SERVICES=(
       "auth"
       "classifier"
       "crawler"
       "publisher"
       "search"
       "source-manager"
       "signal-crawler"
       "rfp-ingestor"
       "alert-crawler"
       # ... (preserve existing alphabetization or whatever convention)
   )
   ```

**Files**:
- `scripts/detect-changed-services.sh` (modified).

**Validation**:
- Touch a file under `alert-crawler/` and run the script; assert alert-crawler appears in the changed-services output.

### T092 — Add `vuln:alert-crawler` delegation in root `Taskfile.yml`

**Purpose**: govulncheck CI step must run alert-crawler.

**Steps**:
1. Read `/home/jones/dev/north-cloud/Taskfile.yml`. Find the `vuln:` task.
2. Add a delegation:
   ```yaml
   vuln:alert-crawler:
     desc: Run govulncheck for alert-crawler
     cmds:
       - cd alert-crawler && task vuln
   ```
3. If the root `vuln:` task aggregates per-service tasks, add `task: vuln:alert-crawler` to the dependency list:
   ```yaml
   vuln:
     desc: Run govulncheck for all services
     deps:
       - vuln:auth
       - vuln:classifier
       - vuln:crawler
       - vuln:publisher
       - vuln:search
       - vuln:source-manager
       - vuln:signal-crawler
       - vuln:rfp-ingestor
       - vuln:alert-crawler
   ```

**Files**:
- `Taskfile.yml` (root, modified).

**Validation**:
- `task vuln:alert-crawler` runs `govulncheck ./...` against alert-crawler.
- `task vuln` includes alert-crawler in the run.

### T093 — Verify CI workflow detects and tests alert-crawler

**Purpose**: End-to-end verification that touching alert-crawler files triggers its CI checks.

**Steps**:
1. Open a draft PR with a no-op change in `alert-crawler/` (e.g., a CLAUDE.md typo fix).
2. Observe the CI workflow run.
3. Confirm:
   - The "test" job includes alert-crawler.
   - The "lint" job includes alert-crawler.
   - The "vuln" job includes alert-crawler (if a vuln check runs in CI).
   - The drift-detector does NOT flag the unrelated CLAUDE.md change as drift.
4. Close the draft PR (or convert to a real WP).

**Files**:
- None (manual verification).

**Validation**:
- CI green for the no-op PR.
- Each gate runs alert-crawler.

## Definition of Done

- alert-crawler in deploy.sh skip list.
- alert-crawler in GO_SERVICES.
- vuln:alert-crawler delegation in root Taskfile.
- CI workflow correctly includes alert-crawler on diff.

## Risks

- **Easy to miss one of the three**: repo CLAUDE.md explicitly enumerates all three; reviewer should grep for each.
- **CI yaml workflow files**: if `.github/workflows/*.yml` enumerates services explicitly, those may also need updating. Check during T093.

## Reviewer Guidance

- Grep all three integration points for `alert-crawler`.
- Confirm a draft-PR run exercised each gate.
- If `.github/workflows/*` enumerates services, confirm alert-crawler is included there too.

## Implementation Command

```bash
spec-kitty agent action implement WP21 --agent <name>
```

Depends on WP05.

## Activity Log

- 2026-05-07T13:32:30Z – claude:sonnet:implementer:implementer – shell_pid=520624 – Started implementation via action command
- 2026-05-07T13:35:47Z – claude:sonnet:implementer:implementer – shell_pid=520624 – Ready for review: deploy skip list + changed-services + vuln wiring updated; local validation completed (script + task). CI draft PR check to verify in GitHub.
- 2026-05-07T13:35:57Z – claude:opus:reviewer:reviewers2 – shell_pid=523397 – Started review via action command
