---
work_package_id: WP02
title: Profiling Config Cleanup
dependencies:
  - WP01
requirement_refs:
  - FR-002
  - FR-004
  - FR-006
  - C-001
  - C-002
  - C-004
subtasks:
  - T006
  - T007
  - T008
  - T009
  - T010
authoritative_surface: infrastructure/profiling/
execution_mode: code_change
owned_files:
  - infrastructure/config/loader.go
  - infrastructure/config/validate.go
  - infrastructure/profiling/pprof.go
  - infrastructure/profiling/pyroscope.go
history:
  - timestamp: "2026-04-30T12:55:00Z"
    agent: claude
    action: created
    note: Initial WP prompt generated from infrastructure lint cleanup mission.
---

# WP02 - Profiling Config Cleanup

## Objective

Remove profiling-specific direct environment reads, add explicit pprof server
timeouts, and update direct callers when configuration signatures change.

Implementation command:

```bash
spec-kitty agent action implement WP02 --mission infrastructure-lint-cleanup-01KQDW1C --agent <name>
```

## Context

Research found direct env reads in `infrastructure/profiling/pprof.go` and
`infrastructure/profiling/pyroscope.go`. `infrastructure/config/loader.go`
also reads env as its core responsibility; do not treat that generic config
loader behavior as a smell unless lint requires a narrow change.

## Subtasks

### T006: Inventory profiling callers

Find all direct callers of pprof and pyroscope helpers:

```bash
rg -n "StartPprof|StartPyroscope|ENABLE_PROFILING|PYROSCOPE|PPROF" .
```

Decide whether call sites should pass a config object or explicit parameters.

### T007: Replace profiling env reads with config inputs

Refactor profiling helpers so env lookup happens in service config loading or
call sites, not inside the profiling package. Preserve defaults.

### T008: Add pprof HTTP server timeouts

Replace bare `http.ListenAndServe` with an `http.Server` configured with
explicit read, write, and idle timeouts that satisfy gosec.

### T009: Update callers and tests

Update all direct callers required by signature changes. Keep public API changes
small and document them in code only where the call site is not obvious.

### T010: Validate profiling/config surfaces

Run:

```bash
cd infrastructure
GOWORK=off go test ./profiling ./config
GOWORK=off golangci-lint run --config ../.golangci.yml ./profiling ./config
```

If caller updates touch services outside `infrastructure/`, run their focused
tests too.

## Definition of Done

- Profiling package no longer owns direct env reads except for justified narrow
  exceptions.
- Pprof server has explicit timeouts.
- Direct callers compile and tests pass where tools are available.
- No broad `nolint` comments are added.

## Reviewer Guidance

Check that config defaults match prior runtime behavior and that no profiling
secret/config value is logged unexpectedly.
