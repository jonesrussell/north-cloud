# Crawler — Developer Guide

## Quick Reference

```bash
# Daily commands
task dev              # Start with hot reload (Air)
task test             # Run all tests
task lint             # Run linter
task lint:force       # Force lint (bypasses cache — use before pushing)
task migrate:up       # Run database migrations

# API (port 8060)
curl http://localhost:8060/health
curl -H "Authorization: Bearer $JWT" http://localhost:8060/api/v1/jobs
curl -H "Authorization: Bearer $JWT" http://localhost:8060/api/v1/scheduler/metrics
```

### Useful One-Liners

```bash
# Create a recurring job (6-hour interval)
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT" \
  -d '{
    "source_id": "uuid-from-source-manager",
    "url": "https://example.com",
    "interval_minutes": 360,
    "interval_type": "minutes",
    "schedule_enabled": true
  }'

# Create a one-time job (run immediately)
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT" \
  -d '{
    "source_id": "uuid-from-source-manager",
    "url": "https://example.com",
    "schedule_enabled": false
  }'

# Pause a job
curl -X POST http://localhost:8060/api/v1/jobs/{id}/pause \
  -H "Authorization: Bearer $JWT"

# Retry a failed job
curl -X POST http://localhost:8060/api/v1/jobs/{id}/retry \
  -H "Authorization: Bearer $JWT"

# Force-run a scheduled job now (v2 API)
curl -X POST http://localhost:8060/api/v2/jobs/{id}/force-run \
  -H "Authorization: Bearer $JWT"

# View scheduler-wide metrics
curl -H "Authorization: Bearer $JWT" http://localhost:8060/api/v1/scheduler/metrics

# View job distribution across time slots
curl -H "Authorization: Bearer $JWT" http://localhost:8060/api/v1/scheduler/distribution

# Sync all enabled sources to jobs (production reconciliation)
JWT="your-token" ./scripts/sync-enabled-sources-jobs.sh
```

## Architecture

### Directory Map

```
crawler/
├── main.go                   # Entry point: bootstrap.Start()
├── internal/
│   ├── bootstrap/            # Phased startup: config → logger → db → services → server
│   │   ├── app.go            # Top-level Start() orchestration
│   │   ├── config.go         # Config loading and logger creation
│   │   ├── database.go       # PostgreSQL initialization
│   │   ├── server.go         # HTTP server and route registration
│   │   ├── services.go       # Service wiring (scheduler, crawler, storage, etc.)
│   │   ├── storage.go        # Elasticsearch and MinIO setup
│   │   ├── redis.go          # Redis client setup
│   │   ├── events.go         # Redis event consumer wiring
│   │   ├── fetcher_adapters.go  # Frontier fetcher adapter wiring
│   │   ├── frontier_logging.go  # Frontier logger setup
│   │   └── lifecycle.go      # Graceful shutdown
│   │
│   ├── config/               # Config structs with env: tags
│   │   ├── config.go         # Root config (auth, redis, feed, source-manager)
│   │   ├── crawler/          # Crawler behavior (concurrency, delays, proxies, etc.)
│   │   ├── database/         # PostgreSQL connection
│   │   ├── elasticsearch/    # Elasticsearch connection and bulk indexing
│   │   ├── logs/             # Per-job log capture settings
│   │   ├── minio/            # MinIO archiving settings
│   │   └── server/           # HTTP server settings
│   │
│   ├── api/                  # HTTP handlers (Gin)
│   │   ├── api.go            # Route registration
│   │   ├── jobs_handler.go   # CRUD, control (pause/resume/cancel/retry/force-run), stats
│   │   ├── logs_handler.go   # Log metadata, download, view
│   │   ├── logs_stream_v2_handler.go  # SSE v2 log streaming
│   │   ├── frontier_handler.go        # Frontier management
│   │   ├── discovered_links_handler.go
│   │   ├── migration_handler.go
│   │   ├── sse_handler.go    # Crawler/health/metrics SSE events
│   │   └── middleware/       # Auth, logging, recovery middleware
│   │
│   ├── scheduler/            # Interval-based job scheduler (NOT cron)
│   ├── crawler/              # Core Colly-based scraping logic
│   ├── database/             # PostgreSQL repositories (jobs, executions, frontier, links)
│   ├── domain/               # Job and JobExecution models, state machine validation
│   ├── sources/              # Source manager API client
│   ├── storage/              # Elasticsearch document indexing
│   ├── adaptive/             # Hash-based adaptive scheduling (SHA-256 content comparison)
│   ├── admin/                # Admin endpoints (sync-enabled-sources)
│   ├── archive/              # MinIO HTML archiving
│   ├── coordination/         # Distributed leader election (redlock)
│   ├── content/              # Content extraction helpers
│   ├── events/               # Redis event consumer (source enable/disable)
│   ├── feed/                 # RSS/Atom feed polling and discovery
│   ├── fetcher/              # Frontier fetcher worker pool
│   ├── frontier/             # URL frontier (crawl queue) management
│   ├── job/                  # Job orchestration helpers
│   ├── logs/                 # Per-job log capture and streaming infrastructure
│   ├── metrics/              # Prometheus-style metrics collection
│   ├── queue/                # Internal work queue
│   └── worker/               # Worker pool for concurrent crawling
│
├── cmd/                      # CLI subcommands (migrate, etc.)
├── docs/
│   └── INTERVAL_SCHEDULER.md # Comprehensive scheduler reference
├── migrations/               # SQL migration files
├── fixtures/                 # HTTP replay fixtures for deterministic testing
└── scripts/
    └── sync-enabled-sources-jobs.sh
```

