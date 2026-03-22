# social-publisher/CLAUDE.md

Social media publishing service. Subscribes to `social:publish` Redis channel, delivers content to platform adapters (X/Twitter), tracks delivery lifecycle in Postgres, and retries failed deliveries with exponential backoff.

---

## Layer Rules

The social-publisher's internal packages form a strict DAG organized into 4 layers.
A package may import from its own layer or any lower layer. Never from a higher layer.

| Layer | Packages | Role |
|-------|----------|------|
| L0 | `domain`, `config` | Foundation — no internal imports |
| L1 | `database`, `crypto`, `adapters` | Persistence / Crypto |
| L2 | `orchestrator`, `redis`, `workers` | Processing |
| L3 | `api` | HTTP |

**Rules:**
- `domain/` must not import any other social-publisher package (it is the leaf)
- All shared infrastructure imports go through `infrastructure/` (no cross-service imports)
- Lateral imports within the same layer are allowed
- Bootstrap lives in `main.go` (no dedicated bootstrap package)

---

## Architecture

```
Redis social:publish channel
  → redis.Subscriber
  → orchestrator.PriorityQueue (realtime lane, capacity 100)
  → queue consumer goroutine
  → repo.CreateDelivery (Postgres)
  → orchestrator.Orchestrator.ProcessJob
  → adapters/x (Transform → Validate → Publish)
  → repo.UpdateDeliveryStatus

Postgres content table (scheduled=false, scheduled_at <= now)
  → workers.Scheduler (every 60s)
  → PriorityQueue (retry lane, capacity 50)
  → same consumer path

Postgres deliveries (status=retrying, next_retry_at <= now)
  → workers.RetryWorker (every 30s)
  → re-enqueue → same consumer path
```

---

## Directory Structure

```
social-publisher/
  main.go                             # Bootstrap: config → logger → DB → Redis → workers + server
  migrations/                         # SQL migrations (001 schema, 002 list indexes)
  internal/
    adapters/
      x/                              # X (Twitter) platform adapter
        adapter.go                    # Implements domain.PlatformAdapter (Transform/Validate/Publish)
        client.go                     # HTTP client for X API
    api/
      router.go                       # Gin router + JWT middleware
      handler.go                      # /api/v1/content, /api/v1/status/:id, /api/v1/publish, /api/v1/retry/:id
      accounts_handler.go             # CRUD /api/v1/accounts
    config/config.go                  # All env vars and defaults
    crypto/crypto.go                  # AES-256-GCM credential encryption
    database/repository.go            # Postgres: content, accounts, deliveries tables
    domain/
      adapter.go                      # PlatformAdapter interface (extension point)
      delivery.go                     # Delivery struct + status constants
      content.go                      # Content struct
      account.go                      # Account struct (encrypted credentials)
      list.go                         # ListParams / pagination helpers
    orchestrator/
      orchestrator.go                 # ProcessJob + Backoffs()
      queue.go                        # PriorityQueue (realtime + retry lanes)
    redis/
      subscriber.go                   # Subscribes to social:publish
      publisher.go                    # Publishes to social:delivery-status and social:dead-letter
    workers/
      scheduler.go                    # Polls content table for scheduled items
      retry.go                        # Polls deliveries table for retryable items
```

---

## Common Commands

```bash
task dev                              # Hot-reload dev server
task build                            # Build binary
task test                             # Run tests
task lint                             # golangci-lint

# Run migrations
task migrate:up

# Check service status
curl -H "Authorization: Bearer $TOKEN" http://localhost:8078/api/v1/content
```

---

## Configuration

