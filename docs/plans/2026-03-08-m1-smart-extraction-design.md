# M1: Smart Extraction — Design Document

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Raise extraction success rate from 25% to 60%+ by improving static extraction
and introducing dynamic rendering for JS-heavy sources.

**Architecture:** Two parallel tracks converge at the existing extraction pipeline.
Track A improves static extraction (URL filter, page-type classifier, CMS templates,
readability fallback). Track B introduces a Playwright render worker as a Docker sidecar
with per-source opt-in. The render worker is a stateless HTTP service that returns
fully-rendered HTML — all extraction logic stays in the crawler.

**Tech Stack:** Go 1.26+ (crawler, source-manager), Node.js + Playwright (render worker),
Elasticsearch (metrics), Grafana (dashboard)

**GitHub Issues:** #185 (infra constraints), #186 (MinIO ceiling), #187 (baseline data)

---

## 1. Architecture Overview

M1 has two tracks that converge at the extraction pipeline:

**Track A: Static Extraction Improvements** (crawler-internal)
- Enable readability fallback globally
- URL pre-filter (skip non-content pages before extraction)
- Page-type classifier (article vs listing vs stub vs other)
- CMS template registry (Postmedia, Torstar, WordPress, Village Media)

**Track B: Dynamic Rendering** (new sidecar + crawler integration)
- Playwright render worker (Docker sidecar, HTTP API)
- Per-source `render_mode` field in source-manager
- Crawler calls render worker instead of Colly for dynamic sources
- Returns full DOM HTML — existing extraction pipeline processes it normally

The render worker is a **dumb HTML pipe**. It takes a URL, renders it in Chromium,
returns the final DOM HTML. The crawler's existing extraction code (extractor.go,
readability fallback, content detection) processes the HTML identically regardless of
source. No extraction logic lives in the render worker.

```
Source (static)  → Colly fetch → HTML → ExtractRawContent → ES
Source (dynamic) → Render Worker → HTML → ExtractRawContent → ES
```

**Services touched:** crawler (integration), source-manager (render_mode field), new
render-worker sidecar. No changes to classifier, publisher, or search.

---

## 2. Goals & Success Metrics

### Primary Goal

Raise extraction success rate from 25% to 60%+ across all `*_raw_content` docs.

### Metrics

Measured against `*_raw_content` Elasticsearch indices.

| Metric | Baseline (2026-03-08) | M1 Target |
|--------|----------------------|-----------|
| word_count > 0 | 25.0% (60,768 / 243,006) | >= 60% |
| word_count >= 50 | 22.3% (54,177) | >= 55% |
| word_count >= 200 | 19.7% (47,941) | >= 45% |
| Postmedia extraction rate | 0% | >= 70% |
| Top 20 sources with 0% extraction | 17 of 20 | <= 5 of 20 |

### Where the Gains Come From

| Improvement | Docs Affected | Expected Lift |
|-------------|---------------|---------------|
| Dynamic rendering (Postmedia, Torstar, Epoch Times, etc.) | ~80,000 | +25-30% |
| Readability fallback enabled globally | ~15,000 | +5-8% |
| URL pre-filter (stop indexing non-content pages) | ~20,000 | Removes denominator bloat |
| Template registry (better selectors for known CMS) | ~5,000 | +2-3% |

### Non-Goals

