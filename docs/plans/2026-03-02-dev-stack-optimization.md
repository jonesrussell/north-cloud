# Dev Stack Optimization Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reduce dev stack from 27 containers / ~13GB RAM to ~20 containers / ~7.5GB RAM by consolidating 7 Postgres instances into 1 and gating ML sidecars behind a profile.

**Architecture:** Dev compose overlay replaces 7 individual Postgres containers with a single `postgres` service that hosts all databases. ML sidecars get `profiles: [ml]` in the base compose. No code changes — only compose and Taskfile modifications. Production compose is untouched.

**Tech Stack:** Docker Compose, PostgreSQL 16, Taskfile

---

### Task 1: Create dev Postgres init script

**Files:**
- Create: `infrastructure/postgres/init-dev.sql`

**Step 1: Write the init script**

```sql
-- Dev-only: creates all service databases in a single Postgres instance.
-- Mounted to /docker-entrypoint-initdb.d/ in docker-compose.dev.yml.
-- Production uses separate Postgres containers per service.

CREATE DATABASE source_manager;
CREATE DATABASE crawler;
CREATE DATABASE classifier;
CREATE DATABASE index_manager;
CREATE DATABASE publisher;
CREATE DATABASE pipeline;
CREATE DATABASE click_tracker;
```

**Step 2: Verify file exists**

Run: `cat infrastructure/postgres/init-dev.sql`
Expected: Shows the CREATE DATABASE statements.

**Step 3: Commit**

```bash
git add infrastructure/postgres/init-dev.sql
git commit -m "feat(dev): add multi-database init script for single Postgres"
```

---

### Task 2: Add `ml` profile to ML sidecars in base compose

**Files:**
- Modify: `docker-compose.base.yml` — add `profiles: [ml]` to all 5 ML sidecar service definitions

**Step 1: Identify the ML sidecar sections**

The 5 ML sidecars are defined in `docker-compose.base.yml`:
- `crime-ml` (search for `crime-ml:`)
- `mining-ml` (search for `mining-ml:`)
- `coforge-ml` (search for `coforge-ml:`)
- `entertainment-ml` (search for `entertainment-ml:`)
- `indigenous-ml` (search for `indigenous-ml:`)

**Step 2: Add profiles to each ML sidecar**

Add this block to each ML sidecar service definition, right after the service name line:

```yaml
    profiles:
      - ml
```

Do NOT change any other properties. Each sidecar should now look like:
```yaml
  crime-ml:
    profiles:
      - ml
    <<: *service-defaults
    # ... rest of existing definition
```

**Step 3: Verify syntax**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config --services | sort`
Expected: ML sidecars should NOT appear in the output (they're behind a profile now).

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml --profile ml config --services | grep ml`
Expected: All 5 ML sidecars listed.

**Step 4: Commit**

```bash
git add docker-compose.base.yml
git commit -m "feat(dev): gate ML sidecars behind 'ml' profile"
```

---

### Task 3: Add single Postgres service to dev compose

**Files:**
- Modify: `docker-compose.dev.yml` — add `postgres` service definition

**Step 1: Add the single Postgres service**

Add this service definition to `docker-compose.dev.yml` in the services section, before the crawler definition:

```yaml
  # ------------------------------------------------------------
  # Single Postgres for all services (Dev only)
  # Replaces 7 per-service Postgres containers from base compose.
  # Production keeps separate instances via docker-compose.prod.yml.
  # ------------------------------------------------------------
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    deploy:
      resources:
        limits:
          cpus: "1.0"
          memory: 1024M
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-postgres}
      POSTGRES_DB: postgres
    volumes:
      - postgres_dev_data:/var/lib/postgresql/data
      - ./infrastructure/postgres/init-dev.sql:/docker-entrypoint-initdb.d/01-init-dev.sql:ro
    ports:
      - "${POSTGRES_DEV_PORT:-5432}:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-postgres}"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - north-cloud-network
```

**Step 2: Add the volume**

