---
work_package_id: WP04
title: Version Bump v1.1.0 and Tag
dependencies:
- WP01
- WP02
- WP03
requirement_refs:
- C-003
- FR-009
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T012
- T013
- T014
- T015
phase: A
agent: "claude:opus:reviewer:reviewer"
shell_pid: "209604"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: ../indigenous-taxonomy/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- ../indigenous-taxonomy/generated/go/taxonomy/version.go
- ../indigenous-taxonomy/go.mod
- ../indigenous-taxonomy/CHANGELOG.md
- ../indigenous-taxonomy/tests/**
priority: P1
tags: []
---

# WP04 — Version Bump v1.1.0 and Tag

## Objective

Audit the existing test suite for hardcoded counter assertions (RR-004), bump `taxonomy/version.go` and `go.mod` to v1.1.0, update the changelog, open the PR for the entire Phase A change set, and tag `v1.1.0` after merge. This WP is the gate that releases the additive vocabulary expansion to consumers — including alert-crawler (which will pin v1.1.0 in WP19).

## Context

- Mission spec: `kitty-specs/community-alert-pipeline-01KQZC7A/spec.md` §5 C-003
- Research: `kitty-specs/community-alert-pipeline-01KQZC7A/research.md` §R-004 (versioning convention: additive=minor bump)
- Plan: `kitty-specs/community-alert-pipeline-01KQZC7A/plan.md` §Phased Build Sequence Phase A.5/A.6

## Branch Strategy

Cross-repo. WP04 is the consolidation step for Phase A — it depends on WP01, WP02, and WP03 having merged into `../indigenous-taxonomy/main`. The PR carries the entire Phase A delta (all four WPs); the `git tag v1.1.0` is applied after the PR merges.

## Subtasks

### T012 — Audit `tests/` for hardcoded counter assertions

**Purpose**: RR-004 mitigation. Find any `len(All*)` style assertions that hardcode the pre-mission count and fix them to either match the new counts or refactor to "contains all known constants" assertions.

**Steps**:
1. Search the entire test surface of `../indigenous-taxonomy/`:
   ```bash
   grep -rE 'len\((All|all_)[A-Z][a-z]+s\)\s*==\s*[0-9]+' ../indigenous-taxonomy/tests/ ../indigenous-taxonomy/generated/
   ```
2. For each match, decide:
   - **Update count**: if the test is "smoke checking the package has roughly this many entries", update the count to the new value.
   - **Refactor**: if the test is asserting specific constants exist, use `slices.Contains` or set membership instead of count equality.
3. Verify all changes by running the full Go test suite:
   ```bash
   cd /home/jones/dev/indigenous-taxonomy && go test ./...
   ```
4. Also check Python tests if any exist (`tests/python/`); apply the same audit.

**Files**:
- Anything found under `../indigenous-taxonomy/tests/` (modified as needed).
- Possibly additional test files in `../indigenous-taxonomy/generated/go/taxonomy/*_test.go`.

**Validation**:
- `go test ./...` passes after all schema and generator changes from WP01–WP03 are merged.
- No counter assertion mentions a pre-mission value (e.g., 16 for regions, 0 for treaties).

**Edge Cases**:
- A test that asserts `AllCategories` has exactly 10 entries — categories are not changed in this mission, so 10 is still correct. Leave alone.

### T013 — Update `taxonomy/version.go` and `go.mod` to v1.1.0

**Purpose**: Reflect the additive change set in the package's stated version. Consumers (alert-crawler in WP19) pin against the released tag.

**Steps**:
1. Find the version constant in `../indigenous-taxonomy/generated/go/taxonomy/version.go`:
   ```go
   const TaxonomyVersion = "1.0.0"
   ```
   Update to `"1.1.0"`.
2. Update `SchemaHash` if the package computes one from YAML content. The hash should change automatically when the schema files change (it's content-derived); verify the new hash is committed.
3. If the generator manages `version.go`, ensure regenerating it produces the bumped version. If `version.go` is hand-edited, the comment header must reflect that.
4. Update `../indigenous-taxonomy/go.mod` if it carries a major-version subdirectory pattern (`v1` already; bump to v1 stays — Go module convention treats v1.x.y as backwards compatible at the major level, so no path change for v1.0 → v1.1).

**Files**:
- `../indigenous-taxonomy/generated/go/taxonomy/version.go` (modified, ~5 lines).
- `../indigenous-taxonomy/go.mod` (modified only if version subdirectory pattern requires).

**Validation**:
- `go build ./...` clean.
- `go list -m` reports the new version locally (post-tag).

### T014 — Update `CHANGELOG.md`

**Purpose**: Record the additive changes for downstream consumers and future maintainers. Required by DIRECTIVE_003 (decision documentation).

**Steps**:
1. Find or create `../indigenous-taxonomy/CHANGELOG.md`.
2. Add a `## v1.1.0 — 2026-05-06` (or whatever date the release lands) section above the prior entries:
   ```markdown
   ## v1.1.0 — 2026-05-06

   ### Added
   - **Treaty namespace**: 11 new constants (`treaty:1` through `treaty:11`) covering Canadian numbered treaties, with `taxonomy.Treaty` type alias and `IsValidTreaty(string) bool` helper.
   - **City-level region children** under provinces: `canada:manitoba:winnipeg`, `canada:ontario:toronto`, `canada:ontario:ottawa`, `canada:british-columbia:vancouver`, `canada:alberta:calgary`, `canada:saskatchewan:saskatoon`.
   - **Sample First Nations community slug**: `canada:manitoba:sagkeeng-fn` (Treaty 1 territory) — establishes the `-fn` suffix pattern for community-level slugs.
   - **`ParentRegion(Region) (Region, bool)`** function: walks the region hierarchy upward, sourced from existing YAML `children:` metadata.

   ### Changed
   - Generator now emits `treaties.go` from `schema/treaties.yaml` and a `regionParents` lookup table from existing YAML `children:` data.

   ### Removed
   - Nothing (additive release).
   ```
3. Reference the upstream consumer if useful: "Released to support `alert-crawler` (north-cloud monorepo, mission community-alert-pipeline-01KQZC7A) for sovereignty-aware scope resolution in community safety alerts."

**Files**:
- `../indigenous-taxonomy/CHANGELOG.md` (modified or created, +~30 lines).

**Validation**:
- Format follows existing changelog style if one exists; otherwise use Keep a Changelog v1.1.0 conventions.

### T015 — Open PR; tag `v1.1.0` after merge

**Purpose**: Land the entire Phase A delta in `../indigenous-taxonomy/main` and tag the release for consumers.

**Steps**:
1. Confirm WP01–WP03 commits are present in the cross-repo branch (or as a stack of branches).
2. Open a PR in `jonesrussell/indigenous-taxonomy` titled `feat: v1.1.0 additive — Treaty namespace, city regions, ParentRegion walk`.
3. PR body should:
   - Summarize the four WPs.
   - Link to the consuming mission (`community-alert-pipeline-01KQZC7A` in north-cloud).
   - Note backward compatibility (no removals, no renames).
   - Include the generated `regions.go` and `treaties.go` diffs as evidence.
4. Resolve any reviewer comments.
5. After merge, tag the release:
   ```bash
   git checkout main
   git pull
   git tag -a v1.1.0 -m "v1.1.0 — additive: Treaty namespace, city regions, sample communities, ParentRegion walk"
   git push origin v1.1.0
   ```
6. Verify the tag is fetchable via `go list -m -versions github.com/jonesrussell/indigenous-taxonomy` (may take a moment for proxy.golang.org to propagate).

**Files**:
- No new files; this subtask is git/PR coordination.

**Validation**:
- `git tag` shows `v1.1.0`.
- `go get github.com/jonesrussell/indigenous-taxonomy@v1.1.0` succeeds from a fresh module (test in a scratch directory).

**Edge Cases**:
- Module proxy caching may delay tag availability; if `go get` fails immediately after pushing, retry after 60s. The Phase B WP19 explicitly waits for the tag to be available.

## Definition of Done

- All four subtasks complete.
- PR merged into `../indigenous-taxonomy/main`.
- `v1.1.0` tag pushed.
- `go get github.com/jonesrussell/indigenous-taxonomy@v1.1.0` succeeds from a clean Go module.
- WP19 (the consumer-side pinning step) is now unblocked.

## Risks

- **RR-004**: hardcoded counter assertions. Audit step T012 mitigates explicitly.
- **Module proxy lag**: rare but possible delay before `proxy.golang.org` indexes the new tag. Mitigation: retry; the WP19 dependency check explicitly handles this.
- **PR review bottleneck**: external repo with its own review cadence. Maintainer is the same person; mitigation: prioritize this PR.

## Reviewer Guidance

- Verify the changelog accurately summarizes all four WPs.
- Verify the `v1.1.0` version is reflected in `version.go` (and `go.mod` if applicable).
- Verify no existing constants were removed or renamed (compare diff against `v1.0.0` tag).
- Verify the test suite is fully green; no skipped tests.
- After merge, verify `go list -m -versions github.com/jonesrussell/indigenous-taxonomy` shows `v1.1.0`.

## Implementation Command

```bash
spec-kitty agent action implement WP04 --agent <name>
```

Depends on WP01, WP02, WP03. The agent should confirm those are merged before opening the consolidation PR. Note: this WP's "merge" happens in the sibling repo, not in north-cloud. Update the spec-kitty mission state by manually marking the WP done after the tag is pushed.

## Activity Log

- 2026-05-06T21:50:55Z – claude:sonnet:implementer:implementer – shell_pid=207988 – Started implementation via action command
- 2026-05-06T21:55:32Z – claude:sonnet:implementer:implementer – shell_pid=207988 – v1.1.0 tagged and indexed by proxy.golang.org
- 2026-05-06T21:56:02Z – claude:opus:reviewer:reviewer – shell_pid=209604 – Started review via action command
- 2026-05-06T21:59:11Z – claude:opus:reviewer:reviewer – shell_pid=209604 – Moved to planned
