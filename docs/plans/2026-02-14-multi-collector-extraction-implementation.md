# Multi-Collector Architecture & Extraction Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Split the crawler's single Colly collector into a link collector + detail collector, adopt Colly's built-in extensions, and improve the extraction fallback chains.

**Architecture:** Two Colly collectors created via `Clone()` from a shared base. The link collector discovers URLs and detects articles via a lightweight heuristic. It hands article URLs to the detail collector via `detailCollector.Request()`. The detail collector runs the full extraction pipeline. Colly's `extensions` package replaces hand-rolled UA rotation, referer headers, and URL length filtering.

**Tech Stack:** Go 1.24+, Colly v2.3.0 (`extensions` package), goquery

---

### Task 1: Add `extensions` package to vendor

**Files:**
- Modify: `crawler/go.mod` (no changes needed — `extensions` is part of `colly/v2`)
- Verify: `crawler/vendor/github.com/gocolly/colly/v2/extensions/` exists after vendor

The `extensions` package ships with `colly/v2` but isn't vendored yet because it's unused.

**Step 1: Vendor the extensions package**

```bash
cd crawler && GOWORK=off go mod tidy && cd ..
task vendor
```

**Step 2: Verify the extensions package is available**

```bash
ls crawler/vendor/github.com/gocolly/colly/v2/extensions/
```

Expected: `referer.go`, `random_user_agent.go`, `url_length_filter.go` files present.

**Step 3: Commit**

```bash
git add crawler/vendor/github.com/gocolly/colly/v2/extensions/
git commit -m "chore(crawler): vendor colly extensions package"
```

---

### Task 2: Create article detector

**Files:**
- Create: `crawler/internal/crawler/article_detector.go`
- Create: `crawler/internal/crawler/article_detector_test.go`

This component decides whether a page is an article (for the link→detail handoff). It runs inside the link collector's `OnHTML("html")` callback, so it must be lightweight — no full content extraction.

**Step 1: Write the failing tests**

```go
// crawler/internal/crawler/article_detector_test.go
package crawler

import (
	"regexp"
	"testing"
)

func TestIsArticleURL(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		url      string
		patterns []*regexp.Regexp
		expected bool
	}{
		{
			name:     "matches explicit pattern",
			url:      "https://example.com/news/2026/02/story",
			patterns: []*regexp.Regexp{regexp.MustCompile(`/news/`)},
			expected: true,
		},
		{
			name:     "no match with explicit patterns",
			url:      "https://example.com/about",
			patterns: []*regexp.Regexp{regexp.MustCompile(`/news/`)},
			expected: false,
		},
		{
			name:     "date pattern in URL (no explicit patterns)",
			url:      "https://example.com/2026/02/14/headline",
			patterns: nil,
			expected: true,
		},
		{
			name:     "article path segment (no explicit patterns)",
			url:      "https://example.com/article/headline",
			patterns: nil,
			expected: true,
		},
		{
			name:     "news path segment (no explicit patterns)",
			url:      "https://example.com/news/headline",
			patterns: nil,
			expected: true,
		},
		{
			name:     "story path segment (no explicit patterns)",
			url:      "https://example.com/story/headline",
			patterns: nil,
			expected: true,
		},
		{
			name:     "homepage (no explicit patterns)",
			url:      "https://example.com/",
			patterns: nil,
			expected: false,
		},
		{
			name:     "category page (no explicit patterns)",
			url:      "https://example.com/sports",
			patterns: nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			result := isArticleURL(tt.url, tt.patterns)
			if result != tt.expected {
				t.Errorf("isArticleURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestCompileArticlePatterns(t *testing.T) {
	t.Helper()

	t.Run("compiles valid patterns", func(t *testing.T) {
		t.Helper()

		patterns := []string{`/news/\d+`, `/article/`}
		result := compileArticlePatterns(patterns)
		if len(result) != 2 {
			t.Errorf("expected 2 compiled patterns, got %d", len(result))
		}
	})

	t.Run("skips invalid patterns", func(t *testing.T) {
		t.Helper()

		patterns := []string{`/news/`, `[invalid`}
		result := compileArticlePatterns(patterns)
		if len(result) != 1 {
			t.Errorf("expected 1 compiled pattern (invalid skipped), got %d", len(result))
		}
	})

	t.Run("returns nil for empty input", func(t *testing.T) {
		t.Helper()

		result := compileArticlePatterns(nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})
}
```

**Step 2: Run tests to verify they fail**

```bash
cd crawler && GOWORK=off go test ./internal/crawler/ -run TestIsArticleURL -v
```

Expected: FAIL — functions not defined.

**Step 3: Write the implementation**

```go
// crawler/internal/crawler/article_detector.go
package crawler

import (
	"regexp"
	"strings"
)

// Built-in URL heuristics for article detection when no explicit patterns are configured.
// These are common URL patterns found on news and content sites.
var defaultArticleURLPatterns = []*regexp.Regexp{
	// Date-based paths: /2026/02/14/headline or /2026/02/headline
	regexp.MustCompile(`/\d{4}/\d{2}/\d{2}/`),
	regexp.MustCompile(`/\d{4}/\d{2}/[^/]+$`),
	// Common article path segments
	regexp.MustCompile(`/(?:article|story|post|news)/[^/]+`),
	// Slug-like paths with 3+ hyphenated words (e.g., /this-is-a-headline)
	regexp.MustCompile(`/[a-z0-9]+-[a-z0-9]+-[a-z0-9]+-[a-z0-9]+`),
}

// Non-article path patterns — pages that are clearly not articles.
var nonArticlePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)/(?:login|signup|register|search|contact|about|privacy|terms|tag|category|author|page)/?\??`),
	regexp.MustCompile(`(?i)\.(pdf|xml|json|rss|atom|css|js|png|jpg|gif|svg|ico)$`),
}

