---
work_package_id: WP03
title: Test Package Hygiene
dependencies:
- WP01
requirement_refs:
- FR-003
- FR-005
- C-001
- C-003
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T011
- T012
- T013
- T014
agent: "claude:gpt-5:implementer:implementer"
shell_pid: "32864"
history:
- timestamp: '2026-04-30T12:55:00Z'
  agent: claude
  action: created
  note: Initial WP prompt generated from infrastructure lint cleanup mission.
authoritative_surface: infrastructure/
execution_mode: code_change
owned_files:
- infrastructure/**/*_test.go
tags: []
---

# WP03 - Test Package Hygiene

## Objective

Move affected infrastructure tests to external `_test` packages or introduce
the smallest possible test seams so lint can pass without weakening behavioral
coverage.

Implementation command:

```bash
spec-kitty agent action implement WP03 --mission infrastructure-lint-cleanup-01KQDW1C --agent <name>
```

## Context

This WP owns infrastructure test files only. If a test needs access to private
behavior, prefer black-box assertions. Add production seams only if unavoidable
and coordinate with the owner WP for that production file.

## Subtasks

### T011: Inventory package-name findings

Run or inspect lint output for `testpackage` findings under `infrastructure/`.
If `golangci-lint` is not installed, search current package declarations:

```bash
rg -n "^package .+$" infrastructure -g "*_test.go"
```

### T012: Convert affected tests to external packages

Rename package declarations from `foo` to `foo_test` where lint requires it.
Update imports to reference the package under test.

### T013: Preserve behavioral coverage

Avoid deleting assertions to make tests compile. If private helpers were tested
directly, rewrite tests around public behavior or document the narrow seam
needed from another WP.

### T014: Validate infrastructure tests

Run:

```bash
cd infrastructure
GOWORK=off go test ./...
```

Record module tidy or missing-tool blockers exactly.

## Definition of Done

- Affected tests use external packages where required.
- Test intent remains equivalent.
- No production code is changed by this WP unless explicitly coordinated.

## Reviewer Guidance

Look for weakened assertions and production code edits outside the WP ownership
surface.


## Activity Log

- 2026-04-30T13:11:07Z – claude:gpt-5:implementer:implementer – shell_pid=32864 – Started implementation via action command
