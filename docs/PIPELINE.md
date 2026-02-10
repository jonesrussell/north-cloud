# Pipeline Service Design

> **Status**: Approved design
> **Date**: 2026-02-10
> **Scope**: New microservice for canonical pipeline event tracking

## Summary

The Pipeline Service is a new Go microservice that provides a single, authoritative event stream for the entire content pipeline. Every stage transition (crawled, indexed, classified, routed, published) emits a durable, append-only event to this service, replacing the current per-service stats counters that produce inconsistent "today" boundaries and a broken funnel model.

**The problem**: The dashboard pipeline monitor fetches stats independently from four services, each using its own database and time zone logic. This produces impossible numbers (e.g., classified < published) and duplicate counts (routed always equals published).

**The solution**: A canonical `pipeline_events` table that all services write to via HTTP POST. The dashboard queries this single source for all pipeline metrics, ensuring consistent time boundaries and true funnel semantics.

**Design principles**:
- **Best-effort observational layer**: The Pipeline Service records what happened but never orchestrates or blocks pipeline work. If it's down, services continue normally.
- **Per-service tables remain the audit trail**: `job_executions`, `classification_history`, and `publish_history` remain the source of truth for legal/audit-grade exactness. The Pipeline Service is the source of truth for operational visibility and trends.
- **Fire-and-forget emission**: Services emit events asynchronously with a short timeout and circuit breaker. A degraded dashboard is acceptable; a blocked crawler is not.
- **Idempotent by construction**: Duplicate events are rejected via unique idempotency keys. Backfills and retries are safe.

---

## 1. Core Architecture

### Service Layout

```
pipeline/
├── main.go
├── config.yml
├── Dockerfile
├── .air.toml
└── internal/
    ├── bootstrap/              # Standard phased bootstrap
    ├── api/
    │   ├── ingest_handler.go   # POST /api/v1/events (write path)
    │   ├── events_handler.go   # GET  /api/v1/events (read-by-event)
    │   ├── funnel_handler.go   # GET  /api/v1/funnel
    │   ├── stats_handler.go    # GET  /api/v1/stats
    │   └── health_handler.go   # GET  /health, /ready
    ├── service/
    │   ├── pipeline.go         # Event semantics, validation
    │   └── aggregation/        # Funnel computation, analytics
    └── database/
        ├── repository.go       # Event persistence
        └── migrations/
            ├── 001_create_pipeline_events.up.sql
            └── 001_create_pipeline_events.down.sql
```

### Infrastructure

- **Port**: 8075
- **Database**: `postgres-pipeline` (new container, following one-DB-per-service pattern)
- **Auth**: Shared JWT (same `AUTH_JWT_SECRET` as all services)
- **Nginx**: `/api/pipeline/*` -> `pipeline:8075`
- **Docker**: Added to both `docker-compose.base.yml` and dev/prod overrides
- **Bootstrap**: Standard phased pattern (Profiling -> Config -> Logger -> Database -> Services -> Server -> Lifecycle)

---

## 2. Event Schema & Data Contracts

### Database Schema

