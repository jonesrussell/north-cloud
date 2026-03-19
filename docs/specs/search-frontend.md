# Search Frontend Specification

> Last verified: 2026-03-19 (fix: topic-only browsing now triggers search without query text)

## Purpose

Public-facing search SPA with dual modes: news portal (home page) and full-text search with faceted filtering. No authentication. Routes requests to the Search service backend via Vite dev proxy.

## File Map

| Path | Purpose |
|------|---------|
| `search-frontend/src/App.vue` | Root layout |
| `search-frontend/src/router/index.ts` | Routes (/, /search, /advanced, 404) |
| `search-frontend/src/api/search.ts` | Axios clients (searchApi, feedApi) |
| `search-frontend/src/composables/useSearch.ts` | Search state + URL sync |
| `search-frontend/src/types/search.ts` | SearchRequest, SearchResponse, SearchFilters |
| `search-frontend/src/views/` | HomeView, ResultsView, AdvancedSearchView |
| `search-frontend/src/components/` | SearchBar, SearchResults, FilterSidebar, etc. |
| `search-frontend/vite.config.ts` | Dev proxy rules |

## Interface Signatures

### API Clients

- `searchApi`: Axios instance, baseURL `/api/search`
- `feedApi`: Axios instance for public feed

### Vite Dev Proxy Rewrites

| Frontend Path | Backend Path |
|---------------|-------------|
| `/api/search/*` | `{SEARCH_API_URL}/api/v1/search/*` |
| `/feed.json` | `/api/v1/feeds/latest` |
| `/feed/{slug}.json` | `/api/v1/feeds/{slug}` |

### Search Payload

Request: `query`, `filters` (topics, content_type, quality, dates, sources), `pagination` (page, size), `sort` (field, order), `options` (highlights, facets).

Response: `hits[]`, `total_hits`, `facets` (nullable), `took`.

### URL Query Parameters (source of truth for shareable links)

`q`, `topics[]`, `content_type`, `min_quality_score`, `from_date`, `to_date`, `source_names[]`, `page`, `sort_by`, `sort_order`.

## Data Flow

```
User → SearchBar → useSearch.search()
  → POST /api/search → Vite proxy → Search service (8092)
  → Response → results[] + facets → ResultsView

Home page → feedApi → /feed.json → Vite proxy → Search service feeds API
  → Response → trending + channel grids → HomeView
```

### useSearch Composable

- Owns: query, results[], facets, totalHits, currentPage, filters
- `search()`: validate → build payload → POST → update state → `updateUrl()`
- `syncFromUrl()`: reads all route query params, rebuilds state on navigation
- URL is source of truth for shareable/bookmarkable links

## Storage / Schema

- **localStorage**: Recent searches (per-browser, 10 entry cap)
- No database — stateless SPA

## Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `SEARCH_API_URL` | http://localhost:8092 | Backend proxy target (server-side only, not bundled) |
| Dev port | 3003 | Vite dev server |

## Edge Cases

- **No authentication** — fully public SPA
- **`SEARCH_API_URL`** is a server-side proxy var (not `VITE_*`), never bundled into client JS
- **Facets** always requested; filter panels hide when null
- **`sanitizeHighlight()`** strips all HTML except `<em>` (XSS prevention for ES snippets)
- **Autocomplete** fires at 2+ characters only (280ms debounce)
- **Design system**: Obsidian Editorial theme, Source Sans 3 font, CSS custom properties (`--nc-*`)
