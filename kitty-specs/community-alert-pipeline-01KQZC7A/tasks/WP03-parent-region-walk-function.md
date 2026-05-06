---
work_package_id: WP03
title: ParentRegion Walk Function
dependencies:
- WP01
- WP02
requirement_refs:
- FR-009
- FR-010
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T009
- T010
- T011
phase: A
agent: "claude:opus:reviewer:reviewer"
shell_pid: "207140"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: ../indigenous-taxonomy/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- ../indigenous-taxonomy/scripts/generate.py
- ../indigenous-taxonomy/generated/go/taxonomy/regions.go
- ../indigenous-taxonomy/generated/go/taxonomy/parent_region_test.go
priority: P1
tags: []
---

# WP03 — ParentRegion Walk Function

## Objective

Extend the indigenous-taxonomy code generator to emit a `ParentRegion(Region) (Region, bool)` lookup function in `regions.go`, sourced from the existing YAML `children:` data. Enables consumers to walk the region hierarchy upward — necessary for alert-crawler's scope resolver to answer "does alert X apply to community Y?" deterministically (FR-010).

## Context

- Mission spec: `kitty-specs/community-alert-pipeline-01KQZC7A/spec.md` §3 FR-009, FR-010
- Research: `kitty-specs/community-alert-pipeline-01KQZC7A/research.md` §R-004 (current package has no `ParentRegion` function; hierarchy is implicit only in slug colons)
- The existing YAML schema **already** carries `children:` data; the generator simply doesn't emit a parent-lookup table. This WP closes that gap.

## Branch Strategy

Same as WP01/WP02 — cross-repo. Depends on WP01 and WP02 for region/treaty data; should run after both have committed their schema changes (otherwise the generated `ParentRegion` table will miss the new entries).

## Subtasks

### T009 — Extend `scripts/generate.py` to emit `ParentRegion` lookup table

**Purpose**: Teach the generator to walk the YAML `children:` structure and produce an inverse map (child → parent) emitted as a Go map literal.

**Steps**:
1. Read `../indigenous-taxonomy/scripts/generate.py`. Find the regions-generation function added or extended in WP02.
2. Add a `build_parent_map(regions_yaml) -> dict[str, str]` helper that walks the `children:` tree and builds a flat dict mapping each child slug to its direct parent slug.
3. Top-level entries (e.g., `canada`) have no parent. The map should NOT contain those keys (the function returns `(false, "")` for unknown lookups).
4. Emit the parent map as a Go `var regionParents = map[Region]Region{...}` declaration, sorted by key for deterministic output.
5. Emit a `ParentRegion(child Region) (Region, bool)` function that wraps the map lookup.

**Files**:
- `../indigenous-taxonomy/scripts/generate.py` (modified, +~40 lines).

**Validation**:
- The generated map is sorted (so `git diff` is stable across regenerations).
- All known children are present in the map.
- Top-level entries are absent from the map.

**Edge Cases**:
- A child appearing under multiple parents (shouldn't happen in valid YAML; if it does, the generator should error explicitly).
- A `children:` entry referencing a slug that doesn't exist as a top-level entry: error with a clear message.

### T010 — Generate `ParentRegion` in `regions.go`

**Purpose**: Run the extended generator to materialize the `ParentRegion` function. Mechanical step.

**Steps**:
1. Run `python scripts/generate.py`.
2. Verify the regenerated `regions.go`:
   - Includes the `regionParents` map literal (alphabetically sorted by key).
   - Includes the `ParentRegion(Region) (Region, bool)` function.
   - Does not break any existing Region constant or function.
3. Run `gofmt -w` and `go vet`.

**Files**:
- `../indigenous-taxonomy/generated/go/taxonomy/regions.go` (regenerated, +~50 lines for the parent map and function).

**Validation**:
- `go build ./generated/go/taxonomy/...` succeeds.
- Sample call works: `ParentRegion("canada:manitoba:winnipeg")` returns `("canada:manitoba", true)`.
- `ParentRegion("canada")` returns `("", false)`.

### T011 — Unit tests for hierarchy walks

**Purpose**: Verify `ParentRegion` returns correct ancestors at every tier of the hierarchy.

**Steps**:
1. Create `../indigenous-taxonomy/generated/go/taxonomy/parent_region_test.go`.
2. Test cases:
   - **Leaf → province**: `ParentRegion("canada:manitoba:winnipeg")` → `("canada:manitoba", true)`.
   - **Community → province**: `ParentRegion("canada:manitoba:sagkeeng-fn")` → `("canada:manitoba", true)`.
   - **Province → canada**: `ParentRegion("canada:manitoba")` → `("canada", true)`.
   - **Top-level → no parent**: `ParentRegion("canada")` → `("", false)`.
   - **Unknown slug**: `ParentRegion("invalid-region")` → `("", false)`.
   - **Deep walk**: `Ancestors("canada:manitoba:winnipeg")` walks up to `canada` (build a small helper that calls ParentRegion in a loop and asserts the chain).
3. Use `t.Helper()` in helpers.
4. Coverage on the changed file ≥80%.

**Files**:
- `../indigenous-taxonomy/generated/go/taxonomy/parent_region_test.go` (new, ~80 lines).

**Validation**:
- `go test ./generated/go/taxonomy/... -run TestParentRegion` passes.

**Edge Cases**:
- Cyclic parent relationships are impossible in a tree-shaped YAML; no test needed.
- Map iteration order doesn't matter for `ParentRegion` (single lookup).

## Definition of Done

- Generator emits a deterministic, sorted parent map.
- `ParentRegion(Region) (Region, bool)` function works for every known slug at every tier.
- Tests cover all hierarchy depths.
- `go build`, `go vet`, `gofmt`, `go test` clean.

## Risks

- **Generator complexity**: easy to introduce a subtle bug that misses a child. Coverage tests across all known children mitigate.
- **Output instability**: if the map is not deterministically sorted, regenerations produce noisy diffs. T009 step 4 explicitly requires sorting.

## Reviewer Guidance

- Verify the parent map is alphabetically sorted by key.
- Verify the function returns `("", false)` for unknown inputs (not a panic, not a misleading default).
- Verify all existing constants in `regions.go` are still present after regeneration.
- Verify the generator extension is testable in isolation (the parent-map builder).

## Implementation Command

```bash
spec-kitty agent action implement WP03 --agent <name>
```

Depends on WP01 and WP02. The agent should wait until those are merged in `../indigenous-taxonomy/` before regenerating, since WP03's regenerated `regions.go` must include all WP02 entries.

## Activity Log

- 2026-05-06T21:45:53Z – claude:sonnet:implementer:implementer – shell_pid=204261 – Started implementation via action command
- 2026-05-06T21:48:28Z – claude:sonnet:implementer:implementer – shell_pid=204261 – Ready for review: ParentRegion walk fn + tests
- 2026-05-06T21:48:59Z – claude:opus:reviewer:reviewer – shell_pid=207140 – Started review via action command
