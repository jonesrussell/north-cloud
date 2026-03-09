# CLAUDE.md

This file provides quick-reference guidance for Claude Code when working in this repository.
For deep architecture documentation, see `ARCHITECTURE.md`.

---

## Orchestration — Where to Look

When modifying files, read the relevant service CLAUDE.md first. Deep specs in `docs/specs/`.

| File pattern | Service context | Spec |
|---|---|---|
| `crawler/**` | `crawler/CLAUDE.md` | `docs/specs/content-acquisition.md` |
| `classifier/**`, `ml-sidecars/**` | `classifier/CLAUDE.md` | `docs/specs/classification.md` |
| `publisher/**` | `publisher/CLAUDE.md` | `docs/specs/content-routing.md` |
| `search/**`, `index-manager/**` | `search/CLAUDE.md`, `index-manager/CLAUDE.md` | `docs/specs/discovery-querying.md` |
| `infrastructure/**` | — | `docs/specs/shared-infrastructure.md` |
| `source-manager/**` | `source-manager/CLAUDE.md` | — |
| `dashboard/**` | `dashboard/CLAUDE.md` | — |
| `pipeline/**` | `pipeline/CLAUDE.md` | — |
| `social-publisher/**` | `social-publisher/CLAUDE.md` | `docs/specs/social-publisher.md` |
| `rfp-ingestor/**` | `rfp-ingestor/CLAUDE.md` | `docs/specs/rfp-ingestor.md` |
| `mcp-north-cloud/**` | `mcp-north-cloud/CLAUDE.md` | `docs/specs/mcp-server.md` |
| `ai-observer/**` | `ai-observer/CLAUDE.md` | `docs/plans/2026-03-07-ai-observer-design.md` |
| `docs/specs/**`, `.claude/**`, `**/CLAUDE.md` | updating-codified-context | — |
| `docker-compose*.yml`, `Taskfile.yml` | `DOCKER.md` | — |

---

## Content Pipeline Layers

```
Sources → [Crawler] → ES raw_content → [Classifier + ML Sidecars] → ES classified_content
  → [Publisher Router] → Redis channels → [Consumers: Streetcode, Social Publisher]
```

**Publisher routing** (11 layers, evaluated in order):
- L1: Topic auto-detect (skips: mining, indigenous, coforge, recipe, jobs, rfp)
- L2: DB Channels | L3: Crime | L4: Location | L5: Mining | L6: Entertainment | L7: Indigenous | L8: CoForge | L9: Recipe | L10: Job | L11: RFP

**Drift Governor** (within ai-observer, 6h ticker):
- Computes KL divergence, PSI, cross-matrix stability against rolling 7-day baseline
- On threshold breach → LLM analysis → GitHub issue + draft PR with rule patches
- Config: `AI_OBSERVER_DRIFT_ENABLED`, thresholds configurable per-metric

**RFP ingestor** (bypasses classifier — indexes directly to ES):
- Polls CanadaBuys CSV feed → parses → bulk-indexes to `rfp_classified_content` ES index
- Index name uses `*_classified_content` pattern so search service wildcard picks it up
- `content_type` must be `text` (not `keyword`) — search queries `content_type.keyword` sub-field

**Dependency rule**: Services import only from `infrastructure/`. No cross-service imports.

---

## Common Operations

**Add a new source**: Add via source-manager API → crawler picks up on next schedule → raw content indexed → classifier processes → publisher routes

**Add a new ML sidecar**: Create `ml-sidecars/{name}-ml/` with Flask app → add `{name}mlclient` in classifier → add env flag `{NAME}_ENABLED` → add routing layer in publisher → update docker-compose

**Add a new publisher channel**: Create channel via publisher API with topic rules → content matching rules routes to Redis channel → consumers subscribe

**Modify ES mappings**: Update `classifier/internal/elasticsearch/mappings/` → reindex affected indices via index-manager → verify with search queries. **Note**: `content_type` must be `text` type (not `keyword`) — search service queries `content_type.keyword` sub-field which only exists on `text` fields

**Add a migration**: Create up/down SQL in `{service}/internal/database/migrations/` → run `task migrate:SERVICE` → test with `task test:SERVICE`

---

## Quick Reference

### Most Common Commands