```sql
-- articles: stable lookup table for cross-service analytics
CREATE TABLE articles (
    url             TEXT PRIMARY KEY,
    url_hash        CHAR(64) NOT NULL,      -- SHA-256 for indexing
    domain          TEXT NOT NULL,           -- extracted from URL on upsert
    source_name     TEXT NOT NULL,
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(url_hash)
);

CREATE INDEX idx_articles_source ON articles(source_name);
CREATE INDEX idx_articles_hash ON articles(url_hash);
CREATE INDEX idx_articles_domain ON articles(domain);

-- pipeline stages enum
CREATE TYPE pipeline_stage AS ENUM (
    'crawled',
    'indexed',
    'classified',
    'routed',
    'published'
);

-- stage ordering lookup (avoids CASE statements as enum grows)
CREATE TABLE stage_ordering (
    stage       pipeline_stage PRIMARY KEY,
    sort_order  SMALLINT NOT NULL UNIQUE
);

INSERT INTO stage_ordering (stage, sort_order) VALUES
    ('crawled', 1),
    ('indexed', 2),
    ('classified', 3),
    ('routed', 4),
    ('published', 5);

-- pipeline_events: append-only canonical event stream
-- partitioned by month for query performance and retention management
CREATE TABLE pipeline_events (
    id                      BIGSERIAL,
    article_url             TEXT NOT NULL REFERENCES articles(url),
    stage                   pipeline_stage NOT NULL,
    occurred_at             TIMESTAMPTZ NOT NULL,   -- UTC, set by emitting service
    received_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    service_name            TEXT NOT NULL,
    metadata                JSONB,
    metadata_schema_version SMALLINT NOT NULL DEFAULT 1,
    idempotency_key         TEXT,
    PRIMARY KEY (id, occurred_at),
    UNIQUE (idempotency_key, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- Create initial partitions (extend as needed)
CREATE TABLE pipeline_events_2026_01 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE pipeline_events_2026_02 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE pipeline_events_2026_03 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE INDEX idx_events_stage_time ON pipeline_events(stage, occurred_at);
CREATE INDEX idx_events_article ON pipeline_events(article_url);
CREATE INDEX idx_events_occurred ON pipeline_events(occurred_at);
```

### Validation Rules

- `occurred_at` **must** be UTC (ends with `Z` or `+00:00`). Non-UTC timestamps are rejected with `400 Bad Request`.
- `occurred_at` must not be in the future and must not be more than 24 hours in the past. Events outside this window are rejected.
- Late event warning: if `received_at - occurred_at > 1 hour`, log a structured warning.

### Idempotency Key

Auto-computed by the shared client if omitted:

```
{service}:{stage}:{url_hash_8}:{occurred_at_iso}
```

The 8-character URL hash prefix keeps the index lean while avoiding pathological key lengths. The `UNIQUE` constraint on `idempotency_key` ensures duplicate emissions are safely rejected.

For reclassification, the key includes the model version:

```
classifier:classified:{url_hash_8}:v2.1:{occurred_at_iso}
```

This allows multiple classification events per article while preventing duplicates within the same model version and timestamp.

### Event HTTP Contract

**Request**:
```json
POST /api/v1/events
{
    "article_url": "https://example.com/article-123",
    "source_name": "example_com",
    "stage": "classified",
    "occurred_at": "2026-02-10T14:30:00Z",
    "service_name": "classifier",
    "idempotency_key": null,
    "metadata": {
        "quality_score": 78,
        "topics": ["crime", "news"],
        "model_version": "v2.1"
    }
}
```

**Batch request**:
```json
POST /api/v1/events/batch
{
    "events": [
        {
            "article_url": "https://example.com/article-1",
            "source_name": "example_com",
            "stage": "classified",
            "occurred_at": "2026-02-10T14:30:00Z",
            "service_name": "classifier",
            "metadata": { "quality_score": 78 }
        },
        ...
    ]
}
```

Single HTTP request for the entire batch. Each event has its own `occurred_at` to preserve ordering.

**Responses**:
- `201 Created` — event(s) ingested
- `200 OK` — idempotent duplicate, already exists
- `400 Bad Request` — validation failure (non-UTC timestamp, future date, missing fields)
- `429 Too Many Requests` — rate limit exceeded (1000 events/second)

The Pipeline Service upserts the `articles` row on first sight, extracting `domain` and computing `url_hash`. Callers don't need to pre-register articles.

### Per-Stage Metadata Examples

| Stage | Metadata fields |
|-------|----------------|
| `crawled` | `response_code`, `content_length`, `is_duplicate` |
| `indexed` | `index`, `doc_id` |
| `classified` | `quality_score`, `topics`, `content_type`, `model_version`, `processing_time_ms`, `is_reclassification`, `confidence`, `content_hash` |
| `routed` | `route_id`, `channel`, `min_quality_score`, `matched_topics` |
| `published` | `channel`, `redis_subscribers`, `attempt` |

