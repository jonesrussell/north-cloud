# Article-to-ContentItem Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rename `Article` to `ContentItem` across all services, switching Redis channels from `articles:*` to `content:*` and API paths from `/articles/*` to `/content/*`.

**Architecture:** Domain-first, schema-last. Phases 1-4 (types, API, dashboard, Redis channels) deploy together as one coordinated release. Phase 5 (DB migrations) deploys separately afterward. Streetcode is cut over to `content:crime` before north-cloud deploys.

**Tech Stack:** Go 1.26, Vue 3 + TypeScript, PostgreSQL, Redis Pub/Sub, Elasticsearch

**Design doc:** `docs/plans/2026-02-24-article-to-content-item-design.md`

---

## Pre-flight

Before starting, create a feature branch:

```bash
git checkout -b claude/article-to-content-item-<session-id>
```

---

## Task 1: Infrastructure — Pipeline Client and SSE Types

These are shared libraries imported by multiple services. Rename them first so downstream services compile.

**Files:**
- Modify: `infrastructure/pipeline/client.go:29` — `Event.ArticleURL` field
- Modify: `infrastructure/sse/types.go:109-120` — `ArticlesFound`, `ArticlesIndexed` fields
- Modify: `infrastructure/sse/types.go:162,176` — constructor param names
- Test: `infrastructure/pipeline/client_test.go` (if exists)
- Test: `infrastructure/sse/types_test.go` (if exists)

**Step 1: Rename pipeline Event field**

In `infrastructure/pipeline/client.go`, rename:
- `ArticleURL string \`json:"article_url"\`` → `ContentURL string \`json:"content_url"\``

**Step 2: Rename SSE fields**

In `infrastructure/sse/types.go`, rename:
- `JobProgressData.ArticlesFound` → `ItemsFound` (json tag: `"items_found"`)
- `JobProgressData.ArticlesIndexed` → `ItemsIndexed` (json tag: `"items_indexed"`)
- `JobCompletedData.ArticlesIndexed` → `ItemsIndexed` (json tag: `"items_indexed"`)
- `NewJobProgressEvent` params: `articlesFound, articlesIndexed` → `itemsFound, itemsIndexed`
- `NewJobCompletedEvent` param: `articlesIndexed` → `itemsIndexed`

**Step 3: Fix all callers**

Search the entire codebase for `.ArticleURL` (pipeline client usage) and `.ArticlesFound`, `.ArticlesIndexed` (SSE usage). Update every caller. Key locations:
- `publisher/internal/router/service.go:483` — `pipeline.Event{ArticleURL: ...}`
- `crawler/` — any SSE event emission using `NewJobProgressEvent` or `NewJobCompletedEvent`

Run: `cd infrastructure && go build ./...`

**Step 4: Run tests**

Run: `cd infrastructure && go test ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add infrastructure/
git commit -m "refactor(infrastructure): rename ArticleURL to ContentURL, ArticlesFound/Indexed to ItemsFound/Indexed"
```

---

## Task 2: Publisher — Core Domain Types

The publisher has the deepest embedding. Start with the central `Article` struct, the `RoutingDomain` interface, and the models.

**Files:**
- Rename: `publisher/internal/router/article.go` → `publisher/internal/router/content_item.go`
- Modify: `publisher/internal/router/domain.go:18` — `RoutingDomain` interface
- Modify: `publisher/internal/models/publish_history.go` — field names
- Modify: `publisher/internal/metrics/types.go:6` — `RecentArticle` struct
- Modify: `publisher/internal/metrics/keys.go:15,19,23` — constant names
- Modify: `publisher/internal/metrics/interface.go:18,22` — interface methods

**Step 1: Rename article.go to content_item.go and rename the struct**

```bash
cd publisher && git mv internal/router/article.go internal/router/content_item.go
```

In `content_item.go`:
- Rename `type Article struct` → `type ContentItem struct`
- Rename comment: `// Article represents an article` → `// ContentItem represents a content item`
- Rename receiver: `func (a *Article)` → `func (c *ContentItem)`
- Rename comment: `// flat Article fields` → `// flat ContentItem fields`

**Step 2: Update RoutingDomain interface**

In `publisher/internal/router/domain.go:14-18`:
- Comment: `// Routes returns the channels this domain produces for the given article.` → `// Routes returns the channels this domain produces for the given content item.`
- Signature: `Routes(a *Article) []ChannelRoute` → `Routes(item *ContentItem) []ChannelRoute`

**Step 3: Update PublishHistory models**

In `publisher/internal/models/publish_history.go`:
- `PublishHistory.ArticleID` → `.ContentID` (keep `db:"article_id"` tag unchanged — DB column rename is Phase 5)
- `PublishHistory.ArticleTitle` → `.ContentTitle` (keep `db:"article_title"`)
- `PublishHistory.ArticleURL` → `.ContentURL` (keep `db:"article_url"`)
- Update JSON tags: `"article_id"` → `"content_id"`, `"article_title"` → `"content_title"`, `"article_url"` → `"content_url"`
- Same for `PublishHistoryCreateRequest` and `PublishHistoryFilter` fields
- Update comments

