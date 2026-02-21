# Click Tracker — Developer Guide

## Quick Reference

```bash
# Run all tests
cd click-tracker && go test ./...

# Lint (matches CI)
task lint

# Lint with empty cache (exact CI parity before pushing)
task lint:no-cache

# Build binary
task build

# Run with hot reload
task dev   # requires Air

# Apply migrations
go run cmd/migrate/main.go up

# Health check
curl http://localhost:8093/health

# Trigger a click (requires a valid signature — use the search service to generate one)
curl -v "http://localhost:8093/click?q=q_abc&r=r_doc&p=1&pg=1&t=$(date +%s)&u=https%3A%2F%2Fexample.com&sig=SIGNATURE"
```

## Architecture

```
click-tracker/
├── main.go                        # Entry point: config → logger → DB → server
├── cmd/migrate/main.go            # Database migration runner (up/down)
├── config.yml.example
├── migrations/
│   └── 001_create_click_events.*  # Partitioned click_events table
└── internal/
    ├── api/
    │   ├── server.go              # Gin server via infragin builder
    │   └── routes.go              # Route wiring; applies BotFilter + RateLimiter
    ├── config/config.go           # Config struct, defaults, env binding, validation
    ├── domain/click_event.go      # ClickEvent value type
    ├── handler/
    │   ├── click.go               # HandleClick: parse → verify → expiry → buffer
    │   └── health.go              # /health endpoint
    ├── middleware/
    │   ├── botfilter.go           # Sets is_bot=true for 24 crawler UA patterns
    │   └── ratelimit.go           # In-memory per-IP sliding window rate limiter
    └── storage/postgres.go        # Buffer (channel) + Store (batch INSERT to PG)
```

## Key Concepts

**Signed redirect URLs**: The `infrastructure/clickurl` package produces and verifies HMAC-SHA256 signatures. The signed message is `{query_id}|{result_id}|{position}|{page}|{timestamp}|{destination_url}`. Only the first 12 hex characters of the digest are included in the URL to keep it short. Verification uses `hmac.Equal` (constant-time) to prevent timing attacks.

**In-memory buffer**: Click events are sent to a `chan domain.ClickEvent` (default capacity 1,000) via a non-blocking `select`. If the channel is full, the event is dropped and a warning is logged — the redirect still completes. The buffer is drained on graceful shutdown.

**Batch flush**: `storage.Store` runs a background goroutine that flushes the buffer to PostgreSQL when either the batch reaches `flush_threshold` (default 500) or `flush_interval` (default 1 second) elapses. Batches are split into chunks of up to 50 rows per `INSERT` statement.

**Privacy by design**: The raw destination URL and User-Agent string are never written to the database. The destination URL is stored as its full SHA-256 hex digest (`destination_hash`); the UA is stored as the first 12 hex characters of its SHA-256 digest (`user_agent_hash`).

**Bot passthrough**: Bots are still redirected (so crawlers follow links correctly), but their events are never enqueued. The `BotFilter` middleware sets a `is_bot` context key; `HandleClick` checks this key before calling `enqueueEvent`.

**Timestamp expiry**: Each click URL embeds a Unix timestamp (`t`). The handler rejects URLs where `time.Since(generated) > maxAge` (default 24 hours) with `410 Gone`.

**Partitioned table**: `click_events` is partitioned by `RANGE (clicked_at)`. A `click_events_default` partition catches all rows until named partitions are added. This supports efficient time-based purging and archival without full-table scans.

