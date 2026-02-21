# Crawler

> Interval-based web crawler with distributed job scheduling for the North Cloud content pipeline.

## Overview

The crawler fetches web content from configured sources and indexes raw articles into Elasticsearch (`{source}_raw_content`) with `classification_status: "pending"`. It uses **source-manager** to look up source configurations (selectors, max depth, etc.) and schedules crawl jobs via an interval-based scheduler — no cron expressions required. Downstream, the classifier picks up pending documents and continues the pipeline.

## Features

- **Interval-based job scheduling** (not cron) — schedule jobs every N minutes, hours, or days
- **7 job states** with validated transitions: pending, scheduled, running, paused, completed, failed, cancelled
- **Distributed locking** via PostgreSQL CAS for safe multi-instance deployments
- **Adaptive scheduling** — exponential backoff when content is unchanged, resets on change
- **Self-balancing load distribution** — jobs spread across 15-minute time slots via BucketMap
- **Frontier-based crawling** with redirect canonicalization (final URL stored, not redirect URL)
- **Extraction quality metrics** — empty title/body counts per execution to detect selector drift
- **Readability fallback extractor** — Mozilla Readability-based last-resort extraction when selectors fail
- **Proxy rotation** — round-robin HTTP and SOCKS5 proxy switching via Colly
- **Redis-backed Colly storage** — persists visited URLs, cookies, and request queue across restarts
- **MinIO HTML archiving** — stores raw HTML for offline reprocessing
- **Real-time job log streaming** — SSE v2 stream for live log tailing from the dashboard
- **RSS/Atom feed polling and discovery** — automatic feed detection and polling
- **Source-manager event consumption** — reacts to source enable/disable events via Redis
- **Force-run API** (v2) — immediately trigger a scheduled job without waiting for its interval
- **Admin sync endpoint** — reconcile enabled sources to jobs after missed events

## Quick Start

### Docker (Recommended)

The crawler runs as part of the North Cloud stack. From the repo root:

```bash
# Start core services
task docker:dev:up

# The crawler API is available at
curl http://localhost:8060/health

# View crawler logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f crawler
```

### Local Development

```bash
cd crawler

# Copy and edit config
cp config.example.yaml config.yml

# Install tools and run migrations
task install:tools
task migrate:up

# Start with hot reload
task dev

# Or build and run directly
go run main.go
```

## API Reference

All `/api/v1/*`, `/api/v2/*`, and `/api/{crawler,health,metrics}/events` routes require JWT authentication (`Authorization: Bearer <token>`). Only the `/health` endpoint is public.

### Jobs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check (public) |
| GET | `/api/v1/jobs` | List all jobs (with filters) |
| POST | `/api/v1/jobs` | Create a new job |
| GET | `/api/v1/jobs/:id` | Get a single job |
| PUT | `/api/v1/jobs/:id` | Update a job |
| DELETE | `/api/v1/jobs/:id` | Delete a job |
| GET | `/api/v1/jobs/status-counts` | Counts grouped by status |

### Job Control

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/jobs/:id/pause` | Pause a scheduled job |
| POST | `/api/v1/jobs/:id/resume` | Resume a paused job |
| POST | `/api/v1/jobs/:id/cancel` | Cancel a job |
| POST | `/api/v1/jobs/:id/retry` | Retry a failed job |
| POST | `/api/v2/jobs/:id/force-run` | Immediately trigger a scheduled job |

### Execution History and Stats

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/jobs/:id/executions` | Execution history (paginated) |
| GET | `/api/v1/jobs/:id/stats` | Success rate, avg duration |
| GET | `/api/v1/executions/:id` | Single execution detail |

