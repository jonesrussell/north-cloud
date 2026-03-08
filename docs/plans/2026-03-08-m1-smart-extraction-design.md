# M1: Smart Extraction

**Date:** 2026-03-08
**Status:** Approved
**Goal:** Make extraction page-type aware and source-aware, fixing 75% of broken sources without headless browser support.
**Depends on:** M0 (Architecture Review & Versioning)

---

## Context

NorthCloud's crawler was built with an implicit assumption: **every URL is an article**. This was fine when the goal was "find street crime articles from a handful of news sites." But with 468 enabled sources and 242,965 indexed documents, the platform is now a general-purpose web indexer — and that assumption is catastrophic.

### Production Data (2026-03-08)

- **182,238 of 242,965 docs (75%) have word_count = 0**
- **291 sources** with avg word count < 30
- Top offenders: Postmedia chain (30+ outlets), Torstar, Radio-Canada, Le Devoir, Global News

### Three Root Causes

| Cause | % of Failures | Description |
|-------|---------------|-------------|
| Selector mismatches | ~40% | CMS-specific class names not caught by fallback chain. Postmedia uses `article.story-v2-article-content-story`; crawler tries `article > div`. |
| Non-article URLs indexed | ~30% | PDFs, store pages, category pages, CDN URLs, off-domain redirects. No page-type detection exists. |
| ExtractionProfile unused | ~5% | `ExtractionProfile` and `TemplateHint` fields exist in source model but crawler ignores them. |
| Other (paywall, JS, encoding) | ~25% | Remaining failures from paywalls, JS rendering, encoding issues. Partially addressed by M2. |

### Architecture Pivot

The crawler must evolve from **"news-article extractor"** to **"page-type-aware web indexer"**:

```
Old: URL → Fetch → Extract-as-Article → Index
New: URL → Filter → Fetch → Classify-Page-Type → Route → Extract-by-Template → Quality-Gate → Index
```

---

## Proposed Pipeline

```
URL → URL Pre-Filter → Fetch HTML → Page Type Classifier → Route by Type
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    ▼                     ▼                     ▼
               [article]            [listing]              [skip]
                    │                     │                     │
           Template Detect          Extract Links          Log reason
                    │                     │                 (metric++)
           Template Extract         Feed to Frontier
                    │
           Generic Fallback
                    │
            Readability Fallback
                    │
              Quality Gate
              (word_count >= 50)
                    │
               Index to ES
```

---

## Components

### Component 1: URL Pre-Filter

**Package:** `crawler/internal/content/urlfilter/`

Fast, zero-fetch filtering before HTTP request:

```go
type Decision struct {
    Action  Action  // Allow, Skip
    Reason  string  // "pdf", "cdn", "off_domain", "store", "media_file"
}

func Classify(rawURL string, sourceDomain string) Decision
```

**Rules (evaluated in order):**

1. **File extension:** `.pdf`, `.jpg`, `.png`, `.mp4`, `.zip`, `.docx` → skip
2. **Known non-content domains:** `cdn.*`, `*.cloudfront.net`, `play.google.com`, `apps.apple.com` → skip
3. **Off-domain:** URL host doesn't match source host (and isn't a known redirect pattern) → skip
4. **Store/commerce patterns:** `/store/`, `/shop/`, `/cart/`, `/checkout/` → skip
5. **Listing patterns:** `/category/`, `/tag/`, `/author/`, `/archive/`, `/page/\d+` → allow but flag as potential listing

### Component 2: Page Type Classifier

**Package:** `crawler/internal/content/pagetype/`

Post-fetch HTML analysis to determine page type:

```go
type PageType string
const (
    Article  PageType = "article"
    Listing  PageType = "listing"
    Stub     PageType = "stub"     // paywall gate, login page, empty shell
    Other    PageType = "other"    // homepage, about page
)

func Classify(url string, html string, doc *goquery.Document) PageType
```

**Scoring heuristics:**