---

## 3. Per-Service Emission Contracts

### Shared Client Library

Location: `infrastructure/pipeline/client.go`

```go
type Client struct {
    baseURL        string
    httpClient     *http.Client
    serviceName    string
    circuitBreaker *CircuitBreaker
    logger         *infralogger.Logger
}

type Event struct {
    ArticleURL  string         `json:"article_url"`
    SourceName  string         `json:"source_name"`
    Stage       string         `json:"stage"`
    OccurredAt  time.Time      `json:"occurred_at"`
    ServiceName string         `json:"service_name"`
    Metadata    map[string]any `json:"metadata,omitempty"`
}

func (c *Client) Emit(ctx context.Context, event Event) error
func (c *Client) EmitBatch(ctx context.Context, events []Event) error
```

**Behavior**:
- `Emit`: Fire-and-forget with 2s timeout. On failure, logs a structured warning with `stage`, `url_hash`, `service`, `error`. Does not block the calling service.
- `EmitBatch`: Single HTTP request with array payload. No internal looping. Each event retains its own `occurred_at`.
- **Circuit breaker**: Trips after 5 consecutive failures. Half-open after 30s. Closes after 2 successes. State exposed via calling service's `/health` endpoint.
- **Feature flag**: If `PIPELINE_SERVICE_URL` is empty, the client is a no-op. All `Emit`/`EmitBatch` calls return nil immediately.
- **Timestamp enforcement**: Client sets `occurred_at` to `time.Now().UTC()` at call time, ensuring UTC.

### Crawler

Emits `crawled` and `indexed` per article.

**Integration point**: `internal/crawler/crawler.go` — after extracting article content, and after writing to Elasticsearch.

| Stage | When | Metadata |
|-------|------|----------|
| `crawled` | Article content extracted from page | `response_code`, `content_length`, `is_duplicate` |
| `indexed` | Document written to `{source}_raw_content` | `index`, `doc_id` |

For bulk crawls, use `EmitBatch` at the end of the page processing loop.

### Classifier

Emits `classified` after processing each article.

**Integration point**: `internal/classifier/classifier.go` — after writing to `{source}_classified_content`.

| Stage | When | Metadata |
|-------|------|----------|
| `classified` | Article classified and written to classified index | `quality_score`, `topics`, `content_type`, `model_version`, `processing_time_ms`, `is_reclassification`, `confidence`, `content_hash` |

For reclassification runs, `is_reclassification: true` and the idempotency key includes the model version. This enables drift detection by querying events grouped by `metadata->model_version`.

For bulk reclassification, use `EmitBatch` with batches of 500 and a configurable delay between batches (default 100ms).

### Publisher

Emits `routed` and `published` as two distinct events.

**Integration point**: `internal/router/service.go` — after route matching (`routed`), and after Redis PUBLISH succeeds (`published`).

| Stage | When | Metadata |
|-------|------|----------|
| `routed` | Article matched a route's filters | `route_id`, `channel`, `min_quality_score`, `matched_topics` |
| `published` | Redis PUBLISH confirmed | `channel`, `redis_subscribers`, `attempt` |

This is the key fix for the original bug: `routed` and `published` become genuinely separate events. An article can be routed but fail to publish (Redis down). The funnel correctly shows `routed >= published`.

---

## 4. Stats & Funnel API

### `GET /api/v1/funnel`

The primary endpoint for the pipeline flow visualization.

**Query params**:
- `period` — `today` (default), `24h`, `7d`, `30d`
- `source` — optional filter by source_name
- `tz` — optional timezone for "today" boundary (default: `UTC`)