// isArticleURL checks if a URL looks like an article page.
// If explicit patterns are provided, only those are used.
// Otherwise, falls back to built-in heuristics.
func isArticleURL(pageURL string, explicitPatterns []*regexp.Regexp) bool {
	// Explicit patterns take priority — if configured, only they decide
	if len(explicitPatterns) > 0 {
		for _, p := range explicitPatterns {
			if p.MatchString(pageURL) {
				return true
			}
		}
		return false
	}

	// Filter out obvious non-article URLs
	for _, p := range nonArticlePatterns {
		if p.MatchString(pageURL) {
			return false
		}
	}

	// Check built-in heuristics
	for _, p := range defaultArticleURLPatterns {
		if p.MatchString(pageURL) {
			return true
		}
	}

	return false
}

// isArticlePage checks page-level signals that indicate an article.
// Used as a secondary check on the HTML content (og:type, JSON-LD).
// Returns true if the page has strong article signals regardless of URL pattern.
func isArticlePage(ogType string, hasNewsArticleJSONLD bool) bool {
	if strings.EqualFold(ogType, "article") {
		return true
	}
	return hasNewsArticleJSONLD
}

// compileArticlePatterns compiles string patterns into regexps, skipping invalid ones.
func compileArticlePatterns(patterns []string) []*regexp.Regexp {
	if len(patterns) == 0 {
		return nil
	}
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		result = append(result, re)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
```

**Step 4: Run tests to verify they pass**

```bash
cd crawler && GOWORK=off go test ./internal/crawler/ -run "TestIsArticleURL|TestCompileArticlePatterns" -v
```

Expected: PASS

**Step 5: Lint**

```bash
cd crawler && GOWORK=off golangci-lint run ./internal/crawler/article_detector.go ./internal/crawler/article_detector_test.go
```

**Step 6: Commit**

```bash
git add crawler/internal/crawler/article_detector.go crawler/internal/crawler/article_detector_test.go
git commit -m "feat(crawler): add article detection heuristic for multi-collector"
```

---

### Task 3: Add `ArticleURLPatterns` to source config

**Files:**
- Modify: `crawler/internal/config/types/source.go:8-33` — add field
- Modify: `crawler/internal/config/types/selectors.go:92-104` — relax validation

**Step 1: Add field to Source struct**

In `crawler/internal/config/types/source.go`, add the `ArticleURLPatterns` field to the `Source` struct:

```go
// ArticleURLPatterns are regex patterns identifying article URLs.
// Used by the link collector to decide which URLs to pass to the detail collector.
// Optional — if empty, uses heuristic detection (og:type, JSON-LD, URL patterns).
ArticleURLPatterns []string `yaml:"article_url_patterns"`
```

Add it after the `Rules` field.

**Step 2: Relax ArticleSelectors validation**

In `crawler/internal/config/types/selectors.go`, change `ArticleSelectors.Validate()` to make all fields optional:

```go
// Validate validates the article selectors.
// All selectors are optional — auto-detection fills gaps when selectors are missing.
func (s *ArticleSelectors) Validate() error {
	return nil
}
```

**Step 3: Run existing tests**

```bash
cd crawler && GOWORK=off go test ./...
```

Expected: PASS (this is a relaxation, not a tightening — existing code should still work).

**Step 4: Lint**

```bash
cd crawler && GOWORK=off golangci-lint run
```

**Step 5: Commit**

```bash
git add crawler/internal/config/types/source.go crawler/internal/config/types/selectors.go
git commit -m "feat(crawler): add ArticleURLPatterns to source config, relax selector validation"
```

---

### Task 4: Refactor collector into multi-collector pattern

This is the core change. Split `setupCollector` and `setupCallbacks` to create link + detail collectors.

**Files:**
- Modify: `crawler/internal/crawler/crawler.go:149-177` — add `detailCollector` field
- Modify: `crawler/internal/crawler/collector.go` — split into base/link/detail setup, adopt extensions, fix keep-alives, delete manual UA/referer code
- Modify: `crawler/internal/crawler/start.go` — use link collector for Visit, Wait on both
- Modify: `crawler/internal/crawler/processing.go` — add post-extraction validation
- Modify: `crawler/internal/crawler/link_handler.go` — remove manual URL length check and referer context

**Step 1: Add detailCollector field to Crawler struct**

In `crawler/internal/crawler/crawler.go`, add after the `collector` field (line 152):

```go
detailCollector *colly.Collector // Detail collector for content extraction (multi-collector pattern)
```

**Step 2: Refactor collector.go — multi-collector setup**

Replace `setupCollector` to create base → link (clone) → detail (clone):

Key changes:
- `setupCollector` creates base options, then clones into `c.collector` (link) and `c.detailCollector`
- Apply `extensions.RandomUserAgent()`, `extensions.Referer()`, `extensions.URLLengthFilter()` to both collectors
- Delete `randomUserAgents` var
- Fix `DisableKeepAlives: true` in `configureTransport()`
- The link collector gets `OnHTML("a[href]")` for link discovery and `OnHTML("html")` for article detection
- The detail collector gets `OnHTML("html")` for full extraction
- Remove manual UA rotation from `requestCallback()` — handled by extension
- Remove manual referer from `requestCallback()` — handled by extension

The `setupCallbacks` method is split into `setupLinkCallbacks` and `setupDetailCallbacks`.

In `setupLinkCallbacks`, the `OnHTML("html")` callback does a lightweight article check:
1. Extract `og:type` meta tag (`e.ChildAttr("meta[property='og:type']", "content")`)
2. Check for JSON-LD `@type: "NewsArticle"` (quick string search on `e.DOM.Find("script[type='application/ld+json']").Text()`)
3. Call `isArticleURL()` with the page URL and compiled source patterns
4. If any check passes: `c.detailCollector.Request("GET", e.Request.URL.String(), nil, e.Request.Ctx, nil)`

In `setupDetailCallbacks`, `OnHTML("html")` calls `c.ProcessHTML(e)` (existing extraction logic).

**Step 3: Update start.go — dual collector Wait**

In `Start()`, change:
- `c.collector.Visit(source.URL)` stays (link collector visits the start URL)
- `c.collector.Wait()` becomes waiting on both collectors:

```go
go func() {
    c.collector.Wait()       // Wait for link collector
    c.detailCollector.Wait() // Then wait for detail collector
    close(waitDone)
}()
```

- `setupInitialPageTracking` stays on link collector (it tracks the start page)

**Step 4: Update link_handler.go — remove manual URL length + referer**

Remove the manual URL length check in `HandleLink()` (lines 65-73) — now handled by `extensions.URLLengthFilter`.

Remove the referer context put in `visitWithRetries()` (line 138):
```go
// DELETE this line:
e.Request.Ctx.Put(refererCtxKey, e.Request.URL.String())
```

The `extensions.Referer()` extension handles this automatically.

**Step 5: Update processing.go — add post-extraction validation**

In `ProcessHTML`, after `processor.Process()` succeeds, add a validation check. Actually, the better place is in `RawContentService.Process()` — add validation after extraction but before indexing.

In `crawler/internal/content/rawcontent/service.go`, after `ExtractRawContent()` call (line 70-77), add:

```go
// Validate extracted content before indexing
if rawData.Title == "" && rawData.RawText == "" {
    s.logger.Debug("Skipping page with no extractable content",
        infralogger.String("url", sourceURL))
    return nil
}

minWordCount := 50
wordCount := len(strings.Fields(rawData.RawText))
if wordCount < minWordCount {
    s.logger.Debug("Skipping page with insufficient content",
        infralogger.String("url", sourceURL),
        infralogger.Int("word_count", wordCount),
        infralogger.Int("min_word_count", minWordCount))
    return nil
}
```

**Step 6: Run all tests**

```bash
cd crawler && GOWORK=off go test ./...
```

**Step 7: Lint**

```bash
cd crawler && GOWORK=off golangci-lint run
```

**Step 8: Commit**

```bash
git add crawler/internal/crawler/ crawler/internal/content/rawcontent/service.go
git commit -m "feat(crawler): implement multi-collector pattern with Colly extensions

Split single collector into link collector (discovery) and detail
collector (extraction) following Colly's multi-collector best practice.

- Link collector discovers URLs and detects articles via heuristic
- Detail collector runs full extraction pipeline on article pages only
- Replace hand-rolled UA rotation with extensions.RandomUserAgent()
- Replace manual referer with extensions.Referer()
- Replace manual URL length check with extensions.URLLengthFilter()
- Fix DisableKeepAlives: true per Colly best practices
- Add post-extraction validation (min word count, non-empty content)"
```

---

### Task 5: Improve extraction fallback chains

**Files:**
- Modify: `crawler/internal/content/rawcontent/extractor.go` — improved title, date, author fallbacks
- Modify: `crawler/internal/content/rawcontent/extractor_test.go` — tests for new fallbacks

**Step 1: Write tests for improved title extraction**

Add to `extractor_test.go`:

```go
func TestExtractTitleWithJSONLDFallback(t *testing.T) {
	t.Helper()

	// Test that JSON-LD headline is used when selector and og:title are empty
	// This requires creating a mock HTMLElement — use the existing test patterns
}
```

Note: Testing with real `colly.HTMLElement` requires HTTP fixtures. Write the tests using the extractor's internal helper functions where possible. For integration-level tests, use the proxy fixture approach documented in `nc-http-proxy/README.md`.

**Step 2: Update title extraction**

In `extractor.go`, modify `extractTitle()` to add JSON-LD headline as a fallback after selector but before og:title:

```go
func extractTitle(e *colly.HTMLElement, selector string) string {
	// Try selector if provided
	if selector != "" {
		title := extractText(e, selector)
		if title != "" {
			return title
		}
	}

	// Try JSON-LD headline (often cleanest title on news sites)
	jsonldTitle := extractJSONLDHeadline(e)
	if jsonldTitle != "" {
		return jsonldTitle
	}

	// Try OG title
	ogTitle := extractMeta(e, "og:title")
	if ogTitle != "" {
		return ogTitle
	}

	// Try title tag
	title := e.ChildText("title")
	if title != "" {
		return strings.TrimSpace(title)
	}

	// Try h1 as fallback
	h1 := e.ChildText("h1")
	if h1 != "" {
		return strings.TrimSpace(h1)
	}

	return ""
}
```

Add helper:

```go
// extractJSONLDHeadline extracts the headline from JSON-LD NewsArticle/Article schema.
func extractJSONLDHeadline(e *colly.HTMLElement) string {
	var headline string
	e.DOM.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		if headline != "" {
			return
		}
		jsonText := strings.TrimSpace(s.Text())
		if jsonText == "" {
			return
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
			return
		}
		typeVal, _ := data["@type"].(string)
		if typeVal != "NewsArticle" && typeVal != "Article" {
			return
		}
		if h, ok := data["headline"].(string); ok && h != "" {
			headline = strings.TrimSpace(h)
		}
	})
	return headline
}
```

**Step 3: Update published date extraction**

In `extractMetadata()`, expand the date fallback chain after the existing `article:published_time` and `article:published` checks:

```go
// Try JSON-LD datePublished
if data.PublishedDate == nil && len(data.JSONLDData) > 0 {
	if dateStr, ok := data.JSONLDData["jsonld_date_published"].(string); ok && dateStr != "" {
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			data.PublishedDate = &t
		}
	}
}