**Step 4: Update metrics types**

In `publisher/internal/metrics/types.go`:
- `RecentArticle` → `RecentItem`

In `publisher/internal/metrics/keys.go`:
- `KeyRecentArticles = "metrics:recent:articles"` → `KeyRecentItems = "metrics:recent:items"`
- `MaxRecentArticles = 100` → `MaxRecentItems = 100`
- `RecentArticlesTTLDays = 7` → `RecentItemsTTLDays = 7`
- Update all comments

In `publisher/internal/metrics/interface.go`:
- `AddRecentArticle(ctx context.Context, article any) error` → `AddRecentItem(ctx context.Context, item any) error`
- `GetRecentArticles(ctx context.Context, limit int) ([]RecentArticle, error)` → `GetRecentItems(ctx context.Context, limit int) ([]RecentItem, error)`
- Update comments

**Step 5: Verify compilation**

Run: `cd publisher && go build ./...`
Expected: Compilation errors in files that reference old names — these are fixed in Tasks 3-4.

**Step 6: Commit**

```bash
git add publisher/internal/router/content_item.go publisher/internal/router/domain.go publisher/internal/models/ publisher/internal/metrics/
git commit -m "refactor(publisher): rename Article to ContentItem, RecentArticle to RecentItem"
```

---

## Task 3: Publisher — Routing Domains and Service

Update all routing domain implementations and the main service loop.

**Files:**
- Modify: `publisher/internal/router/service.go` — all function names and variables
- Modify: `publisher/internal/router/domain_topic.go:25,31` — Routes signature, `"articles:"` prefix
- Modify: `publisher/internal/router/domain_dbchannel.go:25` — Routes signature
- Modify: `publisher/internal/router/crime.go:31` — Routes signature
- Modify: `publisher/internal/router/location.go` — Routes signature
- Modify: `publisher/internal/router/mining.go:41,51` — Routes signature, `"articles:mining"`
- Modify: `publisher/internal/router/entertainment.go:25` — Routes signature
- Modify: `publisher/internal/router/anishinaabe.go:26,36` — Routes signature, `"articles:anishinaabe"`
- Modify: `publisher/internal/router/domain_coforge.go:27` — Routes signature
- Modify: `publisher/internal/router/domain_recipe.go:19,24` — Routes signature, `"articles:recipes"`
- Modify: `publisher/internal/router/domain_job.go:19,24` — Routes signature, `"articles:jobs"`
- Modify: `publisher/internal/domain/outbox.go:55,58,64` — `"articles:crime"`, `"articles:news"` literals

**Step 1: Update all RoutingDomain.Routes signatures**

In every domain file, change:
- `func (d *XxxDomain) Routes(a *Article) []ChannelRoute` → `func (d *XxxDomain) Routes(item *ContentItem) []ChannelRoute`
- Update all uses of `a.` to `item.` within each Routes method

Files: `domain_topic.go`, `domain_dbchannel.go`, `crime.go`, `location.go`, `mining.go`, `entertainment.go`, `anishinaabe.go`, `domain_coforge.go`, `domain_recipe.go`, `domain_job.go`

**Step 2: Rename Redis channel prefixes**

- `domain_topic.go:31` — `"articles:"+topic` → `"content:"+topic`
- `mining.go:51` — `"articles:mining"` → `"content:mining"`
- `anishinaabe.go:36` — `"articles:anishinaabe"` → `"content:anishinaabe"`
- `domain_recipe.go:24` — `"articles:recipes"` → `"content:recipes"`
- `domain_job.go:24` — `"articles:jobs"` → `"content:jobs"`
- `domain/outbox.go:55` — `"articles:crime:"` → `"content:crime:"`
- `domain/outbox.go:58` — `"articles:crime"` → `"content:crime"`
- `domain/outbox.go:64` — `"articles:news"` → `"content:news"`

**Step 3: Update service.go functions**

In `publisher/internal/router/service.go`:
- `fetchArticles()` → `fetchContentItems()` (line 240)
- `routeArticle()` → `routeContentItem()` (line 192)
- `publishRoutes(ctx, article, routes)` → `publishRoutes(ctx, item, routes)` (line 180)
- `publishToChannel(ctx, article, ...)` → `publishToChannel(ctx, item, ...)` (line 354)
- `buildHistoryReq(channelID, article, ...)` → `buildHistoryReq(channelID, item, ...)` (line 464)
- `emitPublishedEvent(ctx, article, ...)` → `emitPublishedEvent(ctx, item, ...)` (line 477)
- All local variable names: `article` → `item`, `articles` → `items`
- All `*Article` type references → `*ContentItem`
- Update `pipeline.Event{ArticleURL: article.URL}` → `pipeline.Event{ContentURL: item.URL}` (line 483)
- Log fields: `"article_id"` → `"content_id"`, `"articles_fetched_total"` → `"items_fetched_total"`, etc.
- In `publishToChannel`, update `buildHistoryReq` call: `.ArticleID` → `.ContentID`, `.ArticleTitle` → `.ContentTitle`, `.ArticleURL` → `.ContentURL`

