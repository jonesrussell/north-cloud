# Bing-Style News Portal Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the search-frontend homepage from a search-bar-centric layout into a visual, discovery-focused news portal inspired by Bing's news homepage.

**Architecture:** The backend search service gets `og_image` added to feed and search responses. The frontend homepage is redesigned with news card grids organized by publisher channel (Crime, Mining, Entertainment), plus a "Top Stories" hero section. The existing search results page is preserved with thumbnail enhancement.

**Tech Stack:** Go (search service backend), Vue 3 + TypeScript + Tailwind CSS (search-frontend SPA)

**Design doc:** `docs/plans/2026-02-26-bing-news-portal-design.md`

---

## Task 1: Add `og_image` to Backend Feed Response

**Files:**
- Modify: `search/internal/domain/content.go:11-34` (add OGImage to ClassifiedContent)
- Modify: `search/internal/domain/search.go:75-91` (add OGImage to SearchHit)
- Modify: `search/internal/domain/search.go:264-275` (add OGImage to PublicFeedItem)
- Modify: `search/internal/domain/content.go:44-67` (update ToSearchHit to include OGImage)
- Modify: `search/internal/service/search_service.go:398-421` (add og_image to LatestItems _source)
- Modify: `search/internal/service/search_service.go:437-481` (add og_image to TopicFeed _source)
- Modify: `search/internal/service/search_service.go:483-538` (parse og_image in parseLatestItemsResponse)
- Modify: `search/internal/elasticsearch/query_builder.go:63-68` (add og_image to default _source)
- Test: `search/internal/service/feed_test.go` (existing tests should still pass)

**Step 1: Add OGImage field to ClassifiedContent struct**

In `search/internal/domain/content.go`, add after line 19 (OGDescription):

```go
OGImage         string           `json:"og_image,omitempty"`
```

**Step 2: Update ToSearchHit to pass OGImage**

In `search/internal/domain/content.go`, the `ToSearchHit` method (line 52-66) — add to the returned struct:

```go
OGImage:        c.OGImage,
```

**Step 3: Add OGImage field to SearchHit struct**

In `search/internal/domain/search.go`, add after line 90 (ClickURL):

```go
OGImage        string              `json:"og_image,omitempty"`
```

**Step 4: Add OGImage field to PublicFeedItem struct**

In `search/internal/domain/search.go`, add after line 274 (Source):

```go
OGImage     string    `json:"og_image,omitempty"`
```

**Step 5: Add og_image to feed ES source fields**

In `search/internal/service/search_service.go`, update the `_source` arrays:

Line 408-411 (LatestItems):
```go
"_source": []string{
    "id", "title", "url", "source_name",
    "published_date", "crawled_at", "raw_text", "topics",
    "og_image",
},
```

Line 466-469 (TopicFeed):
```go
"_source": []string{
    "id", "title", "url", "source_name",
    "published_date", "crawled_at", "raw_text", "topics",
    "og_image",
},
```

**Step 6: Parse og_image in parseLatestItemsResponse**

In `search/internal/service/search_service.go`, update the inline struct inside `parseLatestItemsResponse` (line 489-498) — add the field:

```go
OGImage       string     `json:"og_image"`
```

Then in the `out = append(out, domain.PublicFeedItem{...})` block (line 526-535), add:

```go
OGImage:     hit.Source.OGImage,
```

**Step 7: Add og_image to default search _source fields**

In `search/internal/elasticsearch/query_builder.go` line 63-68:

```go
query["_source"] = []string{
    "id", "title", "url", "source_name",
    "published_date", "crawled_at",
    "quality_score", "content_type", "topics",
    "crime", "body", "raw_text",
    "og_image",
}
```

**Step 8: Run existing tests**

Run: `cd search && GOWORK=off go test ./...`
Expected: All tests pass (feed_test.go tests the slug→topics mapping, not the struct fields)

**Step 9: Run linter**

Run: `cd search && golangci-lint run`
Expected: No new violations

**Step 10: Commit**

```bash
git add search/internal/domain/content.go search/internal/domain/search.go \
  search/internal/service/search_service.go search/internal/elasticsearch/query_builder.go
git commit -m "feat(search): add og_image to feed and search hit responses"
```

---

## Task 2: Add Feed Types and API Client to Frontend

**Files:**
- Modify: `search-frontend/src/types/search.ts` (add FeedItem interface)
- Modify: `search-frontend/src/api/search.ts` (add feed API methods)
- Modify: `search-frontend/vite.config.ts` (add feed proxy route)

**Step 1: Add FeedItem type**

