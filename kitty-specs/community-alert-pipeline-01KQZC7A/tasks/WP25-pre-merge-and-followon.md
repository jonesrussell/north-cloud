---
work_package_id: WP25
title: Pre-Merge Checklist and Post-Merge Follow-On Issue
dependencies:
- WP19
- WP20
- WP21
- WP22
- WP23
- WP24
requirement_refs:
- C-008
- C-009
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T106
- T107
- T108
- T109
- T110
- T111
phase: D
agent: "claude:opus:reviewer:reviewers2"
shell_pid: "603675"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: kitty-specs/community-alert-pipeline-01KQZC7A/
execution_mode: planning_artifact
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- kitty-specs/community-alert-pipeline-01KQZC7A/checklists/pre-merge.md
priority: P1
tags: []
---

# WP25 — Pre-Merge Checklist and Post-Merge Follow-On Issue

## Objective

Run the final pre-merge gates (`task lint:force`, `task test`, `task drift:check`, `task ports:check`, `task layers:check`, lefthook), execute a manual end-to-end smoke test against staging, and file the deferred backlog item for migrating classifier and publisher to import `indigenous-taxonomy` directly.

This is the gating WP — once D.4 passes, the mission is ready to merge to `main`.

## Context

- Plan §Phased Build Sequence Phase D.4, D.5
- Spec §5 C-008, C-009
- Research R-004 (deferred classifier/publisher migration)

## Branch Strategy

Standard. This WP creates a checklist artifact under the mission spec directory and files a GitHub issue (no source-code changes).

## Subtasks

### T106 — `task lint:force` clean

**Purpose**: Final lint gate, cache-clean (matches CI).

**Steps**:
1. From the repo root:
   ```bash
   task lint:force
   ```
2. Address any findings. Repeat until clean.
3. Document the final pass in `kitty-specs/community-alert-pipeline-01KQZC7A/checklists/pre-merge.md`:
   ```markdown
   ## Pre-Merge Checklist

   - [x] task lint:force — clean (date, commit hash)
   ```

**Files**:
- `kitty-specs/community-alert-pipeline-01KQZC7A/checklists/pre-merge.md` (new, growing as subtasks complete).

**Validation**:
- Exit 0.

### T107 — `task test` and integration tests clean

**Purpose**: Unit + integration test suites pass.

**Steps**:
1. Unit tests:
   ```bash
   task test
   ```
2. Integration tests for alert-crawler:
   ```bash
   task test:alert-crawler -- -tags integration
   ```
3. Address any failures. Repeat until clean.
4. Update checklist.

**Validation**:
- Both runs exit 0.

### T108 — `task drift:check`, `task ports:check`, `task layers:check` clean

**Purpose**: Spec/code/architecture alignment gates.

**Steps**:
1. Run all three:
   ```bash
   task drift:check
   task ports:check
   task layers:check
   ```
2. Address any findings. `drift:check` may flag stale specs if WP23's pointer file is missed; `ports:check` may flag if WP20 missed regeneration; `layers:check` may flag if a new package crossed a tier.
3. Update checklist.

**Validation**:
- All three exit 0.

### T109 — Lefthook pre-commit and pre-push pass

**Purpose**: Hook validation as it would run on a fresh clone.

**Steps**:
1. Verify lefthook is installed and configured:
   ```bash
   lefthook install
   ```
2. Trigger pre-commit on a representative file:
   ```bash
   lefthook run pre-commit
   ```
3. Trigger pre-push:
   ```bash
   lefthook run pre-push
   ```
4. Update checklist.

**Validation**:
- Both runs exit 0.
- No `--no-verify` was used.

### T110 — Manual smoke test against staging

