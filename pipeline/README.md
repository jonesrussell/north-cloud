# Pipeline

Event-sourcing observability layer that tracks per-article stage transitions across the North Cloud content pipeline and exposes a funnel view of throughput.

## Overview

The pipeline service sits alongside the Crawl → Classify → Publish flow and acts as an audit log. Every other service in the platform reports a lightweight event each time an article advances through a stage. The pipeline service persists those events in a partitioned PostgreSQL table and aggregates them into a funnel view so operators can see how many articles successfully move from crawled all the way to published.

**Pipeline stages** (in order):

| Stage | Reported by |
|---|---|
| `crawled` | crawler |
| `indexed` | crawler / index-manager |
| `classified` | classifier |
| `routed` | publisher |
| `published` | publisher |

## Features

- Single-event and batch event ingestion via REST
- Idempotent writes — duplicate events are silently discarded (PostgreSQL `ON CONFLICT DO NOTHING`)
- Automatic idempotency-key generation when the caller omits one
- Time-range funnel query with `today`, `24h`, `7d`, and `30d` presets
- Partitioned `pipeline_events` table (monthly partitions) for predictable query performance
- SHA-256 URL hashing for compact, indexed article identity
- JWT-protected read endpoints; write endpoints are public (internal Docker network only)

## Quick Start

### Docker (Recommended)

```bash
# Start the pipeline service and its database
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d pipeline

# Tail logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f pipeline

# Run migrations
task migrate:pipeline
```

### Local Development

```bash
cd pipeline

# Copy and edit config
cp config.yml.example config.yml

# Install dev tools (air, golangci-lint, goimports, migrate)
task install:tools

# Run migrations against a local or Docker-hosted Postgres
task migrate:up

# Start with hot reload
task dev

# Or run directly
go run .
```

The service listens on port **8075** by default.

## API Reference

