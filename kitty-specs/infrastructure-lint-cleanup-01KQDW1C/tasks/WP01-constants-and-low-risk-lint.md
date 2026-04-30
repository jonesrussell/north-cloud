---
work_package_id: WP01
title: Constants and Low-Risk Lint
dependencies: []
requirement_refs:
- FR-001
- FR-005
- C-001
- C-002
- C-003
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-infrastructure-lint-cleanup-01KQDW1C
base_commit: 216ad02cca5b3448c7dd27196a23872c089ada9c
created_at: '2026-04-30T13:00:34.419520+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
shell_pid: "8900"
agent: "claude:gpt-5:implementer:implementer"
history:
- timestamp: '2026-04-30T12:55:00Z'
  agent: claude
  action: created
  note: Initial WP prompt generated from infrastructure lint cleanup mission.
authoritative_surface: infrastructure/retry/
execution_mode: code_change
owned_files:
- infrastructure/config/types.go
- infrastructure/monitoring/health_handler.go
- infrastructure/monitoring/memory_monitor.go
- infrastructure/retry/retry.go
- infrastructure/sse/options.go
tags: []
---

# WP01 - Constants and Low-Risk Lint

## Objective

Resolve low-risk infrastructure lint findings by replacing repeated duration,
count, and threshold literals with named constants in the owned packages. Keep
behavior identical.

Implementation command:

```bash
spec-kitty agent action implement WP01 --mission infrastructure-lint-cleanup-01KQDW1C --agent <name>
```

## Context

Initial research found likely `mnd` targets in `infrastructure/config/types.go`,
`infrastructure/retry`, `infrastructure/monitoring`, and
`infrastructure/sse/options.go`. Do not broaden this WP into profiling, config
loader, or SSE broker lifecycle changes.

## Subtasks

### T001: Confirm local baseline for owned packages

Run focused tests for owned package surfaces before editing:

```bash
cd infrastructure
GOWORK=off go test ./retry ./monitoring ./sse
```

If the command is blocked by module tidy drift, record the exact output in the
handoff note and continue with source-level cleanup.

### T002: Replace retry literals with named constants

Review `infrastructure/retry/**` for repeated values such as retry counts,
delays, multipliers, or jitter thresholds. Extract constants with names that
describe behavior, not just numeric value.

### T003: Replace monitoring literals with named constants

Review `infrastructure/monitoring/**` for magic durations, thresholds, and
sample counts. Preserve existing defaults and comments.

### T004: Replace SSE option literals with named constants

Review `infrastructure/sse/options.go` for default heartbeat, shutdown,
buffer, and client limits. Extract names only where doing so improves lint and
readability.

### T005: Validate focused packages

Run:

```bash
cd infrastructure
GOWORK=off go test ./retry ./monitoring ./sse
GOWORK=off golangci-lint run --config ../.golangci.yml ./retry ./monitoring ./sse
```

If `golangci-lint` is unavailable, record that limitation and run `go test`.

## Definition of Done

- Owned files have no behavior-changing edits.
- Constants have domain names and are scoped locally where possible.
- No broad `nolint` comments are added.
- Focused tests pass or blockers are recorded exactly.

## Reviewer Guidance

Check that values are unchanged and constants are not exported unless a package
consumer needs them.

## Activity Log

- 2026-04-30T13:00:36Z – claude:gpt-5:implementer:implementer – shell_pid=8900 – Assigned agent via action command
- 2026-04-30T13:06:01Z – claude:gpt-5:implementer:implementer – shell_pid=8900 – Ready for review: named low-risk infrastructure defaults; tests pass; diff-only lint reports 0 issues; full package lint remains blocked by pre-existing out-of-WP findings in config loader and SSE files.
