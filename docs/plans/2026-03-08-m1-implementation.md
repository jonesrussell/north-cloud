# M1: Smart Extraction — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Raise extraction success rate from 25% to 60%+ by improving static extraction and introducing dynamic rendering for JS-heavy sources.

**Architecture:** Two parallel tracks. Track A: static extraction improvements (URL pre-filter, page-type classifier, CMS template registry, readability ON). Track B: Playwright render worker sidecar with per-source opt-in. Both tracks feed HTML into the existing extraction pipeline — no extraction logic changes.

**Tech Stack:** Go 1.26+ (crawler, source-manager), Node.js + Playwright (render-worker), Elasticsearch, Grafana

**Design doc:** `docs/plans/2026-03-08-m1-smart-extraction-design.md`

---

## Pre-Flight

**GitHub issues already created:** #185 (infra constraints), #186 (MinIO ceiling), #187 (baseline data)

**Baseline (2026-03-08):** 243,006 total docs, 25% with word_count > 0, readability fallback OFF.

**Branch strategy:** Each task gets its own branch off `main`. Tasks within a track can be stacked if needed.

---

### Task 1: Baseline Measurement & Readability ON

**GitHub Issue:** #187
**Files:**
- Modify: `/opt/north-cloud/.env` (production, via SSH)
- No code changes

**Step 1: Enable readability fallback in production**

```bash
ssh jones@northcloud.one "echo 'CRAWLER_READABILITY_FALLBACK_ENABLED=true' >> /opt/north-cloud/.env"
```

**Step 2: Restart crawler to pick up the new env var**

```bash
ssh jones@northcloud.one "cd /opt/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d --no-deps crawler"
```

**Step 3: Verify readability is active**

```bash
ssh jones@northcloud.one "docker logs north-cloud-crawler-1 2>&1 | head -50"
```

Look for: readability fallback config being loaded (debug log at startup).

**Step 4: Wait 48 hours, then re-measure**

```bash
ssh jones@northcloud.one "docker exec north-cloud-elasticsearch-1 curl -s 'http://localhost:9200/*_raw_content/_search' -H 'Content-Type: application/json' -d '{\"size\":0,\"aggs\":{\"has_content\":{\"filters\":{\"filters\":{\"gt_0\":{\"range\":{\"word_count\":{\"gt\":0}}},\"gte_50\":{\"range\":{\"word_count\":{\"gte\":50}}},\"gte_200\":{\"range\":{\"word_count\":{\"gte\":200}}}}}}}}'"
```

Record the numbers in #187. Expected: 25% → 30-33%.

---

### Task 2: URL Pre-Filter (Track A)

**Files:**
- Modify: `crawler/internal/crawler/collector.go:67-80` — add `OnRequest` filter callback
- Modify: `crawler/internal/crawler/content_detector.go` — export filter functions for reuse
- Create: `crawler/internal/crawler/url_filter.go` — consolidated URL pre-filter
- Create: `crawler/internal/crawler/url_filter_test.go`

**Step 1: Write the failing test**

Create `crawler/internal/crawler/url_filter_test.go`:

```go
package crawler

import "testing"

func TestShouldSkipURL(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"binary pdf", "https://example.com/report.pdf", true},
		{"binary image", "https://example.com/photo.jpg", true},
		{"css file", "https://example.com/style.css", true},
		{"login page", "https://example.com/login", true},
		{"wp-admin", "https://example.com/wp-admin/edit.php", true},
		{"cart page", "https://example.com/cart", true},
		{"shop page", "https://example.com/shop/item-123", true},
		{"product page", "https://example.com/products/widget", true},
		{"store page", "https://example.com/store/checkout", true},
		{"category page", "https://example.com/category/sports", true},
		{"tag page", "https://example.com/tag/breaking-news", true},
		{"wp uploads", "https://example.com/wp-content/uploads/2026/photo.jpg", true},
		{"assets path", "https://example.com/assets/images/logo.png", true},
		{"article page", "https://example.com/news/2026/03/headline-here", false},
		{"homepage", "https://example.com/", false},
		{"about page", "https://example.com/about", true},
		{"normal article", "https://example.com/story/some-article-title", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := shouldSkipURL(tt.url)
			if result != tt.expected {
				t.Errorf("shouldSkipURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd crawler && GOWORK=off go test ./internal/crawler/ -run TestShouldSkipURL -v
```