**Response**:
```json
{
    "period": "today",
    "timezone": "UTC",
    "from": "2026-02-10T00:00:00Z",
    "to": "2026-02-10T15:42:00Z",
    "stages": [
        {"name": "crawled",    "count": 75480, "unique_articles": 71200},
        {"name": "indexed",    "count": 68345, "unique_articles": 68345},
        {"name": "classified", "count": 12150, "unique_articles": 10002},
        {"name": "routed",     "count": 11990, "unique_articles": 9800},
        {"name": "published",  "count": 11990, "unique_articles": 9800}
    ],
    "generated_at": "2026-02-10T15:42:03Z"
}
```

- `count`: total events (includes reclassifications, retries)
- `unique_articles`: distinct URLs — the true funnel metric displayed on the dashboard
- `generated_at`: enables cache freshness logic on the dashboard

**SQL**:
```sql
SELECT
    pe.stage,
    COUNT(*) AS count,
    COUNT(DISTINCT pe.article_url) AS unique_articles
FROM pipeline_events pe
JOIN stage_ordering so ON pe.stage = so.stage
WHERE pe.occurred_at >= $1 AND pe.occurred_at < $2
GROUP BY pe.stage, so.sort_order
ORDER BY so.sort_order;
```

One query, one time boundary, one source of truth. The broken funnel is eliminated by construction.

**Performance note**: `COUNT(DISTINCT article_url)` is exact for `today` and short windows. For `30d+` periods, see Future Work for approximate distinct strategies.

### `GET /api/v1/stats`

Replaces all per-service stats calls. Returns aggregate analytics for dashboard cards.

**Query params**: same as funnel (`period`, `source`, `tz`)

**Response**:
```json
{
    "period": "today",
    "from": "2026-02-10T00:00:00Z",
    "to": "2026-02-10T15:42:00Z",
    "crawl": {
        "total_crawled": 75480,
        "unique_urls": 71200,
        "duplicates": 4280,
        "by_source": {"example_com": 12300, "news_site": 63180}
    },
    "classification": {
        "total_classified": 12150,
        "unique_articles": 10002,
        "reclassifications": 2148,
        "avg_quality_score": 72,
        "median_quality_score": 75,
        "by_model_version": {"v2.1": 10002, "v2.0": 2148},
        "by_topic": {"crime": 3200, "news": 5100, "mining": 1200}
    },
    "publishing": {
        "total_routed": 11990,
        "total_published": 11990,
        "failed_publishes": 0,
        "by_channel": {
            "articles:crime:violent": 1800,
            "articles:news": 5100,
            "articles:mining": 1200
        }
    },
    "throughput": {
        "median_crawl_to_classify_mins": 11.2,
        "avg_crawl_to_classify_mins": 12.4,
        "median_classify_to_publish_mins": 0.6,
        "avg_classify_to_publish_mins": 0.8,
        "median_end_to_end_mins": 11.8,
        "avg_end_to_end_mins": 13.2
    },
    "generated_at": "2026-02-10T15:42:03Z"
}
```

**Throughput SQL** (sketch):
```sql
WITH stage_times AS (
    SELECT
        article_url,
        stage,
        MIN(occurred_at) AS first_at
    FROM pipeline_events
    WHERE occurred_at >= $1 AND occurred_at < $2
    GROUP BY article_url, stage
)
SELECT
    AVG(c.first_at - cr.first_at) AS avg_crawl_to_classify,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY c.first_at - cr.first_at) AS median_crawl_to_classify,
    AVG(p.first_at - c.first_at) AS avg_classify_to_publish,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY p.first_at - c.first_at) AS median_classify_to_publish
FROM stage_times cr
JOIN stage_times c ON cr.article_url = c.article_url AND c.stage = 'classified'
JOIN stage_times p ON c.article_url = p.article_url AND p.stage = 'published'
WHERE cr.stage = 'crawled';
```

Using `MIN(occurred_at)` per stage ensures reclassifications don't distort latency. Negative latencies (from clock skew) are clamped to zero in the aggregation layer.

