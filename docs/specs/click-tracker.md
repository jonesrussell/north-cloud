# Click Tracker Spec

> Last verified: 2026-03-19 (fix: composite PK includes clicked_at for partitioning)

## Overview

Click event tracking service. Receives HMAC-signed redirect URLs from the search service, verifies signatures, buffers click events in-memory, and batch-flushes to PostgreSQL. Bots are detected and excluded. Privacy by design: destination URLs and user agents are stored as hashes.

---

## File Map

```
click-tracker/
  main.go                          # Config -> logger -> DB -> server
  cmd/migrate/main.go              # Database migration runner
  migrations/
    001_create_click_events.*      # Partitioned click_events table
  internal/
    api/
      server.go                    # Gin server via infragin builder
      routes.go                    # Route wiring (BotFilter + RateLimiter)
    config/config.go               # Config struct, defaults, env binding
    domain/click_event.go          # ClickEvent value type
    handler/
      click.go                     # HandleClick: parse -> verify -> expiry -> buffer
      health.go                    # /health endpoint
    middleware/
      botfilter.go                 # UA-based bot detection (24 patterns)
      ratelimit.go                 # In-memory per-IP sliding window rate limiter
    storage/postgres.go            # Buffer (channel) + Store (batch INSERT)
```

---

## API Reference

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/click` | None | Verify signature, buffer event, 302 redirect |
| GET | `/health` | None | Liveness check |
| GET | `/health/memory` | None | Memory usage stats |

### /click query parameters

`q` (query ID), `r` (result ID), `p` (position), `pg` (page), `t` (Unix timestamp), `u` (destination URL, URL-encoded), `sig` (HMAC signature, 12 hex chars).

### Error responses

| Status | Cause |
|--------|-------|
| 400 | Missing or unparseable required parameters |
| 403 | Invalid HMAC signature |
| 410 | URL older than `max_timestamp_age` (default 24h) |
| 429 | Per-IP rate limit exceeded |

---

## Data Model

### click_events table (partitioned by `RANGE (clicked_at)`)

| Column | Type | Notes |
|--------|------|-------|
| `query_id` | text | Search query identifier |
| `result_id` | text | Clicked result identifier |
| `position` | int | Result position on page |
| `page` | int | Search result page number |
| `destination_hash` | text | SHA-256 of destination URL (privacy) |
| `user_agent_hash` | text | First 12 hex chars of SHA-256 of UA |
| `ip_hash` | text | Hashed client IP |
| `clicked_at` | timestamp | Event timestamp |

### Privacy design

- Raw destination URL never stored (only SHA-256 hash)
- User-Agent stored as truncated hash
- Bot clicks are never enqueued

### Buffering

- In-memory channel (default capacity 1,000)
- Non-blocking send: full buffer drops event (redirect still completes)
- Batch flush: 500 events or 1 second, whichever comes first
- Chunks of 50 rows per INSERT statement

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CLICK_TRACKER_PORT` | `8093` | HTTP listen port |
| `CLICK_TRACKER_SECRET` | — | HMAC signing secret (required, must match search service) |
| `APP_DEBUG` | `false` | Gin debug mode |
| `POSTGRES_CLICK_TRACKER_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_CLICK_TRACKER_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_CLICK_TRACKER_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_CLICK_TRACKER_PASSWORD` | — | PostgreSQL password |
| `POSTGRES_CLICK_TRACKER_DB` | `click_tracker` | PostgreSQL database |
| `LOG_LEVEL` | `info` | Log level |
| `LOG_FORMAT` | `json` | Log format |

---

## Known Constraints

- **Secret mismatch causes all clicks to return 403**: `CLICK_TRACKER_SECRET` must match the search service.
- **Buffer drops are silent to users**: when full, events are dropped but redirects succeed. Monitor `buffer full` log entries.
- **`CLICK_TRACKER_ENABLED` lives in the search service, not here**: this service always processes requests. The flag controls URL rewriting in search.
- **Migrations must run before service start**: no auto-migration.
- **Partitions must be created manually**: `click_events_default` catches all rows until named range partitions are added.
- **Bot filter is UA-based**: 24 crawler patterns checked case-insensitively. Empty UA treated as bot.
- **Rate limiter is in-memory and per-process**: no cross-instance coordination.

<\!-- Reviewed: 2026-03-18 — go.mod dependency update only, no spec changes needed -->