Expected: FAIL — `shouldSkipURL` not defined.

**Step 3: Write the implementation**

Create `crawler/internal/crawler/url_filter.go`:

```go
package crawler

import (
	"net/url"
	"strings"
)

// ecommerceSegments are URL path segments indicating e-commerce pages (always skip).
var ecommerceSegments = map[string]bool{
	"shop":     true,
	"store":    true,
	"product":  true,
	"products": true,
	"cart":     true,
	"checkout": true,
}

// cdnAssetPrefixes are URL path prefixes for CDN/asset directories (skip when binary extension).
var cdnAssetPrefixes = []string{
	"/wp-content/uploads/",
	"/assets/",
	"/static/",
}

// shouldSkipURL returns true if the URL should be skipped before fetching.
// This is the consolidated pre-filter that runs in OnRequest.
// It reuses nonContentSegments and binaryExtensions from content_detector.go.
func shouldSkipURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}

	lowerPath := strings.ToLower(parsed.Path)

	// Check binary file extensions
	if hasBinaryExtension(lowerPath) {
		return true
	}

	// Check CDN asset paths with binary extensions
	if isCDNAssetPath(lowerPath) {
		return true
	}

	// Check path segments for non-content and e-commerce
	segments := strings.Split(strings.TrimLeft(lowerPath, "/"), "/")
	for _, seg := range segments {
		if nonContentSegments[seg] || ecommerceSegments[seg] {
			return true
		}
	}

	return false
}

// hasBinaryExtension checks if the path ends with a known binary file extension.
func hasBinaryExtension(lowerPath string) bool {
	for ext := range binaryExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}
	return false
}

// isCDNAssetPath checks if the path is a CDN/asset directory with a binary extension.
func isCDNAssetPath(lowerPath string) bool {
	for _, prefix := range cdnAssetPrefixes {
		if strings.Contains(lowerPath, prefix) && hasBinaryExtension(lowerPath) {
			return true
		}
	}
	return false
}
```

**Step 4: Run test to verify it passes**

```bash
cd crawler && GOWORK=off go test ./internal/crawler/ -run TestShouldSkipURL -v
```

Expected: PASS

**Step 5: Integrate into collector OnRequest callback**

In `crawler/internal/crawler/collector.go`, inside `setupCollector()`, after the collector is created and extensions are applied, add the `OnRequest` pre-filter. Find the section where `setupCallbacks` is called and add before it:

```go
// URL pre-filter: skip non-content URLs before fetching
c.collector.OnRequest(func(r *colly.Request) {
	if shouldSkipURL(r.URL.String()) {
		c.GetJobLogger().Debug(logs.CategoryLinks, "Skipping non-content URL",
			logs.URL(r.URL.String()))
		r.Abort()
	}
})
```

**Step 6: Lint and test**

```bash
cd crawler && GOWORK=off golangci-lint run && GOWORK=off go test ./...
```

**Step 7: Commit**

```bash
git add crawler/internal/crawler/url_filter.go crawler/internal/crawler/url_filter_test.go crawler/internal/crawler/collector.go
git commit -m "feat(crawler): add URL pre-filter in OnRequest to skip non-content pages

Moves binary extension, non-content segment, and e-commerce checks into
OnRequest callback so non-content URLs are skipped before HTTP fetch.
Reduces wasted bandwidth and denominator bloat in extraction metrics."
```

---

### Task 3: Page-Type Classifier (Track A)

**Files:**
- Create: `crawler/internal/content/rawcontent/page_type.go`
- Create: `crawler/internal/content/rawcontent/page_type_test.go`
- Modify: `crawler/internal/content/rawcontent/service.go:76-125` — add page_type to meta

**Step 1: Write the failing test**

Create `crawler/internal/content/rawcontent/page_type_test.go`:

```go
package rawcontent

import "testing"

func TestClassifyPageType(t *testing.T) {
	t.Helper()

	tests := []struct {
		name      string
		title     string
		wordCount int
		linkCount int
		expected  string
	}{
		{"article with content", "Breaking News", 350, 5, pageTypeArticle},
		{"article at threshold", "Story", 200, 3, pageTypeArticle},
		{"stub with title", "Headline", 20, 2, pageTypeStub},
		{"stub empty body", "Title Only", 0, 1, pageTypeStub},
		{"listing page", "", 100, 30, pageTypeListing},
		{"listing high link ratio", "News", 50, 25, pageTypeListing},
		{"other no title", "", 100, 5, pageTypeOther},
		{"other low content", "", 80, 8, pageTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := classifyPageType(tt.title, tt.wordCount, tt.linkCount)
			if result != tt.expected {
				t.Errorf("classifyPageType(%q, %d, %d) = %q, want %q",
					tt.title, tt.wordCount, tt.linkCount, result, tt.expected)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd crawler && GOWORK=off go test ./internal/content/rawcontent/ -run TestClassifyPageType -v
```

Expected: FAIL

**Step 3: Write the implementation**

Create `crawler/internal/content/rawcontent/page_type.go`:

```go
package rawcontent

// Page type constants for extraction quality tagging.
const (
	pageTypeArticle = "article"
	pageTypeStub    = "stub"
	pageTypeListing = "listing"
	pageTypeOther   = "other"
)

// Page type classification thresholds.
const (
	articleMinWordCount  = 200
	stubMaxWordCount     = 50
	listingMinLinkCount  = 20
	listingMaxWordPerLink = 10
)

// classifyPageType assigns a page type based on extraction results.
// This is a post-extraction tag — it does NOT block indexing.
func classifyPageType(title string, wordCount, linkCount int) string {
	hasTitle := title != ""

	if hasTitle && wordCount >= articleMinWordCount {
		return pageTypeArticle
	}

	if hasTitle && wordCount < stubMaxWordCount {
		return pageTypeStub
	}

	if linkCount >= listingMinLinkCount && (wordCount == 0 || wordCount/linkCount < listingMaxWordPerLink) {
		return pageTypeListing
	}

	return pageTypeOther
}
```

**Step 4: Run test to verify it passes**

```bash
cd crawler && GOWORK=off go test ./internal/content/rawcontent/ -run TestClassifyPageType -v
```

Expected: PASS

**Step 5: Integrate into RawContentService.Process()**

In `crawler/internal/content/rawcontent/service.go`, in the `convertToRawContent` method, after building the `meta` map (around line 380), add:

```go
// Tag page type for extraction quality measurement
pageType := classifyPageType(rawData.Title, wordCount, linkCount)
meta["page_type"] = pageType
```

Note: `linkCount` is not currently available in `convertToRawContent`. You need to pass it through. The simplest approach: add a `linkCount` parameter to `convertToRawContent`, computed in `Process()` from the raw HTML using `strings.Count(rawData.RawHTML, "<a ")` (rough but sufficient for heuristic classification).

**Step 6: Lint and test**

```bash
cd crawler && GOWORK=off golangci-lint run && GOWORK=off go test ./...
```

**Step 7: Commit**

```bash
git add crawler/internal/content/rawcontent/page_type.go crawler/internal/content/rawcontent/page_type_test.go crawler/internal/content/rawcontent/service.go
git commit -m "feat(crawler): add page-type classifier (article/stub/listing/other)

Tags each extracted document with meta.page_type based on word count,
title presence, and link density. Does not block indexing — the tag
enables extraction quality measurement in Grafana."
```

---

### Task 4: CMS Template Registry (Track A)

**Files:**
- Create: `crawler/internal/content/rawcontent/templates.go`
- Create: `crawler/internal/content/rawcontent/templates_test.go`
- Modify: `crawler/internal/content/rawcontent/service.go:229-277` — integrate into `getSourceConfig()`

**Step 1: Write the failing test**

Create `crawler/internal/content/rawcontent/templates_test.go`:

