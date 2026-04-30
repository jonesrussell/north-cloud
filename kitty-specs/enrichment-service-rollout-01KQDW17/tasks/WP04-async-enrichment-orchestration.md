---
work_package_id: WP04
title: Async Enrichment Orchestration
dependencies:
- WP02
- WP03
requirement_refs:
- FR-003
- FR-004
- FR-005
- C-001
- C-004
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T019
- T020
- T021
- T022
- T023
agent: "claude:gpt-5:implementer:implementer"
shell_pid: "18684"
history:
- timestamp: '2026-04-30T00:00:00Z'
  agent: claude
  action: created
  note: Initial WP prompt generated from enrichment rollout mission.
authoritative_surface: enrichment/internal/orchestration/
execution_mode: code_change
owned_files:
- enrichment/internal/orchestration/**
tags: []
---

# WP04 - Async Enrichment Orchestration

## Objective

Wire the API handler to the enricher registry and callback client so each valid request returns `202 Accepted` promptly and processes known enrichment types independently in the background.

Implementation command:

```bash
spec-kitty agent action implement WP04 --mission enrichment-service-rollout-01KQDW17 --agent <name>
```

## Subtasks

### T019: Implement orchestration runner

Create a runner that accepts the validated request from WP01, looks up requested enrichers from WP03, and invokes WP02 callback delivery.

### T020: Isolate requested types

Run known requested types independently so one slow or failed enrichment does not prevent other known types from producing callbacks. Unknown types must be logged and skipped.

### T021: Preserve immediate 202 behavior

Ensure the HTTP response is sent after validation, before enrichment or callback work completes. Use context boundaries that do not accidentally cancel background work immediately after the request returns.

### T022: Wire production construction

Connect config, registry, callback client, and server construction from `cmd/main.go` or the existing WP01 extension point with the smallest compatible change.

### T023: Add async/error isolation tests

Cover immediate 202, unknown type skip behavior, one enricher failing while another succeeds, and callback invocation with redacted secret handling.

## Definition of Done

- Valid requests return 202 without waiting for enrichment completion.
- Known requested types are processed independently.
- Unknown requested types do not fail known requested types.
- Callback API keys remain secret in logs and errors.

## Validation

```bash
cd enrichment
go test ./internal/orchestration ./internal/server ./...
```

Run the root service Task targets added in WP01 when available.

## Reviewer Guidance

Confirm all newly defined orchestration code is reachable from the live HTTP path and not only from tests.

## Activity Log

- 2026-04-30T00:56:28Z – claude:gpt-5:implementer:implementer – shell_pid=18684 – Started implementation via action command
- 2026-04-30T00:59:41Z – claude:gpt-5:implementer:implementer – shell_pid=18684 – Ready for review: async runner, type isolation, callback dispatch, ES search adapter, live cmd wiring, and orchestration tests implemented in commit 5eb29977