// Try <time datetime="..."> element
if data.PublishedDate == nil {
	if dateStr := e.ChildAttr("time[datetime]", "datetime"); dateStr != "" {
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			data.PublishedDate = &t
		}
	}
}

// Try common CSS class selectors for published date
if data.PublishedDate == nil {
	dateSelectors := []string{".published-date", ".post-date", ".entry-date", ".article-date"}
	for _, sel := range dateSelectors {
		dateText := e.ChildAttr(sel+" time", "datetime")
		if dateText == "" {
			dateText = e.ChildText(sel)
		}
		if dateText != "" {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(dateText)); err == nil {
				data.PublishedDate = &t
				break
			}
		}
	}
}
```

**Step 4: Update author extraction**

In `extractMetadata()`, expand author extraction after the existing `extractMeta(e, "author")`:

```go
// Try JSON-LD author
if data.Author == "" && len(data.JSONLDData) > 0 {
	if author, ok := data.JSONLDData["jsonld_author"].(string); ok && author != "" {
		data.Author = author
	}
}

// Try rel="author" link
if data.Author == "" {
	data.Author = strings.TrimSpace(e.ChildText("a[rel='author']"))
}

// Try common byline selectors
if data.Author == "" {
	bylineSelectors := []string{".byline", ".author", ".post-author", ".article-author"}
	for _, sel := range bylineSelectors {
		author := strings.TrimSpace(e.ChildText(sel))
		if author != "" {
			data.Author = author
			break
		}
	}
}
```

**Step 5: Run tests**

```bash
cd crawler && GOWORK=off go test ./internal/content/rawcontent/ -v
```

**Step 6: Lint**

```bash
cd crawler && GOWORK=off golangci-lint run ./internal/content/rawcontent/
```

**Step 7: Commit**

```bash
git add crawler/internal/content/rawcontent/
git commit -m "feat(crawler): improve extraction fallback chains for title, date, author