## API Reference

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/click` | None | Verify signature, buffer event, redirect |
| GET | `/health` | None | Liveness check |
| GET | `/health/memory` | None | Memory usage stats |

### /click query parameters

`q` (query ID), `r` (result ID), `p` (position), `pg` (page, optional), `t` (Unix timestamp), `u` (destination URL, URL-encoded), `sig` (HMAC signature, 12 hex chars).

### Error responses

| Status | Cause |
|--------|-------|
| 400 | Missing or unparseable required parameters |
| 403 | Invalid HMAC signature |
| 410 | URL older than `max_timestamp_age` |
| 429 | Per-IP rate limit exceeded |

## Configuration

Required environment variable: `CLICK_TRACKER_SECRET` (must match the search service secret).

| Variable | Default | Description |
|----------|---------|-------------|
| `CLICK_TRACKER_PORT` | `8093` | HTTP listen port |
| `CLICK_TRACKER_SECRET` | — | HMAC signing secret (required) |
| `APP_DEBUG` | `false` | Enable debug / verbose Gin output |
| `POSTGRES_CLICK_TRACKER_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_CLICK_TRACKER_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_CLICK_TRACKER_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_CLICK_TRACKER_PASSWORD` | — | PostgreSQL password |
| `POSTGRES_CLICK_TRACKER_DB` | `click_tracker` | PostgreSQL database |
| `POSTGRES_CLICK_TRACKER_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | `json` or `console` |

Config values in `config.yml` are overridden by environment variables. Config file path is resolved by `infraconfig.GetConfigPath("config.yml")` (checks `CONFIG_PATH` env var, then working directory).

## Common Gotchas

1. **Secret mismatch causes all clicks to return 403.** The `CLICK_TRACKER_SECRET` must be identical in both click-tracker and the search service (`CLICK_TRACKER_SECRET` / `click_tracker.secret` in search config). A difference of even one character means every signature fails.

2. **Buffer drops under high load are silent to the user.** When the buffer channel is full, `buffer.Send` returns `false`, the event is dropped, and a `WARN` log is emitted. The redirect still succeeds. Monitor `buffer full` log entries; increase `buffer_size` or `flush_threshold` if they appear regularly.

3. **`CLICK_TRACKER_ENABLED` lives in the search service, not here.** Click Tracker has no toggle of its own — it always processes requests. The `CLICK_TRACKER_ENABLED` flag only controls whether the search service rewrites result URLs.

4. **Migrations must run before the service starts.** The service does not auto-migrate. Run `go run cmd/migrate/main.go up` (or the equivalent Docker entrypoint) before first startup, or after deploying a new migration.

5. **The `click_events_default` partition is unbounded.** Named range partitions (e.g. monthly) must be created manually before data volume grows. Without them, all rows go to the default partition, making pruning harder.

6. **Bot filter checks User-Agent substrings, case-insensitively.** Empty User-Agent strings are also treated as bots (`is_bot=true`). Requests with no UA are still redirected but not recorded.

7. **Rate limiter state is in-memory and per-process.** If multiple replicas run behind a load balancer, each instance maintains its own counter. A user may exceed the rate limit on a single instance while appearing under-limit across instances.

## Testing

```bash
# All tests
go test ./...

# Unit tests with verbose output
go test -v ./internal/...

# Race detector
go test -race ./...

# Coverage report (HTML)
task test:coverage
# Opens coverage.html

# Run a specific package
GOWORK=off go test ./internal/handler/... -v
```

Tests do not require a running database. `storage.Buffer` and `handler.ClickHandler` are tested with in-memory state; `postgres_test.go` uses integration-style tests (skipped if `DATABASE_URL` is not set).

## Code Patterns

### Adding a new middleware

Register it in `internal/api/routes.go` inside the route group. Middleware added to the `click` group applies only to `/click`; middleware on `router` itself applies globally.

```go
click := router.Group("")
click.Use(middleware.BotFilter())
click.Use(middleware.RateLimiter(...))
click.Use(middleware.YourNewMiddleware())
click.GET("/click", clickHandler.HandleClick)
```

### Accessing context values set by middleware

```go
isBot, _ := c.Get("is_bot")
if isBot == true { ... }
```

### Non-blocking buffer send pattern

```go
if !h.buffer.Send(event) {
    h.logger.Warn("Click event buffer full, dropping event",
        infralogger.String("query_id", params.QueryID),
    )
}
```

### Config access pattern

Use `infraconfig.LoadWithDefaults[Config](path, setDefaults)`. Never call `os.Getenv` directly outside of `cmd/` or `internal/config/` — the `forbidigo` linter will flag it.
