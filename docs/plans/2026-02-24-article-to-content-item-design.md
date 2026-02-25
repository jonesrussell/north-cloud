# Article-to-ContentItem Domain Rename

**Date:** 2026-02-24
**Status:** Approved
**Approach:** Domain-first, schema-last (Approach A)

## Problem

The system started as an article crawler but now handles articles, pages, videos, images, jobs, recipes, PDFs, and more. The word "article" is embedded throughout the codebase as both a specific `content_type` value and a generic synonym for "any piece of content." This conflation creates confusion as the system scales to new content types (podcasts, datasets, social posts, scientific papers, etc.).

The internal pipeline types (`RawContent`, `ClassifiedContent`, `Document`, `ExtractedContent`) are already content-agnostic. The problem is concentrated at the boundaries: the publisher's central `Article` struct, the `articles:*` Redis namespace, database column names like `article_id`, and the API/UI layer.

## Decision

- **`ContentItem`** is the new primary domain entity (not `Content` — too overloaded in Go).
- **`Article`** becomes one subtype identified by `ContentType == "article"`.
- Internal pipeline types (`RawContent`, `ClassifiedContent`, etc.) are unchanged.
- Migration follows domain-first, schema-last ordering: rename cheap things first, irreversible things last.

## Content Types

Currently flowing through the pipeline:

- HTML articles, HTML pages, videos, images, jobs, recipes, PDFs, other extractable binary documents

Supported by classifier but not fully utilized:

- Product pages, event pages, forum threads, documentation pages

Future direction:

- Podcasts, datasets, social posts, e-commerce listings, scientific papers, transcripts, OCR content

## Deploy Sequence

No fan-out or compatibility shims needed. Streetcode (the only external Redis subscriber) will be cut over to `content:crime` before north-cloud deploys.

1. **Update Streetcode** — switch subscription from `articles:crime` to `content:crime`
2. **Deploy north-cloud** — all renames, `content:*` channels, `/api/v1/content/*` endpoints
3. Done

Brief message gap between steps 1 and 2 is accepted.

## Migration Order

### Phase 1: Domain Types and Functions (zero external impact)

Rename structs, methods, receivers, local variables, and file names. No wire format changes. No database changes. Every service compiles and passes tests independently at the end of this phase.

### Phase 2: API Layer

Introduce `/api/v1/content/*` endpoints. Remove `/api/v1/articles/*` (no shims — Streetcode uses Redis, not the HTTP API; the dashboard is the only HTTP consumer and deploys with north-cloud).

### Phase 3: Dashboard

Rename Vue components, routes, types, composables. Coordinate SSE JSON field changes (`articles_found` -> `items_found`, etc.) in the same deploy as the infrastructure SSE type changes.

### Phase 4: Redis Channels

Switch all `articles:*` channel names to `content:*`. Streetcode is already on `content:crime` by this point.

### Phase 5: Database Migrations

Reversible add-backfill-cutover-drop strategy for all three databases. Each step can soak for days before proceeding.

### Phase 6: Cleanup

Remove any leftover legacy references, update Grafana alerts, update ARCHITECTURE.md and service CLAUDE.md files.

---

## Phase 1: Domain Type Renames

### Proposed ContentItem Struct (Publisher)