### Scheduler

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/scheduler/metrics` | System-wide metrics (counts, rates) |
| GET | `/api/v1/scheduler/distribution` | Hourly job distribution and balance score |
| POST | `/api/v1/scheduler/rebalance/preview` | Preview rebalance moves (dry run) |
| POST | `/api/v1/scheduler/rebalance` | Execute rebalance |

### Job Logs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/jobs/:id/logs` | Log metadata |
| GET | `/api/v1/jobs/:id/logs/stream` | Stream logs (SSE v1) |
| GET | `/api/v1/jobs/:id/logs/stream/v2` | Stream logs (SSE v2, preferred) |
| GET | `/api/v1/jobs/:id/logs/download` | Download log file |
| GET | `/api/v1/jobs/:id/logs/view` | View logs in browser |

### Frontier

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/frontier` | List frontier URLs |
| GET | `/api/v1/frontier/stats` | Frontier statistics |
| POST | `/api/v1/frontier/submit` | Submit a URL to the frontier |
| POST | `/api/v1/frontier/:id/retry` | Retry a frontier entry |
| DELETE | `/api/v1/frontier/:id` | Delete a frontier entry |

### Discovered Links

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/discovered-links` | List discovered links |
| GET | `/api/v1/discovered-links/:id` | Get a discovered link |
| DELETE | `/api/v1/discovered-links/:id` | Delete a discovered link |
| POST | `/api/v1/discovered-links/:id/create-job` | Promote a discovered link to a job |

### SSE Events

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/crawler/events` | Live crawler events stream |
| GET | `/api/health/events` | Live health events stream |
| GET | `/api/metrics/events` | Live metrics events stream |

### Admin

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/admin/sync-enabled-sources` | Reconcile enabled sources to jobs |

## Job States

```
pending ──→ scheduled ──→ running ──→ completed ──→ scheduled (recurring)
                       └─→ failed  ──→ pending   (via retry)
scheduled ──→ paused  ──→ scheduled (via resume)
          └─→ cancelled
running   ──→ cancelled
paused    ──→ cancelled
```

| State | Description |
|-------|-------------|
| `pending` | Created, waiting for first execution or manual retry |
| `scheduled` | Recurring job with a future `next_run_at` |
| `running` | Currently executing |
| `completed` | Finished successfully (recurring jobs auto-reschedule) |
| `failed` | Exhausted all retries |
| `paused` | Manually paused; retains its schedule for later resume |
| `cancelled` | Permanently stopped |

### Valid Control Actions by State

| Action | Valid From States |
|--------|------------------|
| `pause` | `scheduled` only |
| `resume` | `paused` only |
| `cancel` | `scheduled`, `running`, `paused`, `pending` |
| `retry` | `failed` only |
| `force-run` | `scheduled` only |

## Configuration

### Core Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_JWT_SECRET` | — | Shared JWT secret for API authentication |
| `CRAWLER_SERVER_ADDRESS` | `:8080` | HTTP listen address |

### Database

| Variable | Description |
|----------|-------------|
| `POSTGRES_CRAWLER_HOST` | PostgreSQL host |
| `POSTGRES_CRAWLER_PORT` | PostgreSQL port |
| `POSTGRES_CRAWLER_USER` | PostgreSQL user |
| `POSTGRES_CRAWLER_PASSWORD` | PostgreSQL password |
| `POSTGRES_CRAWLER_DB` | Database name |
| `POSTGRES_CRAWLER_SSLMODE` | SSL mode (e.g. `disable`) |

### Crawler Behavior

| Variable | Default | Description |
|----------|---------|-------------|
| `CRAWLER_SOURCES_API_URL` | `http://localhost:8050/api/v1/sources` | Source manager API URL |
| `CRAWLER_MAX_CONCURRENCY` | — | Number of concurrent crawlers |
| `CRAWLER_REQUEST_TIMEOUT` | — | Per-request timeout |
| `CRAWLER_RESPECT_ROBOTS_TXT` | `true` | Honor robots.txt (keep enabled in production) |
| `CRAWLER_USE_RANDOM_USER_AGENT` | `false` | Rotate user agents per request |
| `CRAWLER_DELAY` | — | Minimum delay between requests to same host |
| `CRAWLER_RANDOM_DELAY` | — | Additional random delay |
| `CRAWLER_MAX_BODY_SIZE` | — | Maximum response body size |
| `CRAWLER_READABILITY_FALLBACK_ENABLED` | `false` | Enable Readability extractor as last resort |
| `CRAWLER_SAVE_DISCOVERED_LINKS` | — | Persist discovered links to database |
| `CRAWLER_VALIDATE_URLS` | — | Validate URLs before crawling |
| `CRAWLER_TLS_INSECURE_SKIP_VERIFY` | `false` | Skip TLS verification (dev only) |

