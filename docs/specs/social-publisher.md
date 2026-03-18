# Social Publisher Spec

> Last verified: 2026-03-18

## Overview

Social media delivery service. Subscribes to Redis pub/sub for inbound publish requests, delivers content to platform adapters (currently X/Twitter), tracks delivery lifecycle in Postgres, and retries failures with exponential backoff.

---

## File Map

```
social-publisher/
  main.go                                   # Bootstrap: config → DB → Redis → workers + server
  migrations/
    001_initial_schema.up.sql               # content, accounts, deliveries tables
    002_add_content_list_indexes.up.sql     # Index on content listing queries
  internal/
    adapters/x/
      adapter.go                            # Implements domain.PlatformAdapter
      client.go                            # X API HTTP client
    api/
      router.go                            # Gin router + JWT middleware
      handler.go                           # content, status, publish, retry endpoints
      accounts_handler.go                  # CRUD for platform accounts
    config/config.go                       # Config struct + env var bindings + defaults
    crypto/crypto.go                       # AES-256-GCM credential encryption/decryption
    database/repository.go                 # Postgres: content / accounts / deliveries
    domain/
      adapter.go                           # PlatformAdapter interface (extension point)
      delivery.go                          # Delivery struct + status constants
      content.go                           # Content struct
      account.go                           # Account struct (encrypted credentials)
      list.go                              # ListParams / pagination helpers
    orchestrator/
      orchestrator.go                      # ProcessJob + Backoffs()
      queue.go                             # PriorityQueue (realtime + retry lanes)
    redis/
      subscriber.go                        # Subscribes to social:publish
      publisher.go                         # Publishes to social:delivery-status, social:dead-letter
    workers/
      scheduler.go                         # Polls content table for scheduled items
      retry.go                             # Polls deliveries for retryable items
```

---

## Redis Subscription Interface

### Inbound

| Channel | Message Type | Description |
|---------|-------------|-------------|
| `social:publish` | `domain.PublishMessage` | Publisher routes content here for social delivery |

`PublishMessage` JSON structure:
```json
{
  "content_id": "abc123",
  "title": "Article title",
  "url": "https://...",
  "body": "...",
  "targets": [
    { "platform": "x", "account": "handle" }
  ]
}
```

Messages with no `targets` are logged and dropped.

### Outbound

| Channel | Message Type | Description |
|---------|-------------|-------------|
| `social:delivery-status` | `domain.DeliveryEvent` | Delivery lifecycle events (created, delivered, failed, retrying) |
| `social:dead-letter` | `redis.DeadLetterMessage` | Permanently failed deliveries (after max retries) |

---

## Publishing Pipeline

### Realtime Path (Redis-driven)
```
social:publish → redis.Subscriber → PriorityQueue.EnqueueRealtime (cap 100)
  → queue consumer goroutine
  → repo.CreateDelivery
  → orchestrator.ProcessJob → PlatformAdapter.{Transform, Validate, Publish}
  → repo.UpdateDeliveryStatus → redis.PublishDeliveryEvent
```

### Scheduled Path (Postgres-driven)
```
workers.Scheduler (every 60s)
  → repo.GetScheduledContent (published=false, scheduled_at <= now, batch 50)
  → PriorityQueue.EnqueueRetry (cap 50)
  → same consumer path
```

### Retry Path
```
workers.RetryWorker (every 30s)
  → repo.GetRetryableDeliveries (status=retrying, next_retry_at <= now)
  → re-enqueue → same consumer path
```

---

## Retry Logic

Backoff schedule from `orchestrator.Backoffs()`: **30s → 2m → 10m**

- Default max attempts: 3 (configurable via `SOCIAL_PUBLISHER_MAX_RETRIES`)
- After max attempts: delivery marked `permanently_failed`, published to `social:dead-letter`
- `deliveries.UNIQUE(content_id, platform, account)` prevents duplicate delivery records

---

## Database Schema

