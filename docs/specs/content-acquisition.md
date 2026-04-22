# Content Acquisition Specification

> Last verified: 2026-04-22 (Phase 1B: ES mapping SSoT in infrastructure/esmapping)

Covers the crawler subsystem: web content fetching, job scheduling, frontier URL management, and raw content indexing.

## File Map

| File | Purpose |
|------|---------|
| `crawler/main.go` | Entry point → `bootstrap.Start()` |
| `crawler/internal/bootstrap/app.go` | 7-phase startup orchestration; background workers log errors on failure |
| `crawler/internal/crawler/crawler.go` | Core Crawler struct and CrawlerInterface |
| `crawler/internal/crawler/factory.go` | Factory pattern for per-job isolation |
| `crawler/internal/scheduler/interval_scheduler.go` | Interval-based job scheduler with CAS locking |
| `crawler/internal/scheduler/scheduler_execution.go` | Per-job `runJob` goroutine; execution timeout context and cleanup |
| `infrastructure/esmapping/` | SSoT Elasticsearch `raw_content` / `classified_content` field maps (shared by classifier + index-manager) |
| `crawler/internal/scheduler/state_machine.go` | Job state transitions (pending→scheduled→running→completed/failed) |
| `crawler/internal/fetcher/worker.go` | Frontier fetcher worker pool (lightweight URL fetching) |
| `crawler/internal/storage/types/interface.go` | Storage + IndexManager interfaces |
| `crawler/internal/storage/raw_content_indexer.go` | RawContent model and ES indexing |
| `crawler/internal/database/interfaces.go` | JobRepositoryInterface, ExecutionRepositoryInterface |
| `crawler/internal/database/job_repository.go` | PostgreSQL job persistence |
| `crawler/internal/sources/sources.go` | Source manager API client (lazy, thread-safe) |
| `crawler/internal/domain/job.go` | Job struct (scheduling, locking, state) |
| `crawler/internal/domain/execution.go` | JobExecution + JobStats |
| `crawler/internal/domain/frontier.go` | FrontierURL, HostState, FeedState |
| `crawler/internal/adaptive/hash_tracker.go` | SHA-256 content change detection (Redis-backed) |
| `crawler/internal/proxypool/` | Domain-sticky round-robin proxy rotation |
| `crawler/internal/api/` | REST API handlers (jobs, frontier, logs, scheduler) |
| `crawler/internal/config/` | Configuration structs with env tags |
| `crawler/migrations/` | PostgreSQL schema (20 migrations) |

## Interface Signatures

### CrawlerInterface (`internal/crawler/crawler.go`)
```go
type CrawlerInterface interface {
    Start(ctx context.Context, sourceID string) error
    Stop(ctx context.Context) error
    Subscribe(handler events.EventHandler)
    GetMetrics() *metrics.Metrics
}

type FactoryInterface interface {
    Create() (Interface, error)
}
```

### Storage (`internal/storage/types/interface.go`)
```go
type Interface interface {
    IndexDocument(ctx context.Context, index, id string, document any) error
    IndexDocumentIfAbsent(ctx context.Context, index, id string, document any) error
    GetDocument(ctx context.Context, index, id string, document any) error
    DeleteDocument(ctx context.Context, index, id string) error
    CreateIndex(ctx context.Context, index string, mapping map[string]any) error
    IndexExists(ctx context.Context, index string) (bool, error)
    Close() error
    // Query methods (internal use; not part of public types.Interface)
    SearchDocuments(ctx context.Context, index string, query map[string]any, result any) error
    Search(ctx context.Context, index string, query any) ([]any, error)
    Count(ctx context.Context, index string, query any) (int64, error)
    Aggregate(ctx context.Context, index string, aggs any) (any, error)
}

type IndexManager interface {
    EnsureIndex(ctx context.Context, name string, mapping any) error
    DeleteIndex(ctx context.Context, name string) error
    IndexExists(ctx context.Context, name string) (bool, error)
}
```

### Job Repository (`internal/database/interfaces.go`)
```go
type JobRepositoryInterface interface {
    Create(ctx, *Job) error
    GetByID(ctx, id) (*Job, error)
    List(ctx, params) ([]*Job, error)
    Update(ctx, *Job) error
    Delete(ctx, id) error
    GetJobsReadyToRun(ctx) ([]*Job, error)
    AcquireLock(ctx, jobID, token uuid.UUID, now, duration) (bool, error)
    ReleaseLock(ctx, jobID) error
    ClearStaleLocks(ctx, cutoff) (int, error)
    CountByStatus(ctx) (map[string]int, error)
}
```

### State Machine (`internal/scheduler/state_machine.go`)
```go
// Valid transitions:
// pending   → scheduled, running, cancelled
// scheduled → running, paused, cancelled, pending (force-run)
// paused    → scheduled, cancelled, pending (force-run)
// running   → completed, failed, scheduled (retry), cancelled
// completed → scheduled (recurring auto-reschedule)
// failed    → pending (manual retry)
// cancelled → (terminal)

func ValidateStateTransition(from, to JobState) error
func CanPause(job *Job) bool    // StateScheduled only
func CanResume(job *Job) bool   // StatePaused only
func CanCancel(job *Job) bool   // Scheduled, Running, Paused, Pending
func CanRetry(job *Job) bool    // StateFailed only
```

