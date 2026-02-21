# Pipeline — Developer Guide

## Quick Reference

```bash
# Start service (Docker)
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d pipeline

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f pipeline

# Run tests
cd pipeline && go test ./...

# Lint (cached)
cd pipeline && task lint

# Lint without cache (matches CI)
cd pipeline && task lint:no-cache

# Apply migrations
cd pipeline && task migrate:up

# Hot-reload development
cd pipeline && task dev

# Build binary
cd pipeline && task build
```

---

## Architecture

```
pipeline/
├── main.go                  # Calls bootstrap.Start(); exits 1 on error
├── config.yml.example       # Reference config
├── migrations/
│   └── 001_create_pipeline_events.{up,down}.sql
└── internal/
    ├── bootstrap/           # Startup phases: config → logger → database → HTTP server
    ├── config/              # Config struct, YAML + env loading, validation
    ├── database/            # PostgreSQL connection pool + repository (SQL queries)
    ├── domain/              # Core types: Stage, Article, PipelineEvent, IngestRequest,
    │                        #   FunnelResponse; utility functions: URLHash, ExtractDomain,
    │                        #   GenerateIdempotencyKey
    ├── service/             # PipelineService: validation, idempotency, write + read logic
    └── api/                 # HTTP handlers + route registration
```

### Package roles

| Package | Responsibility |
|---|---|
| `bootstrap` | Wires all layers; each `Setup*` function corresponds to one startup phase |
| `config` | Typed config struct; env vars take precedence over YAML fields |
| `database` | Raw SQL only — no business logic; idempotent writes via `ON CONFLICT DO NOTHING` |
| `domain` | Pure Go types and functions; no I/O, no external dependencies |
| `service` | Orchestrates domain + repository; owns all validation rules |
| `api` | HTTP binding/response only; delegates all logic to `service` |

---

## Key Concepts

### Five Pipeline Stages

Stages are a PostgreSQL `ENUM` and a Go `Stage` type. The ordered set is:

```
crawled → indexed → classified → routed → published
```

`stage_ordering` table drives deterministic `ORDER BY` in the funnel query.

### Idempotency

Every event carries an `idempotency_key`. If the caller omits it, `PipelineService.Ingest` generates one automatically:

```
{serviceName}:{stage}:{urlHash8}:{occurredAt RFC3339}
```

The database enforces uniqueness on `(idempotency_key, occurred_at)`. A duplicate insert returns no rows from `RETURNING id` which the repository treats as success (not an error).

### Article Identity

Articles are keyed by URL (primary key). On first sight the service also stores:
- `url_hash` — full SHA-256 hex (64 chars) for compact indexing
- `domain` — hostname with `www.` stripped
- `source_name` — inherited from the event

`UpsertArticle` uses `ON CONFLICT (url) DO NOTHING` so repeated events for the same URL are cheap.

### Partitioned Event Table

`pipeline_events` is partitioned by `occurred_at` (monthly ranges). The service ships with partitions for 2026 Q1–Q2. Add new partitions via migration before each quarter ends.

### Time Validation

`PipelineService.validateIngestRequest` enforces three rules:
1. `occurred_at` must be in UTC (location check, not offset).
2. `occurred_at` must not be in the future.
3. `occurred_at` must not be more than 24 hours in the past (`maxEventAge`).

### Route Authorization

Write endpoints (`POST /events`, `POST /events/batch`) are **public** — they are meant to be called by other services over the internal Docker network. The funnel read endpoint (`GET /funnel`) requires a `Bearer` JWT signed with `AUTH_JWT_SECRET`.

---

## API Reference

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/events` | None | Ingest one event |
| `POST` | `/api/v1/events/batch` | None | Ingest multiple events |
| `GET` | `/api/v1/funnel` | JWT | Pipeline funnel (period: `today`, `24h`, `7d`, `30d`) |
| `GET` | `/health` | None | Liveness + database ping |

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PIPELINE_PORT` | `8075` | HTTP listen port |
| `APP_DEBUG` | `false` | Gin debug mode |
| `POSTGRES_PIPELINE_HOST` | `localhost` | DB host |
| `POSTGRES_PIPELINE_PORT` | `5432` | DB port |
| `POSTGRES_PIPELINE_USER` | `postgres` | DB user |
| `POSTGRES_PIPELINE_PASSWORD` | _(empty)_ | DB password |
| `POSTGRES_PIPELINE_DB` | `pipeline` | DB name |
| `AUTH_JWT_SECRET` | _(empty)_ | Shared JWT secret |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | `json` | `json` or `console` |

Config precedence: environment variable > `config.yml` value > hardcoded default.

---

## Common Gotchas

1. **Partition missing for the current month.** The initial migration ships partitions for 2026-01 through 2026-04. Inserts outside that range fail with a PostgreSQL error. Add the next quarter's partitions before the last one fills.

2. **Non-UTC `occurred_at` rejected.** The service checks `req.OccurredAt.Location() != time.UTC` — it is a location pointer comparison, not an offset comparison. Always call `time.Now().UTC()` or `t.UTC()` before including a timestamp in an event.

3. **24-hour event age limit.** Events older than `maxEventAge` (24h) are rejected with a validation error. If you replay historical events, either adjust `maxEventAge` or insert them directly via migration.

4. **Batch stops on first error.** `IngestBatch` processes events sequentially and returns as soon as one fails, reporting `ingested: N` for the events that succeeded before the failure. Callers should not assume atomicity.

5. **Funnel stages may be absent.** If no events for a stage exist in the requested period, that stage row is not included in the response `stages` array. Parse defensively.

6. **Write endpoints are intentionally unauthenticated.** They rely on Docker network isolation. Do not expose port 8075 to the public internet without adding authentication at the nginx layer.

7. **`GOWORK=off` is required for correct module resolution.** All Taskfile commands already set this. If you run `go` commands directly from the repo root, set `GOWORK=off` or `cd` into the `pipeline/` directory first.

---

## Testing

```bash
# All tests (unit + integration-style with sqlmock)
go test ./...

# Specific package
go test ./internal/service/...
go test ./internal/domain/...
go test ./internal/database/...
go test ./internal/api/...

# With race detector
go test -race ./...

# Coverage report (outputs coverage.html)
task test:coverage
```

Test packages use `go-sqlmock` for database layer tests and plain `net/http/httptest` for handler tests. No running database is required.

---

## Code Patterns

### Adding a new field to events

1. Add the column to a new migration SQL file.
2. Add the field to `domain.PipelineEvent` and (if caller-supplied) `domain.IngestRequest`.
3. Update `database.Repository.InsertEvent` to pass the new value.
4. Update `database.Repository.InsertEvent` test expectations.
5. Bump `MetadataSchemaVersion` if the change is backwards-incompatible.

### Adding a new API endpoint

Follow the existing split: handler in `api/`, business logic in `service/`, data access in `database/`. Register the route in `api/routes.go`. Protected endpoints go under the `protected` group; internal service-to-service calls go under `public`.

### Logging

Use `infralogger` (the shared infrastructure package). Always pass structured fields:

```go
log.Info("event ingested",
    infralogger.String("stage", string(req.Stage)),
    infralogger.String("article_url", req.ArticleURL),
)
```
