---
work_package_id: WP05
title: Production Deployment
dependencies:
- WP04
requirement_refs:
- FR-008
- FR-009
- C-005
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T024
- T025
- T026
- T027
- T028
agent: "claude:gpt-5:reviewer:reviewer"
shell_pid: "4936"
history:
- timestamp: '2026-04-30T00:00:00Z'
  agent: claude
  action: created
  note: Initial WP prompt generated from enrichment rollout mission.
authoritative_surface: enrichment/deploy/
execution_mode: code_change
owned_files:
- enrichment/deploy/**
- enrichment/docs/**
- docs/**/enrichment*
- docker-compose*.yml
- .env.example
tags: []
---

# WP05 - Production Deployment

## Objective

Add production deployment artifacts and documentation for the enrichment service, including systemd/env expectations and health verification. Changes that belong in `northcloud-ansible` must be captured as companion work or a clearly named follow-up.

Implementation command:

```bash
spec-kitty agent action implement WP05 --mission enrichment-service-rollout-01KQDW17 --agent <name>
```

## Subtasks

### T024: Document runtime configuration

Document required environment variables, default port `8095`, callback behavior, logs, and health endpoint verification.

### T025: Add deploy artifacts in this repo

Add service-local deployment examples or templates under `enrichment/deploy/` without committing real secrets.

### T026: Wire compose or service registration as appropriate

Update repository deployment surfaces only where the north-cloud repo owns them. Keep container/service naming aligned with existing conventions.

### T027: Capture Ansible companion work

If Ansible role changes are required in `northcloud-ansible`, create or update clear handoff documentation in this mission and avoid hiding those changes in this repo.

### T028: Add validation notes

Document local validation, including unavailable tools. Include commands for systemd status, journald logs, and `/health` checks.

## Definition of Done

- Deployment docs/artifacts explain how to run enrichment on port 8095.
- Env templating and secret handling expectations are explicit.
- Any required sibling-repo Ansible changes are clearly captured.
- Health verification and restart-on-failure behavior are documented.

## Validation

Run available local checks:

```bash
task enrichment:build
task enrichment:test
```

If `ansible`, `docker`, or `golangci-lint` are missing, record that limitation and provide the exact deferred validation command.

## Reviewer Guidance

Confirm this WP does not modify Waaseyaa and does not commit secrets. Check whether companion Ansible work is represented truthfully.

## Activity Log

- 2026-04-30T01:05:17Z – claude:gpt-5:implementer:implementer – shell_pid=4592 – Started implementation via action command
- 2026-04-30T01:08:44Z – claude:gpt-5:implementer:implementer – shell_pid=4592 – Ready for review: added enrichment systemd/env deployment artifacts, operator deployment docs, .env.example settings, and companion Ansible handoff notes. Validated build/test/race; lint/vuln blocked only by missing local golangci-lint/govulncheck.
- 2026-04-30T12:17:02Z – claude:gpt-5:reviewer:reviewer – shell_pid=4936 – Started review via action command