### `GET /api/v1/stats/drift`

Model version tracking and reclassification analytics.

**Response**:
```json
{
    "model_versions": [
        {
            "version": "v2.1",
            "articles_classified": 10002,
            "avg_quality_score": 74,
            "topic_distribution": {"crime": 0.32, "news": 0.51},
            "version_age_days": 5
        },
        {
            "version": "v2.0",
            "articles_classified": 2148,
            "avg_quality_score": 68,
            "topic_distribution": {"crime": 0.28, "news": 0.55},
            "version_age_days": 30
        }
    ],
    "drift_indicators": {
        "quality_score_delta": 6,
        "topic_distribution_shift": 0.04
    }
}
```

`version_age_days` is derived from the earliest event for that model version, contextualizing drift signals.

---

## 5. Dashboard Refactor

### Metrics Store Rewrite

The current `stores/metrics.ts` makes four parallel fetches to four services. This collapses to two calls to one service.

```typescript
// Before: 4 fetches, 4 time boundaries, 4 data shapes
await Promise.all([
    fetchCrawlerMetrics(),      // crawler:8060/api/v1/stats
    fetchClassifierMetrics(),   // classifier:8071/api/v1/stats?date=today
    fetchPublisherMetrics(),    // publisher:8070/api/v1/stats/overview?period=today
    fetchIndexMetrics(),        // index-manager:8090/api/v1/stats
])

// After: 2 fetches, 1 time boundary, 1 source of truth
await Promise.all([
    fetchFunnel(),   // pipeline:8075/api/v1/funnel?period=today
    fetchStats(),    // pipeline:8075/api/v1/stats?period=today
])
```

**What gets removed**:
- `fetchCrawlerMetrics()`, `fetchClassifierMetrics()`, `fetchPublisherMetrics()` — all deleted
- `CrawlerMetrics`, `ClassifierMetrics`, `PublisherMetrics` types — replaced by `FunnelResponse` and `StatsResponse`
- Manual stage assignment (`pipelineStages.value[0].count = ...`) — funnel response maps directly to `PipelineFlow` props

**What gets added**:
- Stale-while-revalidate: cache last successful funnel/stats responses so the pipeline monitor renders during transient Pipeline Service outages
- Data freshness indicator: use `generated_at` timestamp to show "Updated 30s ago" on the dashboard
- Throughput metric cards: median crawl-to-classify and classify-to-publish latency (median as primary display, average in tooltip)
- Reclassification volume badge: derived from `count - unique_articles` on the classified stage
- Publish success rate badge: derived from `routed.unique_articles` vs `published.unique_articles`

### API Client

New module in `dashboard/src/api/client.ts`:

```typescript
export const pipelineApi = {
    funnel: (period = 'today') =>
        api.get('/api/pipeline/api/v1/funnel', { params: { period } }),
    stats: (period = 'today') =>
        api.get('/api/pipeline/api/v1/stats', { params: { period } }),
    drift: () =>
        api.get('/api/pipeline/api/v1/stats/drift'),
}
```

### What Stays Unchanged

- `PipelineMonitorView.vue` template structure — same cards, same flow, same layout
- `PipelineFlow` component — still receives `{name, count, status}[]` array
- `HealthBadge`, `QuickActions`, `JobStatsCard` — untouched
- `ContentIntelligenceSummary` — still fetches from its own composable
- `fetchIndexMetrics()` — retained only if used outside the pipeline monitor

---

## 6. Migration Plan

### Phase 1: Deploy Pipeline Service (no integrations)

**Scope**: Scaffold the service, database, migrations, and API endpoints. Nothing calls it yet.

1. Create `pipeline/` service following bootstrap pattern
2. Add `postgres-pipeline` to both compose files
3. Implement `POST /api/v1/events` (ingest) with validation
4. Implement `GET /api/v1/funnel` and `GET /api/v1/stats` (return empty results)
5. Implement `GET /health` and `GET /ready`
6. Add nginx route: `/api/pipeline/*` -> `pipeline:8075`
7. Deploy. Verify health checks pass.