**Step 4: Update comments**

Update all doc comments referencing "article" to say "content item" where appropriate. Keep "article" in comments that specifically discuss the article content type.

**Step 5: Verify compilation**

Run: `cd publisher && go build ./...`

**Step 6: Commit**

```bash
git add publisher/internal/router/ publisher/internal/domain/
git commit -m "refactor(publisher): rename routing functions and switch channels from articles:* to content:*"
```

---

## Task 4: Publisher — API, Database, Metrics, Dedup

Update the remaining publisher packages.

**Files:**
- Modify: `publisher/internal/api/router.go:136,147-148` — route registration
- Modify: `publisher/internal/api/handlers.go:44-66` — `GetRecentArticles`
- Modify: `publisher/internal/api/stats_handler.go:299,327` — handler functions
- Modify: `publisher/internal/api/stats_service.go:22,43` — interface + implementation
- Modify: `publisher/internal/database/repository_history.go:16,176-209` — function names, column constant
- Modify: `publisher/internal/metrics/tracker.go:101-298` — all article functions
- Modify: `publisher/internal/dedup/tracker.go:26-166` — key pattern, param names, log fields

**Step 1: Update API routes**

In `publisher/internal/api/router.go`:
- Line 136: `history.GET("/:article_id", r.getPublishHistoryByArticle)` → `history.GET("/:content_id", r.getPublishHistoryByContent)`
- Lines 147-148: `articles := v1.Group("/articles")` / `articles.GET("/recent", r.getRecentArticles)` → `content := v1.Group("/content")` / `content.GET("/recent", r.getRecentItems)`

**Step 2: Update API handlers**

In `publisher/internal/api/handlers.go`:
- `GetRecentArticles` → `GetRecentItems`
- `h.statsService.GetRecentArticles(...)` → `h.statsService.GetRecentItems(...)`
- JSON response key `"articles"` → `"items"`
- Update log/error strings

In `publisher/internal/api/stats_handler.go`:
- `getPublishHistoryByArticle` → `getPublishHistoryByContent`
- `c.Param("article_id")` → `c.Param("content_id")`
- `getRecentArticles` → `getRecentItems`
- JSON keys: `"article_id"` → `"content_id"`, `"article_title"` → `"content_title"`, `"article_url"` → `"content_url"`, `"articles"` → `"items"`, `"total_articles"` → `"total_items"`, `"article_count"` → `"item_count"`

In `publisher/internal/api/stats_service.go`:
- Interface method `GetRecentArticles` → `GetRecentItems` (return `[]metrics.RecentItem`)
- Implementation `GetRecentArticles` → `GetRecentItems`
- `metrics.MaxRecentArticles` → `metrics.MaxRecentItems`
- `s.tracker.GetRecentArticles(...)` → `s.tracker.GetRecentItems(...)`

**Step 3: Update database repository**

In `publisher/internal/database/repository_history.go`:
- `publishHistoryColumns` constant: no change (still references DB column names `article_id` etc. — Phase 5)
- `GetPublishHistoryByArticleID` → `GetPublishHistoryByContentID` (function name only, SQL unchanged)
- `CheckArticlePublished` → `CheckContentPublished` (function name only, SQL unchanged)
- Update error strings and comments

**Step 4: Update metrics tracker**

In `publisher/internal/metrics/tracker.go`:
- `convertArticleToRecentArticle` → `convertToRecentItem` (returns `RecentItem`)
- `convertMapToRecentArticle` → `convertMapToRecentItem` (returns `RecentItem`)
- `convertViaJSON` → keep name but update return type to `RecentItem`
- `AddRecentArticle` → `AddRecentItem`
- `GetRecentArticles` → `GetRecentItems`
- All `RecentArticle` type references → `RecentItem`
- All `KeyRecentArticles` → `KeyRecentItems`
- All `MaxRecentArticles` → `MaxRecentItems`
- All `RecentArticlesTTLDays` → `RecentItemsTTLDays`
- Log fields: `"article_id"` → `"content_id"`
- Error strings: `"marshal article"` → `"marshal item"`, etc.

**Step 5: Update dedup tracker**

