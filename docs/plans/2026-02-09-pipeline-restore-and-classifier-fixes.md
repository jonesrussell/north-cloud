# Pipeline Restore & Classifier Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restore end-to-end crime article flow from North Cloud to Streetcode by deploying pending fixes, resetting the publisher cursor, and improving classifier accuracy for content_type and crime relevance detection.

**Architecture:** Three-phase approach: (1) Deploy already-written fixes and reset publisher state to restore immediate flow, (2) Fix classifier content_type detection that incorrectly labels articles as "page", (3) Fix crime relevance detection that marks crime articles as "not_crime". Each phase is independently valuable.

**Tech Stack:** Go 1.25+, PostgreSQL, Elasticsearch, Redis Pub/Sub, Docker, GitHub Actions CI/CD

---

## Root Cause Summary

Streetcode stopped receiving crime articles on Feb 7 due to three layered failures:

1. **Publisher location channel bug** (fixed in `9ad18c8`, not yet deployed at time of failure): `GenerateLocationChannels()` routed ALL articles with location data to `crime:*` channels regardless of crime relevance, flooding Streetcode with non-crime articles.

2. **Slug SQL error** (Feb 7 17:49): When actual `core_street_crime` articles arrived, Streetcode's `CrimeArticleProcessor` hit `Field 'slug' doesn't have a default value` due to a code version mismatch (release 10 vs current release 28 which has the fix).

3. **Post-deploy state**: After publisher restart with the fix (Feb 9 01:04 UTC), crime:* channels are correctly filtered but the cursor has moved past all existing articles. No new crime articles flow because recent crawled content is classified as `content_type: "page"` (skipped by publisher) or `street_crime_relevance: "not_crime"` despite having crime keywords.

---

## Phase 1: Deploy & Restore Pipeline (Operational)

### Task 1: Deploy Publisher + MCP Fixes to Production

**Files already modified (local, uncommitted):**
- `publisher/internal/router/service.go` - 3 Debug logs changed to Info
- `docker-compose.prod.yml` - MCP server command changed to `sleep infinity`

**Step 1: Verify local changes are correct**

Run:
```bash
cd /home/fsd42/dev/north-cloud
git diff publisher/internal/router/service.go
git diff docker-compose.prod.yml
```

Expected: See the 3 log level changes (Debug→Info) and MCP command change.

**Step 2: Lint the publisher**

Run:
```bash
cd /home/fsd42/dev/north-cloud/publisher && golangci-lint run
```

Expected: 0 issues.

**Step 3: Run publisher tests**

Run:
```bash
cd /home/fsd42/dev/north-cloud/publisher && go test ./...
```

Expected: All tests pass.

**Step 4: Commit the changes**

```bash
cd /home/fsd42/dev/north-cloud
git add publisher/internal/router/service.go docker-compose.prod.yml
git commit -m "fix(publisher): add INFO logs for publish events and fix MCP stdio container

Publisher: Change 3 Debug-level logs to Info for production visibility:
- 'Published article to channel' now includes article title
- 'Processing articles batch' visible in production logs
- 'No indexes discovered' visible in production logs

MCP: Change container command from running the binary (which exits
immediately due to stdin EOF) to sleep infinity. The stdio-based MCP
binary is invoked on-demand via docker exec -i.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

**Step 5: Push and monitor CI/CD deploy**

```bash
git push -u origin main
gh run watch
```

Expected: Deploy workflow detects publisher change, builds image, deploys to production.

**Step 6: Verify deployment**

```bash
ssh jones@northcloud.biz 'docker ps --format "{{.Names}} {{.Status}}" | grep -E "publisher|mcp"'
```

Expected: Publisher healthy, MCP container running (not restarting).

---

### Task 2: Reset Publisher Cursor and Crime Channel History

After deployment completes, reset the publisher state so existing crime articles get re-published to crime:* channels.

**Step 1: Check current cursor state**

```bash
ssh jones@northcloud.biz 'docker exec north-cloud-postgres-publisher-1 psql -U postgres -d publisher -c "SELECT last_sort, updated_at FROM publisher_cursor WHERE id = 1;"'
```

**Step 2: Count existing crime:* publish history**