In `search-frontend/src/types/search.ts`, add after the SearchResult interface (after line 24):

```typescript
/**
 * Public feed item from /feed.json and /feed/{slug}.json endpoints
 */
export interface FeedItem {
  id: string
  title: string
  slug: string
  url: string
  snippet: string
  published_at: string
  topics: string[]
  source: string
  og_image?: string
}

/**
 * Public feed response
 */
export interface FeedResponse {
  generated_at: string
  count: number
  items: FeedItem[]
}
```

**Step 2: Add feed proxy to Vite config**

In `search-frontend/vite.config.ts`, add a new proxy entry in the `proxy` object (after line 29):

```typescript
// Public feed endpoints
'/feed': {
  target: SEARCH_API_URL,
  changeOrigin: true,
  rewrite: (path: string) => `/api/v1/feeds${path.replace(/^\/feed/, '').replace(/\.json$/, '')}`,
} as ProxyOptions,
```

Wait — need to check the actual feed endpoint paths. The backend serves feeds at `/api/v1/feeds/` and `/api/v1/feeds/:slug`. The public JSON feeds are at `/feed.json` and `/feed/{slug}.json`. Let me check:

Actually, looking at the search backend routes, the feeds are served at:
- `GET /feed.json` → `handler.LatestFeed`
- `GET /api/v1/feeds/:slug` → `handler.TopicFeed`

The vite proxy needs to forward:
- `/feed.json` → `SEARCH_API_URL/feed.json`
- `/feed/crime.json` → `SEARCH_API_URL/api/v1/feeds/crime`

Let me simplify: proxy `/feed` prefix to the search service directly.

```typescript
// Public feed endpoints (latest feed + topic feeds)
'/feed.json': {
  target: SEARCH_API_URL,
  changeOrigin: true,
} as ProxyOptions,
'/feed/': {
  target: SEARCH_API_URL,
  changeOrigin: true,
  rewrite: (path: string) => {
    const slug = path.replace(/^\/feed\//, '').replace(/\.json$/, '')
    return `/api/v1/feeds/${slug}`
  },
} as ProxyOptions,
```

**Step 3: Add feed methods to API client**

In `search-frontend/src/api/search.ts`, add after the existing `searchApi` object (before `export default`):

```typescript
import type { FeedResponse } from '@/types/search'

const feedClient: AxiosInstance = axios.create({
  timeout: 10000,
})

export const feedApi = {
  latest: (): Promise<AxiosResponse<FeedResponse>> => {
    return feedClient.get<FeedResponse>('/feed.json')
  },

  byTopic: (slug: string): Promise<AxiosResponse<FeedResponse>> => {
    return feedClient.get<FeedResponse>(`/feed/${slug}.json`)
  },
}
```

Also update the import at the top of the file to include `FeedResponse`:

```typescript
import type { SearchRequest, SearchResponse, SuggestResponse, FeedResponse } from '@/types/search'
```

**Step 4: Verify build**

Run: `cd search-frontend && npm run build`
Expected: Build succeeds with no TypeScript errors

**Step 5: Commit**

```bash
git add search-frontend/src/types/search.ts search-frontend/src/api/search.ts \
  search-frontend/vite.config.ts
git commit -m "feat(search-frontend): add feed types and API client"
```

---

## Task 3: Create NewsCard Component

**Files:**
- Create: `search-frontend/src/components/news/NewsCard.vue`

**Step 1: Create the NewsCard component**

Create `search-frontend/src/components/news/NewsCard.vue`:

```vue
<template>
  <a
    :href="item.url"
    class="group block rounded-lg overflow-hidden bg-[var(--nc-bg-elevated)] shadow-[var(--nc-shadow-sm)] hover:shadow-[var(--nc-shadow)] transition-shadow duration-[var(--nc-duration)]"
    target="_blank"
    rel="noopener noreferrer"
  >
    <!-- Thumbnail -->
    <div class="aspect-video overflow-hidden bg-[var(--nc-bg-muted)]">
      <img
        v-if="item.og_image"
        :src="item.og_image"
        :alt="item.title"
        class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-[var(--nc-duration-slow)]"
        loading="lazy"
        @error="onImageError"
      >
      <div
        v-else
        class="w-full h-full flex items-center justify-center"
        :class="fallbackClass"
      >
        <span class="text-2xl font-display font-normal opacity-30 text-white select-none">
          {{ fallbackLabel }}
        </span>
      </div>
    </div>

    <!-- Content -->
    <div class="p-4">
      <!-- Source + time -->
      <div class="flex items-center gap-1.5 text-xs text-[var(--nc-text-muted)] mb-2">
        <span class="font-medium">{{ item.source }}</span>
        <span aria-hidden="true">&middot;</span>
        <time :datetime="item.published_at">{{ relativeTime }}</time>
      </div>

      <!-- Headline -->
      <h3 class="font-semibold text-[var(--nc-text)] leading-snug line-clamp-2 mb-1.5 group-hover:text-[var(--nc-primary)] transition-colors duration-[var(--nc-duration)]">
        {{ item.title }}
      </h3>

      <!-- Snippet -->
      <p
        v-if="showSnippet"
        class="text-sm text-[var(--nc-text-secondary)] line-clamp-1"
      >
        {{ truncatedSnippet }}
      </p>
    </div>
  </a>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { FeedItem } from '@/types/search'
import { formatRelativeTime } from '@/utils/dateFormatter'

interface Props {
  item: FeedItem
  showSnippet?: boolean
  channelColor?: string
}

const props = withDefaults(defineProps<Props>(), {
  showSnippet: true,
  channelColor: 'bg-[var(--nc-primary)]',
})

const imageErrored = ref(false)

const relativeTime = computed(() => formatRelativeTime(props.item.published_at))

const truncatedSnippet = computed(() => {
  const maxLength = 120
  if (!props.item.snippet) return ''
  if (props.item.snippet.length <= maxLength) return props.item.snippet
  return props.item.snippet.slice(0, maxLength) + '...'
})

const fallbackLabel = computed(() => {
  if (props.item.topics.length > 0) return props.item.topics[0]
  return props.item.source
})

const fallbackClass = computed(() => {
  if (imageErrored.value || !props.item.og_image) return props.channelColor
  return props.channelColor
})

function onImageError(): void {
  imageErrored.value = true
}
</script>
```

**Step 2: Verify lint**

Run: `cd search-frontend && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add search-frontend/src/components/news/NewsCard.vue
git commit -m "feat(search-frontend): add NewsCard component"
```

---

## Task 4: Create ChannelSection Component

**Files:**
- Create: `search-frontend/src/components/news/ChannelSection.vue`

**Step 1: Create the ChannelSection component**

Create `search-frontend/src/components/news/ChannelSection.vue`:

```vue
<template>
  <section class="channel-section">
    <!-- Section heading -->
    <div class="flex items-center justify-between mb-5">
      <h2 class="font-display text-2xl sm:text-3xl font-normal text-[var(--nc-text)]">
        {{ title }}
      </h2>
      <router-link
        :to="seeMoreLink"
        class="text-sm font-medium text-[var(--nc-primary)] hover:text-[var(--nc-primary-hover)] transition-colors duration-[var(--nc-duration)]"
      >
        See more &rarr;
      </router-link>
    </div>

    <!-- Loading skeleton -->
    <div
      v-if="loading"
      class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5"
    >
      <div
        v-for="n in skeletonCount"
        :key="n"
        class="rounded-lg overflow-hidden bg-[var(--nc-bg-elevated)] animate-pulse"
      >
        <div class="aspect-video bg-[var(--nc-bg-muted)]" />
        <div class="p-4 space-y-2">
          <div class="h-3 w-24 bg-[var(--nc-bg-muted)] rounded" />
          <div class="h-4 w-full bg-[var(--nc-bg-muted)] rounded" />
          <div class="h-4 w-3/4 bg-[var(--nc-bg-muted)] rounded" />
        </div>
      </div>
    </div>

    <!-- Error state -->
    <div
      v-else-if="error"
      class="text-center py-8 text-[var(--nc-text-muted)]"
    >
      <p class="text-sm">Could not load {{ title.toLowerCase() }} articles.</p>
    </div>

    <!-- News cards grid -->
    <div
      v-else-if="items.length > 0"
      class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5"
    >
      <NewsCard
        v-for="item in items"
        :key="item.id"
        :item="item"
        :channel-color="channelColor"
      />
    </div>

    <!-- Empty state -->
    <div
      v-else
      class="text-center py-8 text-[var(--nc-text-muted)]"
    >
      <p class="text-sm">No {{ title.toLowerCase() }} articles yet.</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { FeedItem } from '@/types/search'
import NewsCard from './NewsCard.vue'

interface Props {
  title: string
  slug: string
  items: FeedItem[]
  loading?: boolean
  error?: boolean
  channelColor?: string
}

const skeletonCount = 6

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  error: false,
  channelColor: 'bg-[var(--nc-primary)]',
})

const seeMoreLink = computed(() => ({
  path: '/search',
  query: { topics: props.slug },
}))
</script>
```