```go
// ContentItem represents a content item from Elasticsearch classified_content index.
// Article, page, video, job, recipe, etc. are all subtypes identified by ContentType.
type ContentItem struct {
    ID            string    `json:"id"`
    Title         string    `json:"title"`
    Body          string    `json:"body"`
    RawText       string    `json:"raw_text"`
    RawHTML       string    `json:"raw_html"`
    URL           string    `json:"canonical_url"`
    Source        string    `json:"source"`
    PublishedDate time.Time `json:"published_date"`

    // Classification metadata
    QualityScore     int      `json:"quality_score"`
    Topics           []string `json:"topics"`
    ContentType      string   `json:"content_type"`
    ContentSubtype   string   `json:"content_subtype,omitempty"`
    SourceReputation int      `json:"source_reputation"`
    Confidence       float64  `json:"confidence"`

    // All nested classification objects unchanged
    Crime         *CrimeData         `json:"crime,omitempty"`
    Location      *LocationData      `json:"location,omitempty"`
    Mining        *MiningData        `json:"mining,omitempty"`
    Anishinaabe   *AnishinaabeData   `json:"anishinaabe,omitempty"`
    Entertainment *EntertainmentData `json:"entertainment,omitempty"`
    Coforge       *CoforgeData       `json:"coforge,omitempty"`
    Recipe        *RecipeData        `json:"recipe,omitempty"`
    Job           *JobData           `json:"job,omitempty"`

    // Flat classification fields, OG metadata, Sort — all unchanged
    // ...

    Sort []any `json:"-"`
}

func (c *ContentItem) extractNestedFields() { /* unchanged logic */ }
```

### Pipeline ContentItem Struct

```go
// ContentItem represents a unique piece of content tracked across the pipeline.
type ContentItem struct {
    URL         string    `json:"url"`
    URLHash     string    `json:"url_hash"`
    Domain      string    `json:"domain"`
    SourceName  string    `json:"source_name"`
    FirstSeenAt time.Time `json:"first_seen_at"`
}
```

### Full Rename Inventory by Service

#### Publisher (deepest embedding)

**Types:**

| Current | New | File |
|---|---|---|
| `Article` | `ContentItem` | `internal/router/article.go` -> `content_item.go` |
| `RecentArticle` | `RecentItem` | `internal/metrics/types.go` |
| `PublishHistory.ArticleID` | `.ContentID` | `internal/models/publish_history.go` |
| `PublishHistory.ArticleTitle` | `.ContentTitle` | `internal/models/publish_history.go` |
| `PublishHistory.ArticleURL` | `.ContentURL` | `internal/models/publish_history.go` |
| `PublishHistoryCreateRequest.ArticleID` | `.ContentID` | `internal/models/publish_history.go` |
| `PublishHistoryCreateRequest.ArticleTitle` | `.ContentTitle` | `internal/models/publish_history.go` |
| `PublishHistoryCreateRequest.ArticleURL` | `.ContentURL` | `internal/models/publish_history.go` |
| `PublishHistoryFilter.ArticleID` | `.ContentID` | `internal/models/publish_history.go` |

**Functions:**

| Current | New | File |
|---|---|---|
| `routeArticle()` | `routeContentItem()` | `internal/router/service.go` |
| `fetchArticles()` | `fetchContentItems()` | `internal/router/service.go` |
| `publishRoutes(article)` | `publishRoutes(item)` | `internal/router/service.go` |
| `publishToChannel(article)` | `publishToChannel(item)` | `internal/router/service.go` |
| `buildHistoryReq(article)` | `buildHistoryReq(item)` | `internal/router/service.go` |
| `emitPublishedEvent(article)` | `emitPublishedEvent(item)` | `internal/router/service.go` |
| `GetRecentArticles()` | `GetRecentItems()` | `internal/api/handlers.go` |
| `getPublishHistoryByArticle()` | `getPublishHistoryByContent()` | `internal/api/stats_handler.go` |
| `getRecentArticles()` | `getRecentItems()` | `internal/api/stats_handler.go` |
| `StatsService.GetRecentArticles()` | `.GetRecentItems()` | `internal/api/stats_service.go` |
| `GetPublishHistoryByArticleID()` | `GetPublishHistoryByContentID()` | `internal/database/repository_history.go` |
| `CheckArticlePublished()` | `CheckContentPublished()` | `internal/database/repository_history.go` |
| `convertArticleToRecentArticle()` | `convertToRecentItem()` | `internal/metrics/tracker.go` |
| `convertMapToRecentArticle()` | `convertMapToRecentItem()` | `internal/metrics/tracker.go` |
| `AddRecentArticle()` | `AddRecentItem()` | `internal/metrics/tracker.go` |
| `GetRecentArticles()` | `GetRecentItems()` | `internal/metrics/tracker.go` |