```bash
ssh jones@northcloud.biz 'docker exec north-cloud-postgres-publisher-1 psql -U postgres -d publisher -c "SELECT channel_name, COUNT(*) FROM publish_history WHERE channel_name LIKE '\''crime:%'\'' GROUP BY channel_name ORDER BY COUNT(*) DESC;"'
```

**Step 3: Clear crime:* publish history and reset cursor**

```bash
ssh jones@northcloud.biz 'docker exec north-cloud-postgres-publisher-1 psql -U postgres -d publisher -c "
BEGIN;
DELETE FROM publish_history WHERE channel_name LIKE '\''crime:%'\'';
UPDATE publisher_cursor SET last_sort = '\''[]'\'', updated_at = NOW() WHERE id = 1;
COMMIT;
SELECT '\''Cursor reset. Crime history cleared.'\'' as status;
"'
```

**Step 4: Restart publisher to reload cursor**

```bash
ssh jones@northcloud.biz 'cd /opt/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml restart publisher'
```

**Step 5: Monitor publisher logs for crime:* channel activity**

```bash
ssh jones@northcloud.biz 'docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs -f publisher 2>&1 | grep -i "crime\|Published article\|Processing articles"'
```

Expected: Within 5 minutes, see "Published article to channel" entries for crime:homepage, crime:category:*, crime:province:*, crime:canada channels.

**Step 6: Verify Streetcode receives articles**

```bash
ssh deployer@streetcode.net 'tail -f /home/deployer/streetcode-laravel/current/storage/logs/laravel.log | grep -i "Article processed\|core_street_crime\|Skipping"'
```

Expected: Mix of "Article processed" (core_street_crime articles) and "Skipping non-core-crime" (not_crime articles filtered by CrimeArticleProcessor). If only "Skipping" entries appear, the crime relevance classifier issue (Phase 3) is the bottleneck.

---

### Task 3: Verify Streetcode Subscriber Health

**Step 1: Confirm subscriber processes are running current release**

```bash
ssh deployer@streetcode.net 'readlink /home/deployer/streetcode-laravel/current; echo "---"; ps aux | grep "articles:subscribe" | grep -v grep'
```

Expected: `current` → `releases/28`, subscriber PIDs running.

**Step 2: Kill old subscriber processes from previous releases**

If PID 917051 (started at 01:21, before release 28 at 02:41) is still running:

```bash
ssh deployer@streetcode.net 'kill 917051 2>/dev/null; echo "Killed old subscriber"'
```

**Step 3: Verify slug fix works in current release**

The Article model (`App\Models\Article`) has a `creating` event that auto-generates slug. The `ArticleIngestionService.ingest()` also sets slug. Confirm no more slug errors after publisher starts sending:

```bash
ssh deployer@streetcode.net 'tail -f /home/deployer/streetcode-laravel/current/storage/logs/laravel.log | grep -i "slug\|error\|failed"'
```

Expected: No "Field 'slug' doesn't have a default value" errors.

---

## Phase 2: Fix Classifier Content Type Detection

### Task 4: Fix URL Exclusion Pattern Over-Matching

The content_type classifier (`classifier/internal/classifier/content_type.go`) has URL exclusion patterns that are too broad. URLs containing `/news`, `/articles`, `/blog`, `/local-news`, etc. are immediately classified as "page" even when they are single articles within those sections.

**Files:**
- Modify: `classifier/internal/classifier/content_type.go` (lines 28-45, the `pageURLPatterns` slice)
- Modify: `classifier/internal/classifier/content_type_test.go`

**Step 1: Write failing tests for article URLs within news sections**

Add test cases to `content_type_test.go`:

```go
// Test: URLs under /news/ with article slug should NOT be excluded
func TestContentTypeClassifier_ArticleInNewsSection(t *testing.T) {
    t.Helper()
    tests := []struct {
        name string
        url  string
    }{
        {"news section article", "https://example.com/news/six-men-charged-drug-bust"},
        {"local-news article", "https://example.com/local-news/mayor-announces-policy"},
        {"blog post", "https://example.com/blog/my-post-title"},
        {"articles section", "https://example.com/articles/some-article-slug"},
        {"ontario-news article", "https://example.com/ontario-news/crime-report-2026"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            raw := &domain.RawContent{
                URL:             tt.url,
                Title:           "Test Article Title",
                RawText:         strings.Repeat("word ", 300), // 300 words
                PublishedDate:   timePtr(time.Now()),
                MetaDescription: "Test description",
                WordCount:       300,
            }
            result := classifier.ClassifyContentType(raw)
            assert.Equal(t, "article", result.ContentType,
                "URL %s should not be excluded as page", tt.url)
        })
    }
}

// Test: Actual listing/index URLs should still be excluded
func TestContentTypeClassifier_ListingURLsStillExcluded(t *testing.T) {
    t.Helper()
    tests := []struct {
        name string
        url  string
    }{
        {"news index", "https://example.com/news"},
        {"news trailing slash", "https://example.com/news/"},
        {"category page", "https://example.com/category/crime"},
        {"search results", "https://example.com/search?q=crime"},
        {"paginated", "https://example.com/stories?page=2"},
        {"homepage", "https://example.com/"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            raw := &domain.RawContent{
                URL:   tt.url,
                Title: "Some Title",
            }
            result := classifier.ClassifyContentType(raw)
            assert.Equal(t, "page", result.ContentType,
                "URL %s should be excluded as page", tt.url)
        })
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./internal/classifier/ -run TestContentTypeClassifier_ArticleInNewsSection -v
```

Expected: FAIL - articles in `/news/slug` are currently classified as "page".

**Step 3: Fix the URL exclusion logic**

In `content_type.go`, change `pageURLPatterns` from substring matching to **exact path or path-with-trailing-slash** matching. The key insight: `/news` (index page) should be excluded, but `/news/some-article-slug` (article within news section) should NOT be excluded.

Replace the current `isExcludedURL()` function logic:

```go
// isExcludedURL checks if a URL is a known non-article page.
// Matches exact paths (e.g., "/news", "/news/") but NOT paths with
// additional segments (e.g., "/news/article-slug").
func isExcludedURL(rawURL string) bool {
    parsed, err := url.Parse(strings.ToLower(rawURL))
    if err != nil {
        return false
    }

    path := strings.TrimRight(parsed.Path, "/")

    // Check pagination query params (always exclude)
    for _, param := range paginationParams {
        if strings.Contains(parsed.RawQuery, param) {
            return true
        }
    }

    // Check homepage
    if path == "" || path == "/" {
        return true
    }

    // Check exact section paths (exclude /news but not /news/article-slug)
    for _, pattern := range sectionPaths {
        if path == pattern {
            return true
        }
    }

    // Check always-excluded paths (auth, classifieds, etc.)
    for _, pattern := range alwaysExcludedPaths {
        if strings.HasPrefix(path, pattern) {
            return true
        }
    }

    return false
}
```

With separate pattern lists:

```go
// sectionPaths are excluded ONLY when they are the exact path (index pages).
// Articles WITHIN these sections (e.g., /news/article-slug) pass through.
var sectionPaths = []string{
    "/news", "/articles", "/stories", "/posts", "/blog",
    "/ontario-news", "/local-news", "/breaking-news",
    "/category", "/categories", "/browse", "/listings",
    "/search", "/results",
}

// alwaysExcludedPaths are excluded regardless of sub-paths.
var alwaysExcludedPaths = []string{
    "/account", "/login", "/signin", "/signup", "/register",
    "/classifieds", "/classified", "/ads", "/advertisements",
    "/directory", "/submissions",
}

// paginationParams indicate paginated content.
var paginationParams = []string{
    "page=", "p=", "pagenum=", "offset=", "start=", "pg=",
}
```

**Step 4: Run tests to verify they pass**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./internal/classifier/ -run "TestContentTypeClassifier_ArticleInNewsSection|TestContentTypeClassifier_ListingURLsStillExcluded" -v
```

Expected: All PASS.

**Step 5: Run full test suite and lint**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./... && golangci-lint run
```

Expected: All pass, 0 lint issues.

**Step 6: Commit**