- Title: selector → JSON-LD headline → og:title → <title> → <h1>
- Date: selector → JSON-LD → article:published_time → time[datetime] → CSS classes
- Author: selector → JSON-LD → meta → rel=author → CSS classes"
```

---

### Task 6: Integration verification

**Files:** No new files — verification only.

**Step 1: Run full test suite**

```bash
cd crawler && GOWORK=off go test ./... -count=1
```

Expected: All tests pass.

**Step 2: Run full linter**

```bash
cd crawler && GOWORK=off golangci-lint run
```

Expected: No new violations.

**Step 3: Build the service**

```bash
cd crawler && GOWORK=off go build -o /dev/null .
```

Expected: Builds successfully.

**Step 4: Docker build test**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build crawler
```

Expected: Image builds successfully.

**Step 5: Smoke test with dev environment**

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d crawler
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f crawler --tail=50
```

Check logs for:
- "Collector configured" log line (confirms setup completed)
- No panics or initialization errors
- Link collector and detail collector IDs in debug output

**Step 6: Final commit (if any fixups needed)**

```bash
git add -A
git commit -m "fix(crawler): integration fixups for multi-collector"
```

---

## Task Dependency Graph

```
Task 1 (vendor extensions)
  ↓
Task 2 (article detector) ←─── Task 3 (source config changes)
  ↓                                ↓
Task 4 (multi-collector refactor) ←┘
  ↓
Task 5 (extraction fallbacks)
  ↓
Task 6 (integration verification)
```

Tasks 2 and 3 can be done in parallel. Task 4 depends on both. Task 5 can technically be done independently but should come after Task 4 to avoid merge conflicts in the extractor.