### Redis Storage (Colly)

| Variable | Default | Description |
|----------|---------|-------------|
| `CRAWLER_REDIS_STORAGE_ENABLED` | `false` | Persist Colly state in Redis |
| `CRAWLER_REDIS_STORAGE_EXPIRES` | `168h` (7 days) | TTL for Redis Colly state |
| `REDIS_ADDRESS` | — | Redis address (e.g. `redis:6379`) |
| `REDIS_PASSWORD` | — | Redis password |
| `REDIS_DB` | `0` | Redis database index |
| `REDIS_EVENTS_ENABLED` | `false` | Consume source events from Redis |

### Proxy Rotation

| Variable | Description |
|----------|-------------|
| `CRAWLER_PROXIES_ENABLED` | Set to `true` to enable proxy rotation |
| `CRAWLER_PROXY_URLS` | Comma-separated HTTP or SOCKS5 proxy URLs |

### MinIO Archiving

| Variable | Default | Description |
|----------|---------|-------------|
| `CRAWLER_MINIO_ENABLED` | `false` | Enable HTML archiving to MinIO |
| `CRAWLER_MINIO_ENDPOINT` | — | MinIO endpoint URL |
| `CRAWLER_MINIO_ACCESS_KEY` | — | MinIO access key |
| `CRAWLER_MINIO_SECRET_KEY` | — | MinIO secret key |
| `CRAWLER_MINIO_BUCKET` | — | Bucket for raw HTML |
| `CRAWLER_MINIO_METADATA_BUCKET` | — | Bucket for metadata |
| `CRAWLER_MINIO_USE_SSL` | `false` | Use SSL for MinIO connection |

### Elasticsearch

| Variable | Description |
|----------|-------------|
| `ELASTICSEARCH_ADDRESSES` | Comma-separated Elasticsearch node URLs |
| `ELASTICSEARCH_API_KEY` | API key for authentication |
| `ELASTICSEARCH_USERNAME` | Basic auth username |
| `ELASTICSEARCH_PASSWORD` | Basic auth password |
| `ELASTICSEARCH_INDEX_NAME` | Index name override |
| `ELASTICSEARCH_BULK_SIZE` | Bulk indexing batch size |
| `ELASTICSEARCH_FLUSH_INTERVAL` | Bulk flush interval |

### Feed Polling

| Variable | Default | Description |
|----------|---------|-------------|
| `CRAWLER_FEED_POLL_ENABLED` | `true` | Enable RSS/Atom feed polling |
| `CRAWLER_FEED_POLL_INTERVAL_MINUTES` | — | Feed poll interval |
| `CRAWLER_FEED_DISCOVERY_ENABLED` | `true` | Auto-discover feeds from source URLs |
| `CRAWLER_FEED_DISCOVERY_INTERVAL_MINUTES` | — | Feed discovery interval |

### Frontier Fetcher

| Variable | Description |
|----------|-------------|
| `FETCHER_ENABLED` | Enable the frontier fetcher worker pool |
| `FETCHER_WORKER_COUNT` | Number of frontier fetch workers |
| `FETCHER_FOLLOW_REDIRECTS` | Follow HTTP redirects (with canonicalization) |
| `FETCHER_MAX_REDIRECTS` | Maximum redirect hops to follow |
| `FETCHER_REQUEST_TIMEOUT` | Per-request timeout for frontier fetches |
| `FETCHER_MAX_RETRIES` | Max retries for frontier fetches |

### Job Logs