```bash
git add classifier/internal/classifier/content_type.go classifier/internal/classifier/content_type_test.go
git commit -m "fix(classifier): allow articles within news section URLs

URL exclusion patterns were too broad: /news, /articles, /blog etc.
matched as substrings, causing single articles within those sections
(e.g., /news/article-slug) to be classified as 'page'.

Now uses exact path matching for section URLs so only index pages
(/news, /news/) are excluded. Articles with additional path segments
pass through to heuristic classification.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Fix Missing PublishedDate Fallback in Content Type Heuristics

The heuristic classifier (Strategy 3) requires ALL of: word count >= 200, title, published_date, and description. If `published_date` is missing (crawler didn't extract it), the article defaults to "page" even with strong article signals.

**Files:**
- Modify: `classifier/internal/classifier/content_type.go` (the `classifyByHeuristics()` function)
- Modify: `classifier/internal/classifier/content_type_test.go`

**Step 1: Write failing test**

```go
func TestContentTypeClassifier_ArticleWithoutDate(t *testing.T) {
    t.Helper()
    raw := &domain.RawContent{
        URL:             "https://example.com/some-article",
        Title:           "Six men now charged in 2024 multi-city drug bust",
        RawText:         strings.Repeat("The police arrested several suspects. ", 100), // 600+ words
        PublishedDate:   nil, // Missing!
        MetaDescription: "Six men have been charged in connection with a drug bust.",
        WordCount:       600,
    }
    result := classifier.ClassifyContentType(raw)
    assert.Equal(t, "article", result.ContentType)
    // Confidence should be lower than with date
    assert.Less(t, result.TypeConfidence, 0.75)
}
```

**Step 2: Run test to verify it fails**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./internal/classifier/ -run TestContentTypeClassifier_ArticleWithoutDate -v
```

Expected: FAIL - currently classified as "page" due to missing date.

**Step 3: Implement relaxed heuristic**

In `classifyByHeuristics()`, add a secondary check: if word count is high (>= 300) AND title is present AND description is present, classify as article with reduced confidence (0.65) even without published_date.

```go
func (c *ContentTypeClassifier) classifyByHeuristics(raw *domain.RawContent) *contentTypeResult {
    hasTitle := raw.Title != ""
    hasDate := raw.PublishedDate != nil
    hasDescription := raw.MetaDescription != "" || raw.OGDescription != ""
    wordCount := raw.WordCount
    if wordCount == 0 {
        wordCount = len(strings.Fields(raw.RawText))
    }

    // Full signal: all 4 present
    if wordCount >= minArticleWordCount && hasTitle && hasDate && hasDescription {
        return &contentTypeResult{
            contentType: ContentTypeArticle,
            confidence:  heuristicArticleConfidence, // 0.75
            method:      "heuristic",
        }
    }

    // Relaxed: high word count + title + description (no date)
    if wordCount >= relaxedMinWordCount && hasTitle && hasDescription && !hasDate {
        return &contentTypeResult{
            contentType: ContentTypeArticle,
            confidence:  relaxedHeuristicConfidence, // 0.65
            method:      "heuristic_relaxed",
        }
    }

    return nil
}
```

Add constants:

```go
const (
    minArticleWordCount        = 200
    relaxedMinWordCount        = 300
    heuristicArticleConfidence = 0.75
    relaxedHeuristicConfidence = 0.65
)
```

**Step 4: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./internal/classifier/ -run TestContentTypeClassifier -v
```

Expected: All PASS including the new test.

**Step 5: Lint and full test suite**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./... && golangci-lint run
```

**Step 6: Commit**

