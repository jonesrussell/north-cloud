# Feed Endpoints for Personal Site — Design

**Date**: 2026-02-15
**Status**: Approved

## Problem

The "me" personal site (jonesrussell.github.io/me/) shows the same generic North Cloud JSON feed on 4 pages (homepage, blog, projects, resources). The feed endpoint (`northcloud.biz/feed.json`) doesn't exist yet and the content isn't differentiated per page.

## Decision

**Approach A: Platform Showcase** — Use feeds to demonstrate North Cloud as a live, working platform. Each page shows content relevant to its context. Pages that don't benefit from feeds have them removed.

## Architecture

### Content Flow

```
Crawler → ES (raw_content) → Classifier → ES (classified_content) → Search Service → /api/v1/feeds/* → "me" site
```

### North Cloud: New Feed Endpoints (Search Service)

Four public JSON endpoints on the search service (no auth required):

| Endpoint | Content | Filters |
|----------|---------|---------|
| `GET /api/v1/feeds/pipeline` | Mixed — latest across all topics | `min_quality: 60`, 2 per topic, deduped |
| `GET /api/v1/feeds/crime` | Crime articles | `topics: [violent_crime, property_crime, drug_crime, organized_crime, criminal_justice]`, `min_quality: 50` |
| `GET /api/v1/feeds/mining` | Mining articles | `topics: [mining]`, `min_quality: 50` |
| `GET /api/v1/feeds/entertainment` | Entertainment articles | `topics: [entertainment]`, `min_quality: 50` |

**Query params**: `?limit=N` (default 10, max 20)

**Response format** (matches existing "me" site expectations):

```json
{
  "generated_at": "2026-02-15T12:00:00Z",
  "articles": [
    {
      "id": "abc123",
      "title": "Article Title",
      "url": "https://source.com/article",
      "snippet": "First 200 chars...",
      "published_at": "2026-02-15T10:00:00Z",
      "topics": ["mining", "gold"],
      "source": "source_name"
    }
  ]
}
```

**Caching**: 5-minute in-memory TTL per endpoint. News feeds don't need real-time freshness.

**Nginx**: Add `location /api/v1/feeds/ { proxy_pass http://search:8092; }` to route feeds through the search service.

### "me" Site: Page-by-Page Changes

| Page | Current | New |
|------|---------|-----|
| **Homepage** | Generic 5-article feed | "Pipeline in Action" — mixed feed grouped by vertical (crime, mining, entertainment). Fetches `/api/v1/feeds/pipeline?limit=6` |
| **Blog** | North Cloud sidebar (5 articles) | **Remove feed**. Page focuses on personal Dev.to articles only |
| **Projects** | Single sidebar feed (5 articles) | Per-project domain feeds beneath consumer projects only |
| **Resources** | North Cloud section (5 articles) | **Remove feed**. Page stays a curated resource directory |

**Projects page detail** — only 3 projects get live feeds:

| Project | Feed Endpoint | Display |
|---------|--------------|---------|
| StreetCode | `/api/v1/feeds/crime?limit=3` | 3 recent crime articles |
| OreWire | `/api/v1/feeds/mining?limit=3` | 3 recent mining articles |
| Movies of War | `/api/v1/feeds/entertainment?limit=3` | 3 recent entertainment articles |

Other projects (North Cloud, Coforge, MP Emailer, Goforms, Gimbal) show no feed.

### "me" Site: Data Fetching

- Existing `northcloud-service.ts` extended with multiple feed URL support
- Build-time fetch for prerendered pages (homepage, projects) — stale data at build time is acceptable
- 30-minute client-side cache (existing pattern)
- Graceful fallback already exists — sections hide if feed unavailable

## Three Workstreams

1. **North Cloud (search service)**: Add 4 feed endpoints + nginx route
2. **"me" site**: Update homepage feed, add per-project feeds on projects page, remove feeds from blog and resources
3. **Content**: Verify North Cloud crawls sources for all 3 verticals (crime, mining, entertainment)