**Rollback**: Remove the service from compose. Nothing depends on it.

### Phase 2: Build the shared client library

**Scope**: Create `infrastructure/pipeline/client.go`. No service imports it yet.

1. Implement client with `Emit`, `EmitBatch`, circuit breaker
2. Unit test: mock HTTP, test circuit breaker state transitions, test idempotency key generation, test UTC enforcement
3. Add `PIPELINE_SERVICE_URL` to `.env.example` (default: `http://pipeline:8075`)
4. Add config support to `infrastructure/config/`

**Rollback**: Delete the package. Nothing imports it.

### Phase 3: Wire up services (one per day)

**Order**: Crawler -> Classifier -> Publisher (follows pipeline flow).

For each service:
1. Import the pipeline client
2. Initialize in bootstrap (no-op if `PIPELINE_SERVICE_URL` is empty)
3. Add `Emit` calls at integration points (Section 3)
4. Deploy the service
5. Verify events appear in `pipeline_events` table
6. Monitor for latency impact (<5ms per emit expected)

**Rollback per service**: Set `PIPELINE_SERVICE_URL=""` and restart. No code revert needed.

### Phase 4: Backfill historical events

**Scope**: Populate `pipeline_events` with historical data.

**Tool**: `pipeline/cmd/backfill/main.go`

**Sources**:

| Stage | Source table | Source DB | Key columns |
|-------|-------------|-----------|-------------|
| `crawled` + `indexed` | `job_executions` | `postgres-crawler` | `started_at`, `items_crawled`, `items_indexed` |
| `classified` | `classification_history` | `postgres-classifier` | `classified_at`, `quality_score`, `topics` |
| `routed` + `published` | `publish_history` | `postgres-publisher` | `published_at`, `channel_name` |

**Process**:
- Backfill per-service, not all-at-once. Verify after each.
- Use `EmitBatch` with batches of 500
- Set `occurred_at` from original timestamps
- Generate idempotency keys from original data (re-running is safe)
- Backfilled `routed` and `published` events get identical timestamps (historically accurate — they were the same step)
- Tag backfilled events with `"backfilled": true` in metadata

**Rollback**: `TRUNCATE pipeline_events; TRUNCATE articles;` and re-run.

### Phase 5: Switch the dashboard

**Scope**: Replace metrics store with pipeline API calls.

1. Add `pipelineApi` to `dashboard/src/api/client.ts`
2. Rewrite `stores/metrics.ts` to call pipeline endpoints
3. Add stale-while-revalidate caching and freshness indicator
4. Deploy dashboard
5. Verify numbers match (compare old per-service stats for a few hours)

**Parallel running**: Both old and new stats remain available. Dashboard shows pipeline numbers; old endpoints available for manual cross-checking. Maintain parallel state for at least one week.

**Rollback**: Revert dashboard to previous version. Old stats endpoints still work.

### Phase 6: Deprecate old stats endpoints

**Scope**: After 1-2 weeks of confidence.

1. Add deprecation warnings to old stats handlers (log when called)
2. Remove old fetch functions from dashboard (already unused since Phase 5)
3. After another week with no calls, remove handlers

**Do NOT delete**: `job_executions`, `classification_history`, `publish_history` remain — they serve their primary purpose (job management, classification records, publish audit trail). Only the stats aggregation logic is deprecated.

### Summary Timeline

| Phase | What | Risk | Rollback |
|-------|------|------|----------|
| 1 | Deploy Pipeline Service | None | Remove from compose |
| 2 | Build shared client | None | Delete package |
| 3 | Wire services (3 days) | Low — fire-and-forget, feature-flagged | Empty env var |
| 4 | Backfill history | Low — append-only, idempotent | Truncate and re-run |
| 5 | Switch dashboard | Medium — user-visible | Revert dashboard deploy |
| 6 | Deprecate old stats | Low — monitoring confirms no callers | Re-enable handlers |