All endpoints are prefixed with `/api/v1`.

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/events` | None | Ingest a single pipeline event |
| `POST` | `/api/v1/events/batch` | None | Ingest multiple pipeline events in one request |
| `GET` | `/api/v1/funnel` | JWT | Retrieve the aggregated pipeline funnel |
| `GET` | `/health` | None | Liveness probe (returns service + database status) |

### POST /api/v1/events

Ingest a single event. Returns `201 Created` on success.

**Request body**

```json
{
  "article_url":      "https://example.com/article/123",
  "source_name":      "example_com",
  "stage":            "crawled",
  "occurred_at":      "2026-02-20T14:00:00Z",
  "service_name":     "crawler",
  "idempotency_key":  "crawler:crawled:a1b2c3d4:2026-02-20T14:00:00Z",
  "metadata":         { "http_status": 200 }
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `article_url` | string | yes | Full canonical URL |
| `source_name` | string | yes | Source identifier (e.g., `example_com`) |
| `stage` | string | yes | One of `crawled`, `indexed`, `classified`, `routed`, `published` |
| `occurred_at` | RFC 3339 | yes | Must be UTC, not in the future, not older than 24 hours |
| `service_name` | string | yes | Reporting service (e.g., `crawler`, `classifier`) |
| `idempotency_key` | string | no | Auto-generated when omitted |
| `metadata` | object | no | Arbitrary key-value pairs stored as JSONB |

**Response**

```json
{ "status": "ingested" }
```

### POST /api/v1/events/batch

Ingest up to N events in one request. Events are processed sequentially; the first failure stops processing and returns a partial count.

**Request body**

```json
{
  "events": [
    {
      "article_url":  "https://example.com/article/1",
      "source_name":  "example_com",
      "stage":        "crawled",
      "occurred_at":  "2026-02-20T14:00:00Z",
      "service_name": "crawler"
    },
    {
      "article_url":  "https://example.com/article/2",
      "source_name":  "example_com",
      "stage":        "classified",
      "occurred_at":  "2026-02-20T14:01:00Z",
      "service_name": "classifier"
    }
  ]
}
```

**Response**

```json
{ "status": "ingested", "ingested": 2 }
```

On partial failure:

```json
{ "error": "event 1: invalid pipeline stage", "ingested": 1 }
```

### GET /api/v1/funnel

Returns the aggregated pipeline funnel for a time period. Requires a `Bearer` JWT token.

**Query parameters**

| Parameter | Default | Options |
|---|---|---|
| `period` | `today` | `today`, `24h`, `7d`, `30d` |

**Response**

```json
{
  "period":       "today",
  "timezone":     "UTC",
  "from":         "2026-02-20T00:00:00Z",
  "to":           "2026-02-20T15:32:00Z",
  "generated_at": "2026-02-20T15:32:04Z",
  "stages": [
    { "name": "crawled",    "count": 430, "unique_articles": 412 },
    { "name": "indexed",    "count": 410, "unique_articles": 405 },
    { "name": "classified", "count": 390, "unique_articles": 385 },
    { "name": "routed",     "count": 370, "unique_articles": 365 },
    { "name": "published",  "count": 310, "unique_articles": 308 }
  ]
}
```

Stages are always returned in pipeline order (`crawled` → `published`). Stages with zero events in the period are omitted.

## Configuration

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PIPELINE_PORT` | `8075` | HTTP listen port |
| `APP_DEBUG` | `false` | Enable Gin debug mode and verbose logging |
| `POSTGRES_PIPELINE_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PIPELINE_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_PIPELINE_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_PIPELINE_PASSWORD` | _(empty)_ | PostgreSQL password |
| `POSTGRES_PIPELINE_DB` | `pipeline` | PostgreSQL database name |
| `AUTH_JWT_SECRET` | _(empty)_ | Shared JWT secret (required for protected endpoints) |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | Log format: `json` or `console` |

### Config File

The service reads `config.yml` from the working directory (or the path given by `CONFIG_PATH`). See `config.yml.example` for the full structure and all defaults.

```yaml
service:
  port: 8075
  debug: false

database:
  host: postgres-pipeline
  port: 5432
  user: postgres
  password: changeme
  database: pipeline
  sslmode: disable
  max_connections: 25
  max_idle_connections: 5
  connection_max_lifetime: "1h"

auth:
  jwt_secret: ""

logging:
  level: info
  format: json
```

Environment variables take precedence over `config.yml` values.

## Architecture

```
pipeline/
├── main.go                      # Entry point — calls bootstrap.Start()
├── config.yml.example           # Reference config (copied to config.yml at startup)
├── Dockerfile                   # Production multi-stage build
├── Dockerfile.dev               # Development image with Air hot reload
├── Taskfile.yml                 # Task runner targets (build, test, lint, migrate, …)
├── .air.toml                    # Air hot-reload config
├── migrations/
│   ├── 001_create_pipeline_events.up.sql    # Schema: articles, pipeline_stage enum,
│   │                                        #   stage_ordering, pipeline_events (partitioned)
│   └── 001_create_pipeline_events.down.sql  # Rollback
└── internal/
    ├── bootstrap/       # Startup orchestration (config → logger → database → HTTP server)
    │   ├── app.go       # Start() wires the phases in order
    │   ├── config.go    # LoadConfig() + CreateLogger()
    │   ├── database.go  # SetupDatabase()
    │   └── server.go    # SetupHTTPServer() — wires repo → service → handlers → router
    ├── config/
    │   └── config.go    # Config struct with YAML + env-var loading and validation
    ├── database/
    │   ├── connection.go  # PostgreSQL connection pool
    │   └── repository.go  # UpsertArticle, InsertEvent, GetFunnel queries
    ├── domain/
    │   └── models.go    # Stage, Article, PipelineEvent, IngestRequest, FunnelResponse,
    │                    #   URLHash, ExtractDomain, GenerateIdempotencyKey
    ├── service/
    │   └── pipeline.go  # PipelineService — validation, idempotency-key generation,
    │                    #   article upsert, event insert, funnel aggregation
    └── api/
        ├── routes.go          # Route registration (public write, JWT-protected read)
        ├── ingest_handler.go  # POST /events and POST /events/batch
        └── funnel_handler.go  # GET /funnel with period resolution
```

### Database Schema

```
articles
  url          TEXT  PRIMARY KEY
  url_hash     CHAR(64) UNIQUE
  domain       TEXT
  source_name  TEXT
  first_seen_at TIMESTAMPTZ

pipeline_events (partitioned by occurred_at — monthly partitions)
  id                       BIGSERIAL
  article_url              TEXT → articles.url
  stage                    pipeline_stage ENUM
  occurred_at              TIMESTAMPTZ
  received_at              TIMESTAMPTZ  DEFAULT NOW()
  service_name             TEXT
  metadata                 JSONB
  metadata_schema_version  SMALLINT
  idempotency_key          TEXT
  PRIMARY KEY (id, occurred_at)
  UNIQUE (idempotency_key, occurred_at)

stage_ordering
  stage       pipeline_stage  PRIMARY KEY
  sort_order  SMALLINT        UNIQUE
```

## Development

### Running Tests

```bash
cd pipeline

# All tests
go test ./...

# Unit tests only
task test:unit

# With race detector
task test:race

# With HTML coverage report
task test:coverage
```

### Linting

```bash
cd pipeline

# Lint (cached)
task lint

# Lint without cache (matches CI output exactly)
task lint:no-cache
```

### Building

```bash
cd pipeline

# Build binary to ./bin/pipeline
task build

# Build Docker image
task docker:build
```

### Migrations

```bash
cd pipeline

# Apply all pending migrations
task migrate:up

# Roll back the last migration
task migrate:down

# Show current migration version
task migrate:version

# Force a specific version (fix dirty state)
VERSION=1 task migrate:force
```

## Integration

The pipeline service is a passive observer — no other service depends on it at startup. Upstream services call it by posting events; the pipeline service never calls them back.

**Typical integration pattern for a Go service:**

```go
// Fire-and-forget: post an event after each successful stage transition.
// Failures are logged but must not block the main pipeline.
func reportStage(ctx context.Context, articleURL, sourceName, stage, serviceName string) {
    body := map[string]any{
        "article_url":  articleURL,
        "source_name":  sourceName,
        "stage":        stage,
        "occurred_at":  time.Now().UTC().Format(time.RFC3339),
        "service_name": serviceName,
    }
    // POST to http://pipeline:8075/api/v1/events
}
```

**Obtaining a JWT for the funnel endpoint:**

```bash
# Get a token from the auth service
curl -s -X POST http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<password>"}'

# Query the funnel
curl -s http://localhost:8075/api/v1/funnel?period=today \
  -H "Authorization: Bearer <token>"
```
