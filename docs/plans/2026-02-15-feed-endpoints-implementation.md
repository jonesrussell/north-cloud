# Feed Endpoints Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 4 topic-filtered public feed endpoints to the search service and update the "me" site to consume them with per-page content differentiation.

**Architecture:** Extend the existing `LatestArticles` pattern in the search service with a `TopicFeed` method that accepts topic + quality filters. Add routes at `/api/v1/feeds/{topic}`. On the "me" side, update `northcloud-service.ts` to accept a feed slug, then update homepage and projects page loaders.

**Tech Stack:** Go 1.24+ (search service), Gin HTTP framework, Elasticsearch, SvelteKit 5 (me site), TypeScript

---

### Task 1: Add TopicFeed service method (search service)

**Files:**
- Modify: `search/internal/service/search_service.go`
- Test: `search/internal/service/search_service_test.go` (create if needed)

**Step 1: Write the failing test**

Create `search/internal/service/feed_test.go`:

```go
package service

import (
	"testing"
)

func TestFeedTopicFilter(t *testing.T) {
	t.Helper()
	// Verify that valid feed slugs map to correct topic filters
	tests := []struct {
		name       string
		slug       string
		wantTopics []string
		wantMin    int
	}{
		{"pipeline returns nil topics", "pipeline", nil, 60},
		{"crime returns crime topics", "crime", []string{"violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice"}, 50},
		{"mining returns mining topic", "mining", []string{"mining"}, 50},
		{"entertainment returns entertainment topic", "entertainment", []string{"entertainment"}, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topics, minQuality := feedFilterForSlug(tt.slug)
			if len(topics) != len(tt.wantTopics) {
				t.Errorf("feedFilterForSlug(%q) topics = %v, want %v", tt.slug, topics, tt.wantTopics)
			}
			if minQuality != tt.wantMin {
				t.Errorf("feedFilterForSlug(%q) minQuality = %d, want %d", tt.slug, minQuality, tt.wantMin)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd search && go test ./internal/service/ -run TestFeedTopicFilter -v`
Expected: FAIL — `feedFilterForSlug` not defined

**Step 3: Implement feedFilterForSlug and TopicFeed**

Add to `search/internal/service/search_service.go`:

```go
const (
	pipelineFeedMinQuality      = 60
	topicFeedMinQuality         = 50
	pipelineFeedPerTopicLimit   = 2
	defaultFeedLimit            = 10
	maxFeedLimit                = 20
)

// feedFilterForSlug returns the topic filter and min quality for a feed slug.
func feedFilterForSlug(slug string) (topics []string, minQuality int) {
	switch slug {
	case "crime":
		return []string{"violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice"}, topicFeedMinQuality
	case "mining":
		return []string{"mining"}, topicFeedMinQuality
	case "entertainment":
		return []string{"entertainment"}, topicFeedMinQuality
	default: // "pipeline"
		return nil, pipelineFeedMinQuality
	}
}

// TopicFeed returns recent articles filtered by feed slug (crime, mining, entertainment, pipeline).
func (s *SearchService) TopicFeed(ctx context.Context, slug string, limit int) ([]domain.PublicFeedArticle, error) {
	if limit <= 0 || limit > maxFeedLimit {
		limit = defaultFeedLimit
	}

	topics, minQuality := feedFilterForSlug(slug)

	filters := []map[string]any{
		{"term": map[string]any{"content_type.keyword": "article"}},
		{"range": map[string]any{"quality_score": map[string]any{"gte": minQuality}}},
	}
	if len(topics) > 0 {
		filters = append(filters, map[string]any{
			"terms": map[string]any{"topics.keyword": topics},
		})
	}

	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{"filter": filters},
		},
		"size": limit,
		"sort": []any{
			map[string]any{"published_date": map[string]any{"order": "desc", "missing": "_last"}},
			map[string]any{"crawled_at": map[string]any{"order": "desc", "missing": "_last"}},
		},
		"_source": []string{
			"id", "title", "url", "source_name",
			"published_date", "crawled_at", "raw_text", "topics",
		},
	}

	res, err := s.executeSearch(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	return s.parseLatestArticlesResponse(res.Body)
}
```