Three tables:

**`content`**: items queued for publishing
- `id`, `title`, `body`, `url`, `source_name`, `published` (bool), `scheduled_at`, `created_at`

**`accounts`**: platform credentials
- `id`, `platform` (e.g. `"x"`), `handle`, `credentials` (BYTEA — AES-256-GCM encrypted)

**`deliveries`**: delivery lifecycle tracking
- `id`, `content_id` (FK), `platform`, `account`, `status`, `platform_id` (returned by platform), `error_message`, `attempt`, `max_attempts`, `next_retry_at`, `created_at`, `updated_at`
- `UNIQUE(content_id, platform, account)` — deduplication constraint

Delivery status values: `pending`, `delivered`, `failed`, `retrying`, `permanently_failed`

---

## Platform Adapter Extension

To add a new platform:
1. Implement `domain.PlatformAdapter` in `internal/adapters/{platform}/`
2. Register in `main.go` adapters map: `"{platform}": myAdapter`

Interface:
```go
type PlatformAdapter interface {
    Transform(msg *PublishMessage) (any, error)
    Validate(payload any) error
    Publish(ctx context.Context, payload any) (PlatformResult, error)
}
```

---

## Config Vars

| Env Var | Default | Required |
|---------|---------|----------|
| `SOCIAL_PUBLISHER_ADDRESS` | `:8078` | |
| `SOCIAL_PUBLISHER_DEBUG` | `false` | |
| `SOCIAL_PUBLISHER_ENCRYPTION_KEY` | — | **yes** — 64-char hex (32-byte AES key) |
| `SOCIAL_PUBLISHER_JWT_SECRET` | — | **yes** |
| `SOCIAL_PUBLISHER_RETRY_INTERVAL` | `30s` | |
| `SOCIAL_PUBLISHER_SCHEDULE_INTERVAL` | `60s` | |
| `SOCIAL_PUBLISHER_MAX_RETRIES` | `3` | |
| `SOCIAL_PUBLISHER_BATCH_SIZE` | `50` | |
| `POSTGRES_SOCIAL_PUBLISHER_HOST` | — | **yes** |
| `POSTGRES_SOCIAL_PUBLISHER_PORT` | — | |
| `POSTGRES_SOCIAL_PUBLISHER_USER` | — | **yes** |
| `POSTGRES_SOCIAL_PUBLISHER_PASSWORD` | — | **yes** |
| `POSTGRES_SOCIAL_PUBLISHER_DB` | — | **yes** |
| `POSTGRES_SOCIAL_PUBLISHER_SSL_MODE` | — | |
| `REDIS_ADDR` | — | **yes** |
| `REDIS_PASSWORD` | — | |

---

## API Reference

All endpoints require `Authorization: Bearer <JWT>` (signed with `SOCIAL_PUBLISHER_JWT_SECRET`).

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/content` | List content items |
| `POST` | `/api/v1/publish` | Trigger immediate publish |
| `GET` | `/api/v1/status/:id` | Get delivery status by ID |
| `POST` | `/api/v1/retry/:id` | Manually retry a failed delivery |
| `GET/POST/PUT/DELETE` | `/api/v1/accounts` | Manage platform accounts |

---

## Known Constraints

- **Encryption key required at startup** — no default. Generate: `openssl rand -hex 32`. Rotating requires re-encrypting all `accounts.credentials` rows.
- **Realtime queue drops on overflow** — if `PriorityQueue` realtime lane (cap 100) is full, the message is dropped and an error is logged. Scale batch size or add backpressure if this occurs.
- **Queue consumer is single-goroutine** — delivery is serialized. Horizontal scaling requires multiple instances with partitioned account sets.
- **X adapter bearer token loaded from accounts at publish time** — not from env var. Accounts must be registered via the `/api/v1/accounts` API before any content can be published.

<\!-- Reviewed: 2026-03-18 — go.mod dependency update only, no spec changes needed -->
