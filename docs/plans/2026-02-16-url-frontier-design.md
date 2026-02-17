# URL Frontier & Feed-Driven Ingestion Pipeline

**Date:** 2026-02-16
**Status:** Approved

## Problem

North Cloud's content discovery is manual and inefficient:
- 200+ news sources managed via curated spreadsheets, added to source-manager one by one
- Crawler jobs manually created per source, relying on Colly to blindly spider entire sites
- `discovered_links` table is an MVP audit trail with no automation — external links require manual conversion to jobs
- No RSS/sitemap parsing, no URL normalization, no content deduplication, no adaptive scheduling
- Current server IP is Cloudflare-challenged from crawling

## Solution

Replace manual, spider-driven content discovery with a feed-first ingestion pipeline built around a **URL Frontier** — a single PostgreSQL-backed queue that all URL producers submit to and all content fetching flows through.

## Architecture

```
  ┌────────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐
  │ Feed Poller│  │  Spider   │  │  Manual  │  │   Future    │
  │ RSS/Sitemap│  │  (Colly)  │  │  Import  │  │   Agents    │
  └─────┬──────┘  └─────┬────┘  └─────┬────┘  └──────┬─────┘
        │               │             │               │
        └───────────────┴──────┬──────┴───────────────┘
                               │
                    ┌──────────▼──────────┐
                    │    URL Frontier     │
                    │    (PostgreSQL)     │
                    │                    │
                    │  - Deduplication   │
                    │  - Prioritization  │
                    │  - Politeness      │
                    │  - Retry/errors    │
                    │  - Crawl history   │
                    └──────────┬─────────┘
                               │
                    ┌──────────▼─────────┐
                    │  Fetcher Workers   │
                    │  (HTTP + goquery)  │
                    │  (Separate droplet)│
                    └──────────┬─────────┘
                               │
                    ┌──────────▼──────────────────────────┐
                    │      Elasticsearch raw_content       │
                    └──────────┬──────────────────────────┘
                               │
                    ┌──────────▼──────────────────────────┐
                    │  Classifier → Publisher (unchanged)  │
                    └─────────────────────────────────────┘
```

**The frontier is the single ingestion queue.** Every URL producer — feed poller, spider, manual import, future agents — submits URLs to the frontier. The fetcher workers are the only path to Elasticsearch. One deduplication point, one prioritization system, one retry model, one politeness layer, one crawl history.

The existing Colly spider is preserved. Its role shifts from "discover + extract + index" to "discover URLs and submit to frontier." The fetcher workers handle content extraction.

## Data Model

### Source Manager — New Fields on `sources`

```sql
ALTER TABLE sources ADD COLUMN feed_url        TEXT;
ALTER TABLE sources ADD COLUMN sitemap_url     TEXT;
ALTER TABLE sources ADD COLUMN ingestion_mode  VARCHAR(10) NOT NULL DEFAULT 'feed';
ALTER TABLE sources ADD COLUMN feed_poll_interval_minutes INTEGER DEFAULT 15;
```

`ingestion_mode` values: `feed`, `spider`, `both`.

### Crawler DB — New Tables

#### `url_frontier`

The core queue. All article URLs flow through here regardless of origin.

```sql
CREATE TABLE url_frontier (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url             TEXT NOT NULL,
    url_hash        CHAR(64) NOT NULL,          -- SHA-256 of normalized URL
    host            TEXT NOT NULL,               -- extracted hostname
    source_id       UUID NOT NULL,

    -- Discovery metadata
    origin          VARCHAR(20) NOT NULL,        -- 'feed', 'sitemap', 'spider', 'manual'
    parent_url      TEXT,                        -- referrer (spider-discovered)
    depth           SMALLINT DEFAULT 0,

    -- Priority & scheduling
    priority        SMALLINT NOT NULL DEFAULT 5, -- 1 (lowest) to 10 (highest)
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    next_fetch_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Fetch state
    last_fetched_at TIMESTAMPTZ,
    fetch_count     INTEGER DEFAULT 0,
    content_hash    CHAR(64),                    -- SHA-256 of extracted body (content dedup)
    etag            TEXT,                        -- stored for conditional GET
    last_modified   TEXT,                        -- stored for conditional GET

    -- Error handling
    retry_count     SMALLINT DEFAULT 0,
    last_error      TEXT,

    -- Timestamps
    discovered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_url_hash UNIQUE (url_hash)
);

-- Hot path: fetcher workers claiming next URL
CREATE INDEX idx_frontier_claimable
    ON url_frontier (priority DESC, next_fetch_at ASC)
    WHERE status = 'pending' AND next_fetch_at <= NOW();

-- Politeness: check last fetch per host
CREATE INDEX idx_frontier_host ON url_frontier (host, last_fetched_at DESC);

-- Dashboard: filter by source, status
CREATE INDEX idx_frontier_source_status ON url_frontier (source_id, status);

-- Content dedup
CREATE INDEX idx_frontier_content_hash ON url_frontier (content_hash)
    WHERE content_hash IS NOT NULL;
```