```go
package rawcontent

import "testing"

func TestLookupTemplate(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		domain   string
		wantName string
		wantHit  bool
	}{
		{"postmedia calgary", "calgaryherald.com", "postmedia", true},
		{"postmedia vancouver", "vancouversun.com", "postmedia", true},
		{"postmedia national", "nationalpost.com", "postmedia", true},
		{"torstar", "thestar.com", "torstar", true},
		{"unknown domain", "random-site.com", "", false},
		{"empty domain", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			tmpl, ok := lookupTemplate(tt.domain)
			if ok != tt.wantHit {
				t.Errorf("lookupTemplate(%q) hit=%v, want %v", tt.domain, ok, tt.wantHit)
			}
			if ok && tmpl.Name != tt.wantName {
				t.Errorf("lookupTemplate(%q) name=%q, want %q", tt.domain, tmpl.Name, tt.wantName)
			}
		})
	}
}

func TestTemplateSelectorsNotEmpty(t *testing.T) {
	t.Helper()

	for _, tmpl := range templateRegistry {
		t.Run(tmpl.Name, func(t *testing.T) {
			t.Helper()
			if tmpl.Selectors.Body == "" && tmpl.Selectors.Container == "" {
				t.Errorf("template %q has no body or container selector", tmpl.Name)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd crawler && GOWORK=off go test ./internal/content/rawcontent/ -run TestLookupTemplate -v
```

**Step 3: Write the implementation**

Create `crawler/internal/content/rawcontent/templates.go`:

```go
package rawcontent

// CMSTemplate defines known-good CSS selectors for a CMS platform.
type CMSTemplate struct {
	Name      string
	Domains   []string
	Selectors SourceSelectors
}

// templateRegistry is the list of known CMS templates.
// Order does not matter — lookup is by domain map.
var templateRegistry = []CMSTemplate{
	{
		Name: "postmedia",
		Domains: []string{
			"calgaryherald.com", "vancouversun.com", "montrealgazette.com",
			"edmontonjournal.com", "ottawacitizen.com", "nationalpost.com",
			"leaderpost.com", "thestarphoenix.com", "lfpress.com",
			"windsorstar.com", "theprovince.com",
		},
		Selectors: SourceSelectors{
			Container: "article.article-content",
			Body:      ".article-content__content-group",
			Title:     "h1.article-title",
		},
	},
	{
		Name:    "torstar",
		Domains: []string{"thestar.com"},
		Selectors: SourceSelectors{
			Container: "article",
			Body:      ".c-article-body__content, .article-body-text",
			Title:     "h1",
		},
	},
	{
		Name:    "village_media",
		Domains: []string{"villagemedia.ca", "baytoday.ca", "sudbury.com", "northernontario.ctvnews.ca"},
		Selectors: SourceSelectors{
			Container: ".article-detail",
			Body:      ".article-detail__body",
			Title:     "h1.article-detail__title",
		},
	},
	{
		Name:    "black_press",
		Domains: []string{"blackpress.ca", "abbynews.com", "nanaimobulletin.com"},
		Selectors: SourceSelectors{
			Container: "article",
			Body:      ".article-body-text, .article-body",
			Title:     "h1",
		},
	},
}

// domainTemplateIndex is a map from domain → template for O(1) lookup.
// Built at init time from templateRegistry.
var domainTemplateIndex map[string]*CMSTemplate

func init() {
	domainTemplateIndex = make(map[string]*CMSTemplate, len(templateRegistry)*4) //nolint:mnd // rough pre-alloc
	for i := range templateRegistry {
		tmpl := &templateRegistry[i]
		for _, domain := range tmpl.Domains {
			domainTemplateIndex[domain] = tmpl
		}
	}
}

// lookupTemplate returns the CMS template for a domain, if one exists.
func lookupTemplate(domain string) (*CMSTemplate, bool) {
	if domain == "" {
		return nil, false
	}
	tmpl, ok := domainTemplateIndex[domain]
	return tmpl, ok
}
```

**Step 4: Run test to verify it passes**

```bash
cd crawler && GOWORK=off go test ./internal/content/rawcontent/ -run "TestLookupTemplate|TestTemplateSelectorsNotEmpty" -v
```

**Step 5: Integrate into getSourceConfig()**

In `crawler/internal/content/rawcontent/service.go`, modify `getSourceConfig()` to check the template registry after source-manager selectors but before returning empty selectors. After line 271 (after checking source selectors), add:

```go
// If source-manager has no selectors, try template registry by domain
if selectors.Title == "" && selectors.Body == "" && selectors.Container == "" {
	hostname := extractHostFromURL(sourceURL)
	if tmpl, ok := lookupTemplate(hostname); ok {
		selectors = tmpl.Selectors
		s.logger.Debug("Using CMS template selectors",
			infralogger.String("url", sourceURL),
			infralogger.String("template", tmpl.Name))
	}
}
```

