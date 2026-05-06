---
work_package_id: WP02
title: Region YAML Extension (Cities and Sample Communities)
dependencies: []
requirement_refs:
- FR-009
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T005
- T006
- T007
- T008
phase: A
agent: "claude:sonnet:implementer:implementer"
shell_pid: "201197"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: ../indigenous-taxonomy/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- ../indigenous-taxonomy/schema/regions.yaml
- ../indigenous-taxonomy/generated/go/taxonomy/regions_test.go
priority: P1
tags: []
---

# WP02 — Region YAML Extension (Cities and Sample Communities)

## Objective

Extend `schema/regions.yaml` with city-level children under existing province nodes (e.g., `canada:manitoba:winnipeg`) and one First Nations community sample per active treaty area to establish the pattern (e.g., `canada:manitoba:sagkeeng-fn`). Regenerate `regions.go`. This adds the granularity needed for alert-crawler's scope resolution to express community-level routing.

## Context

- Mission spec: `kitty-specs/community-alert-pipeline-01KQZC7A/spec.md` §3 FR-009, FR-010
- Research: `kitty-specs/community-alert-pipeline-01KQZC7A/research.md` §R-004 (existing region count: 16)
- Plan: `kitty-specs/community-alert-pipeline-01KQZC7A/plan.md` §Phased Build Sequence Phase A.2

## Branch Strategy

Same as WP01 — cross-repo work in `../indigenous-taxonomy/`. WP02 can run in parallel with WP01; both extend the schema directory but touch different YAML files.

## Subtasks

### T005 — Extend `schema/regions.yaml` with city children

**Purpose**: Add city-level slugs under the relevant provincial parent so alerts originating from a specific city can be routed precisely.

**Steps**:
1. Read `../indigenous-taxonomy/schema/regions.yaml` to learn the existing structure (uses `children:` keys for hierarchy).
2. Add the following city children under their respective provinces:
   - `canada:manitoba:winnipeg` (under `canada:manitoba`)
   - `canada:ontario:toronto`, `canada:ontario:ottawa` (under `canada:ontario`)
   - `canada:british-columbia:vancouver` (under `canada:british-columbia`)
   - `canada:alberta:calgary` (under `canada:alberta`)
   - `canada:saskatchewan:saskatoon` (under `canada:saskatchewan`)
3. Each entry must follow the existing format (slug, name, optional description).
4. Preserve the YAML's existing ordering and indentation.

**Files**:
- `../indigenous-taxonomy/schema/regions.yaml` (modified, +~30 lines).

**Validation**:
- YAML loads cleanly.
- Hierarchy preserved (city is a child of its province; province is a child of `canada`).

**Edge Cases**:
- If a province does not yet have a `children:` section, add one.
- Do not add cities that are not relevant to v1 mission scope (Toronto/Ottawa/Vancouver/Calgary/Saskatoon/Winnipeg only).

### T006 — Add sample First Nations community per active treaty

**Purpose**: Establish the pattern for community-level slugs (deepest hierarchy tier). v1 ships one sample per treaty area to demonstrate the shape; comprehensive coverage is out of mission scope.

**Steps**:
1. For each active treaty area being introduced in WP01, add one community-level child under the relevant province:
   - `canada:manitoba:sagkeeng-fn` (Sagkeeng First Nation, in Treaty 1 territory)
2. Use accurate community names; cite source where ambiguous.
3. Mark community entries with a tag or convention (e.g., `kind: community`) if the existing schema supports a kind discriminator; if not, the slug naming convention (`...-fn` suffix) is sufficient for v1.
4. The mission spec deliberately limits this to "one sample per active treaty area" to establish the pattern; broader enumeration is in the post-mission backlog.

**Files**:
- `../indigenous-taxonomy/schema/regions.yaml` (modified, +~10 lines).

**Validation**:
- Slug pattern is consistent (`canada:{province}:{community}-fn` for First Nations; future entries can introduce `-mn` for Métis Nations or `-inuit` for Inuit communities).
- Community is correctly nested under its province.

**Edge Cases**:
- Treaty 6 and Treaty 7 cross provinces — pick one province for the sample; document.
- Communities that span treaty territories or are urban Indigenous (not provincially nested) are out of v1 scope.

### T007 — Regenerate `taxonomy/regions.go`

**Purpose**: Run the existing generator to update the Go file with the new constants. Mechanical step.

**Steps**:
1. Run `python scripts/generate.py` (or canonical generator command).
2. Verify the regenerated `../indigenous-taxonomy/generated/go/taxonomy/regions.go` includes:
   - All existing constants unchanged.
   - New constants for each city and community slug.
   - `AllRegions` slice expanded.
3. Run `gofmt -w` and `go vet`.

**Files**:
- `../indigenous-taxonomy/generated/go/taxonomy/regions.go` (regenerated, ~22 entries → ~30 entries).

**Validation**:
- `go build ./generated/go/taxonomy/...` succeeds.
- `gofmt` produces no diff.
- Constants follow PascalCase naming convention used elsewhere in the file.

**Edge Cases**:
- Generator's PascalCase derivation from slugs containing colons and hyphens must produce valid Go identifiers (e.g., `CanadaManitobaWinnipeg`, not `canada:manitoba:winnipeg`).

### T008 — Update region tests

**Purpose**: Make sure existing tests still pass with the new region count and new constants.

**Steps**:
1. Find the existing region test file (likely `../indigenous-taxonomy/generated/go/taxonomy/regions_test.go`).
2. If a hardcoded `len(AllRegions) == N` assertion exists, update it (or refactor to assert "≥ N for known constants" rather than an exact count).
3. Add tests for the new city and community constants:
   - `IsValidRegion("canada:manitoba:winnipeg")` returns true.
   - `IsValidRegion("canada:manitoba:sagkeeng-fn")` returns true.
   - `IsValidRegion("invalid-region")` returns false.
4. Use `t.Helper()` in helpers; coverage ≥80% on changed file.

**Files**:
- `../indigenous-taxonomy/generated/go/taxonomy/regions_test.go` (modified, +~30 lines).

**Validation**:
- `go test ./generated/go/taxonomy/... -run TestRegion` passes.

**Edge Cases**:
- See RR-004: hardcoded counter assertions exist in some tests. WP04 audits broadly; this WP fixes the regions test specifically.

## Definition of Done

- `schema/regions.yaml` extended with cities and sample communities.
- `regions.go` regenerated and clean.
- Tests pass for the new constants.
- Hierarchy is preserved (a city's parent slug is its province; a community's parent is its province via WP03's hierarchy walk).

## Risks

- **RR-004**: hardcoded `len(AllRegions)` assertions. T008 fixes the regions test directly; WP04's audit catches anything else.
- **Slug naming**: `-fn` suffix for First Nations is a v1 convention; if future Métis or Inuit additions use a different suffix, the convention may need formalizing in CLAUDE.md (out of WP02 scope).

## Reviewer Guidance

- Verify YAML hierarchy is correct.
- Verify generated Go file passes `gofmt` and `go vet`.
- Verify tests do not have brittle counter assertions.
- Verify the new sample community is real (citation acceptable).

## Implementation Command

```bash
spec-kitty agent action implement WP02 --agent <name>
```

No prerequisites. Parallel-safe with WP01.

## Activity Log

- 2026-05-06T21:40:10Z – claude:sonnet:implementer:implementer – shell_pid=201197 – Started implementation via action command
- 2026-05-06T21:43:17Z – claude:sonnet:implementer:implementer – shell_pid=201197 – Ready for review: regions.yaml + city/community children