**RoutingDomain interface:** `Routes(*Article)` -> `Routes(*ContentItem)`. Affects every domain file.

**Dedup tracker:**

| Current | New |
|---|---|
| `key(articleID)` -> `posted:article:{id}` | `key(contentID)` -> `posted:content:{id}` |
| `HasPosted(articleID)` | `HasPosted(contentID)` |
| `MarkPosted(articleID)` | `MarkPosted(contentID)` |
| `Clear(articleID)` | `Clear(contentID)` |
| `FlushAll()` pattern `posted:article:*` | `posted:content:*` |

**Log field renames:** `"article_id"` -> `"content_id"`, `"articles_fetched_total"` -> `"items_fetched_total"`, `"articles_in_batch"` -> `"items_in_batch"`, `"articles_published_total"` -> `"items_published_total"`.

#### Pipeline

| Current | New | File |
|---|---|---|
| `Article` | `ContentItem` | `internal/domain/models.go` |
| `PipelineEvent.ArticleURL` | `.ContentURL` | `internal/domain/models.go` |
| `IngestRequest.ArticleURL` | `.ContentURL` | `internal/domain/models.go` |
| `FunnelStage.UniqueArticles` | `.UniqueItems` | `internal/domain/models.go` |
| `UpsertArticle()` | `UpsertContentItem()` | `internal/database/repository.go` |
| `GenerateIdempotencyKey(..., articleURL)` | `...contentURL` | `internal/domain/models.go` |

JSON tag on `IngestRequest` changes from `"article_url"` to `"content_url"`. All callers (infrastructure/pipeline client) update in the same PR.

#### Infrastructure

**pipeline/client.go:**

| Current | New |
|---|---|
| `Event.ArticleURL` (json `"article_url"`) | `Event.ContentURL` (json `"content_url"`) |

**sse/types.go:**

| Current | New |
|---|---|
| `JobProgressData.ArticlesFound` (json `"articles_found"`) | `.ItemsFound` (json `"items_found"`) |
| `JobProgressData.ArticlesIndexed` (json `"articles_indexed"`) | `.ItemsIndexed` (json `"items_indexed"`) |
| `JobCompletedData.ArticlesIndexed` (json `"articles_indexed"`) | `.ItemsIndexed` (json `"items_indexed"`) |
| `NewJobProgressEvent(articlesFound, articlesIndexed)` | `(itemsFound, itemsIndexed)` |
| `NewJobCompletedEvent(..., articlesIndexed)` | `(..., itemsIndexed)` |

SSE JSON tags are consumed by the dashboard — must deploy together.

#### Search

| Current | New | File |
|---|---|---|
| `PublicFeedArticle` | `PublicFeedItem` | `internal/domain/search.go` |
| `PublicFeedResponse.Articles` (json `"articles"`) | `.Items` (json `"items"`) | `internal/domain/search.go` |
| `LatestArticles()` | `LatestItems()` | `internal/service/search_service.go` |
| `parseLatestArticlesResponse()` | `parseLatestItemsResponse()` | `internal/service/search_service.go` |

#### Crawler

**Rename (misnamed — these filter binary URLs, not "non-articles"):**

| Current | New | File |
|---|---|---|
| `isNonArticleURL()` | `isBinaryURL()` | `internal/fetcher/worker.go` |
| `nonArticleExtensions` | `binaryExtensions` | `internal/fetcher/worker.go` |
| `nonArticlePathSubstrings` | `binaryPathSubstrings` | `internal/fetcher/worker.go` |

**Rename (detect content types, not just articles):**

| Current | New | File |
|---|---|---|
| `article_detector.go` | `content_detector.go` | `internal/crawler/` |
| `isArticleURL()` | `isContentURL()` | `content_detector.go` |
| `isNonArticlePath()` | `isBinaryPath()` | `content_detector.go` |
| `isArticlePage()` | `isContentPage()` | `content_detector.go` |
| `compileArticlePatterns()` | `compileContentPatterns()` | `content_detector.go` |
| `hasArticlePathSegment()` | `hasContentPathSegment()` | `content_detector.go` |

