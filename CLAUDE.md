# CLAUDE.md

This file provides quick-reference guidance for Claude Code when working in this repository.
For deep architecture documentation, see `ARCHITECTURE.md`.

---

## Quick Reference

### Most Common Commands

**Docker (Development)**:
```bash
# Start core services only (no Loki/Grafana/Pyroscope)
task docker:dev:up
# Or: docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# Include logging/observability (Loki, Alloy, Grafana, Pyroscope)
task docker:dev:up:observability

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f SERVICE

# Rebuild and restart
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build SERVICE

# Stop all
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

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

# Install dev tools (golangci-lint, air, goimports, migrate)
task install:tools

# Use task lint:force (or task ci:force) before pushing — runs golangci-lint after
# "cache clean" so local results match CI exactly.
```

**Service Development (Manual)**:
```bash
cd SERVICE && go test ./...           # Run tests
cd SERVICE && golangci-lint run       # Lint
cd SERVICE && go run cmd/migrate/main.go up  # Migrations
cd SERVICE && go build -o bin/SERVICE .      # Build
```

**Before Committing**:
1. Run tests: `go test ./...`
2. Run linter: `task lint` (or `task lint:force` before pushing to bypass cache and match CI)
3. Check no magic numbers, `interface{}`, or unchecked JSON errors
4. Check multi-service dependencies if applicable

**Go Workspace Isolation**: Each service Taskfile sets `GOWORK=off` so all Go commands resolve
dependencies from the module's own `go.mod`, not the workspace. The `go.work` file is only for
IDE navigation. Note: A workspace refactor is in progress — see `docs/plans/2026-02-20-idiomatic-go-workspaces.md` for details.

**Vendoring**: Each Go module has its own `vendor/` (gitignored — CI uses the module proxy).
After changing deps, run `task vendor` from the repo root (local only) to refresh all module vendors. Do not run `go work vendor`.

---

## Service Ports

| Service | Port | Purpose |
|---------|------|---------|
| crawler | 8060 | Crawler API |
| source-manager | 8050 | Source Manager API |
| classifier | 8071 | Classifier HTTP API |
| publisher | 8070 | Publisher API |
| auth | 8040 | Authentication |
| index-manager | 8090 | Index Manager API (shares port with search in dev) |
| search | 8092 | Search API (dev), 8090 (prod via nginx) |
| dashboard | 3002 | Dashboard UI |
| nc-http-proxy | 8055 | HTTP Replay Proxy |
| mining-ml | 8077 | Mining ML Classifier |
| anishinaabe-ml | 8080 | Anishinaabe ML Classifier |

ML sidecars (crime-ml, mining-ml, coforge-ml, entertainment-ml, anishinaabe-ml) live under `ml-sidecars/`.

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
  - Import: `infralogger "github.com/north-cloud/infrastructure/logger"`
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

**Service won't start**:
1. Check logs: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs SERVICE`
2. Check environment: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config`
3. Verify dependencies: check `depends_on` in docker-compose files
4. Check port conflicts: `netstat -tulpn | grep PORT`

**Database connection issues**:
1. Verify database running: `docker ps | grep postgres`
2. Check connection string in `.env`
3. Test connection: `docker exec -it north-cloud-postgres-SERVICE psql -U postgres -d DATABASE`

**Cannot access service**:
1. Verify port mapping: `docker ps | grep SERVICE`
2. Check nginx configuration (if using reverse proxy)
3. Verify service health: `curl http://localhost:PORT/health`

---

## Further Reading

- `ARCHITECTURE.md` — Full architecture, service descriptions, content pipeline, version history
- Each service's own `CLAUDE.md` or `README.md` — Service-specific guidelines and API details