**Status lifecycle:** `pending` → `fetching` → `fetched` / `failed` → `pending` (retry) / `dead` (404, robots blocked, max retries).

#### `host_state`

Per-host politeness and robots.txt cache.

```sql
CREATE TABLE host_state (
    host                TEXT PRIMARY KEY,
    last_fetch_at       TIMESTAMPTZ,
    min_delay_ms        INTEGER NOT NULL DEFAULT 1000,
    robots_txt          TEXT,
    robots_fetched_at   TIMESTAMPTZ,
    robots_ttl_hours    INTEGER DEFAULT 24,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

`min_delay_ms` sourced from: robots.txt `Crawl-delay` > source-level `rate_limit` > default 1000ms.

#### `feed_state`

Polling state per source feed.

```sql
CREATE TABLE feed_state (
    source_id           UUID PRIMARY KEY,
    feed_url            TEXT NOT NULL,
    last_polled_at      TIMESTAMPTZ,
    last_etag           TEXT,
    last_modified       TEXT,
    last_item_count     INTEGER DEFAULT 0,
    consecutive_errors  INTEGER DEFAULT 0,
    last_error          TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### URL Normalization

Before computing `url_hash`, URLs are normalized:

1. Lowercase scheme and host
2. Remove default ports (`:80`, `:443`)
3. Resolve path segments (`/../`, `/./`)
4. Remove trailing slash (configurable per source)
5. Sort query parameters alphabetically
6. Strip tracking parameters (`utm_*`, `fbclid`, `gclid`, etc.)
7. Remove fragment identifiers (`#...`)
8. Resolve relative URLs against base

`url_hash = SHA-256(normalized_url)`

### Priority Scoring

```
priority = base_source_priority      -- source importance (1-10, from source config)
         + origin_bonus              -- feed=+2, sitemap=+1, spider=0, manual=0
         - depth_penalty             -- 1 per depth level (spider only)

clamped to [1, 10]
```

On conflict (URL exists), priority updated to the higher value. Only pending URLs are updated — fetched/dead URLs are not re-queued.

### Existing Tables

| Table | Fate |
|-------|------|
| `url_frontier` | New |
| `host_state` | New |
| `feed_state` | New |
| `jobs` | Stays — used for spider-mode sources |
| `job_executions` | Stays |
| `discovered_links` | Deprecated — spider submits to frontier instead |

## Feed Poller

Long-running goroutine inside the crawler process. Polls RSS/Atom feeds and sitemaps, submits discovered article URLs to the frontier.

### Core Loop

Every 30 seconds:
1. Query source-manager for sources where `ingestion_mode IN ('feed', 'both')`
2. Join with `feed_state` to find sources due for polling
3. For each due source, spawn a poll task (bounded concurrency, max 10)

### RSS/Atom Polling

Per source:
1. Load `feed_state` (etag, last_modified)
2. HTTP GET `feed_url` with conditional headers (`If-None-Match`, `If-Modified-Since`)
3. 304 → update `last_polled_at`, done
4. 200 → parse with [gofeed](https://github.com/mmcdole/gofeed)
5. For each entry: normalize URL, upsert into `url_frontier` with `origin='feed'`, `priority = base + 2`
6. Update `feed_state`: timestamps, etag, item count, reset errors

### Sitemap Polling

Secondary channel, polled every 6 hours:
1. Fetch sitemap URL (or try `/sitemap.xml`, `/news-sitemap.xml`)
2. Parse XML, follow sitemap indexes
3. Filter by `<lastmod>`: only URLs modified in last 48 hours
4. Submit to frontier with `origin='sitemap'`, priority bonus +1

### Error Handling

- Track `consecutive_errors` in `feed_state`
- Exponential backoff: `normal_interval * 2^(consecutive_errors)`, capped at 24 hours
- After 10 consecutive errors: log at error level (alertable), continue retrying

### Feed Auto-Discovery (Nice-to-Have)

When `feed_url = NULL`:
1. Fetch source base URL
2. Look for `<link rel="alternate" type="application/rss+xml">` in HTML head
3. Try common paths: `/feed`, `/rss`, `/atom.xml`, `/feed.xml`, `/rss.xml`
4. If found, update source-manager; if not, log and skip

## Fetcher Workers

The outbound-facing component. Runs on a dedicated DigitalOcean droplet with a fresh IP. Claims URLs from the frontier, fetches pages, extracts content, indexes to Elasticsearch.

### Deployment

Single Go binary in a Docker container on the droplet. Connects to PostgreSQL, Elasticsearch, and source-manager on the main server via DigitalOcean VPC.

### Worker Pool

N goroutines (default 10, configurable) sharing a claim loop.

### Claiming URLs (Host-Aware Politeness)

Workers only claim URLs whose host is not rate-limited:

```sql
SELECT f.id, f.url, f.host, f.source_id, f.etag, f.last_modified, f.fetch_count
FROM url_frontier f
LEFT JOIN host_state h ON h.host = f.host
WHERE f.status = 'pending'
  AND f.next_fetch_at <= NOW()
  AND (
    h.host IS NULL
    OR h.last_fetch_at + (h.min_delay_ms * INTERVAL '1 ms') <= NOW()
  )
ORDER BY f.priority DESC, f.next_fetch_at ASC
LIMIT 1
FOR UPDATE OF f SKIP LOCKED;
```

Workers never block on rate limits. If all hosts are rate-limited, sleep 1s and retry.

### Robots.txt

Before fetching, check cached robots.txt from `host_state`. Refresh if stale (TTL expired). If URL disallowed, mark as `dead` (reason: `robots_blocked`). Using [temoto/robotstxt](https://github.com/temoto/robotstxt).

### Response Handling

| Code | Action |
|------|--------|
| 200 | Extract content, compute content_hash, check content dedup, index to ES, status → `fetched` |
| 304 | Content unchanged, update `last_fetched_at`, skip re-indexing |
| 301/302/307/308 | Follow redirects (max 5), submit final URL to frontier, mark original as `dead` (redirect) |
| 404 | Mark as `dead` (not_found) |
| 429 | Respect `Retry-After`, double `min_delay_ms` in host_state, re-queue with backoff |
| 5xx | Increment `retry_count`, re-queue with exponential backoff. After max retries → `dead` |

After every fetch: update `host_state.last_fetch_at`.

### Content Extraction

Uses goquery + source selectors (fetched from source-manager, cached 5 minutes). Produces the same `raw_content` document schema as the current Colly pipeline. The classifier doesn't know which pipeline generated the content.

### User Agent

```
NorthCloud-Crawler/1.0 (+https://northcloud.biz/crawler)
```

## Spider Integration

The existing Colly spider is preserved. Its role changes:

**Before:** Spider visits page → extracts content → indexes to ES, follows links, saves external links to `discovered_links`.

**After:** Spider visits page → identifies article URLs → submits to frontier, follows navigation/listing links for discovery, submits external links to frontier.

### Changes

- `link_handler.go`: submits to `url_frontier` instead of `discovered_links`
- Article URLs identified via existing `ArticleURLPatterns` regex
- Content extraction callbacks removed for frontier-enabled sources
- Spider becomes discovery-only; fetcher workers handle extraction

### Per-Source Mode

| Mode | Feed Poller | Spider | Fetcher Workers |
|------|------------|--------|-----------------|
| `feed` | Polls RSS/sitemaps | Not used | Fetches from frontier |
| `spider` | Not used | Crawls site, submits to frontier | Fetches from frontier |
| `both` | Polls feeds | Also crawls | Fetches all from frontier |

## Deployment & Infrastructure

### Topology

- **Main server** (northcloud.biz): source-manager, crawler (feed poller + spider + frontier DB), classifier, publisher, auth, search, PostgreSQL, ES, Redis
- **Crawler droplet** (nc-fetcher-01): fetcher workers only, $6/mo (1 vCPU, 1GB), DigitalOcean VPC for private connectivity

### Provisioning

```bash
doctl vpcs create --name north-cloud-vpc --region tor1 --ip-range 10.132.0.0/20

doctl compute droplet create nc-fetcher-01 \
  --region tor1 --size s-1vcpu-1gb \
  --image docker-24-04 --vpc-uuid <vpc-id> \
  --ssh-keys <key-fingerprint> --tag-names north-cloud,fetcher
```

### Networking

PostgreSQL and Elasticsearch on main server listen on VPC interface. DigitalOcean cloud firewall restricts DB/ES ports to VPC only.

### IP Pooling (Future)

- Phase 1: 1 droplet, 1 fresh IP
- Phase 2: Add reserved IPs, rotate per source
- Phase 3: Multiple droplets, distributed workers

## Migration & Rollout

### Phase 1: Foundation

- Database migrations (url_frontier, host_state, feed_state, source-manager fields)
- URL normalization package
- Frontier repository
- API endpoints for dashboard

### Phase 2: Feed Poller

- gofeed dependency
- Feed poller module (RSS/Atom, conditional GET, sitemap)
- Integration with frontier
- Test with 5 sources with known-good RSS feeds

### Phase 3: Fetcher Workers + Droplet

- Fetcher worker binary
- Content extraction with goquery
- Robots.txt + host politeness
- Droplet provisioned, VPC connected, container deployed
- End-to-end: feed → frontier → fetcher → ES → classifier → publisher

### Phase 4: Spider Integration

- Modified link_handler: submits to frontier
- Spider content extraction removed for frontier-enabled sources
- Run in parallel with old behavior on test sources

### Phase 5: Cleanup & Dashboard

- Dashboard views for frontier
- Deprecate discovered_links
- Manual import endpoint

### Source Migration Strategy

```
Week 1:  5 sources with known-good RSS feeds → ingestion_mode = 'feed'
Week 2:  Verify quality, tune politeness, fix bugs
Week 3:  50 sources → 'feed'
Week 4:  Remaining sources → 'feed'
         Sources without feeds stay on 'spider' mode
```