**Step 4: Run test to verify it passes**

Run: `cd search && go test ./internal/service/ -run TestFeedTopicFilter -v`
Expected: PASS

**Step 5: Run linter**

Run: `cd search && golangci-lint run ./internal/service/`
Expected: No errors

**Step 6: Commit**

```bash
git add search/internal/service/search_service.go search/internal/service/feed_test.go
git commit -m "feat(search): add TopicFeed service method with slug-based filtering"
```

---

### Task 2: Add feed handler and routes (search service)

**Files:**
- Modify: `search/internal/api/handlers.go`
- Modify: `search/internal/api/server.go`

**Step 1: Add the TopicFeed handler**

Add to `search/internal/api/handlers.go`:

```go
// TopicFeed serves a topic-filtered public feed. Slug comes from URL param.
// Public endpoint (no auth), 5-minute cache.
func (h *Handler) TopicFeed(c *gin.Context) {
	slug := c.Param("slug")
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}

	articles, feedErr := h.searchService.TopicFeed(c.Request.Context(), slug, limit)
	if feedErr != nil {
		h.logger.Error("Topic feed failed",
			infralogger.Error(feedErr),
			infralogger.String("slug", slug),
		)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:     "Feed temporarily unavailable",
			Code:      "FEED_ERROR",
			Timestamp: time.Now(),
		})
		return
	}
	c.Header("Cache-Control", "public, max-age="+strconv.Itoa(publicFeedCacheMaxAge))
	c.JSON(http.StatusOK, domain.PublicFeedResponse{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Articles:    articles,
	})
}
```

**Step 2: Register the route**

In `search/internal/api/server.go`, inside `SetupServiceRoutes`, add after the `/feed.json` line:

```go
	// Topic-filtered feeds (no auth): /api/v1/feeds/{slug}
	feeds := v1.Group("/feeds")
	feeds.GET("/:slug", handler.TopicFeed)
```

**Step 3: Run linter**

Run: `cd search && golangci-lint run`
Expected: No errors

**Step 4: Commit**

```bash
git add search/internal/api/handlers.go search/internal/api/server.go
git commit -m "feat(search): add /api/v1/feeds/:slug endpoint for topic-filtered feeds"
```

---

### Task 3: Add nginx route for feeds (production)

**Files:**
- Modify: `infrastructure/nginx/nginx.conf`
- Modify: `infrastructure/nginx/nginx.dev.conf`

**Step 1: Add feeds location to nginx.conf**

After the existing `/feed.json` block (around line 257), add:

```nginx
        # Topic-filtered feeds (no auth)
        location /api/v1/feeds/ {
            proxy_pass http://$search_api;
            proxy_http_version 1.1;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_read_timeout 60s;
            proxy_connect_timeout 10s;
        }
```

**Step 2: Add the same block to nginx.dev.conf**

Add the same location block in the equivalent position in `nginx.dev.conf`.

**Step 3: Commit**

```bash
git add infrastructure/nginx/nginx.conf infrastructure/nginx/nginx.dev.conf
git commit -m "feat(nginx): route /api/v1/feeds/ to search service"
```

---

### Task 4: Update northcloud-service.ts to support multiple feeds ("me" site)

**Files:**
- Modify: `/home/fsd42/dev/me/src/lib/services/northcloud-service.ts`

**Step 1: Update the service to accept a feed slug**

Replace the entire file:

```typescript
import type { NorthCloudArticle, NorthCloudFeedResponse } from '$lib/types/northcloud';

const FEED_BASE_URL = 'https://northcloud.biz/api/v1/feeds';
const CACHE_DURATION = 1000 * 60 * 30; // 30 minutes

interface FeedCache {
	data: NorthCloudArticle[];
	timestamp: number;
	errorCount: number;
	lastError?: string;
}

const feedCache = (() => {
	const cache = new Map<string, FeedCache>();

	const getCache = (key: string): FeedCache | null => {
		const cached = cache.get(key);
		if (cached && Date.now() - cached.timestamp < CACHE_DURATION) {
			return cached;
		}
		cache.delete(key);
		return null;
	};

	const updateCache = (
		key: string,
		data: NorthCloudArticle[],
		errorCount: number = 0,
		lastError?: string
	) => {
		cache.set(key, {
			data,
			timestamp: Date.now(),
			errorCount,
			lastError
		});
	};

	const resetCache = () => {
		cache.clear();
	};

	return { getCache, updateCache, resetCache };
})();

function normalizeArticle(
	a: NorthCloudFeedResponse['articles'][number]
): NorthCloudArticle {
	return {
		id: a.id,
		title: a.title,
		url: a.url,
		snippet: a.snippet,
		published: new Date(a.published_at),
		topics: a.topics ?? [],
		source: a.source ?? 'northcloud'
	};
}

/**
 * Fetches a North Cloud topic feed. Uses in-memory cache (30 min TTL).
 * @param fetchFn - SvelteKit fetch for SSR compatibility
 * @param slug - Feed slug: 'pipeline', 'crime', 'mining', 'entertainment'
 * @param limit - Max articles to return (default 10)
 */
export async function fetchNorthCloudFeed(
	fetchFn: typeof fetch,
	slug: string = 'pipeline',
	limit: number = 10
): Promise<NorthCloudArticle[]> {
	const cacheKey = `nc-feed-${slug}-${limit}`;
	const cached = feedCache.getCache(cacheKey);
	if (cached) {
		return cached.data;
	}

	const url = `${FEED_BASE_URL}/${slug}?limit=${limit}`;
	const response = await fetchFn(url, {
		headers: { Accept: 'application/json' }
	});

	if (!response.ok) {
		throw new Error(`NorthCloud feed error: ${response.status}`);
	}

	const data = (await response.json()) as NorthCloudFeedResponse;
	const articles = (data.articles ?? []).map(normalizeArticle);
	feedCache.updateCache(cacheKey, articles);
	return articles;
}
```

**Step 2: Run existing tests**

Run: `cd /home/fsd42/dev/me && npm run test:unit:run -- --project=server`
Expected: PASS (or update any northcloud service tests if they reference the old URL)

**Step 3: Commit**

```bash
cd /home/fsd42/dev/me
git add src/lib/services/northcloud-service.ts
git commit -m "feat: update northcloud service to support topic-filtered feed endpoints"
```

---

### Task 5: Update homepage to use pipeline feed ("me" site)

**Files:**
- Modify: `/home/fsd42/dev/me/src/routes/+page.ts`

**Step 1: Update the load function**

Change the feed call to pass the `pipeline` slug and limit of 6:

```typescript
let northCloudArticles: Awaited<ReturnType<typeof fetchNorthCloudFeed>> = [];
try {
	northCloudArticles = await fetchNorthCloudFeed(fetch, 'pipeline', 6);
} catch {
	// Feed optional on homepage; continue with empty list
}

return {
	youtube: YOUTUBE,
	terminalCommand: TERMINAL_COMMAND,
	specialties,
	navLinks,
	northCloudArticles
};
```

Remove the `.slice(0, 5)` since the limit is now server-side.

**Step 2: Commit**

```bash
cd /home/fsd42/dev/me
git add src/routes/+page.ts
git commit -m "feat: homepage uses pipeline feed with server-side limit"
```

---

### Task 6: Update projects page with per-project feeds ("me" site)

**Files:**
- Modify: `/home/fsd42/dev/me/src/routes/projects/+page.ts`
- Modify: `/home/fsd42/dev/me/src/routes/projects/+page.svelte`

**Step 1: Update the data loader to fetch 3 domain feeds**

Replace `projects/+page.ts`:

```typescript
import type { PageLoad } from './$types';
import { fetchNorthCloudFeed } from '$lib/services/northcloud-service';

export const prerender = true;

export const load: PageLoad = async ({ fetch }) => {
	const feedLimit = 3;

	const [crimeArticles, miningArticles, entertainmentArticles] = await Promise.all([
		fetchNorthCloudFeed(fetch, 'crime', feedLimit).catch(() => []),
		fetchNorthCloudFeed(fetch, 'mining', feedLimit).catch(() => []),
		fetchNorthCloudFeed(fetch, 'entertainment', feedLimit).catch(() => [])
	]);

	return {
		crimeArticles,
		miningArticles,
		entertainmentArticles
	};
};
```

**Step 2: Update the page component**

In `projects/+page.svelte`, replace the single `northCloudArticles` section with per-project feed sections rendered near their respective project cards. The exact placement depends on the existing layout — look for the StreetCode, OreWire, and Movies of War project cards and add a small feed list after each.

Replace the existing `{#if data.northCloudArticles?.length}` block with individual feed blocks for each consumer project. Use the same styling pattern (`.northcloud-recent-*` classes) but with the domain-specific data:

- StreetCode section: `{#each data.crimeArticles as article (article.id)}`
- OreWire section: `{#each data.miningArticles as article (article.id)}`
- Movies of War section: `{#each data.entertainmentArticles as article (article.id)}`

**Step 3: Run type check**

Run: `cd /home/fsd42/dev/me && npm run check`
Expected: No errors

**Step 4: Commit**

```bash
cd /home/fsd42/dev/me
git add src/routes/projects/+page.ts src/routes/projects/+page.svelte
git commit -m "feat: projects page shows per-project domain feeds (crime, mining, entertainment)"
```

---

### Task 7: Remove feeds from blog and resources pages ("me" site)

**Files:**
- Modify: `/home/fsd42/dev/me/src/routes/blog/+page.ts`
- Modify: `/home/fsd42/dev/me/src/routes/blog/+page.svelte`
- Modify: `/home/fsd42/dev/me/src/routes/resources/+page.ts`
- Modify: `/home/fsd42/dev/me/src/routes/resources/+page.svelte`

**Step 1: Remove feed from blog loader**

In `blog/+page.ts`:
- Remove `import { fetchNorthCloudFeed }`
- Remove the `northCloudArticles` fetch block
- Remove `northCloudArticles` from both return statements

**Step 2: Remove feed markup from blog page**

In `blog/+page.svelte`:
- Remove the `{#if data.northCloudArticles?.length}` block and its CSS

**Step 3: Remove feed from resources loader**

In `resources/+page.ts`:
- Remove `import { fetchNorthCloudFeed }`
- Remove the `northCloudArticles` fetch block
- Remove `northCloudArticles` from the return

**Step 4: Remove feed markup from resources page**

In `resources/+page.svelte`:
- Remove the `{#if data.northCloudArticles?.length}` block and its CSS

**Step 5: Run type check and tests**

Run: `cd /home/fsd42/dev/me && npm run check && npm run test:unit:run`
Expected: PASS

**Step 6: Commit**

```bash
cd /home/fsd42/dev/me
git add src/routes/blog/+page.ts src/routes/blog/+page.svelte src/routes/resources/+page.ts src/routes/resources/+page.svelte
git commit -m "refactor: remove North Cloud feed from blog and resources pages"
```

---

### Task 8: Build and deploy verification

**Step 1: Build search service**

Run: `cd search && go build -o bin/search .`
Expected: Build succeeds

**Step 2: Run all search service tests**

Run: `cd search && go test ./...`
Expected: All pass

**Step 3: Build "me" site**

Run: `cd /home/fsd42/dev/me && npm run build`
Expected: Build succeeds

**Step 4: Run "me" site tests**

Run: `cd /home/fsd42/dev/me && npm run test:unit:run`
Expected: All pass

**Step 5: Lint both**

Run: `cd search && golangci-lint run`
Run: `cd /home/fsd42/dev/me && npm run lint`
Expected: No errors
