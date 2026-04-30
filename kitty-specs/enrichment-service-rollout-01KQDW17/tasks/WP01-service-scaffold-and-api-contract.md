---
work_package_id: WP01
title: Service Scaffold and API Contract
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-007
- FR-008
- C-001
- C-003
- C-004
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-enrichment-service-rollout-01KQDW17
base_commit: 29ef57c8fb6e3a076e70b69039c8d9e0406c25e1
created_at: '2026-04-30T00:29:53.311574+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
- T007
shell_pid: "8596"
agent: "claude:gpt-5:implementer:implementer"
history:
- timestamp: '2026-04-30T00:00:00Z'
  agent: claude
  action: created
  note: Initial WP prompt generated from enrichment rollout mission.
authoritative_surface: enrichment/
execution_mode: code_change
owned_files:
- enrichment/cmd/**
- enrichment/internal/api/**
- enrichment/internal/config/**
- enrichment/internal/server/**
- enrichment/CLAUDE.md
- enrichment/Taskfile.yml
- enrichment/go.mod
- enrichment/go.sum
- Taskfile.yml
tags: []
---

# WP01 - Service Scaffold and API Contract

## Objective

Create the new `enrichment/` Go service scaffold and the HTTP API contract surface. This WP must not implement callback delivery, Elasticsearch enrichment, or async orchestration internals beyond safe interfaces/stubs needed for compilation and tests.

## Context

The service accepts Waaseyaa enrichment requests, validates them, returns `202 Accepted`, and eventually runs async enrichment callbacks in later WPs. Treat Waaseyaa callback URL/API key semantics as external. Do not modify Waaseyaa.

Implementation command:

```bash
spec-kitty agent action implement WP01 --mission enrichment-service-rollout-01KQDW17 --agent <name>
```

## Subtasks

### T001: Create the Go module scaffold

Create `enrichment/go.mod`, `enrichment/cmd/main.go`, `enrichment/internal/`, `enrichment/Taskfile.yml`, and `enrichment/CLAUDE.md`. Match nearby Go service conventions where practical, but keep this service isolated.

### T002: Add configuration loading

Add a small config package that reads service address, port, and timeout settings. Keep direct environment access in `cmd/` or `internal/config/` only. Default the HTTP port to `8095`.

### T003: Define API request and result contract types

Add request structs for `lead_id`, `company_name`, optional `domain`, optional `sector`, `requested_types`, `signals`, `callback_url`, and `callback_api_key`. Add minimal response/error structs and validation errors suitable for handler tests.

### T004: Implement request validation

Validate required fields, callback URL parseability, non-empty `requested_types`, and basic JSON shape. Do not log raw `callback_api_key`. Unknown requested types may be accepted at this layer; WP04 will skip/log them during orchestration.

### T005: Add HTTP handlers

Expose `GET /health` returning HTTP 200 with a compact JSON status object. Expose `POST /api/v1/enrich` that validates input and returns `202 Accepted` after validation. Use an injectable runner/no-op interface so WP04 can wire async work without rewriting validation.

### T006: Add focused tests

Cover health response, malformed JSON, missing required fields, invalid callback URL, empty requested types, and valid request returning 202. Use `httptest`; all helpers must call `t.Helper()`.

### T007: Add Taskfile hooks

Add service-level build/test/lint/vuln targets and root Taskfile hooks such as `enrichment:build` and `enrichment:test` following existing naming patterns. If a tool is unavailable locally, make the target conventional but document local verification limits.

## Definition of Done

- `enrichment/` builds as an isolated Go module.
- `GET /health` and `POST /api/v1/enrich` are covered by unit tests.
- Valid enrichment requests return `202 Accepted` without invoking real callbacks.
- No callback API keys are printed in logs or errors.
- Root and service Taskfile targets exist for build and test.

## Validation

Run from the WP worktree:

```bash
task enrichment:build
task enrichment:test
```

If Task targets are not available yet, run:

```bash
cd enrichment
go test ./...
go test ./... -run Test
```

Record any skipped lint/vuln checks caused by missing local tools.

## Reviewer Guidance

Confirm this WP stays within scaffold/API scope. Callback retry behavior, Elasticsearch queries, and production deployment belong to later WPs.

## Activity Log

- 2026-04-30T00:29:55Z – claude:gpt-5:implementer:implementer – shell_pid=8596 – Assigned agent via action command
- 2026-04-30T00:37:25Z – claude:gpt-5:implementer:implementer – shell_pid=8596 – Ready for review: service scaffold, API validation, health endpoint, tests, and Taskfile hooks implemented in commit 360c6cb5
- 2026-04-30T00:49:31Z – claude:gpt-5:implementer:implementer – shell_pid=8596 – Approved per user request: approve all current for_review WPs
- 2026-04-30T12:46:44Z – claude:gpt-5:implementer:implementer – shell_pid=8596 – Mission accepted and merged locally; marking WP done after acceptance repair. | Done override: Spec Kitty merge already merged and deleted the lane branch after gate evidence passed; acceptance commit 11dfb50c records final acceptance.