**Step 2: Verify lint**

Run: `cd search-frontend && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add search-frontend/src/components/news/ChannelSection.vue
git commit -m "feat(search-frontend): add ChannelSection component"
```

---

## Task 5: Create useFeed Composable

**Files:**
- Create: `search-frontend/src/composables/useFeed.ts`

**Step 1: Create the useFeed composable**

Create `search-frontend/src/composables/useFeed.ts`:

```typescript
import { ref, onMounted } from 'vue'
import type { FeedItem } from '@/types/search'
import { feedApi } from '@/api/search'
import axios from 'axios'

interface UseFeedReturn {
  items: ReturnType<typeof ref<FeedItem[]>>
  loading: ReturnType<typeof ref<boolean>>
  error: ReturnType<typeof ref<boolean>>
  refresh: () => Promise<void>
}

export function useFeed(slug?: string): UseFeedReturn {
  const items = ref<FeedItem[]>([])
  const loading = ref(true)
  const error = ref(false)

  async function refresh(): Promise<void> {
    loading.value = true
    error.value = false
    try {
      const response = slug
        ? await feedApi.byTopic(slug)
        : await feedApi.latest()
      items.value = response.data.items ?? []
    } catch (err: unknown) {
      error.value = true
      if (axios.isAxiosError(err)) {
        console.error(`[Feed] Error loading ${slug ?? 'latest'}:`, err.message)
      }
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void refresh()
  })

  return { items, loading, error, refresh }
}
```

**Step 2: Verify lint**

Run: `cd search-frontend && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add search-frontend/src/composables/useFeed.ts
git commit -m "feat(search-frontend): add useFeed composable"
```

---

## Task 6: Redesign the Header with Compact Search Bar

**Files:**
- Modify: `search-frontend/src/App.vue`

**Step 1: Replace the App.vue header**

Replace the entire `search-frontend/src/App.vue` with a persistent header that includes a compact search bar on all pages. The logo always shows (remove the conditional), and a search input appears in the header on non-home pages:

```vue
<template>
  <div class="min-h-screen flex flex-col">
    <header class="sticky top-0 z-30 bg-[var(--nc-bg-elevated)]/95 backdrop-blur-sm border-b border-[var(--nc-border)]">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center h-14 sm:h-16 gap-4">
          <router-link
            to="/"
            class="flex-shrink-0 flex items-center gap-2 text-[var(--nc-text)] hover:text-[var(--nc-primary)] transition-colors duration-[var(--nc-duration)]"
            aria-label="North Cloud — Home"
          >
            <span class="font-display text-xl sm:text-2xl font-normal tracking-tight">
              North Cloud
            </span>
          </router-link>

          <!-- Compact search bar (visible on all pages) -->
          <form
            class="flex-1 max-w-lg"
            @submit.prevent="handleHeaderSearch"
          >
            <div class="relative">
              <svg
                class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--nc-text-muted)]"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
              <input
                v-model="headerQuery"
                type="search"
                placeholder="Search articles..."
                class="w-full rounded-full bg-[var(--nc-bg-muted)] border border-[var(--nc-border)] pl-10 pr-4 py-2 text-sm text-[var(--nc-text)] placeholder:text-[var(--nc-text-muted)] focus:outline-none focus:border-[var(--nc-primary)] focus:ring-1 focus:ring-[var(--nc-primary)] transition-colors duration-[var(--nc-duration)]"
              >
            </div>
          </form>
        </div>
      </div>
    </header>

    <main class="flex-1">
      <router-view />
    </main>

    <footer class="mt-auto border-t border-[var(--nc-border)] bg-[var(--nc-bg-elevated)]">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <p class="text-center text-sm text-[var(--nc-text-muted)]">
          &copy; {{ currentYear }} North Cloud. All rights reserved.
        </p>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const currentYear = computed(() => new Date().getFullYear())
const headerQuery = ref('')

function handleHeaderSearch(): void {
  const q = headerQuery.value.trim()
  if (q) {
    router.push({ path: '/search', query: { q } })
    headerQuery.value = ''
  }
}
</script>
```

Key changes:
- Logo always shows (no conditional based on route)
- Max width increased from `max-w-6xl` to `max-w-7xl` for wider card grids
- Compact pill-shaped search input in header
- "Advanced search" link removed from header (accessible via /advanced URL still)

**Step 2: Verify build**

Run: `cd search-frontend && npm run build`
Expected: Builds successfully

**Step 3: Commit**

```bash
git add search-frontend/src/App.vue
git commit -m "feat(search-frontend): redesign header with compact search bar"
```