In `publisher/internal/dedup/tracker.go`:
- `key(articleID)` → `key(contentID)` with format `"posted:content:%s"`
- `HasPosted(ctx, articleID)` → `HasPosted(ctx, contentID)` (param name only)
- `MarkPosted(ctx, articleID)` → `MarkPosted(ctx, contentID)`
- `Clear(ctx, articleID)` → `Clear(ctx, contentID)`
- `FlushAll` scan pattern: `"posted:article:*"` → `"posted:content:*"`
- All log fields: `"article_id"` → `"content_id"`
- All log messages: `"article"` → `"content item"` where referring to generic content

**Step 6: Run tests**

Run: `cd publisher && go test ./...`
Expected: Some test failures from renamed functions/types — fix test files.

**Step 7: Fix tests**

Update all test files that reference old names. Key files:
- `publisher/internal/router/*_test.go` — `Article{}` → `ContentItem{}`
- `publisher/internal/dedup/tracker_test.go` — `"posted:article:"` → `"posted:content:"`
- `publisher/internal/metrics/tracker_test.go` — `RecentArticle` → `RecentItem`, function names
- `publisher/internal/api/*_test.go` — endpoint paths, response field names

Run: `cd publisher && go test ./...`
Expected: PASS

**Step 8: Run linter**

Run: `cd publisher && golangci-lint run`
Expected: PASS

**Step 9: Commit**

```bash
git add publisher/
git commit -m "refactor(publisher): rename API endpoints, handlers, metrics, and dedup from article to content"
```

---

## Task 5: Pipeline — Domain Types and Database

**Files:**
- Modify: `pipeline/internal/domain/models.go:64-151` — `Article` struct, `PipelineEvent`, `IngestRequest`, `FunnelStage`, `GenerateIdempotencyKey`
- Modify: `pipeline/internal/database/repository.go:28-47,57-58,93` — `UpsertArticle`, SQL references

**Step 1: Rename domain types**

In `pipeline/internal/domain/models.go`:
- `type Article struct` → `type ContentItem struct` (line 65)
- `PipelineEvent.ArticleURL` → `.ContentURL` (line 76, json tag `"content_url"`)
- `IngestRequest.ArticleURL` → `.ContentURL` (line 88, json tag `"content_url"`)
- `FunnelStage.UniqueArticles` → `.UniqueItems` (line 106, json tag `"unique_items"`)
- `GenerateIdempotencyKey` param: `articleURL` → `contentURL` (line 144)
- Update all comments

**Step 2: Update database repository**

In `pipeline/internal/database/repository.go`:
- `UpsertArticle(ctx, article *domain.Article)` → `UpsertContentItem(ctx, item *domain.ContentItem)` (line 29)
- SQL: `INSERT INTO articles` stays unchanged (DB column rename is Phase 5)
- Error string: `"upsert article"` → `"upsert content item"`
- `InsertEvent` SQL: column name `article_url` stays unchanged (DB rename is Phase 5), but Go field access changes from `.ArticleURL` → `.ContentURL`
- Funnel query: `unique_articles` alias stays unchanged in SQL, but mapped to `.UniqueItems` field

**Step 3: Fix all callers within pipeline**

Search for `.ArticleURL`, `UpsertArticle`, `domain.Article` in `pipeline/internal/service/` and `pipeline/internal/api/`. Update all references.

**Step 4: Run tests and lint**

Run: `cd pipeline && go test ./... && golangci-lint run`
Expected: Fix any test failures from renamed types, then PASS.

**Step 5: Commit**

```bash
git add pipeline/
git commit -m "refactor(pipeline): rename Article to ContentItem, ArticleURL to ContentURL"
```

---

## Task 6: Search — Domain and Service

**Files:**
- Modify: `search/internal/domain/search.go:264-281` — `PublicFeedArticle`, `PublicFeedResponse`
- Modify: `search/internal/service/search_service.go:398-538` — `LatestArticles`, `parseLatestArticlesResponse`

**Step 1: Rename domain types**

In `search/internal/domain/search.go`:
- `PublicFeedArticle` → `PublicFeedItem` (line 266)
- `PublicFeedResponse.Articles` → `.Items` (line 280, json tag `"items"`)
- Update comments

**Step 2: Rename service functions**

In `search/internal/service/search_service.go`:
- `LatestArticles()` → `LatestItems()` (line 400, return type `[]domain.PublicFeedItem`)
- `TopicFeed()` return type → `[]domain.PublicFeedItem` (line 437)
- `parseLatestArticlesResponse()` → `parseLatestItemsResponse()` (line 484, return type)
- All internal `domain.PublicFeedArticle` → `domain.PublicFeedItem`
- Error strings: `"latest articles"` → `"latest items"`

**Step 3: Fix callers**

Search for `LatestArticles`, `PublicFeedArticle` in `search/internal/api/`. Update all references.

**Step 4: Run tests and lint**

Run: `cd search && go test ./... && golangci-lint run`
Expected: PASS after fixing test references.

**Step 5: Commit**

