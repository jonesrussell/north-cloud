# Search Frontend — Developer Guide

Public-facing search SPA (Vue 3 + TypeScript + Vite). No authentication. Proxies all requests to the Search service.

## Quick Reference

```bash
# Daily development
npm run dev          # Start dev server at http://localhost:3003
npm run lint         # Run ESLint
npm run lint:fix     # Auto-fix lint violations
npm run build        # Production build to dist/
npm run preview      # Preview production build locally

# Via Taskfile (caches results — re-runs only on source changes)
task dev
task lint
task build
task preview
```

Docker (from repo root):
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d search-frontend
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f search-frontend
```

## Architecture

```
src/
├── main.ts                    # Mounts app, installs router
├── App.vue                    # Root layout, router-view
├── style.css                  # CSS custom properties (--nc-*), global styles
├── router/index.ts            # Routes: /, /search, /advanced, 404
├── api/
│   └── search.ts              # Axios client — baseURL /api/search
├── views/
│   ├── HomeView.vue           # Landing: search bar + suggested topics + recent searches
│   ├── ResultsView.vue        # Results: sidebar/drawer filters, pagination, sort
│   ├── AdvancedSearchView.vue # Advanced query builder (all/any/exact/exclude words)
│   └── NotFoundView.vue       # 404
├── components/
│   ├── search/
│   │   ├── SearchBar.vue          # Input + autocomplete dropdown (API + recent)
│   │   ├── SearchResults.vue      # Result list container
│   │   ├── SearchResultItem.vue   # Individual card with Elasticsearch highlights
│   │   ├── SearchResultsSkeleton.vue  # Skeleton placeholder during fetch
│   │   ├── FilterSidebar.vue      # Desktop filter panel (topics, sources, quality, dates)
│   │   ├── FilterDrawer.vue       # Mobile slide-out filter panel
│   │   ├── FilterChips.vue        # Active-filter pills above results
│   │   ├── SearchPagination.vue   # Page navigation
│   │   ├── EmptySearchState.vue   # No-results UI
│   │   └── RelatedContent.vue     # Related content suggestions
│   └── common/
│       ├── EmptyState.vue         # Generic empty state
│       ├── ErrorAlert.vue         # Error banner
│       └── LoadingSpinner.vue     # Spinner
├── composables/
│   ├── useSearch.ts           # Core: state, API calls, URL sync, pagination
│   ├── useRecentSearches.ts   # localStorage recent searches (cap 10, deduplicated)
│   ├── useDebounce.ts         # Reactive debounce composable (default 300 ms)
│   └── useUrlParams.ts        # Generic URL query-param sync helper
├── types/
│   ├── search.ts              # SearchRequest, SearchResponse, SearchFilters, facet types
│   ├── api.ts                 # SearchApi interface, interceptor types
│   └── router.ts              # RouteMeta augmentation (title field)
└── utils/
    ├── queryBuilder.ts        # buildSearchPayload(), buildAdvancedQuery(), validateSearchForm()
    ├── dateFormatter.ts       # Date display helpers
    ├── highlightHelper.ts     # parseHighlight(), sanitizeHighlight(), truncateText()
    └── analytics.ts           # trackEvent() — fires CustomEvent north-cloud-analytics
```

## Key Concepts

### Search Flow

```
User types query
  → SearchBar debounces input (280 ms) → calls /api/search/suggest for autocomplete
  → User submits → useSearch.search() → POST /api/search → update results + facets
  → updateUrl() serialises state to query params (/search?q=...&topics=...&page=...)
