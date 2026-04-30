---
work_package_id: WP04
title: Security and Complexity Fixes
dependencies:
- WP02
- WP03
requirement_refs:
- FR-004
- FR-005
- FR-006
- C-001
- C-002
- C-003
- C-004
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T015
- T016
- T017
- T018
- T019
- T020
agent: "claude:gpt-5:implementer:implementer"
shell_pid: "31168"
history:
- timestamp: '2026-04-30T12:55:00Z'
  agent: claude
  action: created
  note: Initial WP prompt generated from infrastructure lint cleanup mission.
authoritative_surface: infrastructure/sse/
execution_mode: code_change
owned_files:
- infrastructure/sse/broker.go
- infrastructure/sse/client.go
- infrastructure/sse/middleware.go
- infrastructure/sse/types.go
- infrastructure/http/client.go
- infrastructure/gin/builder.go
- infrastructure/gin/config.go
- infrastructure/gin/health.go
- infrastructure/gin/internal_auth.go
- infrastructure/gin/metrics.go
- infrastructure/gin/middleware.go
- infrastructure/gin/server.go
tags: []
---

# WP04 - Security and Complexity Fixes

## Objective

Resolve remaining issue #646 security and complexity findings in owned
infrastructure files after lower-risk constants, profiling, and test hygiene
work is complete.

Implementation command:

```bash
spec-kitty agent action implement WP04 --mission infrastructure-lint-cleanup-01KQDW1C --agent <name>
```

## Context

Research called out the SSE broker context/cancel lifecycle. This WP is the
place for gosec, `nilnil`, `govet`, `nestif`, `exhaustive`, and `gocognit`
findings that are not owned by earlier WPs.

## Subtasks

### T015: Re-run or inspect lint after dependencies

Run the full infrastructure lint command if available:

```bash
cd infrastructure
GOWORK=off golangci-lint run --config ../.golangci.yml ./...
```

Use the output to focus only on remaining owned-file findings.

### T016: Fix SSE broker context/cancel lifecycle

Ensure broker start/stop, client cleanup, and shutdown paths cancel exactly once
and do not leak goroutines or blocked channels.

### T017: Fix gosec/security findings in owned files

Treat gosec findings as real. Prefer explicit timeouts, bounded resources, and
input validation over suppressions.

### T018: Reduce complexity findings

Split complex functions/tests in owned files with small private helpers. Avoid
changing public behavior.

### T019: Fix nilnil, govet, exhaustive, and shadow findings

Apply direct correctness fixes. Where a linter is wrong, use only a narrow
`nolint` with a specific explanation.

### T020: Validate the full infrastructure module

Run:

```bash
cd infrastructure
GOWORK=off go test ./...
GOWORK=off golangci-lint run --config ../.golangci.yml ./...
```

## Definition of Done

- Full infrastructure tests pass or any pre-existing blocker is documented.
- Full infrastructure lint passes where `golangci-lint` is available.
- Security findings are fixed rather than broadly suppressed.
- No unrelated infrastructure refactor is included.

## Reviewer Guidance

Pay close attention to concurrency behavior in SSE and to any `nolint`
comments. They must be narrow and justified.

## Activity Log

- 2026-04-30T13:16:57Z – claude:gpt-5:implementer:implementer – shell_pid=31168 – Started implementation via action command
- 2026-04-30T13:19:29Z – claude:gpt-5:implementer:implementer – shell_pid=31168 – Ready for review: hardened SSE broker start/stop context lifecycle; focused go test -mod=mod ./sse ./http ./gin passes; focused golangci-lint ./sse ./http ./gin reports 0 issues; exact GOWORK=off full-module test/lint remain blocked by existing go mod tidy drift.