- Full auto-detection of render_mode (that's M2)
- Crawling new sources — M1 improves extraction of existing sources
- Changing the classifier or publisher pipeline
- Mobile/AMP rendering

### Measurement Plan

A Grafana dashboard panel showing extraction success rate over time. First measurement
after each Track A and Track B deployment. Baseline query:

```json
{
  "size": 0,
  "aggs": {
    "has_content": {
      "filters": {
        "filters": {
          "word_count_gt_0": { "range": { "word_count": { "gt": 0 } } },
          "word_count_gte_50": { "range": { "word_count": { "gte": 50 } } },
          "word_count_gte_200": { "range": { "word_count": { "gte": 200 } } }
        }
      }
    }
  }
}
```

---

## 3. Render Worker Spec

The render worker is the only new service in M1.

### API Contract

```
POST /render
Content-Type: application/json

{
  "url": "https://calgaryherald.com/news/some-article",
  "timeout_ms": 15000,
  "wait_until": "networkidle"
}

→ 200 OK
{
  "html": "<!DOCTYPE html>...",
  "final_url": "https://calgaryherald.com/news/some-article",
  "render_time_ms": 3200,
  "status_code": 200
}

→ 4xx/5xx on failure
{
  "error": "navigation timeout exceeded"
}
```

### Internals

- Single Playwright browser instance, reused across requests
- Requests queued in-process (Node.js event loop)
- One tab at a time — request N+1 waits for request N to complete
- Tab created per request, closed after HTML extraction
- Browser recycled every 100 requests to prevent memory creep

### Implementation

**Language:** Node.js + Playwright. The render worker is ~100 lines of code. Playwright's
Node API is the most mature, best documented, and has the most predictable memory profile.
The service is isolated in its own container and shares nothing with the Go services.

### Deployment Constraints (see #185)

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Memory limit | 512 MB | Fits within 3.7 GB server headroom |
| Concurrent tabs | 1 | Serialized rendering; queue additional requests |
| CPU limit | 1.0 | 4 CPUs shared across ~35 containers |
| Browser recycle | Every 100 requests | Prevents Chromium memory creep |
| Default timeout | 15,000 ms | Generous for JS-heavy sites |
| Restart policy | unless-stopped | Auto-recover from crashes |

### Docker Compose

```yaml
render-worker:
  build: ./render-worker
  mem_limit: 512m
  cpus: 1.0
  restart: unless-stopped
  networks:
    - north-cloud-network
  environment:
    - MAX_CONCURRENT_TABS=1
    - BROWSER_RECYCLE_AFTER=100
    - DEFAULT_TIMEOUT_MS=15000
```

### Health Check

`GET /health` returns browser process status and queue depth.

---

## 4. Track A Components (Static Extraction Improvements)

Four components, all inside the crawler service.

### 4a. URL Pre-Filter

**Where:** New middleware in Colly collector's `OnRequest` callback, before any HTML
processing.

**What it does:** Skips URLs that will never yield extractable content. Reduces the
denominator and saves render worker budget for dynamic sources.

**Filter rules** (evaluated in order, first match skips):

1. **Binary extensions:** `.pdf`, `.xml`, `.json`, `.css`, `.js`, `.png`, `.jpg`, `.gif`,
   `.svg`, `.mp4`, `.zip`, `.woff`
2. **Off-domain CDN paths:** `/wp-content/uploads/`, `/assets/`, `/static/`, `/media/`
   (when image/file paths, not article paths)
3. **Non-content paths:** `/login`, `/signup`, `/search`, `/contact`, `/about`, `/privacy`,
   `/terms`, `/tag/`, `/category/`, `/author/`, `/page/`, `/cart/`, `/checkout/`,
   `/account/`, `/wp-admin/`
4. **Store/e-commerce:** `/shop/`, `/product/`, `/products/`, `/store/`

Most of this already exists in `content_detector.go` (`nonContentSegments`,
`binaryExtensions`). The change is moving the check **earlier** — into `OnRequest`
instead of waiting until `OnHTML`.

### 4b. Page-Type Classifier

**Where:** Runs inside `RawContentService.Process()`, after extraction but before indexing.

**What it does:** Tags each document with a `page_type` field. Does NOT block indexing —
all content still gets indexed, but the tag enables downstream filtering and quality
measurement.

**Page types:**
- `article` — has title + body with word_count >= 200
- `stub` — has title but word_count < 50 (headlines, teasers, paywalled previews)
- `listing` — multiple links, low text-to-link ratio, no dominant body content
- `other` — everything else

**Detection heuristic** (lightweight, no ML):

1. If word_count >= 200 and title is non-empty → `article`
2. If word_count < 50 and title is non-empty → `stub`
3. If link count > 20 and word_count / link_count < 10 → `listing`
4. Else → `other`

**Stored as:** `meta.page_type` in the raw_content ES document.

### 4c. CMS Template Registry

**Where:** New file `crawler/internal/content/rawcontent/templates.go`. Lookup table
keyed by source domain or OG site name.

**What it does:** Provides known-good CSS selectors for major CMS platforms. When a source
matches a template, its selectors override the generic fallback chain.

**Initial templates:**

| Template | Domains | Selectors |
|----------|---------|-----------|
| Postmedia | calgaryherald.com, vancouversun.com, montrealgazette.com, edmontonjournal.com, ottawacitizen.com, nationalpost.com, leaderpost.com, thestarphoenix.com, lfpress.com | `article.article-content` / `.article-content__content-group` |
| Torstar | thestar.com | `.article-body-text` / `.c-article-body` |
| WordPress (generic) | detected via `<meta name="generator" content="WordPress">` | `.entry-content` / `.post-content` / `article .content` |
| Village Media | various `.villagemedia.ca` | `.article-content` |
| Black Press | various | `.article-body` |

**Fallback chain with templates:**

```
source-manager selectors → template registry → generic fallback → readability
```

### 4d. Readability Fallback — Enable Globally

Set `CRAWLER_READABILITY_FALLBACK_ENABLED=true` in production `.env`.

The code already exists and works. This is a config change, not a code change.

**Expected impact:** Catches ~15,000 additional docs where selector-based extraction
yields < 50 words but the page has extractable content.

---

## 5. Task Breakdown & Execution Order

Eight tasks, two tracks. Track A and Track B can run in parallel after Task 1.

### Task 1: Baseline & Measurement Infrastructure

**GitHub Issue:** #187 (already created)
**Services:** None (ES queries + Grafana)
**Depends on:** Nothing

- Create saved ES query or script for the 5 success metrics
- Build Grafana panel showing extraction success rate over time
- Record baseline numbers (done: 25% / 243k docs)
- Enable `CRAWLER_READABILITY_FALLBACK_ENABLED=true` in production
- Re-measure after 24-48 hours to capture readability lift

### Task 2: URL Pre-Filter (Track A)

**Services:** crawler
**Depends on:** Task 1

- Move existing `nonContentSegments` and `binaryExtensions` checks into `OnRequest` callback
- Add store/e-commerce patterns and CDN asset path patterns
- Log skipped URLs at debug level with skip reason
- Test: unit tests for filter rules + integration test

### Task 3: Page-Type Classifier (Track A)

**Services:** crawler
**Depends on:** Task 1

- Add `classifyPageType()` function in rawcontent package
- Call in `RawContentService.Process()` after extraction
- Store result as `meta.page_type` in ES document
- Test: unit tests for each page type heuristic

### Task 4: CMS Template Registry (Track A)

**Services:** crawler
**Depends on:** Task 1

- Create `crawler/internal/content/rawcontent/templates.go`
- Implement domain-based lookup returning selectors
- Integrate into `getSourceConfig()` as second-tier fallback
- Add WordPress generator meta detection
- Test: unit tests for template matching

### Task 5: Render Worker Service (Track B)

**GitHub Issue:** #185 (constraints documented)
**Services:** New `render-worker/` directory
**Depends on:** Nothing

- Create `render-worker/` with Node.js + Playwright
- `POST /render` endpoint (URL → HTML)
- `GET /health` endpoint (browser status + queue depth)
- Single browser instance, single tab, request queuing
- Browser recycling every 100 requests
- Dockerfile with Playwright + Chromium
- Docker Compose entry with 512MB limit, 1 CPU
- Test: integration test hitting endpoint with known URL

### Task 6: Source Manager — render_mode Field (Track B)

**Services:** source-manager
**Depends on:** Nothing

- Add `render_mode` column to sources table (enum: `static`, `dynamic`, default `static`)
- Database migration (up/down)
- Expose in API (GET/POST/PUT responses)
- Update source-manager API client in crawler to include `render_mode`

### Task 7: Crawler — Render Worker Integration (Track B)

**Services:** crawler
**Depends on:** Task 5, Task 6

- Add render worker HTTP client (`crawler/internal/render/client.go`)
- In fetcher/collector flow, check source `render_mode`
- If `dynamic`: call render worker, feed returned HTML into `ExtractRawContent`
- If `static`: existing Colly path (unchanged)
- Config: `CRAWLER_RENDER_WORKER_URL` env var (default `http://render-worker:3000`)
- Test: unit test with mocked render worker

### Task 8: Rollout & Validation

**Services:** All M1 services
**Depends on:** All previous tasks

- Deploy Track A to production, measure lift (expect 25% → 35-40%)
- Deploy render worker + Track B
- Set `render_mode: "dynamic"` for top 10 JS-heavy sources in batches
- Measure final extraction rate (expect 60%+)
- Validate Postmedia extraction > 70%
- Monitor render worker memory/queue for 48 hours
- Update #187 with final numbers

### Execution Order

```
Task 1 (baseline + readability ON)
  ↓
  ├── Task 2 (URL filter)      ─┐
  ├── Task 3 (page-type)        ├── Track A (parallel)
  ├── Task 4 (templates)       ─┘
  │
  ├── Task 5 (render worker)   ─┐
  ├── Task 6 (render_mode)      ├── Track B (parallel)
  └──→ Task 7 (integration)    ─┘ (depends on 5+6)
       ↓
     Task 8 (rollout + validation)
```

Tasks 2/3/4 are independent. Tasks 5/6 are independent. Task 7 needs 5+6. Task 8 is last.

---

## 6. Rollout Plan & Risk Mitigation

### Rollout Sequence

**Phase 1: Measure + Quick Win** (Task 1)
- Enable readability fallback in production
- Deploy Grafana extraction dashboard
- Wait 48 hours for crawl cycles to populate new data
- Expected: 25% → 30-33%

**Phase 2: Track A Deploy** (Tasks 2/3/4)
- Deploy URL pre-filter, page-type classifier, template registry together
- Re-measure after 48 hours
- Expected: 30-33% → 35-40%

**Phase 3: Track B — Render Worker** (Task 5)
- Deploy render worker container in production
- Health check only — no sources pointed at it yet
- Validate memory stays under 512MB with synthetic requests
- Validate browser recycling works

**Phase 4: Track B — Source Onboarding** (Tasks 6/7)
- Deploy render_mode field + crawler integration
- Onboard sources in batches:
  - **Batch 1:** Calgary Herald (single Postmedia property, ~2,800 docs). Validate
    extraction. Monitor render worker resources for 24 hours.
  - **Batch 2:** Remaining Postmedia (7 properties, ~18,000 docs). One migration flips
    them all since they share a CMS.
  - **Batch 3:** Toronto Star, Epoch Times, The Conversation Canada (~21,000 docs).
  - **Batch 4:** Radio-Canada, Le Devoir, Cinema Scope, remaining 0% sources.
- Re-measure after each batch

**Phase 5: Validation** (Task 8)
- Final measurement against all 5 success metrics
- Close #187 with final numbers
- Update ROADMAP.md status

### Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Render worker OOM | 512MB Docker limit + browser recycling every 100 requests. If OOM: reduce to 50 requests or lower timeout. |
| Render worker queue backup | Single-tab is deliberate. If queue > 10: log warning, drop oldest. Crawler retries on next cycle. |
| Postmedia blocks headless Chrome | Rotate User-Agent via Playwright launch args. If blocked: per-source cookie/header config (future, not M1). |
| Readability produces garbage | Gated by 50-word minimum. Output goes through same pipeline and page-type classifier. Stubs tagged as `stub`. |
| Template selectors break on CMS update | Templates are a fallback tier. If empty, chain falls through to generic → readability. Monitor via page_type distribution. |
| Production regression | Track A is additive. Track B is opt-in. Static sources never touch the render worker. |

### Rollback Plan

- **Track A:** Revert crawler deploy. All changes are additive — removing them returns
  to current behavior.
- **Track B:** Set all sources to `render_mode: "static"`. Stop render worker. Zero
  coupling to static crawling.
- **Readability:** Set `CRAWLER_READABILITY_FALLBACK_ENABLED=false`, restart crawler.