```bash
git add search/
git commit -m "refactor(search): rename PublicFeedArticle to PublicFeedItem, LatestArticles to LatestItems"
```

---

## Task 7: Crawler — Content Detector and Fetcher

**Files:**
- Rename: `crawler/internal/crawler/article_detector.go` → `content_detector.go`
- Rename: `crawler/internal/crawler/article_detector_test.go` → `content_detector_test.go`
- Modify: `crawler/internal/fetcher/worker.go:33,36-50,333-334,436-455` — binary URL detection

**Step 1: Rename article_detector files**

```bash
cd crawler && git mv internal/crawler/article_detector.go internal/crawler/content_detector.go
cd crawler && git mv internal/crawler/article_detector_test.go internal/crawler/content_detector_test.go
```

**Step 2: Rename exported functions in content_detector.go**

- `isArticleURL()` → `isContentURL()` (line 145)
- `isNonArticlePath()` → `isBinaryPath()` (line 195)
- `hasArticlePathSegment()` → `hasContentPathSegment()` (line 218)
- `isArticlePage()` → `isContentPage()` (line 352)
- `compileArticlePatterns()` → `compileContentPatterns()` (line 359)

Also rename internal variables:
- `nonArticleSegments` → `nonContentSegments` (line 34)
- `nonArticleExtensions` → `binaryExtensions` (line 59)
- `articlePathSegments` → `contentPathSegments` (line 111)

**Keep as-is:** `DetectedContentArticle = "article"` (line 17), `hasNewsArticleJSONLD()` (line 249), `structuredContentJSONLDTypes` containing `"NewsArticle"` and `"Article"` (line 29-32) — these are about the article content type specifically.

**Step 3: Update callers of renamed functions**

Search for `isArticleURL`, `isArticlePage`, `compileArticlePatterns`, `IsArticleURL`, `IsArticlePage`, `CompileArticlePatterns` across the crawler. Update all call sites.

Key locations:
- `internal/crawler/content_detector.go` itself (internal calls between functions)
- `internal/crawler/*.go` (any file calling the exported wrappers)
- Anywhere `IsStructuredContentPage` calls these internally

**Step 4: Rename fetcher binary URL detection**

In `crawler/internal/fetcher/worker.go`:
- `reasonNonArticleURL` → `reasonBinaryURL` (line 33, value: `"binary_url"`)
- `nonArticleExtensions` → `binaryExtensions` (line 36)
- `nonArticlePathSubstrings` → `binaryPathSubstrings` (line 47)
- `isNonArticleURL()` → `isBinaryURL()` (line 438)
- Comment: `"non-article file extension"` → `"binary file extension"` (line 437)
- Update call site at line 333: `isNonArticleURL(...)` → `isBinaryURL(...)`

**Step 5: Update test file**

In `content_detector_test.go`, rename all test functions:
- `TestIsArticleURL_*` → `TestIsContentURL_*`
- `TestIsArticlePage_*` → `TestIsContentPage_*`
- `TestCompileArticlePatterns_*` → `TestCompileContentPatterns_*`
- Update all `crawler.IsArticleURL(...)` → `crawler.IsContentURL(...)`
- Update all `crawler.IsArticlePage(...)` → `crawler.IsContentPage(...)`
- Update all `crawler.CompileArticlePatterns(...)` → `crawler.CompileContentPatterns(...)`

**Step 6: Update SSE event emissions in crawler**

Search for `NewJobProgressEvent` and `NewJobCompletedEvent` calls. Update param variable names from `articlesFound`/`articlesIndexed` to `itemsFound`/`itemsIndexed` if used as local vars.

**Step 7: Update pipeline client usage**

Search for `pipeline.Event{ArticleURL:` in crawler code. Update to `pipeline.Event{ContentURL:`.

**Step 8: Run tests and lint**

Run: `cd crawler && go test ./... && golangci-lint run`
Expected: PASS after fixing test references.

**Step 9: Commit**

```bash
git add crawler/
git commit -m "refactor(crawler): rename article_detector to content_detector, isNonArticleURL to isBinaryURL"
```

---

## Task 8: Classifier — Content Type Functions

**Files:**
- Modify: `classifier/internal/classifier/content_type.go:407-655` — four function renames

**Step 1: Rename functions**

In `classifier/internal/classifier/content_type.go`:
- `isNonArticleURL()` → `isBinaryURL()` (line 408)
- `isNonArticleURLFallback()` → `isBinaryURLFallback()` (line 493)
- `hasArticleCharacteristics()` → `hasContentCharacteristics()` (line 609)
- `hasRelaxedArticleCharacteristics()` → `hasRelaxedContentCharacteristics()` (line 636)

