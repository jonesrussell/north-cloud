# Dev Stack Optimization Design

**Date**: 2026-03-02
**Status**: Approved
**Goal**: Reduce dev stack from 27 containers / ~13GB RAM to ~20 containers / ~7.5GB RAM

---

## Problem

The dev stack runs 7 separate PostgreSQL containers (3.5GB RAM) and 5 ML sidecars (2.5GB RAM) that are rarely all needed simultaneously. On WSL2 with 16GB RAM, this leaves little headroom for IDE, browser, and other tools.

## Solution

Two changes, dev-only (production compose stays unchanged):

### 1. Consolidate PostgreSQL: 7 containers → 1

Replace 7 dedicated Postgres instances with a single `postgres` container that hosts all 7 databases.

**New init script** (`infrastructure/postgres/init-dev.sql`):
```sql
-- Creates all service databases in a single Postgres instance (dev only)
CREATE DATABASE source_manager;
CREATE DATABASE crawler;
CREATE DATABASE classifier;
CREATE DATABASE index_manager;
CREATE DATABASE publisher;
CREATE DATABASE pipeline;
CREATE DATABASE click_tracker;
```

**Compose changes** (`docker-compose.dev.yml`):
- Add single `postgres` service with `init-dev.sql` mounted to `/docker-entrypoint-initdb.d/`
- Remove 7 `postgres-*` services (they remain in `docker-compose.base.yml` for production)
- Point each service's DB host env var to `postgres`:
  - crawler: `POSTGRES_CRAWLER_HOST: postgres`
  - source-manager: `DB_HOST: postgres`
  - classifier: `POSTGRES_HOST: postgres`
  - publisher: `POSTGRES_PUBLISHER_HOST: postgres`
  - index-manager: `POSTGRES_INDEX_MANAGER_HOST: postgres`
  - pipeline: `POSTGRES_PIPELINE_HOST: postgres`
  - click-tracker: `POSTGRES_CLICK_TRACKER_HOST: postgres`
- Each service keeps its own `*_DB` name (e.g., `crawler`, `publisher`) — schema isolation preserved
- Update `depends_on` to reference `postgres` instead of `postgres-{service}`
- Single volume: `postgres_dev_data` instead of 7 separate volumes

**Migration handling**: The existing `run-migrations.sh` + per-service migration mount pattern works unchanged. Each service runs migrations against its own database within the shared instance. Mount all migration directories:
```yaml
volumes:
  - ./source-manager/migrations:/migrations/source-manager:ro
  - ./crawler/migrations:/migrations/crawler:ro
  - ./classifier/migrations:/migrations/classifier:ro
  - ./index-manager/migrations:/migrations/index-manager:ro
  - ./publisher/migrations:/migrations/publisher:ro
  - ./pipeline/migrations:/migrations/pipeline:ro
  - ./click-tracker/migrations:/migrations/click-tracker:ro
```

Or simpler: each service runs its own `go run cmd/migrate/main.go up` at startup as it does today — the migration runs via the service's database connection, not via the Postgres container.

**Resource limits**: Single Postgres gets `1.0 CPU, 1024M` (vs 7 × 0.5 CPU, 512M).

**Savings**: ~2.5GB RAM, 6 fewer containers, faster startup (1 healthcheck instead of 7).

### 2. ML Sidecars Behind `ml` Profile

Add `profiles: [ml]` to all 5 ML sidecar definitions in `docker-compose.base.yml`:
- crime-ml
- mining-ml
- coforge-ml
- entertainment-ml
- indigenous-ml

The classifier already handles ML unavailability gracefully — it logs a warning and falls back to rules-only classification. No code changes needed.

**Usage**:
```bash
# Without ML (default, saves 2.5GB):
task docker:dev:up

# With ML (when working on classifier):
task docker:dev:up:ml
# OR: docker compose -f ... --profile ml up -d
```

**Savings**: 2.5GB RAM, 5 fewer containers when not needed.

### 3. Taskfile Updates

Add new tasks:
```yaml
docker:dev:up:ml:
  desc: Start dev with ML sidecars
  cmds:
    - docker compose -f docker-compose.base.yml -f docker-compose.dev.yml --profile ml up -d

docker:dev:up:full:
  desc: Start everything (ML + observability)
  cmds:
    - docker compose -f docker-compose.base.yml -f docker-compose.dev.yml --profile ml --profile observability up -d
```

## Impact Summary

| Metric | Before | After (default) | After (full) |
|--------|--------|-----------------|--------------|
| Containers | 27 | ~20 | ~27 |
| RAM (est.) | ~13GB | ~7.5GB | ~13GB |
| Postgres instances | 7 | 1 | 1 |
| ML sidecars | 5 (always) | 0 (default) | 5 |
| Startup time | ~90s | ~45s (est.) | ~90s |

## What Doesn't Change

- **Production compose**: Keeps separate Postgres per service (no changes to `docker-compose.prod.yml`)
- **Base compose**: ML sidecars gain `profiles: [ml]` but otherwise unchanged; Postgres services remain defined for production
- **Service code**: No application code changes — only env var values change in compose
- **Database schemas**: Each service still has its own database with its own migrations
- **.env / .env.example**: No changes needed (host defaults are overridden in compose)

## Migration Path (Existing Dev Data)

Switching to the single Postgres requires fresh databases. Options:
1. **Clean start** (recommended): `docker compose down -v` removes all volumes, fresh init
2. **Export/import**: Dump each DB before, import after (overkill for dev data)

## Risks

- **Postgres shared process**: One service's heavy query could impact another. Mitigated by dev workloads being light.
- **ML classifier accuracy in dev**: Without ML sidecars, classification is rules-only. Acceptable for most dev work.
- **Forgetting `--profile ml`**: When testing classifier changes, must remember to start ML. Mitigated by clear Taskfile commands and docs.
