---
work_package_id: WP20
title: Docker Compose Entry
dependencies:
- WP05
requirement_refs:
- C-007
- C-010
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T086
- T087
- T088
- T089
phase: C
agent: "claude:sonnet:implementer:implementer"
shell_pid: "515304"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: docker-compose.base.yml
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- docker-compose.base.yml
- docs/generated/ports-and-env.md
priority: P1
tags: []
---

# WP20 — Docker Compose Entry

## Objective

Add the `alert-crawler` service to `docker-compose.base.yml` following the oneshot pattern (`restart: "no"`, named volume, no health check). Verify the service brings up cleanly.

## Context

- Plan §Phased Build Sequence Phase C.2
- Spec §5 C-007, C-010
- Research R-002 (signal-crawler compose entry as reference)

## Branch Strategy

Standard. Depends on WP05 (the alert-crawler/ directory + Dockerfile must exist).

## Subtasks

### T086 — Add `alert-crawler` service to `docker-compose.base.yml`

**Purpose**: Wire alert-crawler into the dev/prod compose stack.

**Steps**:
1. Read `/home/jones/dev/north-cloud/signal-crawler/...` and the existing `docker-compose.base.yml` entry for signal-crawler as the reference pattern.
2. Add a service block to `docker-compose.base.yml` at the appropriate location (preserve alphabetical or logical ordering of services):
   ```yaml
   alert-crawler:
     image: ${REGISTRY:-northcloud}/alert-crawler:${TAG:-latest}
     build:
       context: .
       dockerfile: alert-crawler/Dockerfile
     restart: "no"
     networks:
       - north-cloud-network
     volumes:
       - alert-crawler-data:/app/data
     env_file:
       - .env
     environment:
       - DB_PATH=/app/data/state.db
       # Other env vars sourced from .env or defaults via SetDefaults
     depends_on:
       elasticsearch:
         condition: service_healthy
       redis:
         condition: service_healthy
   ```
3. Match the signal-crawler entry's style (no health check, `restart: "no"`).

**Files**:
- `docker-compose.base.yml` (modified, +~25 lines).

### T087 — Add `alert-crawler-data` named volume

**Purpose**: Persistent storage for the SQLite catalogue across container runs.

**Steps**:
1. In `docker-compose.base.yml`, find the `volumes:` top-level block.
2. Add:
   ```yaml
   volumes:
     # ... existing volumes
     alert-crawler-data:
   ```
3. Volume ownership at deploy time is handled by Phase C.4 (Ansible) for production. In dev (`docker compose up`), Docker manages the volume's permissions, but the container user (uid 1000 from WP05's Dockerfile) must be able to write.

**Files**:
- `docker-compose.base.yml` (modified, +1 line).

### T088 — Verify clean bring-up

**Purpose**: Smoke test the compose entry.

**Steps**:
1. Build the image:
   ```bash
   docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build alert-crawler
   ```
2. Run a one-shot:
   ```bash
   docker compose -f docker-compose.base.yml -f docker-compose.dev.yml run --rm alert-crawler
   ```
3. Expected output: alert-crawler boots, does ONE poll cycle, exits 0.
4. Verify the volume persists state:
   ```bash
   docker volume inspect north-cloud_alert-crawler-data
   ```
5. Run a second time:
   ```bash
   docker compose ... run --rm alert-crawler
   ```
   Expected: idempotent run; no spurious lifecycle events (NFR-006).

**Files**:
- None (manual verification).

**Validation**:
- Both runs succeed.
- Volume preserves SQLite state.
- ES `community_alerts` index exists after first run.

### T089 — Run `task ports:check`; regenerate ports SSOT

**Purpose**: Compose changes must regenerate `docs/generated/ports-and-env.md` per repo CLAUDE.md.

**Steps**:
1. Run:
   ```bash
   task ports:generate
   ```
2. Verify the diff in `docs/generated/ports-and-env.md` reflects the new alert-crawler entry.
3. Run:
   ```bash
   task ports:check
   ```
   Should be clean.

**Files**:
- `docs/generated/ports-and-env.md` (regenerated).

**Validation**:
- `task ports:check` exits 0.
- Diff is sensible (one new service entry).

## Definition of Done

- alert-crawler service in docker-compose.base.yml.
- alert-crawler-data volume declared.
- `docker compose ... run --rm alert-crawler` succeeds.
- ports SSOT regenerated and clean.

## Risks

- **Healthcheck mismatch**: oneshot services do NOT have a health check. Don't accidentally copy a health check from a daemon service.
- **`depends_on` precondition**: ES and Redis must be `service_healthy` before alert-crawler runs (otherwise the first poll cycle will fail with connection errors).

## Reviewer Guidance

- Verify the entry mirrors signal-crawler's pattern.
- Verify no health check.
- Verify the volume name is `alert-crawler-data`.
- Verify ports SSOT is regenerated.

## Implementation Command

```bash
spec-kitty agent action implement WP20 --agent <name>
```

Depends on WP05.

## Activity Log

- 2026-05-07T13:30:03Z – claude:sonnet:implementer:implementer – shell_pid=515304 – Started implementation via action command
- 2026-05-07T13:32:22Z – claude:sonnet:implementer:implementer – shell_pid=515304 – Ready for review: compose service + volume added, ports SSOT regenerated, build verified; one-shot run blocked locally by missing .env in worktree