Add `postgres_dev_data:` to the `volumes:` section at the bottom of `docker-compose.dev.yml`.

**Step 3: Verify syntax**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config --services | grep postgres`
Expected: Shows `postgres` plus the 7 base `postgres-*` services (we disable those next).

**Step 4: Commit**

```bash
git add docker-compose.dev.yml
git commit -m "feat(dev): add single Postgres service to dev compose"
```

---

### Task 4: Disable per-service Postgres containers in dev compose

**Files:**
- Modify: `docker-compose.dev.yml` — override each `postgres-*` service with scale 0

**Step 1: Add scale-to-zero overrides**

Add this block to `docker-compose.dev.yml` services section (after the new `postgres` definition, before the application services):

```yaml
  # Disable per-service Postgres containers in dev (using single 'postgres' instead)
  postgres-source-manager:
    deploy:
      replicas: 0
  postgres-crawler:
    deploy:
      replicas: 0
  postgres-classifier:
    deploy:
      replicas: 0
  postgres-index-manager:
    deploy:
      replicas: 0
  postgres-publisher:
    deploy:
      replicas: 0
  postgres-pipeline:
    deploy:
      replicas: 0
  postgres-click-tracker:
    deploy:
      replicas: 0
```

**Step 2: Verify**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config --services | grep postgres`
Expected: Shows `postgres`, `postgres-source-manager`, `postgres-crawler`, etc. — but replicas are 0 for the per-service ones.

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config | grep "replicas: 0" | wc -l`
Expected: 7

**Step 3: Commit**

```bash
git add docker-compose.dev.yml
git commit -m "feat(dev): disable per-service Postgres containers via replicas 0"
```

---

### Task 5: Point all services to single Postgres

**Files:**
- Modify: `docker-compose.dev.yml` — update DB host env vars and depends_on for all 7 services

**Step 1: Update each service's DB host and depends_on**

For each service, change the Postgres host env var and `depends_on` reference. The DB name stays the same — only the host changes.

**crawler** (around line 59-61 and 72):
- Change `depends_on` from `postgres-crawler` to `postgres`
- Change `POSTGRES_CRAWLER_HOST: postgres-crawler` to `POSTGRES_CRAWLER_HOST: postgres`

**source-manager** (around line 158-160 and 167):
- Change `depends_on` from `postgres-source-manager` to `postgres`
- Change `DB_HOST: postgres-source-manager` to `DB_HOST: postgres`

**classifier** (around line 212-214 and 221):
- Change `depends_on` from `postgres-classifier` to `postgres`
- Change `POSTGRES_HOST: postgres-classifier` to `POSTGRES_HOST: postgres`

**index-manager** (around line 281-283 and 289):
- Change `depends_on` from `postgres-index-manager` to `postgres`
- Change `POSTGRES_INDEX_MANAGER_HOST: postgres-index-manager` to `POSTGRES_INDEX_MANAGER_HOST: postgres`

**publisher** (around line 596-597 and 604):
- Change `depends_on` from `postgres-publisher` to `postgres`
- Change `POSTGRES_PUBLISHER_HOST: postgres-publisher` to `POSTGRES_PUBLISHER_HOST: postgres`

**pipeline** (around line 652-654 and 657):
- Change `depends_on` from `postgres-pipeline` to `postgres`
- Change `POSTGRES_PIPELINE_HOST: postgres-pipeline` to `POSTGRES_PIPELINE_HOST: postgres`

**click-tracker** (around line 457 and 469-471):
- Change `POSTGRES_CLICK_TRACKER_HOST: postgres-click-tracker` to `POSTGRES_CLICK_TRACKER_HOST: postgres`
- Change `depends_on` from `postgres-click-tracker` to `postgres`

**Step 2: Verify no stale references**

Run: `grep -n 'postgres-crawler\|postgres-source-manager\|postgres-classifier\|postgres-index-manager\|postgres-publisher\|postgres-pipeline\|postgres-click-tracker' docker-compose.dev.yml`
Expected: Only the `deploy: replicas: 0` override block from Task 4. No references in service env vars or depends_on.

**Step 3: Validate compose config**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config > /dev/null`
Expected: No errors.