**Keep as-is (correctly article-specific within the subtype model):**

- `ArticleSelectors` — CSS selectors for the article content type
- `ArticleMeta` — metadata specific to article extraction
- `TypeArticle = "article"` — a content type enum value
- `extractArticleMeta()`, `extractNewsArticleFields()`, `hasNewsArticleJSONLD()`
- `article_url_patterns` config key

#### Classifier

| Current | New | File |
|---|---|---|
| `isNonArticleURL()` | `isBinaryURL()` | `internal/classifier/content_type.go` |
| `isNonArticleURLFallback()` | `isBinaryURLFallback()` | `internal/classifier/content_type.go` |
| `hasArticleCharacteristics()` | `hasContentCharacteristics()` | `internal/classifier/content_type.go` |
| `hasRelaxedArticleCharacteristics()` | `hasRelaxedContentCharacteristics()` | `internal/classifier/content_type.go` |

Keep: `ContentTypeArticle = "article"` (subtype value, correct by design).

#### Source-Manager

Keep as-is: `ArticleSelectors`, `extractArticleSelectors()`, `extractArticleTitle()`, `extractArticleBody()` — article-specific extraction, correctly named.

#### MCP (mcp-north-cloud)

| Current | New |
|---|---|
| `PreviewArticle` | `PreviewItem` |
| `handleSearchArticles()` | `handleSearchContent()` |
| `handleClassifyArticle()` | `handleClassifyContent()` |
| Tool name `search_articles` | `search_content` |
| Tool name `classify_article` | `classify_content` |

#### Dashboard

| Current | New |
|---|---|
| `RecentArticle` | `RecentItem` |
| `RecentArticlesResponse` | `RecentItemsResponse` |
| `PreviewArticle` | `PreviewItem` |
| `GroupedArticle` | `GroupedItem` |
| `ArticlesFilterBar.vue` | `ContentFilterBar.vue` |
| `ArticlesView.vue` | `ContentView.vue` |
| `components/domain/articles/` | `components/domain/content/` |
| Route `/operations/articles` | `/operations/content` |
| Nav title `"Recent Articles"` | `"Recent Content"` |
| All `article_id`/`article_title`/`article_url` refs | `content_id`/`content_title`/`content_url` |
| `total_articles` | `total_items` |
| `articles_found`/`articles_indexed` | `items_found`/`items_indexed` |
| `crawlerApi.articles.list()` | `crawlerApi.content.list()` |
| `publisherApi.articles.recent()` | `publisherApi.content.recent()` |
| `publisherApi.history.getByArticle()` | `publisherApi.history.getByContent()` |

#### Grafana Alerts

| Current | New |
|---|---|
| `"No articles classified in the last 2 hours"` | `"No content classified in the last 2 hours"` |
| `"No articles published in 4 hours"` | `"No content published in 4 hours"` |
| `"Publisher has not pushed any articles to Redis"` | `"Publisher has not pushed any content to Redis"` |

---

## Phase 2: API Endpoints

| Service | New Endpoint | Replaces |
|---|---|---|
| Publisher | `GET /api/v1/content/recent` | `GET /api/v1/articles/recent` |
| Publisher | `GET /api/v1/publish-history/:content_id` | `GET /api/v1/publish-history/:article_id` |
| Crawler | `GET /api/v1/content` | `GET /api/v1/articles` |
| Search | `GET /feed.json` with `"items"` key | Currently `"articles"` key |

Old endpoints are removed (no shims). Streetcode uses Redis, the dashboard deploys with north-cloud.

---

## Phase 3: Dashboard

See rename inventory above. All changes deploy atomically with the Go services.

---

## Phase 4: Redis Channel Renames

