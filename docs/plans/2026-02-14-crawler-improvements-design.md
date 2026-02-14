# Crawler Improvements Design

**Goal:** Improve content discovery and crawl efficiency through Redis storage, adaptive scheduling, proxy rotation, and increased depth.

**Scope:** Crawler service only. No changes to classifier, publisher, or other services.

**Tech:** Colly v2.3.0, gocolly/redisstorage, go-redis/v9 (already in go.mod)

---

## 1. Redis Storage for Colly

Replace Colly's in-memory visited-URL tracking with `gocolly/redisstorage`.

### What it stores

| Data | Redis key pattern | Purpose |
|------|-------------------|---------|
| Visited URLs | `{prefix}:request:{requestID}` | Dedup across restarts |
| Cookies | `{prefix}:cookie:{hostname}` | Session persistence |
| Request queue | `{prefix}:queue` | Resume interrupted crawls |

### Key prefix

Each job gets its own namespace: `crawler:{job_id}:`. This isolates crawl state between jobs.

### TTL

Visited keys expire after 7 days (configurable via `CRAWLER_REDIS_STORAGE_EXPIRES`). Prevents indefinite growth while preserving state across restarts within a reasonable window.

### Config

```yaml
crawler:
  redis_storage:
    enabled: true     # CRAWLER_REDIS_STORAGE_ENABLED
    expires: 168h     # CRAWLER_REDIS_STORAGE_EXPIRES (7 days)
```

### Integration point

In `setupCollector()`, after creating the collector, call `c.SetStorage(storage)` if Redis storage is enabled. Falls back to in-memory if disabled or Redis unavailable.

---

## 2. Hash-Based Adaptive Scheduling

After each crawl, hash start URL pages to detect changes. Adjust the job's interval based on change frequency.

### Algorithm

1. **During crawl**: For each start URL, compute SHA-256 of the extracted HTML body
2. **After crawl**: Compare hash to previous value in Redis (`crawler:hash:{source_id}:{url_hash}`)
3. **Adjust interval**:
   - **Changed**: Reset to baseline interval (from source config)
   - **Unchanged N consecutive times**: Exponential backoff: `baseline x 2^(unchanged_count)`, capped at max

### Interval bounds

- **Minimum**: Source's configured `interval_minutes` (the baseline)
- **Maximum**: 24 hours (even the stalest source gets checked daily)
- **Backoff curve** (30min baseline): 30m -> 1h -> 2h -> 4h -> 8h -> 16h -> 24h (cap)

### Redis metadata

Key: `crawler:adaptive:{source_id}`

| Field | Type | Purpose |
|-------|------|---------|
| `last_hash` | string | SHA-256 of most recent start URL content |
| `last_change_at` | timestamp | When content last changed |
| `unchanged_count` | int | Consecutive unchanged crawls |
| `current_interval` | duration | The adapted interval |

### Where the logic lives

In the scheduler's reschedule step. After a job completes, instead of blindly adding `interval_minutes` to calculate `next_run_at`, it checks the hash result and computes the adaptive interval.

### Opt-in

New boolean field `adaptive_scheduling` on the job model (default `true` for new jobs). Jobs with `adaptive_scheduling: false` keep fixed intervals.

### Fallback

If hashing fails (page didn't load, timeout, etc.), keep current interval unchanged. Don't penalize or reward.

---

## 3. Proxy Rotation

Use Colly's built-in `proxy.RoundRobinProxySwitcher` with a configurable list of self-hosted proxy URLs.

### How it works

1. List of proxy URLs (HTTP or SOCKS5) in crawler config
2. In `setupCollector()`, if proxies are configured, create `RoundRobinProxySwitcher` and call `c.SetProxyFunc(rp)`
3. Log the proxy used per request via `r.Request.ProxyURL` in the response callback

### Config

```yaml
crawler:
  proxies:
    enabled: false           # CRAWLER_PROXIES_ENABLED
    urls:                    # CRAWLER_PROXY_URLS (comma-separated env var)
      - "socks5://proxy1.example.com:1080"
      - "socks5://proxy2.example.com:1080"
      - "http://proxy3.example.com:8080"
```

### Design decisions

- **Round-robin is sufficient**: With 3-5 self-hosted proxies, even distribution works. No need for weighted selection or health checking in v1.
- **Transport interaction**: `SetProxyFunc` overrides the transport's proxy per-request. Existing TLS and connection pool config stays intact.
- **Global, not per-source**: Proxies are global to the crawler instance. A source doesn't pick its proxy. Keeps it simple.
- **Error handling**: If a proxy is down, Colly's existing retry logic handles it. A failed request through proxy A will be retried, potentially through proxy B (next in rotation).

---

## 4. Max Depth Default

Change the effective default max depth to 3, with per-source overrides.

### What changes

1. **Source config**: If `max_depth` is 0 (unset), default to 3 instead of passing 0 to Colly (which means unlimited)
2. **Audit**: Check current source configs on production, update any that are 0 or 1 to 3
3. **Warning**: Sources with `max_depth > 5` log a warning at startup

### Why depth 3

| Depth | Typical content |
|-------|----------------|
| 0 | Start URL (homepage / section landing) |
| 1 | Section pages / article listing pages |
| 2 | Individual article pages |
| 3 | Paginated listings, sub-sections, articles linked from articles |

### No schema change needed

`MaxDepth` already exists on the `Source` struct. This is purely a default value change and an audit of existing configs.

---

## Implementation Order

1. **Redis storage** (foundation — other features benefit from Redis being available)
2. **Max depth default** (smallest change, immediate impact on content discovery)
3. **Proxy rotation** (independent of other changes)
4. **Adaptive scheduling** (most complex, depends on Redis for hash storage)

---

## Out of Scope

- Per-source proxy assignment
- Proxy health checking / automatic failover
- HTTP conditional requests (ETag/If-Modified-Since) — nice-to-have for future
- Dashboard UI for adaptive scheduling metrics
- Changing the scheduler from interval-based to anything else