### Startup Phase Order

`bootstrap.Start()` wires services in this order to respect dependencies:

1. **Config** — load `config.yml`, apply env overrides
2. **Logger** — structured JSON logger with `service=crawler`
3. **Database** — PostgreSQL connection pool (migrations run via `cmd/migrate`)
4. **Redis** — client for Colly storage, adaptive scheduling state, event consumption, and job log buffering
5. **Storage** — Elasticsearch bulk indexer
6. **MinIO** — HTML archive client (if `CRAWLER_MINIO_ENABLED=true`)
7. **Services** — scheduler, adaptive scheduler, source manager client, fetcher pool, feed poller
8. **Server** — Gin HTTP server with all routes registered
9. **Lifecycle** — graceful shutdown on SIGTERM/SIGINT

## Key Concepts

### Interval-Based Scheduling (NOT Cron)

Jobs use simple intervals, not cron expressions. `interval_type` is `"minutes"`, `"hours"`, or `"days"`:

```json
{
  "interval_minutes": 30,
  "interval_type": "minutes",
  "schedule_enabled": true
}
```

One-time jobs: omit `interval_minutes` (or set to `NULL`) and set `schedule_enabled: false`.

### 7 Job States and Valid Transitions

```
pending ──→ scheduled ──→ running ──→ completed ──→ scheduled (recurring)
                       └─→ failed  ──→ pending   (via retry)
scheduled ──→ paused  ──→ scheduled (via resume)
          └─→ cancelled
running   ──→ cancelled
paused    ──→ cancelled
```

| Action | Valid From States |
|--------|------------------|
| `pause` | `scheduled` only |
| `resume` | `paused` only |
| `cancel` | `scheduled`, `running`, `paused`, `pending` |
| `retry` | `failed` only |
| `force-run` | `scheduled` only |

Terminal states — no further transitions: `completed` (one-time), `failed` (retries exhausted), `cancelled`.

### Distributed Locking

PostgreSQL CAS-based locking prevents duplicate execution across multiple crawler instances:

```sql
UPDATE jobs SET lock_token = $1, lock_acquired_at = NOW()
WHERE id = $2 AND lock_token IS NULL
```

- If the UPDATE affects 1 row: lock acquired — proceed with execution
- If the UPDATE affects 0 rows: another instance holds the lock — skip
- **Stale lock cleanup**: runs every 1 minute, clears locks older than 5 minutes
- Jobs stuck with a stale lock will recover automatically within 5 minutes

### Scheduler Internal Flow

1. **Job Poller** (every 10s): queries `WHERE next_run_at <= NOW() AND NOT is_paused AND status IN ('pending','scheduled') AND lock_token IS NULL`
2. **Lock Acquisition**: atomic CAS update (see above)
3. **Execution**: creates `job_executions` row (status=`running`), runs Colly crawler with cancellable context
4. **Completion (success)**: marks execution `completed`, sets job `completed`, calculates `next_run_at` for recurring jobs, releases lock
5. **Completion (failure)**: marks execution `failed`, applies exponential backoff retry or marks job `failed`, releases lock
6. **Metrics Collector** (every 30s): aggregates job counts and success rates into memory for the metrics API

### Adaptive Scheduling

Jobs with `adaptive_scheduling: true` (default) adjust their interval based on content changes:

