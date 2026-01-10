# CLAUDE.md - AI Assistant Guide for North Cloud

**IMPORTANT**: This file is automatically loaded into Claude's context. It's tuned for effectiveness - most critical information is at the top. Follow the structure: read what you need, not everything.

## Quick Reference

### Most Common Commands

**Docker (Development)**:
```bash
# Start all services
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f SERVICE

# Rebuild and restart
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build SERVICE

# Stop all
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

**Service Development**:
```bash
# Run tests
cd SERVICE && go test ./...

# Lint
cd SERVICE && golangci-lint run

# Run migrations
cd SERVICE && go run cmd/migrate/main.go up

# Build
cd SERVICE && go build -o bin/SERVICE main.go
```

**Before Committing**:
1. Run tests: `go test ./...`
2. Run linter: `golangci-lint run`
3. Check no magic numbers, `interface{}`, or unchecked JSON errors

### Service Ports

| Service | Port | Purpose |
|---------|------|---------|
| crawler | 8060 | Crawler API |
| source-manager | 8050 | Source Manager API |
| classifier | 8070 | Classifier HTTP API |
| publisher | 8070 | Publisher API |
| auth | 8040 | Authentication |
| index-manager | 8090 | Index Manager API |
| search | 8090 | Search API |
| dashboard | 3002 | Dashboard UI |

---

## CRITICAL Rules - YOU MUST Follow

### Linting Prevention - CRITICAL

**ALWAYS follow these rules. The linter flags violations as errors:**

- **NEVER use `interface{}`** - always use `any` (Go 1.18+)
  - ❌ `func Process(data map[string]interface{})`
  - ✅ `func Process(data map[string]any)`

- **NEVER ignore JSON marshal/unmarshal errors** - always check them
  - ❌ `body, _ := json.Marshal(reqBody)`
  - ✅ `body, err := json.Marshal(reqBody)` followed by error checking

- **NEVER use magic numbers** - always define named constants
  - ❌ `make(map[string]any, 4)`
  - ✅ `make(map[string]any, qualityFactorCount)` where `qualityFactorCount = 4`

- **Pre-allocate slices when capacity is known**
  - ❌ `var items []Item` when you know the size
  - ✅ `items := make([]Item, 0, len(results))`

- **ALL test helper functions MUST start with `t.Helper()`**
  - ❌ `func verifyResult(t *testing.T, result Result) { ... }`
  - ✅ `func verifyResult(t *testing.T, result Result) { t.Helper(); ... }`

- **Keep cognitive complexity ≤ 20** - break down complex functions into smaller helpers
  - ❌ `func complexFunction() { if a { if b { if c { ... } } } }` (high complexity)
  - ✅ `func complexFunction() { helperA(); helperB(); helperC() }` with separate helper functions
  - The `gocognit` linter flags functions with complexity > 20 - refactor immediately if flagged

- **Keep lines under 150 characters** - break long lines
- **Avoid variable shadowing** - use `unmarshalErr`, `marshalErr`, etc. for clarity

**Before committing**: `cd SERVICE && golangci-lint run`

### Before Making Changes - YOU MUST

1. **Read first**: Service README.md or CLAUDE.md, files you'll modify, existing patterns
2. **Check dependencies**: docker-compose `depends_on`, API integrations, database schemas
3. **Plan multi-service changes**: Identify affected services, determine change order
4. **Understand service boundaries**: Each service is independent with its own database

---

## Project Overview

**North Cloud** is a microservices content platform: Crawl → Classify → Publish to Redis Pub/Sub.

**Tech Stack**: Go 1.24+ (some 1.25+), Vue.js 3, Drupal 11, PostgreSQL, Redis, Elasticsearch, Docker

**Content Pipeline**:
1. **Crawler** → `{source}_raw_content` (Elasticsearch) with `classification_status=pending`
2. **Classifier** → `{source}_classified_content` (enriched with quality, topics, crime detection)
3. **Publisher** → Redis Pub/Sub channels (e.g., `articles:crime:violent`, `articles:news`)

---

## Services - Quick Reference

### 1. crawler (`/crawler`)
- **Port**: 8060 (API), 3001 (Frontend)
- **Purpose**: Web crawler with interval-based job scheduler
- **Database**: `postgres-crawler` (crawler database)
- **Key Files**: `cmd/httpd/httpd.go`, `internal/scheduler/interval_scheduler.go`
- **IMPORTANT**: Jobs require `source_id` (not `source_name`)
- **Scheduler**: Interval-based (NOT cron) - use `interval_minutes` + `interval_type`
- **Docs**: `/crawler/docs/INTERVAL_SCHEDULER.md` (recommended), `/crawler/docs/DATABASE_SCHEDULER.md` (deprecated)

### 2. source-manager (`/source-manager`)
- **Port**: 8050
- **Purpose**: Manage content sources and crawling configs
- **Database**: `postgres-source-manager` (gosources database)
- **Test Crawl**: `POST /api/v1/sources/test-crawl` (preview without saving)

### 3. classifier (`/classifier`)
- **Port**: 8070
- **Purpose**: Classify raw content (type, quality 0-100, topics, crime detection)
- **Dependencies**: Elasticsearch (reads `{source}_raw_content`, writes `{source}_classified_content`)
- **IMPORTANT**: Must populate `Body` and `Source` alias fields for publisher
- **Docs**: `/classifier/CLAUDE.md` for detailed guidelines

### 4. publisher (`/publisher`)
- **Port**: 8070
- **Purpose**: Database-backed routing hub (filters articles, publishes to Redis)
- **Database**: `postgres-publisher` (sources, channels, routes, publish_history)
- **Modes**: `use_classified_content: true` (recommended) OR legacy keyword-based
- **Filter**: `content_type: "article"` to exclude pages/listings
- **Docs**: `/publisher/CLAUDE.md` for detailed guidelines

### 5. auth (`/auth`)
- **Port**: 8040
- **Purpose**: Username/password → JWT tokens (24h expiration)
- **Config**: `AUTH_USERNAME`, `AUTH_PASSWORD`, `AUTH_JWT_SECRET` (shared across services)
- **Protected**: All `/api/v1/*` routes (health endpoints are public)

### 6. index-manager (`/index-manager`)
- **Port**: 8090
- **Purpose**: Elasticsearch index and document management
- **Database**: `postgres-index-manager` (index metadata, migration history)
- **Hot Reload**: Air-based development (`.air.toml`)

### 7. search (`/search`)
- **Port**: 8090 (internal), 8092 (dev), `/api/search` (nginx)
- **Purpose**: Full-text search across all `*_classified_content` indexes
- **Features**: Multi-match, field boosting, fuzzy matching, faceted search

### 8. mcp-north-cloud (`/mcp-north-cloud`)
- **Purpose**: MCP server exposing 23 tools for AI integration
- **Protocol**: stdio-based (reads stdin, writes stdout)
- **Docs**: `/mcp-north-cloud/README.md` for comprehensive tool documentation

### 9. dashboard (`/dashboard`)
- **Port**: 3002
- **Tech**: Vue.js 3 + TypeScript + Tailwind CSS
- **Auth**: JWT tokens in localStorage, route guards, API interceptors
- **Types**: All components use proper TypeScript types (no `any`), see `/dashboard/src/types/`

### Infrastructure Services
- **PostgreSQL**: One database per service (`postgres-{service}`)
- **Redis**: Pub/Sub channels (e.g., `articles:crime:violent`, `articles:news`)
- **Elasticsearch**: `{source}_raw_content` and `{source}_classified_content` indexes
- **Nginx**: Reverse proxy, SSL/TLS (northcloud.biz), routes `/api/*` and `/dashboard/*`
- **Loki + Grafana Alloy + Grafana**: Centralized logging infrastructure
  - **Alloy**: Collects logs from Docker containers and forwards to Loki
  - **Loki**: Aggregates and stores logs with label-based indexing
  - **Grafana**: Web UI for querying and visualizing logs
  - **Configuration**: `/infrastructure/alloy/config.alloy` (HCL format)
  - **Port**: 12345 (Alloy debugging UI), 3100 (Loki), 3000 (Grafana)
  - **Docs**: `/infrastructure/alloy/README.md`, `/infrastructure/grafana/README.md`

---

## Development Workflow

### Initial Setup
```bash
cp .env.example .env
# Edit .env with your configuration
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d
```

### Working on a Service

1. **Start service**: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d SERVICE`
2. **View logs**: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f SERVICE`
3. **Make changes**: Code auto-reloads in dev mode (Air or volume mounts)
4. **Test**: `cd SERVICE && go test ./...`
5. **Lint**: `cd SERVICE && golangci-lint run`
6. **Rebuild**: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build SERVICE`

### Database Access
```bash
# Source manager
docker exec -it north-cloud-postgres-source-manager psql -U postgres -d gosources

# Crawler
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler

# Publisher
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher
```

### Running Migrations
```bash
# Go services
cd SERVICE && go run cmd/migrate/main.go up

# Drupal
docker exec -it north-cloud-streetcode drush updb
```

---

## Code Conventions

### Go Services
- **Go Version**: 1.24+ (crawler, source-manager), 1.25+ (classifier, publisher, others)
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
- **Type Safety**: No `any` types - use `unknown` for generic values, specific interfaces for known types
- **Types**: Defined in `/dashboard/src/types/` directory

### Docker
- **Development**: `docker-compose.base.yml` + `docker-compose.dev.yml`
- **Production**: `docker-compose.base.yml` + `docker-compose.prod.yml`
- **Naming**: Container names follow `north-cloud-{service}` pattern
- **Always use**: `docker compose` (not `docker-compose`)

---

## Service-Specific Guidelines

### Crawler
- **IMPORTANT**: Read `/crawler/docs/INTERVAL_SCHEDULER.md` for comprehensive scheduler guide
- **Interval-based scheduling**: `{"interval_minutes": 30, "interval_type": "minutes"}` (NOT cron)
- **7 job states**: pending, scheduled, running, paused, completed, failed, cancelled
- **Source ID required**: Jobs must use `source_id` field (from source-manager)
- **Distributed locking**: PostgreSQL CAS locks for multi-instance safety
- **8 API endpoints**: `/pause`, `/resume`, `/cancel`, `/retry`, `/executions`, `/stats`, `/scheduler/metrics`

### Classifier
- **IMPORTANT**: Read `/classifier/CLAUDE.md` for detailed guidelines
- **Pipeline**: Processes `{source}_raw_content` with `classification_status=pending`
- **Output**: `{source}_classified_content` with quality (0-100), topics, crime sub-categories
- **Publisher compatibility**: Must populate `Body` and `Source` alias fields
- **Crime sub-categories**: violent_crime, property_crime, drug_crime, organized_crime, criminal_justice

### Publisher
- **IMPORTANT**: Read `/publisher/CLAUDE.md` for detailed guidelines
- **Mode**: `use_classified_content: true` (recommended) - queries `{source}_classified_content`
- **Filter**: `content_type: "article"` to exclude pages/listings
- **Routes**: Database-backed (PostgreSQL), many-to-many (sources → channels)
- **Redis channels**: Topic-based (e.g., `articles:crime:violent`, `articles:news`)
- **Preview**: `GET /api/v1/routes/preview` to preview articles without publishing

### Dashboard Frontend
- **Type Safety**: All components use proper TypeScript types (no `any`)
- **Shared types**: `/dashboard/src/types/` directory (`Source`, `Channel`, `Route`, `PreviewArticle`, `ApiError`, etc.)
- **Auth**: JWT tokens in localStorage, `useAuth` composable, route guards
- **Error handling**: Use `ApiError` interface with type assertions

### mcp-north-cloud
- **IMPORTANT**: Read `/mcp-north-cloud/README.md` for comprehensive tool documentation
- **23 tools**: Crawler (7), Source Manager (5), Publisher (6), Search (1), Classifier (1), Index Manager (2)
- **Protocol**: stdio-based JSON-RPC 2.0 (no HTTP server)
- **Development**: Air hot reloading, Taskfile.yml for build/test/lint

---

## Common Tasks

### Adding a New Service
1. Create service directory with `Dockerfile`
2. Add to `docker-compose.base.yml`
3. Add database service (if needed)
4. Update `.env.example` with required variables
5. Follow existing service patterns

### Managing Crawler Jobs
```bash
# Create immediate job
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{"source_id": "uuid", "url": "https://example.com", "schedule_enabled": false}'

# Schedule interval-based job
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{"source_id": "uuid", "url": "https://example.com", "interval_minutes": 360, "interval_type": "minutes", "schedule_enabled": true}'
```

### Profiling and Performance
```bash
# Capture heap profile
./scripts/profile.sh SERVICE heap

# Run benchmarks
./scripts/run-benchmarks.sh

# Check for memory leaks
./scripts/check-memory-leaks.sh -s SERVICE -i 600 -c 5

# Memory health endpoint
curl http://localhost:6060/health/memory  # Adjust port per service
```

### SSL/TLS Certificate Management
```bash
# Check certificate expiry
bash infrastructure/certbot/scripts/check-cert-expiry.sh

# Manual renewal
bash infrastructure/certbot/scripts/renew-and-reload.sh
```
**Note**: Certbot service auto-checks every 12 hours, renews 30 days before expiry

---

## Git Workflow

**Branch Naming**: MUST start with `claude/` and end with session ID
- Format: `claude/{description}-{session-id}`
- Example: `claude/create-claude-md-01YMXWZpqv3utVH69jyNnLaE`

**Before Committing**:
1. Run tests: `go test ./...`
2. Run linter: `golangci-lint run`
3. Verify no linting violations (see Critical Rules above)
4. Check multi-service dependencies if applicable

**Pushing**: Always use `git push -u origin {branch-name}` (never force push to main)

---

## Environment Variables

### Naming Convention
- Uppercase with underscores (e.g., `AUTH_JWT_SECRET`)
- Service-specific prefixes (e.g., `POSTGRES_CRAWLER_USER`)

### Required for Production
- `AUTH_USERNAME` - Dashboard username
- `AUTH_PASSWORD` - Dashboard password
- `AUTH_JWT_SECRET` - Shared JWT secret (generate: `openssl rand -hex 32`)

### Configuration Priority
1. Environment variables (highest)
2. `.env` file
3. Service config files (config.yml, etc.)
4. Hardcoded defaults (lowest)

---

## Troubleshooting

### Service Won't Start
1. Check logs: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs SERVICE`
2. Check environment: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config`
3. Verify dependencies: Check `depends_on` in docker-compose files
4. Check port conflicts: `netstat -tulpn | grep PORT`

### Database Connection Issues
1. Verify database running: `docker ps | grep postgres`
2. Check connection string in `.env`
3. Test connection: `docker exec -it north-cloud-postgres-SERVICE psql -U postgres -d DATABASE`

### Cannot Access Service
1. Verify port mapping: `docker ps | grep SERVICE`
2. Check nginx configuration (if using reverse proxy)
3. Verify service health: `curl http://localhost:PORT/health`

---

## Documentation References

### Service-Specific Docs
- `/crawler/README.md` - General crawler docs
- `/crawler/docs/INTERVAL_SCHEDULER.md` - **RECOMMENDED** Interval scheduler guide
- `/crawler/docs/DATABASE_SCHEDULER.md` - Legacy cron scheduler (deprecated)
- `/classifier/CLAUDE.md` - Classifier detailed guidelines
- `/publisher/CLAUDE.md` - Publisher detailed guidelines
- `/publisher/docs/REDIS_MESSAGE_FORMAT.md` - Redis message specification
- `/publisher/docs/CONSUMER_GUIDE.md` - Integration examples
- `/mcp-north-cloud/README.md` - MCP server tool documentation

### Infrastructure Docs
- `/DOCKER.md` - Docker quick reference
- `/infrastructure/certbot/README.md` - SSL/TLS certificate management
- `/infrastructure/alloy/README.md` - Grafana Alloy log collection
- `/infrastructure/grafana/README.md` - Logging infrastructure guide (Loki + Alloy + Grafana)
- `/docs/PROFILING.md` - Profiling and performance monitoring

### Cursor Commands
- `/.cursor/commands/README.md` - All 18 Cursor commands for workflows

---

## Important Notes

### Service Boundaries
- Each service is independent with its own database
- Don't make cross-service changes without understanding dependencies
- Respect API contracts - don't break existing APIs without migration plans

### Content Pipeline Flow
1. **Crawler** extracts → `{source}_raw_content` with `classification_status=pending`
2. **Classifier** processes → `{source}_classified_content` with quality/topics
3. **Publisher** filters → Redis Pub/Sub channels → External consumers

### Authentication
- All `/api/v1/*` routes require JWT tokens (except health endpoints)
- Tokens obtained from `/api/auth/api/v1/auth/login`
- Tokens expire after 24 hours
- Shared `AUTH_JWT_SECRET` across all services

### Docker Compose
- **ALWAYS use**: `docker compose` (not `docker-compose`)
- **Development**: `-f docker-compose.base.yml -f docker-compose.dev.yml`
- **Production**: `-f docker-compose.base.yml -f docker-compose.prod.yml`

---

## Version History

Key architectural changes (see full history in git):

- **Crime Sub-Category Classification** (2026-01-07): Replaced generic "crime" with 5 sub-categories (violent, property, drug, organized, justice)
- **Crawler Scheduler Refactor** (2025-12-29): Interval-based scheduling replaces cron (Migration 003)
- **Publisher Modernization** (2025-12-28): Database-backed Redis Pub/Sub routing hub
- **Dashboard Authentication** (2025-12-27): JWT-based auth with route guards
- **Raw Content Pipeline** (2025-12-23): Three-stage pipeline (raw → classify → publish)

---

*This document is optimized for effectiveness as a prompt. Most critical information is at the top. For detailed architecture, see service-specific README.md or CLAUDE.md files.*