---

## Task 7: Redesign HomeView as News Portal

**Files:**
- Modify: `search-frontend/src/views/HomeView.vue`

**Step 1: Replace HomeView with news portal layout**

Replace the entire `search-frontend/src/views/HomeView.vue`:

```vue
<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-12">
    <!-- Top Stories -->
    <ChannelSection
      title="Top Stories"
      slug=""
      :items="topStories.items.value"
      :loading="topStories.loading.value"
      :error="topStories.error.value"
    />

    <!-- Channel sections -->
    <ChannelSection
      v-for="channel in channels"
      :key="channel.slug"
      :title="channel.title"
      :slug="channel.slug"
      :items="channel.feed.items.value"
      :loading="channel.feed.loading.value"
      :error="channel.feed.error.value"
      :channel-color="channel.color"
    />
  </div>
</template>

<script setup lang="ts">
import ChannelSection from '@/components/news/ChannelSection.vue'
import { useFeed } from '@/composables/useFeed'

const topStories = useFeed()

const channels = [
  {
    title: 'Crime',
    slug: 'crime',
    color: 'bg-[var(--nc-error)]',
    feed: useFeed('crime'),
  },
  {
    title: 'Mining',
    slug: 'mining',
    color: 'bg-[var(--nc-accent)]',
    feed: useFeed('mining'),
  },
  {
    title: 'Entertainment',
    slug: 'entertainment',
    color: 'bg-[var(--nc-primary)]',
    feed: useFeed('entertainment'),
  },
]
</script>
```

**Step 2: Update router meta title**

In `search-frontend/src/router/index.ts` line 14, change the home route meta:

```typescript
meta: { title: 'News' },
```

**Step 3: Verify build**

Run: `cd search-frontend && npm run build`
Expected: Builds successfully

**Step 4: Run linter**

Run: `cd search-frontend && npm run lint`
Expected: No errors

**Step 5: Commit**

```bash
git add search-frontend/src/views/HomeView.vue search-frontend/src/router/index.ts
git commit -m "feat(search-frontend): redesign homepage as news portal"
```

---

## Task 8: Add og_image to SearchResultItem

**Files:**
- Modify: `search-frontend/src/types/search.ts:4-24` (add og_image to SearchResult)
- Modify: `search-frontend/src/components/search/SearchResultItem.vue`

**Step 1: Add og_image to SearchResult type**

In `search-frontend/src/types/search.ts`, add after line 9 (click_url):

```typescript
og_image?: string
```

**Step 2: Add thumbnail to SearchResultItem**

In `search-frontend/src/components/search/SearchResultItem.vue`, add an optional thumbnail image before the title if `og_image` is present. This enhances search results without breaking the existing layout. Add a small thumbnail (64x64 or 80x80) to the left of the result text.

Read the existing component first, then add the image conditionally. The exact implementation depends on the current layout — wrap the content area in a flex container with the optional thumbnail on the left.

**Step 3: Verify build**

Run: `cd search-frontend && npm run build`
Expected: Builds successfully

**Step 4: Commit**

```bash
git add search-frontend/src/types/search.ts search-frontend/src/components/search/SearchResultItem.vue
git commit -m "feat(search-frontend): add og_image thumbnail to search results"
```

---

## Task 9: Manual End-to-End Verification

**Step 1: Start backend services**

Run: `task docker:dev:up` (or the appropriate docker compose command)
Verify the search service is healthy: `curl http://localhost:8092/health`

**Step 2: Verify feed endpoints return og_image**

Run: `curl -s http://localhost:8092/feed.json | jq '.items[0]'`
Expected: Response includes `og_image` field (may be empty string if article has no og:image)

Run: `curl -s http://localhost:8092/api/v1/feeds/crime | jq '.items[0]'`
Expected: Same structure with `og_image` field

**Step 3: Start frontend dev server**

Run: `cd search-frontend && npm run dev`
Open: `http://localhost:3003`

**Step 4: Verify homepage**

Expected:
- Header with North Cloud logo + compact search bar
- "Top Stories" section with 3-column grid of news cards
- Crime, Mining, Entertainment sections each with card grids
- Cards show og:image thumbnails (or gradient fallback)
- "See more" links work and navigate to search page

**Step 5: Verify search still works**

- Type a query in the header search bar
- Press Enter
- Expected: Navigates to `/search?q=...` with results
- Results should show thumbnail images where available

**Step 6: Run linter (both services)**

Run: `task lint:search` and `cd search-frontend && npm run lint`
Expected: No violations

**Step 7: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: address issues found during manual verification"
```
