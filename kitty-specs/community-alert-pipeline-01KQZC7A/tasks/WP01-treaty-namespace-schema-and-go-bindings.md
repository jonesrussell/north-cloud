---
work_package_id: WP01
title: Treaty Namespace Schema and Go Bindings
dependencies: []
requirement_refs:
- FR-009
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-community-alert-pipeline-01KQZC7A
base_commit: bac3ba4be6cd2ed411dd6b45e80720d170b808c3
created_at: '2026-05-06T21:28:40.669472+00:00'
subtasks:
- T001
- T002
- T003
- T004
phase: A
shell_pid: "198739"
agent: "claude:opus:reviewer:reviewer"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: ../indigenous-taxonomy/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- ../indigenous-taxonomy/schema/treaties.yaml
- ../indigenous-taxonomy/generated/go/taxonomy/treaties.go
- ../indigenous-taxonomy/generated/go/taxonomy/treaties_test.go
priority: P1
tags: []
---

# WP01 — Treaty Namespace Schema and Go Bindings

## Objective

Add an additive `Treaty` namespace to the `jonesrussell/indigenous-taxonomy` Go package with 11 numbered-treaty constants (`treaty:1` through `treaty:11`) plus member-region metadata. Establishes the foundation for sovereignty-aware scope resolution in the alert envelope's `scope` field.

This is a cross-repo work package operating on `../indigenous-taxonomy/` (a sibling repo on disk). Other Phase A WPs build on this one via the same generator pipeline.

## Context

- Mission spec: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/spec.md` §3 FR-009, §5 C-003
- Research: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/research.md` §R-004
- Plan: `/home/jones/dev/north-cloud/kitty-specs/community-alert-pipeline-01KQZC7A/plan.md` §Phased Build Sequence Phase A
- The package's own conventions (additive=minor bump; rename/remove=major bump) are documented in `../indigenous-taxonomy/CLAUDE.md`.

## Branch Strategy

- Planning/base branch: `main`
- Merge target branch: `main` (within `../indigenous-taxonomy/`)
- Cross-repo: this WP operates outside the north-cloud monorepo. The agent must commit changes inside `../indigenous-taxonomy/` (separate git repo). A separate PR is opened against that repo's `main`. The lane worktree allocated by spec-kitty's `finalize-tasks` is informational; actual work happens in the sibling repo's checkout.
- Final mission merge into `main` of north-cloud is gated by WP04 (taxonomy v1.1.0 tag).

## Subtasks

### T001 — Create `schema/treaties.yaml`

**Purpose**: Define the 11 numbered-treaty entries with slugs and member-region metadata. The YAML is the source of truth; Go is generated from it.

**Steps**:
1. Inspect existing schema files (`../indigenous-taxonomy/schema/regions.yaml`, `categories.yaml`, `dialects.yaml`) for conventions: top-level key, item shape, comment style.
2. Create `../indigenous-taxonomy/schema/treaties.yaml` with one entry per treaty 1 through 11:
   ```yaml
   treaties:
     - slug: "treaty:1"
       name: "Treaty 1 (Stone Fort Treaty)"
       year: 1871
       member_regions:
         - "canada:manitoba"   # partial
       description: "Areas of southern Manitoba covered by Treaty 1 of 1871."
     # ... treaty:2 through treaty:11
   ```
3. Member-region metadata is informational (used by humans / future generator features); the Go output for v1.1.0 does not consume it. Keep entries factually accurate but conservative (cite Crown-Indigenous Relations Canada material if sources are unclear).
4. Each treaty entry must validate against the YAML pattern used by other schema files.

**Files**:
- `../indigenous-taxonomy/schema/treaties.yaml` (new, ~80 lines).