---

## 7. Risks, Edge Cases & Observability

### Operational Risks

**DB growth on `pipeline_events`**

At ~75k crawled/day, expect ~375k rows/day (5 events per article), ~11M rows/month, ~137M rows/year.

Mitigations:
- **Monthly partitioning**: Queries for "today" only scan the current partition
- **Retention policy**: Cron job drops partitions older than 6 months (configurable). Materialized summaries can be created before dropping raw events.
- **Index discipline**: Three indexes (stage+time, article, occurred_at) are sufficient. Resist adding indexes for one-off queries.

**Hot partition on writes**

~4.3 writes/second average (bursty during crawls) is well within PostgreSQL capacity. Append-only with no updates means no row-level lock contention.

Mitigation if needed: `EmitBatch` reduces round-trips. Only a concern if volume grows 10x+.

**Slow queries**

`COUNT(DISTINCT article_url)` is the most expensive operation — requires scanning all matching rows.

Mitigations:
- Monthly partitioning keeps "today" scans small (~375k rows max)
- For longer periods, consider a materialized summary table refreshed every 5 minutes (see Future Work)

**Circuit breaker misconfiguration**

Default thresholds: trip after 5 consecutive failures, half-open after 30s, close after 2 successes. Expose state via emitting service's `/health` endpoint. Include `PIPELINE_SERVICE_URL` in metric labels so misconfigurations are obvious.

### Data-Quality Risks

**Missing events (gaps in the funnel)**

Events can be lost if: (a) emitting service crashes between work and `Emit`, or (b) Pipeline Service is down and circuit breaker drops the call.

Mitigations:
- **Reconciliation job** (daily): cross-references `pipeline_events` against source-of-truth tables (see Observability section)
- **Gap detection alert**: if `classified/indexed` ratio drops below 80% for a rolling 1-hour window, fire alert
- Accept best-effort nature: dashboard shows "approximately correct" operational numbers. Per-service tables remain the exact audit trail.

**Skewed `occurred_at` timestamps**

Contract: `occurred_at` is "when this specific article completed this stage," not batch start time. Validation rejects future timestamps and >24h old timestamps. `received_at` acts as sanity check — warn if gap exceeds 1 hour.

**Partial backfill**

Backfill per-service with verification between each. Idempotency keys make re-running safe. `backfilled: true` metadata flag distinguishes backfilled from live events.

### Edge Cases

**Clock skew between services**

Docker containers share host clock (skew typically <1ms). If multi-host deployment happens, use NTP. Negative throughput latencies (from skew) are clamped to zero in the aggregation layer.

**Late events**

`occurred_at` vs `received_at` split handles this correctly — events land in the right time bucket regardless of when received. Dashboard freshness indicator (`generated_at`) tells operators data is current.

**Reclassification storms**

50k articles reclassified = 50k events in a short window.

Mitigations:
- `EmitBatch` with batches of 500 reduces to ~100 HTTP requests
- Rate limit: 1000 events/second on ingest endpoint
- Reclassification scripts include configurable delay between batches (default 100ms)
- `is_reclassification: true` metadata allows funnel queries to optionally exclude

**Redis outage affecting publish stats**

If Redis is down, `routed` events still fire (route matching is pre-Redis) but `published` events don't. The funnel correctly shows `routed > published`. The current system shows both as 0 during outages; the new system is strictly more accurate.

**Article URL changes (redirects, canonicalization)**

Same article under two URLs creates two event streams. This is a crawler-level concern — URL normalization should happen before emission. See Future Work for `canonical_url` support.

### Observability

**Pipeline Service metrics**:

