# Dynamic Crawling — M2 Design Spec

> Status: **Draft** | Issue: [#170](https://github.com/jonesrussell/north-cloud/issues/170)
> Builds on M1 render-worker (PR #219, deployed 2026-03-08).

---

## 1. Architecture Overview

M2 extends the render-worker from a simple "navigate-and-return-HTML" service into a
configurable headless browser crawling engine with per-source behavior, scroll strategies,
and priority queue management.

```
                                  ┌─────────────────────────────────┐
                                  │         render-worker           │
                                  │                                 │
Crawler ──POST /render──►         │  queue.js ─► handler.js         │
  (with RenderConfig)             │      │           │              │
                                  │      │     config.js            │
                                  │      │           │              │
                                  │      └──► scroll.js ──► Chromium│
                                  └─────────────────────────────────┘
```

**Key principle**: The `/render` endpoint remains backwards compatible. M1 callers (no
config fields) get identical behavior. M2 fields are optional extensions.

---

## 2. Per-Source Configuration Model

Each render request may include a `config` object. The crawler's source-manager stores
per-source render config; the crawler passes it through to render-worker on each request.

### Config Schema

```json
{
  "url": "https://example.com/news",
  "timeout_ms": 15000,
  "wait_until": "networkidle",
  "config": {
    "scroll": {
      "strategy": "full_page",
      "max_scroll_ms": 10000,
      "scroll_delay_ms": 250,
      "pixels": 0,
      "percent": 0
    },
    "selectors": {
      "wait_for": ".article-list",
      "wait_timeout_ms": 5000,
      "extract": "body"
    },
    "headers": {
      "Accept-Language": "en-CA,en;q=0.9"
    },
    "viewport": {
      "width": 1280,
      "height": 720
    },
    "priority": "normal",
    "source_id": "uuid"
  }
}
```

### Config Defaults

| Field | Default | Description |
|-------|---------|-------------|
| `scroll.strategy` | `viewport` | No scrolling — return what's visible |
| `scroll.max_scroll_ms` | `10000` | Maximum time to spend scrolling |
| `scroll.scroll_delay_ms` | `250` | Pause between scroll steps |
| `scroll.pixels` | `0` | Target pixel offset (for `pixels` strategy) |
| `scroll.percent` | `0` | Target scroll percentage (for `percent` strategy) |
| `selectors.wait_for` | `null` | CSS selector to wait for before returning HTML |
| `selectors.wait_timeout_ms` | `5000` | Max wait time for selector |
| `selectors.extract` | `null` | CSS selector — return only this element's HTML |
| `headers` | `{}` | Custom headers injected into page requests |
| `viewport.width` | `1280` | Browser viewport width |
| `viewport.height` | `720` | Browser viewport height |
| `priority` | `"normal"` | Queue priority: `high`, `normal`, `low` |
| `source_id` | `null` | Source identifier for per-source rate limiting |

---

## 3. Scroll Strategies

Four scroll strategies control how the render-worker handles infinite scroll / lazy-loaded
content before capturing HTML.

### 3.1 `viewport` (default)

No scrolling. Returns HTML as rendered in the initial viewport. Identical to M1 behavior.

### 3.2 `full_page`

Auto-scrolls to the bottom of the page in increments of one viewport height. Stops when:
- Page height stops increasing (no new content loaded), OR
- `max_scroll_ms` elapsed

```
while (scrollY < scrollHeight && elapsed < maxScrollMs) {
  scrollBy(0, viewportHeight)
  wait(scrollDelayMs)
  if (scrollHeight unchanged after 2 consecutive scrolls) break
}
```

### 3.3 `percent`

Scrolls to a target percentage of total page height. Useful for pages where only the top
portion contains relevant content.

```
targetY = scrollHeight * (percent / 100)
scrollTo(0, targetY)
wait(scrollDelayMs)
```

### 3.4 `pixels`

Scrolls to an exact pixel offset. Useful for known page layouts.

```
scrollTo(0, pixels)
wait(scrollDelayMs)
```

### Strategy Comparison

| Strategy | Use Case | Example |
|----------|----------|---------|
| `viewport` | Static pages, articles | News article pages |
| `full_page` | Infinite scroll feeds | Social media feeds, blog listings |
| `percent` | Partial content loading | Forums with "load more" at 50% |
| `pixels` | Fixed layouts | Known page structures |

---

## 4. Queue Management

### 4.1 Priority Levels

Three priority levels replace the current FIFO queue:

| Priority | Weight | Use Case |
|----------|--------|----------|
| `high` | 3 | Breaking news, time-sensitive sources |
| `normal` | 1 | Standard crawl schedule |
| `low` | 0.5 | Background/archival crawls |

Implementation: weighted round-robin. For every 3 high-priority items dequeued, 1 normal
is processed. Low-priority items are only dequeued when no high/normal items are waiting.

### 4.2 Per-Source Rate Limiting

Sources are rate-limited to prevent overwhelming target sites:

| Config | Default | Description |
|--------|---------|-------------|
| `MAX_CONCURRENT_PER_SOURCE` | `2` | Max simultaneous renders per source_id |
| `SOURCE_COOLDOWN_MS` | `1000` | Minimum gap between requests to same source |

When a source hits its concurrency limit, its queued items wait. Items from other sources
continue processing (head-of-line blocking prevention).

### 4.3 Queue Limits

| Limit | Value | Behavior on Breach |
|-------|-------|--------------------|
| `MAX_QUEUE_DEPTH` | `50` | HTTP 503 (unchanged from M1) |
| `MAX_QUEUE_WAIT_MS` | `60000` | Item dropped, HTTP 504 |
| `MAX_CONCURRENT` | `3` | Global concurrency cap (env: `MAX_CONCURRENT_RENDERS`) |

---

## 5. Request/Response Contract

### Extended POST /render

**Request** (M2 fields are optional — M1 requests still work):

```json
{
  "url": "https://example.com",
  "timeout_ms": 15000,
  "wait_until": "networkidle",
  "config": {
    "scroll": { "strategy": "full_page", "max_scroll_ms": 8000 },
    "selectors": { "wait_for": ".content-loaded" },
    "priority": "high",
    "source_id": "abc-123"
  }
}
```

**Response** (extended with scroll metadata):

```json
{
  "html": "<html>...</html>",
  "final_url": "https://example.com/",
  "render_time_ms": 2340,
  "status_code": 200,
  "scroll": {
    "strategy_used": "full_page",
    "pixels_scrolled": 4800,
    "scroll_steps": 7,
    "scroll_time_ms": 1750
  },
  "queue_wait_ms": 120
}
```

### Backwards Compatibility

- M1 requests (no `config` field) → `viewport` scroll, `normal` priority, no selectors
- M1 response fields unchanged — new fields (`scroll`, `queue_wait_ms`) are additive
- Existing crawler `render.Client.Render()` method continues to work without changes

---

## 6. Failure Modes

### 6.1 Timeout Handling

| Timeout | Default | Behavior |
|---------|---------|----------|
| Navigation timeout | `timeout_ms` (15s) | Playwright `page.goto()` timeout → return partial HTML if available |
| Scroll timeout | `max_scroll_ms` (10s) | Stop scrolling, return HTML at current scroll position |
| Selector wait timeout | `wait_timeout_ms` (5s) | Log warning, return HTML without waiting further |
| Queue wait timeout | `MAX_QUEUE_WAIT_MS` (60s) | Drop from queue, return HTTP 504 |

### 6.2 Crash Recovery

- **Browser crash**: Playwright `browser.isConnected()` check → relaunch browser
- **Page crash**: Catch Playwright `page.crash` event → close context, return error
- **OOM**: Docker `--memory=512m` limit → container restart via Docker `restart: unless-stopped`

### 6.3 Browser Recycling

Existing M1 recycling (`BROWSER_RECYCLE_AFTER=100`) continues. M2 adds:

- **Memory-based recycling**: If process RSS exceeds `BROWSER_RECYCLE_MEMORY_MB` (default 400),
  recycle browser immediately after current render completes
- **Error-based recycling**: 3 consecutive render failures → force browser recycle

### 6.4 Stale Context Cleanup

Browser contexts that exceed `timeout_ms * 2` are force-closed by a background interval
(runs every 30s). This prevents leaked contexts from accumulating memory.

---

## 7. Observability

### 7.1 Structured Logging

All log lines are JSON with these standard fields:

```json
{
  "level": "info",
  "msg": "render_complete",
  "url": "https://example.com",
  "source_id": "abc-123",
  "render_time_ms": 2340,
  "scroll_strategy": "full_page",
  "scroll_steps": 7,
  "status_code": 200,
  "queue_wait_ms": 120,
  "queue_depth": 3
}
```

### 7.2 Health Check (Extended)

`GET /health` response adds M2 fields:

```json
{
  "status": "ok",
  "browser_connected": true,
  "request_count": 42,
  "queue_depth": 3,
  "recycle_after": 100,
  "queue_by_priority": { "high": 1, "normal": 2, "low": 0 },
  "active_sources": 2,
  "memory_rss_mb": 210,
  "uptime_seconds": 3600
}
```

### 7.3 Metrics (Future — M3)

Prometheus `/metrics` endpoint is deferred to M3. M2 exposes counters and gauges via
the health endpoint for dashboard integration:

| Metric | Type | Description |
|--------|------|-------------|
| `render_total` | counter | Total render requests received |
| `render_success_total` | counter | Successful renders |
| `render_error_total` | counter | Failed renders |
| `render_duration_ms` | histogram | Render time distribution |
| `scroll_duration_ms` | histogram | Scroll time distribution |
| `queue_depth` | gauge | Current queue depth |
| `queue_wait_ms` | histogram | Queue wait time distribution |
| `browser_recycles_total` | counter | Browser recycle events |
| `active_renders` | gauge | Currently rendering pages |

---

## 8. Resource Limits and Scaling

### 8.1 Single Instance Limits

| Resource | Limit | Rationale |
|----------|-------|-----------|
| Memory | 512 MB | Docker `--memory=512m` — Chromium + Node overhead |
| CPU | 1 core | Docker `--cpus=1` — rendering is CPU-bound |
| Concurrent renders | 3 | Prevents memory exhaustion (each tab ~50-100 MB) |
| Queue depth | 50 | Backpressure — crawler retries on 503 |

### 8.2 Scaling Strategy (Future)

For higher throughput:
1. **Vertical**: Increase memory limit and `MAX_CONCURRENT_RENDERS`
2. **Horizontal**: Multiple render-worker instances behind a load balancer
   - Crawler round-robins across `CRAWLER_RENDER_WORKER_URLS` (comma-separated)
   - Each instance is stateless — no coordination needed

---

## 9. Migration Path: M1 → M2

### Phase 1: Scaffolding (This PR)

- Add `config.js`, `scroll.js`, `queue.js` modules with tests
- Extend `handler.js` to parse config (backwards compatible)
- Extend crawler `render.Client` with `RenderWithConfig()` method
- No behavior changes — all new code paths gated behind config presence

### Phase 2: Implementation (Follow-up PRs)

1. Wire scroll strategies into `renderPage()` in `index.js`
2. Replace FIFO queue with priority queue in `index.js`
3. Add per-source rate limiting
4. Add memory-based browser recycling
5. Add extended health check fields

### Phase 3: Integration

1. Add `render_config` JSONB field to source-manager sources table
2. Crawler reads source render config and passes to `RenderWithConfig()`
3. Dashboard UI for per-source render configuration

### Rollback

- Remove `config` field from requests → immediate fallback to M1 behavior
- No database migrations in M2 — config is passed per-request
- Render-worker M2 serves M1 requests identically

---

## 10. Open Questions

1. **Should scroll strategies be composable?** (e.g., scroll to 50% then wait for selector)
   — Current design: no. Keep strategies atomic for simplicity.

2. **Should render-worker support JavaScript injection?** (e.g., click "Load More" buttons)
   — Deferred to M3. Complex interaction sequences need a DSL or action list.

3. **Per-source browser contexts vs shared?** Each render gets a fresh context (M1 behavior).
   Shared contexts could enable cookie persistence but add complexity.
   — Decision: Keep fresh contexts for M2. Cookie persistence deferred to M3.