| Variable | Default | Description |
|----------|---------|-------------|
| `JOB_LOGS_ENABLED` | `false` | Enable per-job log capture |
| `JOB_LOGS_SSE_ENABLED` | `false` | Enable SSE log streaming |
| `JOB_LOGS_REDIS_ENABLED` | `false` | Buffer logs in Redis |
| `JOB_LOGS_RETENTION_DAYS` | — | How long to retain log archives |
| `JOB_LOGS_MIN_LEVEL` | `info` | Minimum log level to capture |

## Architecture

```
crawler/
├── main.go                   # Entry point: bootstrap.Start()
├── internal/
│   ├── bootstrap/            # Phased startup: config → logger → db → services → server
│   ├── config/               # Config structs with env: tags (per subsystem)
│   │   ├── crawler/          # Crawler behavior (concurrency, delays, proxies, etc.)
│   │   ├── database/         # PostgreSQL connection
│   │   ├── elasticsearch/    # Elasticsearch connection and bulk settings
│   │   ├── logs/             # Job log capture settings
│   │   ├── minio/            # MinIO archive settings
│   │   └── server/           # HTTP server settings
│   ├── api/                  # HTTP handlers (Gin)
│   │   ├── jobs_handler.go   # CRUD, control, stats, executions
│   │   ├── logs_handler.go   # Log metadata and download
│   │   ├── logs_stream_v2_handler.go  # SSE v2 log streaming
│   │   ├── frontier_handler.go        # Frontier management
│   │   ├── discovered_links_handler.go
│   │   ├── migration_handler.go
│   │   └── sse_handler.go    # Crawler/health/metrics SSE streams
│   ├── scheduler/            # Interval-based job scheduler (NOT cron)
│   ├── crawler/              # Core Colly-based scraping logic
│   ├── database/             # PostgreSQL repositories (jobs, executions, frontier)
│   ├── domain/               # Job and JobExecution models, state machine
│   ├── sources/              # Source manager API client
│   ├── storage/              # Elasticsearch indexing
│   ├── adaptive/             # Hash-based adaptive scheduling (SHA-256 content comparison)
│   ├── admin/                # Admin endpoints (sync-enabled-sources)
│   ├── archive/              # MinIO HTML archiving
│   ├── coordination/         # Distributed leader election (redlock)
│   ├── events/               # Redis event consumer (source enable/disable)
│   ├── feed/                 # RSS/Atom feed polling and discovery
│   ├── fetcher/              # Frontier fetcher worker pool
│   ├── frontier/             # URL frontier (crawl queue) management
│   ├── job/                  # Job orchestration helpers
│   ├── logs/                 # Per-job log capture and streaming
│   ├── metrics/              # Prometheus-style metrics collection
│   ├── queue/                # Internal work queue
│   └── worker/               # Worker pool for concurrent crawling
├── cmd/                      # CLI subcommands (migrate, etc.)
├── docs/                     # Additional documentation
│   └── INTERVAL_SCHEDULER.md # Comprehensive scheduler reference
├── migrations/               # SQL migration files
├── fixtures/                 # HTTP replay fixtures for tests
└── scripts/                  # Operational scripts
    └── sync-enabled-sources-jobs.sh
```

## Colly Behavior

**Robots.txt**: Respected by default. Set `CRAWLER_RESPECT_ROBOTS_TXT=false` to disable (not recommended in production).

**Random user agents**: Disabled by default. Set `CRAWLER_USE_RANDOM_USER_AGENT=true` to rotate user agents per request.

**Max crawl depth**: Default is 3 when `max_depth` is unset in the source config. Sources with `max_depth > 5` trigger a startup warning.

## Adaptive Scheduling

Jobs with `adaptive_scheduling: true` (the default) adjust their interval based on whether content has changed:

- After each crawl, a SHA-256 hash of the start URL content is compared to the previous run
- **Changed**: interval resets to the job's configured baseline
- **Unchanged**: interval doubles (`baseline x 2^unchanged_count`), capped at 24 hours
- State is stored in Redis at `crawler:adaptive:{source_id}`
- Set `adaptive_scheduling: false` on a job to use a fixed interval