```

### URL as Source of Truth

`useSearch.syncFromUrl()` reads all route query params and rebuilds state from scratch on every navigation. `updateUrl()` writes state back to the URL after each search. Shareable links are the result — bookmarking or copying the URL captures the full search state.

### Filter Architecture

Filters flow from `useSearch` (owner) down to `FilterSidebar` / `FilterDrawer` via props. Components emit `update:filters` back up. `applyFilters()` resets to page 1 then re-executes the search.

Facets (topic counts, source counts, content-type counts) are returned by the search API and passed to the filter components so counts stay in sync with the current result set.

### Highlight Sanitisation

Elasticsearch returns `<em>` tags inside highlight snippets. `sanitizeHighlight()` strips every HTML tag except `<em>` before the snippet is rendered with `v-html`. Do not remove or bypass this sanitisation.

### Analytics

`trackEvent()` in `utils/analytics.ts` dispatches a `north-cloud-analytics` CustomEvent on `window`. In development it also logs to `console.debug`. Connect a real analytics provider (GA4, Plausible, etc.) by listening to this event — no changes to component code required.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `SEARCH_API_URL` | `http://localhost:8092` | Search service URL for Vite dev proxy |
| `NODE_ENV` | `development` | Standard Node environment flag |

`SEARCH_API_URL` is read at dev-server startup by `vite.config.ts`. It is not bundled into the browser code. The browser always calls `/api/search`.

Dev proxy rewrites:

| Browser path | Rewrites to |
|---|---|
| `/api/search` (POST/GET) | `{SEARCH_API_URL}/api/v1/search` |
| `/api/search/suggest` | `{SEARCH_API_URL}/api/v1/search/suggest` |
| `/api/health/search` | `{SEARCH_API_URL}/health` |

## Common Gotchas

1. **No authentication** — This is a fully public interface. There is no login, no JWT, and no auth headers. Do not add auth middleware here.

2. **`SEARCH_API_URL` is a server-side proxy variable** — It is not a `VITE_*` variable and is not available in browser code. Configure it in `.env` or via Docker environment for the dev server; nginx handles routing in production.

3. **Facets are always requested** — `useSearch` always sends `include_facets: true`. Facet data populates the filter sidebar. If the Search service returns `null` for facets, filter panels hide gracefully via `v-if`.

4. **`sanitizeHighlight()` must stay** — Elasticsearch highlight snippets can contain arbitrary HTML from indexed content. Skipping sanitisation allows XSS.

5. **Recent searches use localStorage** — `useRecentSearches.ts` stores up to 10 entries under the key `north-cloud-search-recent`. This is per-browser; there is no server-side persistence.

6. **Autocomplete fires at 2+ characters** — `SearchBar.vue` ignores debounced queries shorter than 2 characters to avoid noise from single-character input.

7. **TypeScript strict mode is off by default** — `tsconfig.json` does not enable `strict`. Prefer explicit types and avoid `any`; use `unknown` for values of indeterminate shape.

## Testing

No automated test framework is configured. Manual testing procedure:

1. Run `npm run dev` and verify the home page renders at `http://localhost:3003`.
2. Enter a query and confirm results appear with highlights.
3. Apply a topic filter and confirm the URL updates and results refresh.
4. Navigate to `/advanced`, build a query, and confirm it routes to `/search`.
5. Run `npm run build && npm run preview` and verify the production build works identically.

## Code Patterns

### Typed component props

```typescript
// Always use defineProps<T>() with explicit interface — no PropType workarounds
interface Props {
  facets: FacetsFromApi | null
  filters: SearchFilters
}
const props = withDefaults(defineProps<Props>(), {})
```

### Emitting filter updates

```typescript
// Child never mutates props — emit a new object
emit('update:filters', { ...props.filters, topics: next })
```

### Calling the search API

```typescript
// api/search.ts exports a typed searchApi object — import and call directly
import searchApi from '@/api/search'
const response = await searchApi.search(payload)  // returns AxiosResponse<SearchResponse>
```

### Adding a new filter dimension

1. Add the field to `SearchFilters` in `src/types/search.ts`.
2. Wire it into `useSearch.ts` (`filters` ref, `syncFromUrl`, `updateUrl`, payload builder).
3. Add UI control to `FilterSidebar.vue` and `FilterDrawer.vue`.
4. Add a chip to `FilterChips.vue`.