Add the helper:

```go
// extractHostFromURL extracts the hostname from a URL string.
func extractHostFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	// Strip www. prefix for template matching
	return strings.TrimPrefix(host, "www.")
}
```

**Step 6: Lint and test**

```bash
cd crawler && GOWORK=off golangci-lint run && GOWORK=off go test ./...
```

**Step 7: Commit**

```bash
git add crawler/internal/content/rawcontent/templates.go crawler/internal/content/rawcontent/templates_test.go crawler/internal/content/rawcontent/service.go
git commit -m "feat(crawler): add CMS template registry for known news platforms

Provides pre-configured CSS selectors for Postmedia, Torstar, Village
Media, and Black Press. Templates are the second tier in the fallback
chain: source-manager selectors → template registry → generic → readability."
```

---

### Task 5: Render Worker Service (Track B)

**Files:**
- Create: `render-worker/package.json`
- Create: `render-worker/index.js`
- Create: `render-worker/Dockerfile`
- Create: `render-worker/.dockerignore`
- Modify: `docker-compose.base.yml` — add render-worker service

**Step 1: Create package.json**

Create `render-worker/package.json`:

```json
{
  "name": "north-cloud-render-worker",
  "version": "1.0.0",
  "private": true,
  "description": "Playwright-based HTML render worker for NorthCloud crawler",
  "main": "index.js",
  "scripts": {
    "start": "node index.js",
    "test": "node --test test.js"
  },
  "dependencies": {
    "playwright": "^1.52.0"
  }
}
```

**Step 2: Create the render worker**

Create `render-worker/index.js`:

```javascript
const http = require('http');
const { chromium } = require('playwright');

const PORT = process.env.PORT || 3000;
const MAX_CONCURRENT = parseInt(process.env.MAX_CONCURRENT_TABS || '1', 10);
const RECYCLE_AFTER = parseInt(process.env.BROWSER_RECYCLE_AFTER || '100', 10);
const DEFAULT_TIMEOUT = parseInt(process.env.DEFAULT_TIMEOUT_MS || '15000', 10);

let browser = null;
let requestCount = 0;
let queueDepth = 0;
let processing = false;
const queue = [];

async function ensureBrowser() {
  if (!browser || !browser.isConnected()) {
    browser = await chromium.launch({
      args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage'],
    });
    requestCount = 0;
  }
  return browser;
}

async function recycleBrowserIfNeeded() {
  if (requestCount >= RECYCLE_AFTER && browser) {
    await browser.close().catch(() => {});
    browser = null;
  }
}

async function renderPage(url, timeoutMs, waitUntil) {
  const b = await ensureBrowser();
  const context = await b.newContext({
    userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
  });
  const page = await context.newPage();

  try {
    const start = Date.now();
    const response = await page.goto(url, {
      timeout: timeoutMs,
      waitUntil: waitUntil || 'networkidle',
    });

    const html = await page.content();
    const finalUrl = page.url();
    const renderTimeMs = Date.now() - start;
    const statusCode = response ? response.status() : 0;

    return { html, final_url: finalUrl, render_time_ms: renderTimeMs, status_code: statusCode };
  } finally {
    await page.close().catch(() => {});
    await context.close().catch(() => {});
    requestCount++;
    await recycleBrowserIfNeeded();
  }
}

function processQueue() {
  if (processing || queue.length === 0) return;
  processing = true;
  const { req, res, body } = queue.shift();
  queueDepth = queue.length;

  renderPage(body.url, body.timeout_ms || DEFAULT_TIMEOUT, body.wait_until)
    .then((result) => {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(result));
    })
    .catch((err) => {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: err.message }));
    })
    .finally(() => {
      processing = false;
      processQueue();
    });
}

const server = http.createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/health') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      status: 'ok',
      browser_connected: browser ? browser.isConnected() : false,
      request_count: requestCount,
      queue_depth: queueDepth,
      recycle_after: RECYCLE_AFTER,
    }));
    return;
  }

  if (req.method === 'POST' && req.url === '/render') {
    let data = '';
    req.on('data', (chunk) => { data += chunk; });
    req.on('end', () => {
      try {
        const body = JSON.parse(data);
        if (!body.url) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'url is required' }));
          return;
        }
        queue.push({ req, res, body });
        queueDepth = queue.length;
        processQueue();
      } catch (err) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'invalid JSON' }));
      }
    });
    return;
  }

  res.writeHead(404, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ error: 'not found' }));
});

server.listen(PORT, () => {
  console.log(`render-worker listening on port ${PORT}`);
  ensureBrowser().then(() => console.log('browser launched'));
});

process.on('SIGTERM', async () => {
  console.log('shutting down');
  if (browser) await browser.close().catch(() => {});
  server.close();
  process.exit(0);
});
```