- After each crawl, SHA-256 of the start URL content is compared to the previous hash
- **Changed**: interval resets to baseline
- **Unchanged**: interval doubles (`baseline x 2^unchanged_count`), capped at 24 hours
- State stored in Redis at `crawler:adaptive:{source_id}`
- Set `adaptive_scheduling: false` for fixed-interval jobs

### Frontier and Redirects

The frontier fetcher worker pool follows HTTP redirects. On success after redirects, the frontier row's URL is updated to the final URL (canonicalization). Redirect failures are stored with `last_error=too_many_redirects` so they can be distinguished from truly dead URLs in the dashboard.

### Load Balancing (BucketMap)

Jobs are distributed across 15-minute time slots:

- New and resumed jobs land in the least-loaded slot
- Recurring jobs preserve their slot phase across intervals (rhythm preservation)
- Anti-thrashing: no job can be moved if its next run is within 30 minutes, or if it was moved in the last hour
- Distribution score (0–1) accessible at `/api/v1/scheduler/distribution`
- Preview moves before applying: `POST /api/v1/scheduler/rebalance/preview`

### Retry with Exponential Backoff

`base × 2^(attempt-1)`, capped at 1 hour:

```
60s → 120s → 240s → 480s → 960s → 3600s (cap)
```

`max_retries` defaults to 3. Manual retry (`POST /jobs/:id/retry`) resets `current_retry_count` to 0.

### Execution History Pruning

Automatic pruning keeps either the 100 most recent executions per job OR the last 30 days, whichever is more restrictive.

## API Reference

Full endpoint table is in [README.md](README.md). Key summary:

| Group | Prefix |
|-------|--------|
| Jobs CRUD | `GET/POST/PUT/DELETE /api/v1/jobs[/:id]` |
| Job control | `POST /api/v1/jobs/:id/{pause,resume,cancel,retry}` |
| Force-run (v2) | `POST /api/v2/jobs/:id/force-run` |
| Execution history | `GET /api/v1/jobs/:id/executions`, `GET /api/v1/executions/:id` |
| Stats | `GET /api/v1/jobs/:id/stats`, `GET /api/v1/jobs/status-counts` |
| Scheduler | `GET /api/v1/scheduler/metrics`, `/distribution`, `/rebalance[/preview]` |
| Job logs | `GET /api/v1/jobs/:id/logs[/stream/v2]` |
| Frontier | `GET/POST/DELETE /api/v1/frontier[/:id]` |
| Discovered links | `GET/DELETE /api/v1/discovered-links[/:id]` |
| SSE events | `GET /sse/{crawler,health,metrics}/events` |
| Admin | `POST /api/v1/admin/sync-enabled-sources` |

## Configuration

See [README.md](README.md) for the full environment variable table. Key variables:

| Variable | Default | Notes |
|----------|---------|-------|
| `AUTH_JWT_SECRET` | — | Required; shared with all services |
| `CRAWLER_SOURCES_API_URL` | `http://localhost:8050/api/v1/sources` | Overridden in Docker |
| `CRAWLER_RESPECT_ROBOTS_TXT` | `true` | Keep enabled in production |
| `CRAWLER_USE_RANDOM_USER_AGENT` | `false` | Enable for UA rotation |
| `CRAWLER_REDIS_STORAGE_ENABLED` | `false` | Persist Colly state across restarts |
| `CRAWLER_PROXIES_ENABLED` | `false` | Enable proxy rotation |
| `CRAWLER_PROXY_URLS` | — | Comma-separated HTTP/SOCKS5 URLs |
| `CRAWLER_READABILITY_FALLBACK_ENABLED` | `false` | Last-resort content extraction |
| `CRAWLER_MINIO_ENABLED` | `false` | HTML archiving |
| `FETCHER_FOLLOW_REDIRECTS` | `true` | Frontier redirect following |
| `FETCHER_MAX_REDIRECTS` | — | Max redirect hops |
| `REDIS_EVENTS_ENABLED` | `false` | Source enable/disable event consumption |

## Common Gotchas

1. **`source_id` is REQUIRED** — jobs must have a valid `source_id` from source-manager. Job creation succeeds without it, but execution will fail. Always obtain the `source_id` from source-manager before creating a job.

2. **Interval fields are conditional**:
   - `interval_minutes = NULL` → one-time job (runs once immediately)
   - `interval_minutes > 0` → recurring job; `next_run_at` is auto-calculated

3. **Retry backoff is exponential**: `base × 2^(attempt-1)`, capped at 1 hour. Manually retrying resets the counter.

4. **Execution history pruning**: Auto-deletes executions older than 30 days OR beyond the 100 most recent per job.