**Docker (Development)**:
```bash
# Start core services only (no ML sidecars, no Loki/Grafana/Pyroscope)
task docker:dev:up

# Include ML sidecars (crime-ml, mining-ml, coforge-ml, entertainment-ml, indigenous-ml)
task docker:dev:up:ml

# Include search-service and search-frontend
task docker:dev:up:search

# Include logging/observability (Loki, Alloy, Grafana, Pyroscope)
task docker:dev:up:observability

# Start everything (ML + search + observability)
task docker:dev:up:full

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f SERVICE

# Rebuild and restart
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build SERVICE

# Stop all
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

**Dev Postgres**: Dev uses a single shared Postgres instance (7 databases in 1 container).
Prod and test still use per-service Postgres. First `docker:dev:up` auto-creates all databases
via `infrastructure/postgres/init-dev.sql`. The init script only runs on first startup (empty data
directory). To re-initialize: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down -v`.

**Taskfile Commands (Preferred)**:
```bash
# Run all linters / tests
task lint
task test
task test:cover

# Single service (replace SERVICE with: auth, classifier, crawler, etc.)
task lint:SERVICE
task test:SERVICE
task test:cover:SERVICE

# Task caches results — force re-run with: task lint -f
# Changed-services only (CI): task lint:changed, task test:changed, task ci:changed

# Run migrations
task migrate:up
task migrate:SERVICE

# Install dev tools (golangci-lint, goimports, migrate)
task install:tools

# Use task lint:force (or task ci:force) before pushing — runs golangci-lint after
# "cache clean" so local results match CI exactly.
```

**Go Workspace**: Each service Taskfile sets `GOWORK=off`. The `go.work` is for IDE navigation only.
Each module has its own `vendor/` (gitignored). After changing deps: `task vendor` from repo root.

---

## Service Ports

| Service | Port | Purpose |
|---------|------|---------|
| crawler | 8080 | Crawler API |
| source-manager | 8050 | Source Manager API |
| classifier | 8071 | Classifier HTTP API |
| publisher | 8070 | Publisher API |
| auth | 8040 | Authentication |
| index-manager | 8090 | Index Manager API (shares port with search in dev) |
| search | 8092 | Search API (dev), 8090 (prod via nginx) |
| dashboard | 3002 | Dashboard UI |
| nc-http-proxy | 8055 | HTTP Replay Proxy |
| pipeline | 8075 | Pipeline Event Service |
| click-tracker | 8093 | Click Event Tracking |
| rfp-ingestor | 8095 | RFP Feed Ingestor (CanadaBuys CSV) |
| mining-ml | 8077 | Mining ML Classifier |
| indigenous-ml | 8080 | Indigenous ML Classifier |

ML sidecars (crime-ml, mining-ml, coforge-ml, entertainment-ml, indigenous-ml) live under `ml-sidecars/`.

---

## CRITICAL Rules - YOU MUST Follow

### Before Making Changes

1. **Read first**: Service README.md or CLAUDE.md, files you will modify, existing patterns
2. **Check dependencies**: docker-compose `depends_on`, API integrations, database schemas
3. **Plan multi-service changes**: Identify affected services, determine change order
4. **Understand service boundaries**: Each service is independent with its own database

### Linting Prevention - CRITICAL

**ALWAYS follow these rules. The linter flags violations as errors:**

- **NEVER use `interface{}`** - always use `any` (Go 1.18+)
  - `func Process(data map[string]interface{})` — WRONG
  - `func Process(data map[string]any)` — CORRECT

- **NEVER ignore JSON marshal/unmarshal errors** - always check them
  - `body, _ := json.Marshal(reqBody)` — WRONG
  - `body, err := json.Marshal(reqBody)` followed by error checking — CORRECT

- **NEVER use magic numbers** - always define named constants
  - `make(map[string]any, 4)` — WRONG
  - `make(map[string]any, qualityFactorCount)` where `qualityFactorCount = 4` — CORRECT

- **Pre-allocate slices when capacity is known**
  - `var items []Item` when you know the size — WRONG
  - `items := make([]Item, 0, len(results))` — CORRECT

- **ALL test helper functions MUST start with `t.Helper()`**
  - `func verifyResult(t *testing.T, result Result) { ... }` — WRONG
  - `func verifyResult(t *testing.T, result Result) { t.Helper(); ... }` — CORRECT

- **Keep cognitive complexity <= 20** - break down complex functions into smaller helpers
  - The `gocognit` linter flags functions with complexity > 20 — refactor immediately if flagged