**Step 4: Commit**

```bash
git add docker-compose.dev.yml
git commit -m "feat(dev): point all services to single Postgres instance"
```

---

### Task 6: Update Taskfile with new profile commands

**Files:**
- Modify: `Taskfile.yml` — add `docker:dev:up:ml` and `docker:dev:up:full` tasks

**Step 1: Find existing docker tasks**

Search for `docker:dev:up` in `Taskfile.yml` to find the existing task definitions and follow the same pattern.

**Step 2: Add new tasks**

Add alongside the existing docker tasks:

```yaml
  docker:dev:up:ml:
    desc: Start dev stack with ML sidecars
    cmds:
      - docker compose -f docker-compose.base.yml -f docker-compose.dev.yml --profile ml up -d

  docker:dev:up:full:
    desc: Start full dev stack (ML + observability)
    cmds:
      - docker compose -f docker-compose.base.yml -f docker-compose.dev.yml --profile ml --profile observability up -d
```

**Step 3: Verify tasks are registered**

Run: `task --list | grep docker:dev`
Expected: Shows `docker:dev:up`, `docker:dev:up:ml`, `docker:dev:up:full`, `docker:dev:up:observability`.

**Step 4: Commit**

```bash
git add Taskfile.yml
git commit -m "feat(dev): add task commands for ML and full profile"
```

---

### Task 7: Test the full dev stack

**Step 1: Stop existing stack and remove old volumes**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down -v`
Expected: All containers stopped, volumes removed.

**Step 2: Start the new dev stack**

Run: `task docker:dev:up`
Expected: Stack starts. Single `postgres` container, no `postgres-*` containers, no ML sidecars.

**Step 3: Verify container count**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps --format "table {{.Name}}\t{{.Status}}" | grep -v "postgres-"`
Expected: ~20 running containers. Single `postgres` healthy. No `postgres-crawler`, `postgres-classifier`, etc.

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps --format "table {{.Name}}\t{{.Status}}" | grep "postgres"`
Expected: Only `north-cloud-postgres-1` (or similar) running. The per-service postgres containers should not appear.

**Step 4: Verify databases created**

Run: `docker exec -it $(docker ps -qf "name=postgres" | head -1) psql -U postgres -l`
Expected: Lists `crawler`, `classifier`, `publisher`, `source_manager`, `index_manager`, `pipeline`, `click_tracker` databases.

**Step 5: Verify services are healthy**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml ps --format "table {{.Name}}\t{{.Status}}" | grep -c healthy`
Expected: All application services show healthy.

Test a few service health endpoints:
```bash
curl -s http://localhost:8060/health  # crawler
curl -s http://localhost:8050/health  # source-manager
curl -s http://localhost:8071/health  # classifier
curl -s http://localhost:8070/health  # publisher
```
Expected: All return 200 OK.

**Step 6: Verify ML profile works**

Run: `task docker:dev:up:ml`
Expected: ML sidecars start (crime-ml, mining-ml, coforge-ml, entertainment-ml, indigenous-ml).

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml --profile ml ps | grep ml`
Expected: All 5 ML sidecars running.

**Step 7: Commit any final adjustments and create PR**

---

### Task 8: Update documentation

**Files:**
- Modify: `CLAUDE.md` — update Docker commands section to mention profiles
- Modify: `DOCKER.md` — add profile documentation (if it exists and is relevant)

**Step 1: Add profile info to CLAUDE.md**

In the Docker commands section, update or add:
```markdown
# Start core services (no ML, no observability) — saves ~5.5GB RAM
task docker:dev:up

# Include ML sidecars (needed for classifier/publisher work)
task docker:dev:up:ml

# Include everything (ML + Loki/Grafana/Pyroscope)
task docker:dev:up:full
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with dev profile commands"
```