**Step 3: Create Dockerfile**

Create `render-worker/Dockerfile`:

```dockerfile
FROM mcr.microsoft.com/playwright:v1.52.0-noble

WORKDIR /app
COPY package.json ./
RUN npm install --production
COPY index.js ./

EXPOSE 3000
CMD ["node", "index.js"]
```

**Step 4: Create .dockerignore**

Create `render-worker/.dockerignore`:

```
node_modules
npm-debug.log
```

**Step 5: Add to docker-compose.base.yml**

Add the render-worker service to `docker-compose.base.yml` in the services section:

```yaml
  render-worker:
    build: ./render-worker
    container_name: north-cloud-render-worker
    mem_limit: 512m
    cpus: 1.0
    restart: unless-stopped
    networks:
      - north-cloud-network
    environment:
      - PORT=3000
      - MAX_CONCURRENT_TABS=1
      - BROWSER_RECYCLE_AFTER=100
      - DEFAULT_TIMEOUT_MS=15000
    healthcheck:
      test: ["CMD", "node", "-e", "require('http').get('http://localhost:3000/health', r => { r.statusCode === 200 ? process.exit(0) : process.exit(1) }).on('error', () => process.exit(1))"]
      interval: 30s
      timeout: 10s
      retries: 3
```

**Step 6: Test locally**

```bash
cd render-worker && npm install
node index.js &
curl -s http://localhost:3000/health | jq .
curl -s -X POST http://localhost:3000/render -H "Content-Type: application/json" -d '{"url":"https://example.com"}' | jq '.html | length'
kill %1
```

Expected: health returns `{"status":"ok",...}`, render returns HTML length > 0.

**Step 7: Docker build test**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build render-worker
```

**Step 8: Commit**

```bash
git add render-worker/ docker-compose.base.yml
git commit -m "feat(render-worker): add Playwright-based HTML render sidecar

Stateless HTTP service that renders JS-heavy pages and returns full DOM
HTML. Single Chromium instance, single-tab concurrency, request queuing,
browser recycling every 100 requests. 512MB memory limit.

API: POST /render (url → html), GET /health (browser status + queue)."
```

---

### Task 6: Source Manager — render_mode Field (Track B)

**Files:**
- Create: `source-manager/migrations/007_add_render_mode.up.sql`
- Create: `source-manager/migrations/007_add_render_mode.down.sql`
- Modify: `source-manager/internal/models/source.go:14-39` — add RenderMode field
- Modify: `source-manager/internal/handlers/source.go` — include render_mode in CRUD
- Modify: `crawler/internal/sources/apiclient/types.go:9-36` — add RenderMode to APISource

**Step 1: Create migration**

Create `source-manager/migrations/007_add_render_mode.up.sql`:

```sql
ALTER TABLE sources ADD COLUMN render_mode TEXT NOT NULL DEFAULT 'static';
```

Create `source-manager/migrations/007_add_render_mode.down.sql`:

```sql
ALTER TABLE sources DROP COLUMN render_mode;
```

**Step 2: Add field to Source model**

In `source-manager/internal/models/source.go`, add after line 32 (after `TemplateHint`):

```go
// RenderMode: "static" (default) or "dynamic" (use Playwright render worker).
RenderMode string `db:"render_mode" json:"render_mode"`
```

**Step 3: Add field to crawler's APISource**

In `crawler/internal/sources/apiclient/types.go`, add after line 31 (after `TemplateHint`):

```go
// RenderMode: "static" (default) or "dynamic" (use Playwright render worker).
RenderMode string `json:"render_mode"`
```

**Step 4: Run migrations locally**

```bash
task migrate:source-manager
```

**Step 5: Run tests**

```bash
cd source-manager && GOWORK=off go test ./...
cd crawler && GOWORK=off go test ./...
```

**Step 6: Lint**

```bash
task lint:source-manager && task lint:crawler
```

**Step 7: Commit**

```bash
git add source-manager/migrations/007_add_render_mode.up.sql source-manager/migrations/007_add_render_mode.down.sql source-manager/internal/models/source.go crawler/internal/sources/apiclient/types.go
git commit -m "feat(source-manager): add render_mode field (static/dynamic)