## Data Flow

### Colly Crawl Path (rich extraction)
```
1. Scheduler polls GetJobsReadyToRun() every 10s
2. AcquireLock() via CAS (lock_token = UUID WHERE lock_token IS NULL)
3. Factory.Create() → isolated Crawler instance (shared startURLHashes map)
4. Colly collector visits source URLs
5. RawContentProcessor resolves source config by crawled URL host
6. If a source-manager match exists, use the configured source `Name` as the canonical raw-index source identity; if no match exists or the configured name is empty, fall back to a URL-host-derived source name
7. HTML → RawContentProcessor → extracts title, body, OG metadata, JSON-LD
8. IndexRawContent() → `naming.RawContentIndex(sourceName)` / `{sanitized_source}_raw_content` ES index (classification_status: "pending")
9. Completion: mark execution completed, calculate next_run_at, release lock
```

### Frontier Fetcher Path (lightweight)
```
1. Claim frontier URLs: UPDATE status='fetching' WHERE status='pending'
2. HTTP fetch with redirect following (max 5 redirects)
3. Extract content via source selectors
4. IndexRawContentIfAbsent() with op_type=create (won't overwrite Colly docs)
5. Update frontier URL status to 'fetched' or 'failed'
6. Stale recovery: URLs stuck in 'fetching' > 10min reset to 'pending'
```

### Adaptive Scheduling
```
1. Before crawl: compute SHA-256 hash of start URL content
2. Compare with Redis-backed hash tracker
3. If unchanged: extend next_run_at by 2x (up to max interval)
4. If changed: keep current interval, update stored hash
```

## Storage / Schema

### RawContent (Elasticsearch document)
```json
{
  "id": "string",
  "url": "string",
  "source_name": "string",
  "title": "string",
  "raw_text": "string",
  "raw_html": "string (optional)",
  "og_type": "string",
  "og_title": "string",
  "og_description": "string",
  "og_image": "string (optional)",
  "author": "string (optional)",
  "published_date": "datetime (nullable)",
  "canonical_url": "string (optional)",
  "json_ld_data": "object (optional)",
  "classification_status": "pending",
  "crawled_at": "datetime",
  "word_count": "int"
}
```

Index naming notes:
- `source_name` in the document remains the canonical source identity used by the crawler path.
- The Elasticsearch raw index name is always derived through the shared sanitizer (`naming.RawContentIndex`), so configured names such as `Sudbury.com` become `sudbury_com_raw_content`.
- Pipeline indexed events emit the same sanitized `index_name` value used for the actual ES write path.

### PostgreSQL Tables
- **jobs**: id, source_id, url, status, interval_minutes, interval_type, next_run_at, lock_token, lock_acquired_at, is_paused, max_retries, current_retry_count, retry_backoff_seconds, adaptive_scheduling, auto_managed, priority
- **job_executions**: id, job_id, execution_number, status, started_at, completed_at, duration_ms, items_crawled, items_indexed, error_message, retry_attempt, log_object_key
- **url_frontier**: id, url, url_hash, host, source_id, origin, status, priority, next_fetch_at, content_hash, retry_count
- **host_state**: host, min_delay, robots_txt_cached_at
- **feed_state**: source_id, feed_url, etag, last_modified, consecutive_errors

## Configuration

Key environment variables:
- `CRAWLER_SERVER_ADDRESS` (default: :8080)
- `max_depth` source field: -1 = unlimited depth (colly receives 0), 0 = use default, n = crawl n levels
- `CRAWLER_SOURCES_API_URL` (default: http://localhost:8050/api/v1/sources)
- `CRAWLER_PROXY_POOL_URLS` — comma-separated proxy endpoints
- `CRAWLER_PROXY_STICKY_TTL` (default: 10m)
- `CRAWLER_REDIS_STORAGE_ENABLED` (default: false)
- `FETCHER_ENABLED`, `FETCHER_WORKER_COUNT` (default: 16)
- `CRAWLER_FEED_POLL_ENABLED` (default: true)

## Edge Cases

- **Stale locks**: Locks older than 5 minutes cleared every 1 minute. If scheduler crashes mid-crawl, job auto-recovers.
- **Retry cap**: Exponential backoff 60s→120s→240s→480s→960s→3600s. After max_retries (default 3), job marked failed.
- **document_parsing_exception**: Caused by index created before canonical mapping. Fix: delete index, re-crawl.
- **Concurrent schedulers**: CAS locking ensures only one instance runs a job. Zero-row update = another instance holds lock.
- **Redis unavailable**: Colly storage falls back to in-memory (visited URLs don't persist across restarts).
- **Frontier vs Colly conflict**: Frontier uses op_type=create so it never overwrites richer Colly documents.

<\!-- Reviewed: 2026-03-18 — go.mod dependency update only, no spec changes needed -->