```bash
git add classifier/internal/classifier/content_type.go classifier/internal/classifier/content_type_test.go
git commit -m "fix(classifier): classify long articles without published_date

Articles with >= 300 words, title, and description were being classified
as 'page' when published_date was missing (crawler didn't extract it).

Added relaxed heuristic: if word count is high enough with title and
description present, classify as article with reduced confidence (0.65
vs 0.75 for full signals).

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

## Phase 3: Fix Crime Relevance Detection

### Task 6: Understand the Crime Classification Gap

The crime classifier (`crime_rules.go`) requires **authority indicators** (police, RCMP, court, etc.) near crime keywords in the SAME regex match. Topic detection (`topic.go`) uses simple keyword presence. This creates a gap where articles have `crime_types: ["violent_crime"]` but `street_crime_relevance: "not_crime"`.

**Example article that fails:**
- Title: "Repeat offender among two arrested in store robbery"
- ES crime data: `crime_types: ["violent_crime", "property_crime"]`, `street_crime_relevance: "not_crime"`, `confidence: 0.5`
- The title has "arrested" (authority) and "robbery" (crime) but the regex patterns in `crime_rules.go` may not match this exact combination.

**Files to review:**
- `classifier/internal/classifier/crime_rules.go` - pattern definitions
- `classifier/internal/classifier/crime_test.go` - existing tests

**Step 1: Audit which crime patterns would match "Repeat offender among two arrested in store robbery"**

Read `crime_rules.go` and trace through each pattern category:
- Violent crime: patterns require `(murder|shooting|stabbing|assault)` - "robbery" is not in violent patterns
- Property crime: patterns require `(theft|stolen|burglary|break.in|arson)` - "robbery" is not in property patterns
- Drug crime: doesn't apply
- Court outcomes: patterns require `(sentenced|convicted|found guilty)` - "arrested" is not a court outcome

**The gap**: "robbery" and "arrested" are not in the crime regex patterns at all! The topic classifier picks them up via keyword matching, but the crime regex misses them.

**Step 2: Identify missing crime keyword patterns**

Review `crime_rules.go` patterns and list crime keywords that should trigger `core_street_crime` but are missing:

Missing from violent crime patterns:
- `robbery` / `robbed` / `rob`
- `armed robbery`
- `carjacking` / `carjacked`
- `kidnapping` / `kidnapped` / `abducted`
- `hostage`

Missing from property crime patterns:
- `robbery` (also overlaps with violent)
- `shoplifting` / `shoplift`
- `looting`

Missing authority indicators:
- `arrested` (currently missing!)
- `custody`
- `detained`
- `apprehended`
- `wanted`
- `manhunt`

**Step 3: Document the fix approach**

Two complementary fixes:
1. Add missing keywords to crime regex patterns in `crime_rules.go`
2. Add "arrested" to the authority indicators list

---

### Task 7: Add Missing Crime Keywords to Rule Patterns

**Files:**
- Modify: `classifier/internal/classifier/crime_rules.go`
- Modify: `classifier/internal/classifier/crime_test.go`

**Step 1: Write failing tests for missing patterns**

```go
func TestCrimeRules_MissingPatterns(t *testing.T) {
    t.Helper()
    tests := []struct {
        name     string
        title    string
        body     string
        wantRel  string
    }{
        {
            name:    "robbery with arrest",
            title:   "Repeat offender among two arrested in store robbery",
            body:    "Police have arrested two suspects in connection with a robbery.",
            wantRel: "core_street_crime",
        },
        {
            name:    "armed robbery",
            title:   "Armed robbery at downtown convenience store",
            body:    "RCMP are investigating an armed robbery that occurred last night.",
            wantRel: "core_street_crime",
        },
        {
            name:    "carjacking",
            title:   "Police arrest suspect in violent carjacking incident",
            body:    "A man was arrested after a carjacking in the parking lot.",
            wantRel: "core_street_crime",
        },
        {
            name:    "kidnapping",
            title:   "Man charged with kidnapping after Amber Alert",
            body:    "Police have charged a man with kidnapping a child.",
            wantRel: "core_street_crime",
        },
        {
            name:    "arrested as authority indicator",
            title:   "Two arrested after shooting in downtown area",
            body:    "Two suspects were arrested following a shooting.",
            wantRel: "core_street_crime",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            classifier := NewCrimeRulesClassifier()
            result := classifier.Classify(tt.title, tt.body)
            assert.Equal(t, tt.wantRel, result.Relevance,
                "Expected %s for: %s", tt.wantRel, tt.title)
        })
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./internal/classifier/ -run TestCrimeRules_MissingPatterns -v
```

Expected: FAIL - these patterns are not currently matched.

**Step 3: Add missing patterns to crime_rules.go**

Add to the authority indicators:
```go
var authorityIndicators = `(police|rcmp|opp|court|judge|arrest|arrested|charged|convicted|suspect|custody|detained|apprehended|wanted|manhunt|investigation|investigat)`
```

Add robbery/carjacking/kidnapping patterns to the appropriate category:

```go
// In violent crime patterns, add:
{
    pattern:    regexp.MustCompile(`(?i)(robbery|robbed|armed robbery|carjack|kidnap|abduct|hostage).*` + authorityIndicators),
    confidence: 0.85,
    crimeType:  "violent_crime",
},
{
    pattern:    regexp.MustCompile(`(?i)` + authorityIndicators + `.*(robbery|robbed|armed robbery|carjack|kidnap|abduct|hostage)`),
    confidence: 0.85,
    crimeType:  "violent_crime",
},
```

Note: The exact implementation depends on the current pattern structure in `crime_rules.go`. The implementer should read the file and follow the existing pattern format.

**Step 4: Run tests**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./internal/classifier/ -run TestCrimeRules -v
```

Expected: All PASS including new tests.

**Step 5: Run full test suite and lint**

```bash
cd /home/fsd42/dev/north-cloud/classifier && go test ./... && golangci-lint run
```

**Step 6: Commit**

```bash
git add classifier/internal/classifier/crime_rules.go classifier/internal/classifier/crime_test.go
git commit -m "fix(classifier): add missing crime patterns for robbery, carjacking, kidnapping

Crime rules were missing patterns for robbery, armed robbery, carjacking,
kidnapping, and other common crime types. Also added 'arrested', 'custody',
'detained' to authority indicators.

This caused articles like 'Repeat offender among two arrested in store
robbery' to get street_crime_relevance='not_crime' despite having crime
keywords detected by the topic classifier.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 8: Deploy Classifier Fixes and Reclassify Existing Content

**Step 1: Push classifier changes**

```bash
git push origin main
gh run watch
```

Expected: CI builds and deploys classifier service.

**Step 2: Verify classifier deployment**

```bash
ssh jones@northcloud.biz 'docker ps --format "{{.Names}} {{.Status}}" | grep classifier'
```

Expected: Classifier container healthy.

**Step 3: Reclassify a sample article to verify fix**

Use the classifier's reclassify endpoint on the "Repeat offender" article:

```bash
# Get auth token
TOKEN=$(ssh jones@northcloud.biz 'docker exec north-cloud-auth-1 wget -qO- "http://localhost:8040/api/v1/auth/login" --post-data='\''{"username":"admin","password":"f00Bar123!"}'\'' --header="Content-Type: application/json" | python3 -c "import sys,json; print(json.load(sys.stdin)[\"token\"])"')

# Reclassify the article
ssh jones@northcloud.biz "docker run --rm --network=north-cloud_north-cloud-network curlimages/curl:8.1.2 -s -X POST 'http://classifier:8071/api/v1/classify/reclassify/805e15f93695a39afada244a7a9cd3bbbfa4d7bb96dcb4cbfa34d3f3990a56f1' -H 'Authorization: Bearer $TOKEN' | python3 -m json.tool"
```

Expected: `street_crime_relevance` should now be `core_street_crime` (not `not_crime`).

**Step 4: Batch reclassify all misclassified articles**

Query ES for articles with crime keywords but `street_crime_relevance: "not_crime"`:

```bash
ssh jones@northcloud.biz 'docker exec north-cloud-elasticsearch-1 curl -s "http://localhost:9200/*_classified_content/_search" -H "Content-Type: application/json" -d '\''{"query":{"bool":{"must":[{"exists":{"field":"crime"}},{"term":{"crime.street_crime_relevance":"not_crime"}},{"terms":{"crime.crime_types":["violent_crime","property_crime","drug_crime"]}}]}},"_source":["title","crime"],"size":5}'\'' | python3 -m json.tool'
```

For each misclassified article, call the reclassify endpoint. Script this:

```bash
# Get all article IDs that need reclassification
ssh jones@northcloud.biz 'docker exec north-cloud-elasticsearch-1 curl -s "http://localhost:9200/*_classified_content/_search" -H "Content-Type: application/json" -d '\''{"query":{"bool":{"must":[{"exists":{"field":"crime"}},{"term":{"crime.street_crime_relevance":"not_crime"}},{"terms":{"crime.crime_types":["violent_crime","property_crime","drug_crime"]}}]}},"_source":false,"size":100}'\'' | python3 -c "import sys,json; [print(h[\"_id\"]) for h in json.load(sys.stdin)[\"hits\"][\"hits\"]]"'
```

Then loop and reclassify each.

**Step 5: Reset publisher cursor again to re-publish reclassified articles**

```bash
ssh jones@northcloud.biz 'docker exec north-cloud-postgres-publisher-1 psql -U postgres -d publisher -c "
BEGIN;
DELETE FROM publish_history WHERE channel_name LIKE '\''crime:%'\'';
UPDATE publisher_cursor SET last_sort = '\''[]'\'', updated_at = NOW() WHERE id = 1;
COMMIT;
"'
ssh jones@northcloud.biz 'cd /opt/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.prod.yml restart publisher'
```

**Step 6: Verify end-to-end flow**

```bash
# Check publisher is publishing to crime:* channels
ssh jones@northcloud.biz 'docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs -f publisher 2>&1 | head -100 | grep "Published article"'

# Check Streetcode is receiving and processing
ssh deployer@streetcode.net 'tail -50 /home/deployer/streetcode-laravel/current/storage/logs/laravel.log | grep "Article processed"'
```

Expected: Crime articles flowing through the full pipeline to Streetcode.

---

## Phase 4: Content Type Reclassification

### Task 9: Reclassify Articles Currently Marked as "page"

After deploying the content_type fixes (Tasks 4-5), existing articles classified as "page" need reclassification.

**Step 1: Count articles classified as "page" that might be articles**

```bash
ssh jones@northcloud.biz 'docker exec north-cloud-elasticsearch-1 curl -s "http://localhost:9200/*_classified_content/_search" -H "Content-Type: application/json" -d '\''{"query":{"bool":{"must":[{"term":{"content_type":"page"}},{"range":{"word_count":{"gte":200}}}]}},"size":0}'\'' | python3 -c "import sys,json; print(json.load(sys.stdin)[\"hits\"][\"total\"][\"value\"], \"articles classified as page with 200+ words\")"'
```

**Step 2: Batch reclassify using classifier API**

Same approach as Task 8 Step 4 but targeting `content_type: "page"` with high word counts.

**Step 3: Reset cursor and re-publish**

Same as Task 8 Step 5.

---

## Verification Checklist

After all phases are complete, verify each layer:

| Layer | Check | Command |
|-------|-------|---------|
| Crawler | Jobs running | `ssh jones@northcloud.biz 'docker compose ... logs crawler \| grep "crawl complete"'` |
| Classifier | Crime enabled | `ssh jones@northcloud.biz 'docker exec north-cloud-classifier-1 env \| grep CRIME'` |
| Classifier | Content type fix | Reclassified article now `content_type: "article"` |
| Classifier | Crime fix | Reclassified article now `street_crime_relevance: "core_street_crime"` |
| Publisher | Cursor fresh | `SELECT last_sort FROM publisher_cursor` shows recent timestamp |
| Publisher | Crime channels | `grep "crime:" publish logs` shows activity |
| Publisher | INFO logs visible | `docker logs publisher \| grep "Published article"` |
| Redis | Messages flowing | `redis-cli PUBSUB NUMSUB crime:homepage` > 0 |
| Streetcode | Subscriber connected | `ps aux \| grep articles:subscribe` shows processes |
| Streetcode | Articles created | New articles in DB after deploy |
| MCP | Container stable | `docker ps \| grep mcp` shows Up (not Restarting) |

---

## Dependencies Between Tasks

```
Task 1 (Deploy) ──────────────────────────> Task 2 (Reset Cursor) ──> Task 3 (Verify Streetcode)
                                                                          │
Task 4 (URL fix) ──> Task 5 (Date fix) ──> Task 8 (Deploy+Reclassify) ──┤
                                                                          │
Task 6 (Audit) ──> Task 7 (Crime patterns) ─────────────────────────────┘
                                                                          │
                                                              Task 9 (Reclassify pages)
```

Tasks 1-3 can run immediately (code already written).
Tasks 4-5 and 6-7 can run in parallel.
Task 8 depends on 4-5 and 6-7.
Task 9 depends on 8.