Adds render_mode column to sources table. Default 'static' preserves
existing behavior. Sources set to 'dynamic' will use the Playwright
render worker for HTML fetching."
```

---

### Task 7: Crawler — Render Worker Integration (Track B)

**Files:**
- Create: `crawler/internal/render/client.go`
- Create: `crawler/internal/render/client_test.go`
- Modify: `crawler/internal/content/rawcontent/service.go` — add render path
- Modify: `crawler/internal/config/crawler/config.go` — add render worker URL config
- Modify: `crawler/internal/bootstrap/services.go` — wire render client

**Step 1: Write the failing test for the render client**

Create `crawler/internal/render/client_test.go`:

```go
package render

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRender(t *testing.T) {
	t.Helper()

	expectedHTML := "<html><body>rendered content</body></html>"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/render" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req RenderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.URL == "" {
			t.Error("URL is empty")
		}

		resp := RenderResponse{
			HTML:         expectedHTML,
			FinalURL:     req.URL,
			RenderTimeMs: 1500,
			StatusCode:   200,
		}
		w.Header().Set("Content-Type", "application/json")
		if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
			t.Errorf("failed to encode response: %v", encodeErr)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Render("https://example.com/article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.HTML != expectedHTML {
		t.Errorf("HTML = %q, want %q", result.HTML, expectedHTML)
	}
}