| Current | New |
|---|---|
| `articles:{topic}` | `content:{topic}` |
| `articles:crime` | `content:crime` |
| `articles:mining` | `content:mining` |
| `articles:anishinaabe` | `content:anishinaabe` |
| `articles:jobs` | `content:jobs` |
| `articles:recipes` | `content:recipes` |

Domain-specific channels unchanged: `crime:*`, `mining:*`, `entertainment:*`, `anishinaabe:*`, `coforge:*`.

No fan-out layer. Streetcode cuts over to `content:crime` before north-cloud deploys.

---

## Phase 5: Database Migrations

### Publisher — `publish_history`

**Step 1: Add new columns**

```sql
ALTER TABLE publish_history ADD COLUMN content_id VARCHAR(255);
ALTER TABLE publish_history ADD COLUMN content_title TEXT;
ALTER TABLE publish_history ADD COLUMN content_url TEXT;
CREATE INDEX idx_publish_history_content ON publish_history (content_id);
CREATE INDEX idx_publish_history_content_channel ON publish_history (content_id, channel_name);
```

**Step 2: Backfill**

```sql
UPDATE publish_history SET
    content_id = article_id,
    content_title = article_title,
    content_url = article_url
WHERE content_id IS NULL;
ALTER TABLE publish_history ALTER COLUMN content_id SET NOT NULL;
```

**Step 3: Dual-write** (application code writes both column sets; reads use new columns)

**Step 4: Drop old columns**

```sql
DROP INDEX IF EXISTS idx_publish_history_article;
DROP INDEX IF EXISTS idx_publish_history_article_channel;
ALTER TABLE publish_history DROP COLUMN article_id;
ALTER TABLE publish_history DROP COLUMN article_title;
ALTER TABLE publish_history DROP COLUMN article_url;
```

### Pipeline — `articles` -> `content_items`

**Step 1: Create new table and copy**

```sql
CREATE TABLE content_items (
    url TEXT PRIMARY KEY,
    url_hash VARCHAR(64) NOT NULL,
    domain VARCHAR(255) NOT NULL,
    source_name VARCHAR(255) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_content_items_source ON content_items (source_name);
CREATE INDEX idx_content_items_hash ON content_items (url_hash);
CREATE INDEX idx_content_items_domain ON content_items (domain);
INSERT INTO content_items SELECT * FROM articles;
```

**Step 2: Add `content_url` to `pipeline_events`**

```sql
ALTER TABLE pipeline_events ADD COLUMN content_url TEXT;
UPDATE pipeline_events SET content_url = article_url;
ALTER TABLE pipeline_events ALTER COLUMN content_url SET NOT NULL;
ALTER TABLE pipeline_events ADD CONSTRAINT fk_events_content_item
    FOREIGN KEY (content_url) REFERENCES content_items(url);
CREATE INDEX idx_events_content ON pipeline_events (content_url);
```

**Step 3: Drop old references**

```sql
ALTER TABLE pipeline_events DROP CONSTRAINT IF EXISTS fk_events_article;
DROP INDEX IF EXISTS idx_events_article;
ALTER TABLE pipeline_events DROP COLUMN article_url;
DROP TABLE articles;
```

### Classifier — `source_reputation`

```sql
-- Step 1
ALTER TABLE source_reputation ADD COLUMN total_items INT DEFAULT 0;
UPDATE source_reputation SET total_items = total_articles;

-- Step 2 (after dual-write period)
ALTER TABLE source_reputation DROP COLUMN total_articles;
```

### Index-Manager

Remove legacy `index_types.article` entry from `config.yml` (already `auto_create: false`).

---

## Phase 6: Cleanup

- Update `ARCHITECTURE.md` — replace "article routing" terminology with "content routing"
- Update all service `CLAUDE.md` files
- Update `publisher/docs/REDIS_MESSAGE_FORMAT.md` and `CONSUMER_GUIDE.md`
- Update Grafana alert descriptions
- Remove any TODO comments referencing the migration