**Validation**:
- File loads via `yaml.safe_load` in Python (the generator's parser).
- All 11 entries present.
- Slugs follow `treaty:N` (lowercase, colon-separated).

**Edge Cases**:
- Treaties 8 and 11 cross provincial boundaries — list all relevant `canada:*` member regions.
- Avoid asserting completeness of member regions; mark entries as `partial: true` where relevant.

### T002 — Extend `scripts/generate.py` to emit `treaties.go`

**Purpose**: Teach the existing code generator to read `schema/treaties.yaml` and emit a Go file in `generated/go/taxonomy/treaties.go` following the same shape as `regions.go` and `categories.go`.

**Steps**:
1. Read `../indigenous-taxonomy/scripts/generate.py` to understand the existing generator structure.
2. Add a `generate_treaties()` function (analogous to `generate_regions()`) that:
   - Loads `schema/treaties.yaml`.
   - Emits a Go file declaring a `Treaty` string type alias.
   - Emits 11 `Treaty<N>` constants using slugs (e.g., `TreatyArea1 Treaty = "treaty:1"`).
   - Emits an `AllTreaties []Treaty` slice.
   - Emits an `IsValidTreaty(string) bool` lookup function.
3. Wire the function into the generator's main entry so `python scripts/generate.py` produces `treaties.go` alongside the others.
4. Preserve the existing generator's comment header pattern (DO NOT EDIT marker, source YAML reference, generation timestamp).

**Files**:
- `../indigenous-taxonomy/scripts/generate.py` (modified, +~50 lines).

**Validation**:
- Running `python scripts/generate.py` (or whatever the canonical generator command is) produces a valid `treaties.go` file.
- The function is testable in isolation with a mocked YAML input.

**Edge Cases**:
- Generator must produce stable output (deterministic ordering) so `git diff` is meaningful.
- Empty or malformed YAML must produce a clear error, not silent partial output.

### T003 — Generate `taxonomy/treaties.go`

**Purpose**: Run the extended generator to materialize the Go file. This subtask is mechanical (executes the generator from T002) but must produce a clean, lint-passing Go file.

**Steps**:
1. Run the canonical generation command (`python scripts/generate.py` or `make generate`).
2. Verify the output file at `../indigenous-taxonomy/generated/go/taxonomy/treaties.go`.
3. Run `gofmt -w generated/go/taxonomy/treaties.go` and `go vet ./generated/go/taxonomy/...` to confirm cleanliness.
4. Manually inspect the file:
   - Type `Treaty` defined as `type Treaty string`.
   - 11 constants `TreatyArea1` through `TreatyArea11` with the correct slugs.
   - `AllTreaties` slice with all 11 entries in numerical order.
   - `IsValidTreaty(s string) bool` returns true only for known slugs.

**Files**:
- `../indigenous-taxonomy/generated/go/taxonomy/treaties.go` (new, ~80 lines, generated).

**Validation**:
- `go build ./generated/go/taxonomy/...` succeeds.
- `gofmt` produces no diff.
- `go vet` clean.

**Edge Cases**:
- If the generator's output has a `// Code generated...` header, ensure it is preserved on regeneration.

### T004 — Add unit tests for Treaty namespace

**Purpose**: Verify the generated package's `Treaty` namespace via Go tests. Catches future regenerations that drift from expected values.

**Steps**:
1. Create `../indigenous-taxonomy/generated/go/taxonomy/treaties_test.go`.
2. Test that `len(AllTreaties) == 11`.
3. Test that all 11 slugs are present (`treaty:1` through `treaty:11`).
4. Test that `IsValidTreaty("treaty:1")` returns true; `IsValidTreaty("treaty:0")` and `IsValidTreaty("not-a-treaty")` return false.
5. Test that constants and slug strings agree (e.g., `string(TreatyArea1) == "treaty:1"`).
6. Use `t.Helper()` in any helper functions.

**Files**:
- `../indigenous-taxonomy/generated/go/taxonomy/treaties_test.go` (new, ~80 lines).

**Validation**:
- `go test ./generated/go/taxonomy/... -run TestTreaty` passes.
- Coverage of the new file ≥80%.

**Edge Cases**:
- Tests must not depend on slice iteration order (use `slices.Contains` or convert to a set).

## Definition of Done

- All four subtasks complete and committed in `../indigenous-taxonomy/`.
- `python scripts/generate.py` regenerates `treaties.go` deterministically.
- `go build`, `go vet`, `go test ./generated/go/taxonomy/...` all clean.
- The PR (opened in WP04) has been reviewed; this WP merges as part of that PR.

## Risks

- **RR-004**: existing tests in the taxonomy repo may have hardcoded counter assertions (e.g., `len(AllRegions) == 16`). Audit happens in WP04. This WP itself does not modify regions/categories, so RR-004 only applies indirectly via the larger PR.
- **Cross-repo coordination**: the agent must operate in `../indigenous-taxonomy/`. The lane worktree spec-kitty allocates is for the north-cloud repo; the agent should `cd /home/jones/dev/indigenous-taxonomy` and commit there.

## Reviewer Guidance

- Verify treaty slugs match the canonical numbering (Crown-Indigenous Relations Canada).
- Verify the YAML format aligns with the existing `regions.yaml` style.
- Verify the generator extension does not break `regions.go` or `categories.go` generation (regenerate all and `git diff`).
- Verify constants are exported (`TreatyArea1`, not `treatyArea1`).
- Verify `IsValidTreaty` is a deterministic lookup (no map iteration order issues).
- Lint and `go vet` must be clean.

## Implementation Command

```bash
spec-kitty agent action implement WP01 --agent <name>
```

This WP has no prerequisites. Begin immediately. The agent operates primarily in `../indigenous-taxonomy/` for code changes; the spec-kitty lane worktree is informational.

## Activity Log

- 2026-05-06T21:28:46Z – claude:sonnet:implementer:implementer – shell_pid=190941 – Assigned agent via action command
- 2026-05-06T21:35:21Z – claude:sonnet:implementer:implementer – shell_pid=190941 – Ready for review: Treaty namespace v1.1.0 additions committed to indigenous-taxonomy@98bf57c
- 2026-05-06T21:36:45Z – claude:opus:reviewer:reviewer – shell_pid=198739 – Started review via action command