**Purpose**: End-to-end verification on real infrastructure (not just CI's harness).

**Steps**:
1. Deploy the mission branch to staging via the existing `gh workflow run deploy.yml` flow OR manual scp+restart procedure (per repo CLAUDE.md).
2. SSH to staging and:
   ```bash
   sudo docker compose -f docker-compose.base.yml -f docker-compose.prod.yml run --rm alert-crawler
   ```
3. Confirm the run produces structured logs at INFO level and exits 0.
4. Query the staging Elasticsearch:
   ```bash
   sudo docker exec north-cloud-elasticsearch-1 curl -s 'localhost:9200/community_alerts/_count?pretty'
   ```
   Confirm a non-zero count (assuming the upstream feed has at least one alert).
5. Subscribe to staging Redis briefly:
   ```bash
   sudo docker exec -it north-cloud-redis-1 redis-cli -a "$REDIS_PASSWORD" SUBSCRIBE community_alerts:lifecycle
   ```
   Trigger another poll cycle; observe events.
6. Update checklist with timestamps and observations.

**Files**:
- `kitty-specs/community-alert-pipeline-01KQZC7A/checklists/pre-merge.md` (modified).

**Validation**:
- Smoke test succeeds.
- No anomalies in production-like environment.

### T111 — File the deferred classifier/publisher migration follow-on issue

**Purpose**: Per AD-007 / D.5, capture the deferred work as a GitHub issue.

**Steps**:
1. Use `gh issue create`:
   ```bash
   gh issue create --title "Migrate classifier and publisher to import indigenous-taxonomy directly" \
     --label "tech-debt,refactor" \
     --body "## Context

   Per research finding R-004 from mission community-alert-pipeline-01KQZC7A, the repo CLAUDE.md mandate \"NC classifier must import category/region slugs from \`jonesrussell/indigenous-taxonomy\`\" is currently aspirational. Classifier and publisher use a hand-rolled shim at \`infrastructure/indigenous/region.go\` with seven continental-scale slugs.

   alert-crawler (the new service introduced by community-alert-pipeline-01KQZC7A) is the FIRST NC service to import the taxonomy package directly (per mission constraint C-003). This follow-on captures the parallel migration work for classifier and publisher.

   ## Scope

   - Replace \`infrastructure/indigenous/region.go\` with imports from \`github.com/jonesrussell/indigenous-taxonomy/generated/go/taxonomy\`.
   - Update \`publisher/internal/router/indigenous.go\` to use the new package's \`Region\` type and \`IsValidRegion\` function instead of the shim's \`AllowedRegions\` map and \`NormalizeRegionSlug\`.
   - Update classifier rules to reference the new region/category constants where applicable.
   - Verify all existing publisher channel routing still works (\`indigenous:category:{slug}\`, \`indigenous:region:{slug}\`).
   - File any new taxonomy gaps as additive PRs to \`jonesrussell/indigenous-taxonomy\` v1.x.

   ## Out of Scope

   - alert-crawler — already imports the package directly (this mission).
   - Removing the \`infrastructure/indigenous/\` package — keep as a stub with deprecation notice in v1; remove in a follow-on cleanup mission.

   ## References

   - kitty-specs/community-alert-pipeline-01KQZC7A/research.md §R-004
   - kitty-specs/community-alert-pipeline-01KQZC7A/spec.md §5 C-003 (Out of Scope)
   - kitty-specs/community-alert-pipeline-01KQZC7A/plan.md §1 Charter Check (Out of Scope)
   "
   ```
2. Capture the issue URL in `kitty-specs/community-alert-pipeline-01KQZC7A/checklists/pre-merge.md`.
3. Link the issue from `kitty-specs/community-alert-pipeline-01KQZC7A/research.md` (under R-004 / RR-005 / RR-006 if applicable).

**Files**:
- GitHub issue (external).
- `kitty-specs/community-alert-pipeline-01KQZC7A/checklists/pre-merge.md` (modified).
- Optional: `kitty-specs/community-alert-pipeline-01KQZC7A/research.md` (link added).

**Validation**:
- Issue is filed with correct labels and body.
- Issue URL is captured in the mission checklist.

## Definition of Done

- All gates green.
- Manual smoke test passes against staging.
- Follow-on GitHub issue filed and linked.
- Pre-merge checklist file complete.
- Mission ready to merge to `main`.

## Risks

- **Late-breaking lint or drift**: discovered in T106/T108. Mitigation: address and re-run; this WP is a gate, not a rush.
- **Staging deployment friction**: T110 may surface issues that didn't appear in CI (e.g., env var mismatches, secrets handling). Address in-place; do not skip the smoke test.

## Reviewer Guidance

- Verify all gates exited 0.
- Verify the follow-on issue body accurately scopes the deferred migration work.
- Verify the smoke test was actually run on staging (not just dry-run; checklist should include a timestamp and observed alert count).

## Implementation Command

```bash
spec-kitty agent action implement WP25 --agent <name>
```

Depends on WP19, WP20, WP21, WP22, WP23, WP24.

## Activity Log

- 2026-05-07T13:42:04Z – claude:sonnet:implementer:implementer – shell_pid=529866 – Started implementation via action command
- 2026-05-07T14:13:25Z – claude:sonnet:implementer:implementer – shell_pid=529866 – T110 staging smoke requires manual execution on staging host; checklist updated with pending item and issue #717 filed
- 2026-05-07T14:14:10Z – claude:sonnet:implementer:implementer – shell_pid=529866 – Ready for review; T110 staging smoke remains pending due unavailable staging/.env in this environment
- 2026-05-07T14:14:12Z – claude:opus:reviewer:reviewers2 – shell_pid=603675 – Started review via action command
