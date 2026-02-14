# Multi-Collector Architecture & Extraction Improvements

**Date**: 2026-02-14
**Status**: Approved
**Scope**: Crawler service (`/crawler`)

## Problem

The crawler uses a single Colly collector for both link discovery and content extraction. This violates Colly's recommended multi-collector pattern and means every visited page (list pages, nav pages, 404s) runs the full extraction pipeline. Additionally, the crawler hand-rolls features that Colly ships as built-in extensions (user agent rotation, referer headers, URL length filtering). Some extraction failures are occurring due to weak fallback chains and no post-extraction validation.

## Design

### 1. Multi-Collector Architecture

Split the single collector into two, following Colly's [multi-collector best practice](https://go-colly.org/docs/best_practices/multi_collector/):

**Base Collector** — shared configuration (domains, rate limit, TLS, async, storage, proxy). Never used directly; serves as template for `Clone()`.

**Link Collector** (Clone of base):
- `OnHTML("a[href]")` — link discovery (existing `HandleLink` logic)
- `OnHTML("html")` — lightweight article detection; hands off article URLs to detail collector via `detailCollector.Request("GET", url, nil, ctx, nil)`
- `OnResponseHeaders` — abort non-HTML (existing)
- `OnResponse` — metrics, cloudflare detection, adaptive hash capture (existing)
- `OnError` — existing error handling
- `OnScraped` — page count, milestones

**Detail Collector** (Clone of base):
- `OnHTML("html")` — full `ExtractRawContent()` + validation + Elasticsearch indexing
- `OnResponseHeaders` — abort non-HTML
- `OnResponse` — metrics
- `OnError` — error handling
- No `OnHTML("a[href]")` — does NOT discover or follow links

**Article Detection Heuristic** (used when source has no explicit `ArticleURLPatterns`):
1. `og:type == "article"`
2. JSON-LD `@type` is `"NewsArticle"` or `"Article"`
3. `<article>` element with substantial content
4. URL pattern matching (paths with `/news/`, `/article/`, date patterns like `/2026/02/`)

Sources with explicit `ArticleURLPatterns` bypass the heuristic entirely.

### 2. Colly Extensions Adoption

Replace hand-rolled implementations with Colly's built-in `extensions` package:

| Current (hand-rolled) | Replacement |
|---|---|
| `randomUserAgents` array + `rand.Intn()` in `requestCallback()` | `extensions.RandomUserAgent(collector)` |
| Manual referer via `refererCtxKey` context passing | `extensions.Referer(collector)` |
| Manual URL length check in `HandleLink()` | `extensions.URLLengthFilter(collector, maxLen)` |
| `DisableKeepAlives: false` | `DisableKeepAlives: true` (Colly best practice) |

Extensions are applied to both link and detail collectors.

**Deleted code:**
- `randomUserAgents` var in `collector.go`
- `refererCtxKey` const and manual referer logic in `requestCallback()` + `visitWithRetries()`
- Manual URL length check in `HandleLink()`

### 3. Improved Extraction Fallback Chains

**Title** (add JSON-LD headline):
```
selector → JSON-LD headline → og:title → <title> → <h1>
```

**Published Date** (expand from 2 to 6 sources):
```
selector → JSON-LD datePublished → article:published_time →
article:published → time[datetime] → .published-date
```

**Author** (expand from 1 to 6 sources):
```
selector → JSON-LD author → meta author →
[rel="author"] → .byline → .author
```

### 4. Post-Extraction Validation

After `ExtractRawContent()`, reject documents that fail minimum quality checks:
- Title must be non-empty
- RawText must exceed minimum word count (50 words)
- Skip pages that are clearly non-content (login, search results, error pages)

### 5. Source Configuration Changes

**Relax selector validation** — Make all `ArticleSelectors` fields optional (currently Container, Title, Body are required). Auto-detection handles missing selectors:

```go
// All selectors optional — auto-detection fills gaps
func (s *ArticleSelectors) Validate() error {
    return nil
}
```

**Add `ArticleURLPatterns`** — New optional field on `Source`:

```go
type Source struct {
    // ... existing fields ...
    ArticleURLPatterns []string `yaml:"article_url_patterns"`
}
```

Regex patterns identifying article URLs. Used by link collector to decide which URLs to pass to detail collector. Empty means use heuristic detection.

## Files Changed

**Modified:**
- `internal/crawler/collector.go` — Split into base/link/detail collectors, `Clone()`, extensions, fix `DisableKeepAlives`, delete manual UA/referer code
- `internal/crawler/processing.go` — Add post-extraction validation
- `internal/crawler/link_handler.go` — Remove manual URL length check, remove manual referer context
- `internal/content/rawcontent/extractor.go` — Improved fallback chains for title, date, author
- `internal/config/types/selectors.go` — Relax `ArticleSelectors.Validate()`
- `internal/config/types/source.go` — Add `ArticleURLPatterns` field
- `internal/crawler/start.go` — Use link collector for initial Visit, `Wait()` on both collectors

**New:**
- `internal/crawler/article_detector.go` — Article detection heuristic

## Migration

Non-breaking change. No database migrations, no API changes, no config format changes. `ArticleURLPatterns` defaults to empty (heuristic detection). Existing sources with explicit selectors continue to work identically.

## What Stays The Same

- Redis storage, proxy rotation, adaptive scheduling
- Elasticsearch indexing (same `RawContent` document structure, same index naming)
- Job scheduler (still calls `Crawler.Start()`)
- Source manager API (new field is optional)
- Downstream pipeline (classifier, publisher) unaffected