- **Keep function length <= 100 lines** (`funlen` linter) - extract helper functions
  - Example: ES mapping builders use `getCrimeMapping()`, `getMiningMapping()` helpers
  - Example: `classifier.go:Classify()` uses `runOptionalClassifiers()` to stay under limit
  - `func complexFunction() { if a { if b { if c { ... } } } }` — WRONG (high complexity)
  - `func complexFunction() { helperA(); helperB(); helperC() }` with separate helpers — CORRECT

- **Keep lines under 150 characters** - break long lines

- **Avoid variable shadowing** - use `unmarshalErr`, `marshalErr`, etc. for clarity

- **NEVER use `os.Getenv` directly** - use `infrastructure/config` package instead
  - `port := os.Getenv("PORT")` — WRONG
  - Use config struct with `env` tags loaded via `infrastructure/config` — CORRECT
  - The `forbidigo` linter enforces this (exception: `cmd/` and `infrastructure/config/` directories)

**Before committing**: `cd SERVICE && golangci-lint run`

---

## Code Conventions

### Go Services

- **Go Version**: 1.26+ (all services)
- **Standards**: `gofmt`, `goimports`, standard Go formatting
- **Error Handling**: Always wrap errors: `fmt.Errorf("context: %w", err)`
- **Logging**: All services use `infrastructure/logger` package directly
  - Import: `infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"`
  - Always JSON format, structured fields with snake_case
  - Example: `log.Info("Service started", infralogger.String("service", "crawler"))`
- **Database**: Always use context-aware methods (`PingContext()`, `QueryContext()`, etc.)
- **Testing**: Target 80%+ coverage, all helper functions use `t.Helper()`

### Frontend (Vue.js 3)

- **Framework**: Vue 3 Composition API + TypeScript
- **Build**: Vite
- **Type Safety**: No `any` types — use `unknown` for generic values, specific interfaces for known types
- **Types**: Defined in `/dashboard/src/types/` directory

---

## Bootstrap Pattern

All HTTP services follow a consistent bootstrap pattern: simple services (auth, search) use helper
functions in `main.go`; complex services (crawler, classifier, source-manager) use an
`internal/bootstrap/` package with phased modules (config, database, storage, server, lifecycle).
Phase ordering is always: Profiling -> Config -> Logger -> Database -> Services -> Server -> Lifecycle.
See `ARCHITECTURE.md` for the full bootstrap pattern reference.

---

## Docker Conventions

- **ALWAYS use**: `docker compose` (not `docker-compose`)
- **Development**: `-f docker-compose.base.yml -f docker-compose.dev.yml`
- **Production**: `-f docker-compose.base.yml -f docker-compose.prod.yml`
- **Container naming**: `north-cloud-{service}` pattern
- **Database access**: `docker exec -it north-cloud-postgres-SERVICE psql -U postgres -d DATABASE`

### Production Deployment

- Production (`/opt/north-cloud`) is **NOT a git repo** — do not use `git pull`
- CI/CD (GitHub Actions) syncs files via rsync and runs `deploy.sh`
- To deploy manually: push to main → CI runs tests → deploy workflow triggers automatically
- **Nginx uses `--force-recreate`** — volume-mounted config changes aren't detected by `up -d`
- **Force deploy**: `gh workflow run deploy.yml -f force_rebuild_all=true` to rebuild all services

---

## Git Workflow

**Branch Naming**: MUST start with `claude/` and end with the session ID
- Format: `claude/{description}-{session-id}`
- Example: `claude/create-claude-md-01YMXWZpqv3utVH69jyNnLaE`

**Before Committing**:
1. Run tests: `go test ./...`
2. Run linter: `task lint:force` (bypasses cache so local results match CI exactly)
3. Verify no linting violations (see Critical Rules above)
4. Check multi-service dependencies if applicable

**Pushing**: Always use `git push -u origin {branch-name}` — never force push to main

---

## Troubleshooting

Check logs: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs SERVICE`
| Check ports: `netstat -tulpn | grep PORT` | DB test: `docker exec -it north-cloud-postgres-SERVICE psql -U postgres -d DATABASE`
| Health: `curl http://localhost:PORT/health` | See `DOCKER.md` for Docker firewall (UFW) details.

---

## Spec Drift Warning

When refactoring a subsystem, update the relevant service `CLAUDE.md` and (once they exist) `docs/specs/` file. Stale specs cause sessions to generate code conflicting with recent changes.

---

## Further Reading

- `ARCHITECTURE.md` — Full architecture, service descriptions, content pipeline, version history
- Each service's own `CLAUDE.md` or `README.md` — Service-specific guidelines and API details
