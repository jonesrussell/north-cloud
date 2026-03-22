# Pipeline Service Spec

> Last verified: 2026-03-22 (add .layers file for layer boundary checking)

## Overview

Pipeline event tracking service. Records content lifecycle events (crawled, indexed, classified, routed, published) with idempotent writes. Provides a funnel query for observability. No external service dependencies beyond PostgreSQL.

---

## File Map

```
pipeline/
  main.go                            # Calls bootstrap.Start()
  migrations/
    001_create_pipeline_events.up.sql # Partitioned events table + content_items + stage_ordering
  internal/
    bootstrap/                       # Config -> logger -> database -> HTTP server
    config/                          # Config struct, YAML + env loading, validation
    database/                        # PostgreSQL repository (idempotent inserts)
    domain/                          # Stage, ContentItem, PipelineEvent, IngestRequest, FunnelResponse
    service/                         # PipelineService: validation, idempotency, write + read
    api/                             # HTTP handlers + route registration
```

---

## API Reference

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/events` | None | Ingest one event |
| `POST` | `/api/v1/events/batch` | None | Ingest multiple events |
| `GET` | `/api/v1/funnel` | JWT | Pipeline funnel (period: `today`, `24h`, `7d`, `30d`) |
| `GET` | `/health` | None | Liveness + database ping |
| `GET` | `/metrics` | None | Prometheus metrics |

Write endpoints are intentionally unauthenticated (internal Docker network only).

---

## Data Model

### Five Pipeline Stages

```
crawled -> indexed -> classified -> routed -> published
```

### Tables

- **content_items**: `url` (PK), `url_hash` (SHA-256), `domain`, `source_name`, `created_at`
- **pipeline_events**: partitioned by `occurred_at` (monthly ranges). Fields: `id`, `idempotency_key`, `content_url`, `source_name`, `stage`, `service_name`, `occurred_at`, `metadata` (JSONB)
- **stage_ordering**: maps stages to sort order for funnel queries

### Idempotency

Each event has an `idempotency_key`. Auto-generated format: `{serviceName}:{stage}:{urlHash8}:{occurredAt}`. Database enforces uniqueness on `(idempotency_key, occurred_at)`.

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PIPELINE_PORT` | `8075` | HTTP listen port |
| `APP_DEBUG` | `false` | Gin debug mode |
| `POSTGRES_PIPELINE_HOST` | `localhost` | DB host |
| `POSTGRES_PIPELINE_PORT` | `5432` | DB port |
| `POSTGRES_PIPELINE_USER` | `postgres` | DB user |
| `POSTGRES_PIPELINE_PASSWORD` | _(empty)_ | DB password |
| `POSTGRES_PIPELINE_DB` | `pipeline` | DB name |
| `AUTH_JWT_SECRET` | _(empty)_ | Shared JWT secret (for funnel endpoint) |
| `LOG_LEVEL` | `info` | Log level |
| `LOG_FORMAT` | `json` | Log format |

---

## Known Constraints

- **Partition maintenance required**: initial migration ships partitions for 2026 Q1-Q2. Add partitions before each quarter ends, or inserts fail.
- **Non-UTC timestamps rejected**: `occurred_at` must be `time.UTC` location (pointer comparison).
- **24-hour event age limit**: events older than 24h rejected with validation error.
- **Batch stops on first error**: `IngestBatch` processes sequentially, returns on first failure.
- **Write endpoints are unauthenticated**: rely on Docker network isolation. Do not expose port 8075 publicly.

<\!-- Reviewed: 2026-03-18 — go.mod dependency update only, no spec changes needed -->