Also rename internal constants if present:
- `articleConfidence` → `contentConfidence` (line 13-15 area)
- `relaxedArticleConfidence` → `relaxedContentConfidence`
- `articleTypeString` → keep as-is (it's the literal string `"article"`, correct for the subtype)

**Step 2: Update call sites**

In the same file, search for calls to the old function names (around lines 98, 139, 156) and update them.

**Step 3: Run tests and lint**

Run: `cd classifier && go test ./... && golangci-lint run`
Expected: PASS.

**Step 4: Commit**

```bash
git add classifier/
git commit -m "refactor(classifier): rename isNonArticleURL to isBinaryURL, hasArticleCharacteristics to hasContentCharacteristics"
```

---

## Task 9: MCP — Tool Names and Types

**Files:**
- Modify: `mcp-north-cloud/internal/client/publisher.go:65,75,80-87,229` — `PreviewArticle`, `ArticlesByChannel`
- Modify: `mcp-north-cloud/internal/mcp/handlers.go:877,898` — handler function names
- Modify: `mcp-north-cloud/internal/mcp/tools.go` — tool name strings `"search_articles"`, `"classify_article"`

**Step 1: Rename client types**

In `mcp-north-cloud/internal/client/publisher.go`:
- `PreviewArticle` → `PreviewItem`
- `PublishHistory.ArticleID` → `.ContentID`
- `PublisherStats.ArticlesByChannel` → `.ItemsByChannel`
- `PreviewRoute()` return type: `[]PreviewArticle` → `[]PreviewItem`
- Internal response struct `SampleArticles` → `SampleItems`

**Step 2: Rename handler functions**

In `mcp-north-cloud/internal/mcp/handlers.go`:
- `handleSearchArticles` → `handleSearchContent` (line 877)
- `handleClassifyArticle` → `handleClassifyContent` (line 898)
- Error string: `"Failed to classify article"` → `"Failed to classify content"`
- Response key: `"articles"` → `"items"` (line 831 in `handlePreviewRoute`)

**Step 3: Rename tool names**

In `mcp-north-cloud/internal/mcp/tools.go`:
- Find tool name registrations for `"search_articles"` → `"search_content"`
- Find tool name registrations for `"classify_article"` → `"classify_content"`

Also update `server.go` where handler functions are registered by name.

**Step 4: Run tests and lint**

Run: `cd mcp-north-cloud && go test ./... && golangci-lint run`
Expected: PASS.

**Step 5: Commit**

```bash
git add mcp-north-cloud/
git commit -m "refactor(mcp): rename search_articles to search_content, PreviewArticle to PreviewItem"
```

---

## Task 10: Dashboard — Types, Components, Routes

**Files:**
- Modify: `dashboard/src/types/publisher.ts` — all article-named interfaces and fields
- Modify: `dashboard/src/types/classifier.ts:62` — `total_articles` field
- Modify: `dashboard/src/types/metrics.ts:31` — `total_articles` field
- Modify: `dashboard/src/types/realtime.ts:23-34` — `articles_found`, `articles_indexed` fields
- Rename: `dashboard/src/views/distribution/ArticlesView.vue` → `ContentView.vue`
- Rename: `dashboard/src/components/domain/articles/` → `content/`
- Rename: `dashboard/src/components/domain/articles/ArticlesFilterBar.vue` → `content/ContentFilterBar.vue`
- Modify: `dashboard/src/composables/usePublishHistory.ts:7` — `GroupedArticle`
- Modify: `dashboard/src/api/client.ts` — endpoint paths and type imports
- Modify: `dashboard/src/config/navigation.ts:51` — nav title and path
- Modify: `dashboard/src/router/index.ts:9,70-74,285,289` — routes and imports

**Step 1: Rename TypeScript types**

In `dashboard/src/types/publisher.ts`:
- `RecentArticle` → `RecentItem`
- `RecentArticlesResponse` → `RecentItemsResponse` (field: `articles` → `items`)
- `PreviewArticle` → `PreviewItem`
- Fields: `article_id` → `content_id`, `article_title` → `content_title`, `article_url` → `content_url`
- `total_articles` → `total_items`, `article_count` → `item_count`
- `sample_articles` → `sample_items`

In `dashboard/src/types/classifier.ts`: `total_articles` → `total_items`
In `dashboard/src/types/metrics.ts`: `total_articles` → `total_items`
In `dashboard/src/types/realtime.ts`: `articles_found` → `items_found`, `articles_indexed` → `items_indexed`

**Step 2: Rename composable**

In `dashboard/src/composables/usePublishHistory.ts`:
- `GroupedArticle` → `GroupedItem`
- Field `article_id` → `content_id`
- Variable `groupedArticles` → `groupedItems`
- Variable `articleMap` → `itemMap`
- Error message: `"recent articles"` → `"recent content"`

**Step 3: Rename Vue components and directories**

```bash
cd dashboard
git mv src/components/domain/articles src/components/domain/content
git mv src/components/domain/content/ArticlesFilterBar.vue src/components/domain/content/ContentFilterBar.vue
git mv src/views/distribution/ArticlesView.vue src/views/distribution/ContentView.vue
```

Update `src/components/domain/content/index.ts` to export `ContentFilterBar`.

**Step 4: Update ContentView.vue**

- Import path: `'@/components/domain/articles'` → `'@/components/domain/content'`
- Component: `ArticlesFilterBar` → `ContentFilterBar`
- `GroupedArticle` → `GroupedItem`
- `groupedArticles` → `groupedItems`
- Heading: `"Recent Articles"` → `"Recent Content"`
- Subheading: `"Recently published articles across all channels"` → `"Recently published content across all channels"`
- Card title: `"Published Articles"` → `"Published Content"`
- Text: `"unique articles on this page"` → `"unique items on this page"`

**Step 5: Update API client**

In `dashboard/src/api/client.ts`:
- Import `RecentItemsResponse` instead of `RecentArticlesResponse`
- `crawlerApi.articles` → `crawlerApi.content` with path `'/content'`
- `publisherApi.articles` → `publisherApi.content` with path `'/content/recent'`
- `publisherApi.history.getByArticle` → `.getByContent` with path template using `content_id`
- `article_id` filter param → `content_id`

**Step 6: Update navigation**

In `dashboard/src/config/navigation.ts`:
- `{ title: 'Recent Articles', path: '/operations/articles', ... }` → `{ title: 'Recent Content', path: '/operations/content', ... }`

**Step 7: Update router**

In `dashboard/src/router/index.ts`:
- Import: `ArticlesView` → `ContentView` from `'../views/distribution/ContentView.vue'`
- Route path: `'/operations/articles'` → `'/operations/content'`
- Route name: `'operations-articles'` → `'operations-content'`
- Meta title: `'Recent Articles'` → `'Recent Content'`
- Add redirect: `{ path: '/operations/articles', redirect: '/operations/content' }`
- Update legacy redirects to point to `/operations/content`

**Step 8: Verify build**

Run: `cd dashboard && npm run type-check && npm run build`
Expected: PASS.

**Step 9: Commit**

```bash
git add dashboard/
git commit -m "refactor(dashboard): rename Article types to Content, update routes and components"
```

---

## Task 11: Grafana Alerts

**Files:**
- Modify: `infrastructure/grafana/provisioning/alerting/alerts.yml:41,112-113`

**Step 1: Update alert text**

- Line 41: `"No articles classified in the last 2 hours"` → `"No content classified in the last 2 hours"`
- Line 112: `"No articles published in 4 hours"` → `"No content published in 4 hours"`
- Line 113: `"publisher has not pushed any articles to Redis"` → `"publisher has not pushed any content to Redis"`

**Step 2: Commit**

```bash
git add infrastructure/grafana/
git commit -m "refactor(grafana): update alert descriptions from articles to content"
```

---

## Task 12: Documentation Updates

**Files:**
- Modify: `ARCHITECTURE.md` — replace "article routing" terminology
- Modify: `publisher/CLAUDE.md` — update all article references in docs
- Modify: `pipeline/CLAUDE.md` — update Article references
- Modify: `search/CLAUDE.md` — update article references
- Modify: `crawler/CLAUDE.md` — update article_detector references
- Modify: `publisher/docs/REDIS_MESSAGE_FORMAT.md` — channel names
- Modify: `publisher/docs/CONSUMER_GUIDE.md` — channel names
- Modify: `CLAUDE.md` (root) — update any article references in service descriptions

**Step 1: Update ARCHITECTURE.md**

Search for "article" (case-insensitive) and replace with "content item" where it refers to generic content. Keep "article" where it refers to the specific content type.

Key patterns:
- `"Pub/Sub broker for all article routing"` → `"Pub/Sub broker for all content routing"`
- `"routeArticle()"` → `"routeContentItem()"`
- `"articles:*"` channel references → `"content:*"`
- `"Article identity table"` → `"Content item identity table"`

**Step 2: Update service CLAUDE.md files**

In each service's CLAUDE.md:
- Update function names, struct names, endpoint paths, channel names to match the new code
- Keep the structure and style of each file intact
- Focus on accuracy — every code reference should match the renamed code

**Step 3: Update publisher docs**

In `publisher/docs/REDIS_MESSAGE_FORMAT.md` and `publisher/docs/CONSUMER_GUIDE.md`:
- `articles:*` → `content:*`
- `article_id` → `content_id` (in JSON examples)
- "Article" → "Content item" where generic

**Step 4: Update root CLAUDE.md**

Review for any stale "article" references in the service port table comments or quick reference sections.

**Step 5: Commit**

```bash
git add ARCHITECTURE.md CLAUDE.md publisher/CLAUDE.md pipeline/CLAUDE.md search/CLAUDE.md crawler/CLAUDE.md publisher/docs/
git commit -m "docs: update all documentation from article to content item terminology"
```

---

## Task 13: Final Verification

**Step 1: Run all tests across the monorepo**

Run: `task test`
Expected: All services pass.

**Step 2: Run all linters**

Run: `task lint:force`
Expected: No violations.

**Step 3: Search for remaining "article" references**

Run a grep to find any remaining `article` references that should have been renamed:

```bash
grep -rn --include='*.go' 'Article\|article' --exclude-dir=vendor | grep -v '_test.go' | grep -v 'ArticleSelectors\|ArticleMeta\|TypeArticle\|ContentTypeArticle\|articleTypeString\|extractArticleMeta\|extractNewsArticle\|hasNewsArticle\|article_url_patterns\|DetectedContentArticle\|"article"\|"Article"\|"NewsArticle"'
```

Any remaining hits should be either:
- Correctly article-specific (CSS selectors, content type values)
- In vendor directories (ignored)
- In this design/implementation plan doc

**Step 4: Verify Docker build**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build`
Expected: All services build successfully.

**Step 5: Commit any fixes**

If Step 3 found stragglers, fix and commit:

```bash
git add -A
git commit -m "refactor: fix remaining article references missed in rename"
```

---

## Phase 5 Tasks (Separate Deploy — DB Migrations)

These tasks execute AFTER the Phase 1-4 deploy is live and stable. Each can soak for days.

### Task 14: Publisher DB — Add New Columns

**Files:**
- Create: `publisher/migrations/NNN_add_content_columns.up.sql`
- Create: `publisher/migrations/NNN_add_content_columns.down.sql`

**Step 1: Write migration**

```sql
-- UP
ALTER TABLE publish_history ADD COLUMN content_id VARCHAR(255);
ALTER TABLE publish_history ADD COLUMN content_title TEXT;
ALTER TABLE publish_history ADD COLUMN content_url TEXT;
CREATE INDEX idx_publish_history_content ON publish_history (content_id);
CREATE INDEX idx_publish_history_content_channel ON publish_history (content_id, channel_name);

-- Backfill from existing columns
UPDATE publish_history SET
    content_id = article_id,
    content_title = article_title,
    content_url = article_url
WHERE content_id IS NULL;

ALTER TABLE publish_history ALTER COLUMN content_id SET NOT NULL;
```

```sql
-- DOWN
DROP INDEX IF EXISTS idx_publish_history_content_channel;
DROP INDEX IF EXISTS idx_publish_history_content;
ALTER TABLE publish_history DROP COLUMN IF EXISTS content_url;
ALTER TABLE publish_history DROP COLUMN IF EXISTS content_title;
ALTER TABLE publish_history DROP COLUMN IF EXISTS content_id;
```

**Step 2: Update repository to dual-write**

In `publisher/internal/database/repository_history.go`:
- Update `publishHistoryColumns` to include both old and new column names
- Update INSERT to write both `article_id` AND `content_id`
- Update SELECT to read from new columns

**Step 3: Run migration**

Run: `task migrate:publisher`

**Step 4: Run tests**

Run: `cd publisher && go test ./...`

**Step 5: Commit**

```bash
git add publisher/
git commit -m "feat(publisher): add content_id/title/url columns with dual-write"
```

### Task 15: Publisher DB — Drop Old Columns

Only after Task 14 has been live and stable.

**Step 1: Write migration**

```sql
-- UP
DROP INDEX IF EXISTS idx_publish_history_article;
DROP INDEX IF EXISTS idx_publish_history_article_channel;
ALTER TABLE publish_history DROP COLUMN article_id;
ALTER TABLE publish_history DROP COLUMN article_title;
ALTER TABLE publish_history DROP COLUMN article_url;
```

**Step 2: Update repository to single-write**

Remove old column names from `publishHistoryColumns`. Update `db:` struct tags to use new column names.

**Step 3: Commit**

```bash
git add publisher/
git commit -m "feat(publisher): drop legacy article_id/title/url columns from publish_history"
```

### Task 16: Pipeline DB — Migrate articles table to content_items

Follow the same add-backfill-cutover-drop pattern described in the design doc. Create migrations, update repository, run tests, commit.

### Task 17: Classifier DB — Rename total_articles to total_items

Single column rename using add-backfill-drop pattern. Create migration, update repository, run tests, commit.

---

## Parallel Execution Notes

Tasks 1 (infrastructure) must complete first — all other services depend on it.

After Task 1, the following can run **in parallel**:
- Tasks 2-4 (publisher)
- Task 5 (pipeline)
- Task 6 (search)
- Task 7 (crawler)
- Task 8 (classifier)
- Task 9 (MCP)
- Task 10 (dashboard)
- Task 11 (Grafana)

Task 12 (docs) runs after all code tasks complete.
Task 13 (verification) runs last.
Tasks 14-17 (DB migrations) are a separate deploy cycle.