5. **Stale lock recovery takes up to 5 minutes**: If the crawler crashes mid-execution, the job stays `running` with a held lock. The stale lock cleanup (runs every 1 minute) will clear it after 5 minutes. Do not manually clear locks unless the scheduler is fully stopped.

6. **`document_parsing_exception` for `json_ld_data.jsonld_raw.publisher`**: The index was created before the canonical mapping was in place. Elasticsearch applied dynamic mapping and locked `publisher` as an object type; when normalized documents send a string for that field, Elasticsearch rejects them.

   **Fix**: Delete the affected `{source}_raw_content` index (via index-manager API or Elasticsearch directly), then re-crawl so the crawler recreates it with the canonical mapping. To preserve data: create a new index with the correct mapping via index-manager, reindex from the old index, then switch over and drop the old index.

7. **Max depth default is 3**: When `max_depth` is 0 (unset) in the source config, the crawler defaults to depth 3. Sources configured with `max_depth > 5` log a startup warning.

8. **Redis Colly storage falls back to in-memory**: If `CRAWLER_REDIS_STORAGE_ENABLED=true` but Redis is unavailable, the crawler falls back to in-memory storage silently. Visited URL state will not persist across restarts in that case.

9. **Proxy rotation is global**: `CRAWLER_PROXY_URLS` applies to all sources — there is no per-source proxy configuration. Rotation is round-robin.

10. **Feed discovery vs. feed polling**: `CRAWLER_FEED_DISCOVERY_ENABLED` auto-discovers RSS/Atom feeds from source URLs. `CRAWLER_FEED_POLL_ENABLED` polls discovered feeds. Both default to `false` — enable both to get feed-based crawling.

## Testing

```bash
# Run all tests
go test ./...

# Run only scheduler tests
go test -v ./internal/scheduler/...

# Run specific test suites
go test -v ./internal/scheduler/... -run TestValidateStateTransition
go test -v ./internal/scheduler/... -run TestSchedulerMetrics

# Run database integration tests (requires running postgres-crawler)
go test -v ./internal/database/... -tags=integration

# Run with coverage
task test:cover
# or
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## Code Patterns

### Creating a Job (curl)

```bash
# Recurring job — every 6 hours
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT" \
  -d '{
    "source_id": "uuid-from-source-manager",
    "url": "https://example.com/news",
    "interval_minutes": 360,
    "interval_type": "minutes",
    "schedule_enabled": true,
    "max_retries": 3,
    "retry_backoff_seconds": 60
  }'

# One-time job — run immediately
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT" \
  -d '{
    "source_id": "uuid-from-source-manager",
    "url": "https://example.com/news",
    "schedule_enabled": false
  }'
```

### Checking Scheduler Health

```bash
# Overall metrics
curl -H "Authorization: Bearer $JWT" http://localhost:8060/api/v1/scheduler/metrics

# Example response fields to watch:
# executions.success_rate < 0.8 → investigate failing jobs
# stale_locks_cleared > 0 consistently → scheduler may be crashing

# Job distribution
curl -H "Authorization: Bearer $JWT" http://localhost:8060/api/v1/scheduler/distribution
# distribution_score < 0.7 → consider running rebalance

# Preview rebalance (dry run)
curl -X POST -H "Authorization: Bearer $JWT" \
  http://localhost:8060/api/v1/scheduler/rebalance/preview

# Execute rebalance
curl -X POST -H "Authorization: Bearer $JWT" \
  http://localhost:8060/api/v1/scheduler/rebalance
```

### Database Schema Reference

**`jobs` table key fields**:
- `source_id` (required), `url`, `status`
- `interval_minutes`, `interval_type`, `next_run_at`
- `adaptive_scheduling` (default `true`)
- `is_paused`, `paused_at`, `cancelled_at`
- `lock_token`, `lock_acquired_at`
- `max_retries`, `retry_backoff_seconds`, `current_retry_count`
- `metadata` JSONB

**`job_executions` table key fields**:
- `job_id`, `execution_number`, `status`
- `started_at`, `completed_at`, `duration_ms`
- `items_crawled`, `items_indexed`, `error_message`, `stack_trace`
- `cpu_time_ms`, `memory_peak_mb`
- `retry_attempt`, `metadata` JSONB

## Documentation

- [README.md](README.md) — Full public-facing documentation (API, configuration, integration)
- [docs/INTERVAL_SCHEDULER.md](docs/INTERVAL_SCHEDULER.md) — Comprehensive scheduler reference with load balancing, state diagrams, and performance notes