| Env Var | Default | Notes |
|---------|---------|-------|
| `SOCIAL_PUBLISHER_ADDRESS` | `:8078` | HTTP server address |
| `SOCIAL_PUBLISHER_DEBUG` | `false` | Debug logging |
| `SOCIAL_PUBLISHER_ENCRYPTION_KEY` | — | **Required.** 64-char hex string (32 bytes AES key) |
| `SOCIAL_PUBLISHER_JWT_SECRET` | — | JWT signing secret for API auth |
| `SOCIAL_PUBLISHER_RETRY_INTERVAL` | `30s` | How often RetryWorker polls |
| `SOCIAL_PUBLISHER_SCHEDULE_INTERVAL` | `60s` | How often Scheduler polls |
| `SOCIAL_PUBLISHER_MAX_RETRIES` | `3` | Max delivery attempts |
| `SOCIAL_PUBLISHER_BATCH_SIZE` | `50` | Scheduler/RetryWorker batch size |
| `POSTGRES_SOCIAL_PUBLISHER_HOST` | — | **Required** |
| `POSTGRES_SOCIAL_PUBLISHER_PORT` | — | |
| `POSTGRES_SOCIAL_PUBLISHER_USER` | — | |
| `POSTGRES_SOCIAL_PUBLISHER_PASSWORD` | — | |
| `POSTGRES_SOCIAL_PUBLISHER_DB` | — | **Required** |
| `POSTGRES_SOCIAL_PUBLISHER_SSL_MODE` | — | |
| `REDIS_ADDR` | — | **Required** |
| `REDIS_PASSWORD` | — | |

---

## Redis Channels

| Channel | Direction | Purpose |
|---------|-----------|---------|
| `social:publish` | Inbound (subscribe) | Publisher routes content here for social delivery |
| `social:delivery-status` | Outbound (publish) | Delivery lifecycle events (created, delivered, failed) |
| `social:dead-letter` | Outbound (publish) | Permanently failed deliveries after max retries |

**Message format** on `social:publish`: JSON `domain.PublishMessage` with `content_id`, `targets[]` (platform + account pairs).

---

## Database Schema

Three tables (see `migrations/001_initial_schema.up.sql`):

- **`content`** — content items to be published (`id`, `title`, `body`, `url`, `published`, `scheduled_at`, `created_at`)
- **`accounts`** — platform credentials (`id`, `platform`, `handle`, `credentials` BYTEA encrypted with AES-256-GCM)
- **`deliveries`** — delivery lifecycle (`id`, `content_id`, `platform`, `account`, `status`, `platform_id`, `error_message`, `attempt`, `max_attempts`, `next_retry_at`). `UNIQUE(content_id, platform, account)` prevents duplicate deliveries.

---

## Retry Logic

Backoff schedule (from `orchestrator.Backoffs()`): 30s → 2m → 10m. Max 3 attempts (configurable via `SOCIAL_PUBLISHER_MAX_RETRIES`). After max attempts, delivery is marked permanently failed and published to `social:dead-letter`.

---

## Platform Adapter Extension

To add a new platform:
1. Create `internal/adapters/{platform}/adapter.go` implementing `domain.PlatformAdapter`
2. Create `internal/adapters/{platform}/client.go` for the platform HTTP client
3. Register in `main.go`'s `adapters` map: `"{platform}": {platform}adapter.NewAdapter(...)`

The `domain.PlatformAdapter` interface requires: `Transform(msg *PublishMessage) (any, error)`, `Validate(payload any) error`, `Publish(ctx, payload any) (PlatformResult, error)`.

---

## Credential Encryption

Platform credentials (API keys, tokens) are encrypted with AES-256-GCM before storage. The encryption key (`SOCIAL_PUBLISHER_ENCRYPTION_KEY`) must be set before first run — there is no default. Generate with: `openssl rand -hex 32`.

**Important**: The key is loaded at service start. Rotating the key requires re-encrypting all existing `accounts.credentials` rows.

---

## API Reference

All endpoints require `Authorization: Bearer <JWT>`.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/v1/content` | List content items |
| `POST` | `/api/v1/publish` | Trigger immediate publish |
| `GET` | `/api/v1/status/:id` | Get delivery status |
| `POST` | `/api/v1/retry/:id` | Manually retry a failed delivery |
| `GET/POST/PUT/DELETE` | `/api/v1/accounts` | Manage platform accounts |