func TestClientRenderError(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		resp := ErrorResponse{Error: "navigation timeout"}
		if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
			t.Errorf("failed to encode error response: %v", encodeErr)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Render("https://example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd crawler && GOWORK=off go test ./internal/render/ -run TestClientRender -v
```

Expected: FAIL — package doesn't exist.

**Step 3: Write the render client**

Create `crawler/internal/render/client.go`:

```go
// Package render provides an HTTP client for the Playwright render worker.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Default HTTP client timeout for render requests.
const defaultHTTPTimeout = 30 * time.Second

// RenderRequest is the payload sent to POST /render.
type RenderRequest struct {
	URL       string `json:"url"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
	WaitUntil string `json:"wait_until,omitempty"`
}

// RenderResponse is the payload returned by POST /render on success.
type RenderResponse struct {
	HTML         string `json:"html"`
	FinalURL     string `json:"final_url"`
	RenderTimeMs int    `json:"render_time_ms"`
	StatusCode   int    `json:"status_code"`
}

// ErrorResponse is the payload returned on failure.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Client is an HTTP client for the render worker.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new render worker client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Render sends a URL to the render worker and returns the rendered HTML.
func (c *Client) Render(pageURL string) (*RenderResponse, error) {
	reqBody := RenderRequest{URL: pageURL}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal render request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/render",
		"application/json",
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("render request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&errResp); decodeErr == nil && errResp.Error != "" {
			return nil, fmt.Errorf("render worker error (HTTP %d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("render worker returned HTTP %d", resp.StatusCode)
	}

	var result RenderResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode render response: %w", err)
	}

	return &result, nil
}
```

**Step 4: Run test to verify it passes**

```bash
cd crawler && GOWORK=off go test ./internal/render/ -run "TestClientRender" -v
```

Expected: PASS

**Step 5: Add render worker URL to crawler config**

In `crawler/internal/config/crawler/config.go`, add:

```go
// RenderWorkerURL is the base URL of the Playwright render worker.
// Empty means dynamic rendering is disabled.
RenderWorkerURL string `env:"CRAWLER_RENDER_WORKER_URL"`
```

**Step 6: Wire render client into bootstrap and RawContentService**

This step requires modifying `crawler/internal/bootstrap/services.go` to create a render client when the URL is configured, and passing it to `RawContentService`. The `RawContentService` needs a new optional `renderClient` field.

In `crawler/internal/content/rawcontent/service.go`, add to the struct:

```go
renderClient *render.Client // optional; nil when dynamic rendering is disabled
```

In `Process()`, before calling `ExtractRawContent`, check if the source has `render_mode == "dynamic"` and the render client is available. If so, call the render worker to get HTML, then feed it to the extraction pipeline using goquery (bypassing Colly's HTML element).

This is the most complex integration point. The key insight: for dynamic sources, we get HTML from the render worker and need to create a `*colly.HTMLElement`-compatible structure, or refactor `ExtractRawContent` to accept a `*goquery.Document` directly.

**Recommended approach:** Add a new method `ExtractRawContentFromHTML(htmlString, sourceURL, ...)` that creates a goquery document internally and delegates to the same extraction helpers. This avoids coupling to `colly.HTMLElement`.

**Step 7: Lint and test**

```bash
cd crawler && GOWORK=off golangci-lint run && GOWORK=off go test ./...
```

**Step 8: Commit**

```bash
git add crawler/internal/render/ crawler/internal/config/crawler/config.go crawler/internal/content/rawcontent/service.go crawler/internal/bootstrap/services.go
git commit -m "feat(crawler): integrate render worker for dynamic source extraction

Adds render worker HTTP client and integrates it into the extraction
pipeline. Sources with render_mode='dynamic' are fetched via the
Playwright render worker instead of Colly. The returned HTML feeds
into the same extraction pipeline as static sources."
```

---

### Task 8: Rollout & Validation

**Files:** No code changes — production operations only.

**Step 1: Deploy Track A (Tasks 2-4)**

Push the branch, merge to main, let CI deploy.

**Step 2: Measure Track A lift**

Wait 48 hours, then run the baseline query. Expected: 30-33% → 35-40%.

**Step 3: Deploy Track B (Tasks 5-7)**

Push and merge. Verify render worker starts:

```bash
ssh jones@northcloud.one "docker logs north-cloud-render-worker 2>&1 | head -10"
```

Expected: `render-worker listening on port 3000` and `browser launched`.

**Step 4: Onboard Batch 1 — Calgary Herald**

```bash
# Get Calgary Herald source ID
ssh jones@northcloud.one "docker run --rm --network=north-cloud_north-cloud-network curlimages/curl:8.1.2 -s http://source-manager:8050/api/v1/sources" | jq '.sources[] | select(.name | test("calgary"; "i")) | .id'

# Update render_mode to dynamic (replace SOURCE_ID)
# Use the MCP tool or direct API call with auth token
```

Wait 24 hours. Check extraction:

```bash
ssh jones@northcloud.one "docker exec north-cloud-elasticsearch-1 curl -s 'http://localhost:9200/calgary_herald_raw_content/_search' -H 'Content-Type: application/json' -d '{\"size\":0,\"aggs\":{\"has_words\":{\"range\":{\"field\":\"word_count\",\"ranges\":[{\"key\":\"empty\",\"to\":1},{\"key\":\"has_content\",\"from\":1}]}}}}'"
```

Expected: extraction rate > 0% (was 0% before).

**Step 5: Onboard remaining batches**

- Batch 2: Remaining Postmedia (8 properties)
- Batch 3: Toronto Star, Epoch Times, Conversation Canada
- Batch 4: Radio-Canada, Le Devoir, remaining 0% sources

**Step 6: Final measurement**

Run the full baseline query. Update #187 with final numbers. Expected: 60%+.

**Step 7: Update ROADMAP.md**

Change M1 status from "In progress" to "Complete" with final metrics.

---

## Task Dependency Graph

```
Task 1 (baseline + readability ON)
  ↓
  ├── Task 2 (URL filter)      ─┐
  ├── Task 3 (page-type)        ├── Track A (parallel, independent)
  ├── Task 4 (templates)       ─┘
  │
  ├── Task 5 (render worker)   ─┐
  ├── Task 6 (render_mode)      ├── Track B (5+6 parallel, 7 depends on both)
  └──→ Task 7 (integration)    ─┘
       ↓
     Task 8 (rollout + validation)
```