| Signal | Article | Listing | Stub |
|--------|---------|---------|------|
| JSON-LD type = NewsArticle/Article/BlogPosting | +5 | | |
| OG:type = article | +3 | | |
| `<article>` tag present (single) | +2 | | |
| Headline + body text > 200 words | +3 | | |
| `<time>` or datetime attribute present | +2 | | |
| Byline/author element present | +1 | | |
| Multiple `<article>` tags or card patterns | | +4 | |
| URL contains `/category/`, `/tag/` | | +3 | |
| "Sign in" / "Subscribe" dominant, < 50 words body | | | +4 |

Threshold: article >= 5, listing >= 4, stub >= 4, else other.

### Component 3: Template Detection & Registry

**Package:** `crawler/internal/content/templates/`

CMS template definitions with pre-configured selectors:

```go
type Template struct {
    Name       string
    Detect     func(url string, doc *goquery.Document) bool
    Selectors  TemplateSelectors
}

type TemplateSelectors struct {
    Container string
    Body      string
    Title     string
    Byline    string
    Date      string
    Exclude   []string
}
```

**Initial template catalog:**

| Template | Detection Signal | Container | Body |
|----------|-----------------|-----------|------|
| `postmedia` | `story-v2-article-content-story` class | `article.story-v2-article-content-story` | `.article-content__content-group` |
| `torstar` | `thestar.com` OR `data-content-source` attr | `article[data-content-source]` | `.c-article-body__content` |
| `wordpress` | `wp-content` in HTML, `.entry-content` exists | `article.post` | `.entry-content` |
| `drupal` | `node-content` class | `.node-content` | `.field--name-body` |
| `village_media` | `villagelife.com` pattern, `.article-detail` | `.article-detail` | `.article-detail__body` |
| `black_press` | `blackpress.ca` OR `.article-body-text` | `article` | `.article-body-text` |
| `generic_og_article` | OG:type=article + `<article>` tag | `article` | largest text block |

**TemplateHint integration:** If source has `template_hint` set, skip detection and use that template directly.

**ExtractionProfile integration:** If source has `extraction_profile` set, use it as a custom template (overrides detection).

### Component 4: Enhanced Generic Extraction

Improved fallback chain when no template matches:

1. Try source-configured selectors (existing behavior)
2. Try template selectors (new)
3. **New: Largest Text Block Detection** — walk DOM tree, score elements by text density, filter nav/header/footer/sidebar, return highest-scoring element with > 200 chars
4. Try readability fallback (existing, but enabled by default instead of opt-in)
5. Quality gate: skip if word_count < 50

**Largest Text Block heuristic:**
- Walk DOM tree depth-first
- For each element: compute `text_length / total_children_text_length`
- Filter out: `nav`, `header`, `footer`, `aside`, `.sidebar`, `.menu`, `.ad`, `script`, `style`
- Return element with highest score and > 200 chars of direct text content

### Component 5: Quality Gate & Metrics

After extraction, before indexing:

```go
type ExtractionResult struct {
    PageType      PageType
    Template      string   // "postmedia", "wordpress", "generic", "readability"
    WordCount     int
    TitleFound    bool
    BodyFound     bool
    ExtractMethod string   // "selector", "template", "heuristic", "readability"
}
```

**Metrics:**
- `crawler_pages_by_type{type="article|listing|stub|other"}` — counter
- `crawler_extraction_method{method="selector|template|heuristic|readability"}` — counter
- `crawler_extraction_skipped{reason="url_filter|page_type|quality_gate"}` — counter
- `crawler_word_count` — histogram

### Component 6: Extraction Test Harness

Replace mock test_source endpoint with real extraction preview:

```
POST /api/v1/sources/:id/test-extract
Body: {"url": "https://example.com/article/123"}

Response:
{
  "page_type": "article",
  "template_detected": "postmedia",
  "extraction_method": "template",
  "title": "...",
  "word_count": 847,
  "raw_text_preview": "first 500 chars...",
  "selectors_tried": ["source_config", "postmedia_template"],
  "selector_matched": "postmedia_template"
}
```

---

## Task Breakdown