| Metric | Type | Alert threshold |
|--------|------|-----------------|
| `pipeline_events_ingested_total` (by stage) | Counter | Per-stage drop to 0 for >15 min while other stages are active |
| `pipeline_ingest_duration_seconds` | Histogram | p99 > 100ms |
| `pipeline_events_duplicated_total` | Counter | Spike >20% of ingest rate |
| `pipeline_events_rejected_total` (by reason) | Counter | Any non-zero sustained |
| DB connection pool (active/idle/waiting) | Gauge | Waiting > 0 sustained |
| `pipeline_query_duration_seconds` (by endpoint) | Histogram | Funnel p99 > 500ms |
| `pipeline_events_row_count` (per partition) | Gauge | Growth >2x baseline |

**Emitting service metrics** (via shared client):

| Metric | Type | Alert threshold |
|--------|------|-----------------|
| `pipeline_emit_total{status="success"}` | Counter | Per-stage drop to 0 for >15 min |
| `pipeline_emit_total{status="error"}` | Counter | Any sustained |
| `pipeline_circuit_state` (0=closed, 1=open) + target URL label | Gauge | Open for >5 min |
| `pipeline_emit_duration_seconds` | Histogram | p99 > 2s |

**Structured logging** (all via `infralogger`):

```go
// Ingest success (Pipeline Service)
log.Info("event ingested",
    infralogger.String("stage", "classified"),
    infralogger.String("article_url_hash", "a1b2c3d4"),
    infralogger.String("service", "classifier"))

// Duplicate rejected
log.Debug("duplicate event skipped",
    infralogger.String("idempotency_key", "..."))

// Circuit breaker trip (emitting service)
log.Warn("pipeline circuit breaker opened",
    infralogger.String("service", "classifier"),
    infralogger.String("target_url", pipelineURL),
    infralogger.Int("consecutive_failures", 5))

// Late event warning (Pipeline Service)
log.Warn("late event received",
    infralogger.String("stage", "classified"),
    infralogger.Duration("delay", receivedAt.Sub(occurredAt)))
```

**Reconciliation job**: `pipeline/cmd/reconcile/main.go` (daily scheduled task)

Output format:
```
Reconciliation Report (2026-02-10)
  crawled:    pipeline=75,480  source=75,480  delta=0      OK
  indexed:    pipeline=68,340  source=68,345  delta=-5     UNEXPECTED
  classified: pipeline=10,002  source=10,002  delta=0      OK
  routed:     pipeline=11,990  source=11,990  delta=0      OK
  published:  pipeline=11,985  source=11,990  delta=-5     EXPECTED (Redis outage 14:20-14:35 UTC)

Replay plan for unexpected gaps:
  indexed: SELECT ... FROM job_executions WHERE started_at >= '...' AND items_indexed > 0
           → EmitBatch to pipeline with stage=indexed
```

Separates expected gaps (annotated from known outage windows) from unexpected gaps. Each unexpected gap includes a replay query showing which source table and columns to use for auto-backfill.

---

## 8. Future Work

Items deferred from this design to keep scope manageable:

- **`canonical_url` on articles table**: Handle URL redirects and canonicalization at the data layer. Deferred until it's a measurable problem.
- **Approximate distinct counts**: For `30d+` periods, `COUNT(DISTINCT)` may become slow. PostgreSQL's `hll` extension (HyperLogLog) or a materialized summary table refreshed every 5 minutes are both viable. Implement when funnel p99 query latency exceeds 500ms.
- **Long-term retention strategy**: Define tiered storage — hot (current month, full resolution), warm (3-6 months, daily summaries), cold (archive, monthly summaries). Implement when table size exceeds operational comfort.
- **Real-time streaming**: Replace polling-based dashboard with WebSocket/SSE push from Pipeline Service. Deferred until the polling model proves insufficient.
- **Cross-service tracing**: Propagate a `trace_id` from crawler through classifier to publisher, stored in event metadata. Enables per-article journey visualization.
- **Partition management automation**: A cron job or management command that auto-creates next month's partition and drops expired partitions based on retention policy.