## Load Balancing

The scheduler distributes jobs across 15-minute time slots using a BucketMap to prevent resource contention:

- New and resumed jobs are placed in the least-loaded available slot
- Recurring jobs preserve their "rhythm" (slot phase) across intervals
- Anti-thrashing: jobs cannot be moved if the next run is within 30 minutes, or if they were moved within the last hour
- The `/api/v1/scheduler/distribution` endpoint returns hourly counts and a distribution score (0–1, higher is better)
- Use `/api/v1/scheduler/rebalance/preview` to see proposed moves before applying them

## Execution History and Retention

- Every job run creates a record in the `job_executions` table
- Records include: duration, items crawled, items indexed, error message, resource usage
- Automatic pruning: keeps the 100 most recent executions per job, or 30 days, whichever is smaller
- Extraction quality metrics are stored in `metadata.crawl_metrics.extraction_quality`

## Retry Behavior

- Automatic retry on failure: exponential backoff — `base × 2^(attempt-1)`, capped at 1 hour
- Example sequence: 60s → 120s → 240s → 480s → 960s → 3600s
- `max_retries` (default: 3) controls how many automatic retries before the job is marked `failed`
- Manual retry (`POST /api/v1/jobs/:id/retry`) resets the retry counter and puts the job back to `pending`

## Integration

### Source Manager Dependency

Jobs require a valid `source_id` from source-manager. Without it, the job will be created but execution will fail. Fetch the source ID from source-manager before creating a job:

```bash
curl http://localhost:8050/api/v1/sources
```

### Elasticsearch Output

Each crawled article is indexed into `{source_name}_raw_content` with:

```json
{
  "classification_status": "pending",
  "title": "...",
  "body": "...",
  "url": "...",
  "source_name": "...",
  "published_at": "...",
  "json_ld_data": { ... }
}
```

The classifier polls this index for documents with `classification_status: "pending"`.

### Sync Enabled Sources

After missed events (e.g. on production after a restart), call the admin sync endpoint to ensure every enabled source in source-manager has a corresponding scheduled job:

```bash
# From repo root; requires JWT
JWT="your-token" ./scripts/sync-enabled-sources-jobs.sh
```

This creates missing jobs (with deterministic stagger), resumes paused jobs, and returns a reconciliation report.

## Development

```bash
# Run tests
task test
# or
go test ./...

# Run linter
task lint
# or
golangci-lint run

# Run migrations
task migrate:up

# Force lint (bypasses cache — use before pushing)
task lint:force

# Run with hot reload
task dev
```

## Troubleshooting

**Job not executing**: Check `is_paused`, `status`, and `lock_token` on the job. A non-null `lock_token` that is more than 5 minutes old indicates a stale lock — the scheduler cleans these automatically every minute, or you can clear manually:

```sql
UPDATE jobs SET lock_token = NULL, lock_acquired_at = NULL
WHERE lock_acquired_at < NOW() - INTERVAL '5 minutes';
```

**`document_parsing_exception` for `json_ld_data.jsonld_raw.publisher`**: The index was created before the canonical mapping was in place. Elasticsearch applied dynamic mapping and locked `publisher` as an object type; normalized documents send a string and are rejected. Fix: delete the `{source}_raw_content` index (via index-manager API or Elasticsearch directly), then re-crawl to recreate it with the canonical mapping. To preserve existing data, create a new correctly-mapped index, reindex from the old one, then drop the old index.

**High failure rate**: Review execution history at `GET /api/v1/jobs/:id/executions` and check `error_message`. Increase `max_retries` or `retry_backoff_seconds` via `PUT /api/v1/jobs/:id` if the source is unreliable.

## Further Reading

- [docs/INTERVAL_SCHEDULER.md](docs/INTERVAL_SCHEDULER.md) — Comprehensive scheduler reference with load balancing details, state diagrams, and performance notes
- [CLAUDE.md](CLAUDE.md) — Developer guide with architecture, gotchas, and code patterns