### Task 1 — URL Pre-Filter

Create `crawler/internal/content/urlfilter/` package:
- Implement URL pattern matching rules
- Integrate into crawler's fetch pipeline (before HTTP request)
- Add skip metrics
- Unit tests with known bad URL patterns from production data

**Size:** Small
**Files:** New package, integration point in `crawler/internal/content/rawcontent/service.go`

### Task 2 — Page Type Classifier

Create `crawler/internal/content/pagetype/` package:
- Implement scoring heuristics
- Integrate after HTML fetch, before extraction
- Route listings to link extraction (feed to frontier)
- Route stubs/other to skip
- Unit tests with HTML fixtures from production sources

**Size:** Medium
**Files:** New package, integration in service.go

### Task 3 — Template Registry & Detection

Create `crawler/internal/content/templates/` package:
- Define Template struct and registry
- Implement 7 initial templates (postmedia, torstar, wordpress, drupal, village_media, black_press, generic_og)
- Wire TemplateHint from source config into template selection
- Wire ExtractionProfile as custom template override
- Unit tests with HTML from each CMS type

**Size:** Large
**Files:** New package, integration in extraction pipeline

### Task 4 — Enhanced Generic Extraction

Improve existing fallback chain in `crawler/internal/content/rawcontent/extractor.go`:
- Add largest-text-block heuristic after selector chain
- Enable readability fallback by default (remove opt-in flag)
- Ensure all existing tests still pass
- Add tests for the new heuristic

**Size:** Medium
**Files:** Modify existing extractor.go, service.go

### Task 5 — Quality Gate & Extraction Metrics

Add structured extraction results and Prometheus metrics:
- Define ExtractionResult struct
- Emit metrics for page type, extraction method, skip reasons
- Add word count histogram
- Create Grafana dashboard for extraction health

**Size:** Medium
**Files:** Modify service.go, new metrics package, Grafana dashboard JSON

### Task 6 — Real Extraction Test Endpoint

Replace mock test_source with real extraction:
- Implement real fetch + extract pipeline in test endpoint
- Return page type, template, extraction method, word count, preview
- Update MCP tool to use real endpoint
- Integration test

**Size:** Medium
**Files:** Modify source-manager handler, MCP client

### Task 7 — Extraction Regression Suite

Create a test suite that validates extraction quality against known sources:
- Fixture set: 1 URL per template type with expected extraction results
- CI integration: run as part of contract tests
- Alert on regression (extraction quality drops)

**Size:** Medium
**Files:** New test package, CI integration

### Task 8 — Backfill & Validation

Re-crawl top 20 worst-performing sources with new pipeline:
- Compare word_count before/after
- Validate template detection accuracy
- Tune heuristics based on results
- Document results in milestone completion report

**Size:** Medium
**Depends on:** Tasks 1-5 deployed to production

---

## Success Criteria

1. **Word count > 0 for >= 60% of raw_content docs** (up from current 25%)
2. **Zero non-article URLs indexed** (PDFs, stores, CDNs filtered)
3. **Template detection accuracy >= 90%** for known CMS types
4. **Extraction test endpoint returns real results** (not mock data)
5. **Grafana dashboard shows extraction health** by source, template, and method

---

## What M1 Does NOT Cover

- JavaScript rendering (M2: Dynamic Crawling)
- Paywall bypass (out of scope)
- Social media crawling (M2)
- API contract formalization (M4)
- Per-source custom JavaScript execution (M2)

---

## Estimated Size

- Task 1 (URL Filter): Small — 2-3 hours
- Task 2 (Page Classifier): Medium — 4-6 hours
- Task 3 (Template Registry): Large — 6-10 hours
- Task 4 (Enhanced Generic): Medium — 3-4 hours
- Task 5 (Metrics): Medium — 3-4 hours
- Task 6 (Test Endpoint): Medium — 4-6 hours
- Task 7 (Regression Suite): Medium — 4-6 hours
- Task 8 (Backfill): Medium — 2-3 hours

**Total: ~4-6 focused sessions**
