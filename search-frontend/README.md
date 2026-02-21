# North Cloud Search Frontend

> Public-facing search interface for querying North Cloud classified content.

## Overview

The search frontend is a Vue.js 3 single-page application that provides a Google-style search experience over the North Cloud content pipeline. It sits at the end of the pipeline — after content has been crawled, classified, and indexed — and exposes full-text search, faceted filtering, and advanced query construction to end users.

In production it is served at `https://northcloud.biz/` and proxied through nginx to the Search service at port 8090.

## Features

- **Full-text search**: Multi-field search across all classified content indexes
- **Autocomplete suggestions**: API-backed suggestions with keyboard navigation (arrow keys, Enter, Escape)
- **Recent searches**: Up to 10 past queries stored in localStorage, shown on focus and on the home page
- **Faceted filtering**: Filter by topics, content type, sources, and quality score
- **Date range filtering**: Narrow results by publication date
- **Advanced search**: Construct queries using all-words, exact-phrase, any-words, and exclude-words fields
- **Result highlighting**: Matched terms are highlighted using Elasticsearch `<em>` snippets
- **Shareable URLs**: All search state (query, filters, page, sort) is serialised to query parameters
- **Skeleton loading**: Placeholder cards shown during fetch, replacing loading spinners
- **Mobile responsive**: Filter drawer replaces the sidebar on narrow viewports
- **Analytics hooks**: Lightweight `trackEvent` utility fires custom DOM events for downstream analytics integration

## Quick Start

### Docker (Recommended)

```bash
# From the repository root — start the search frontend and its dependency
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d search-frontend

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f search-frontend

# Access at http://localhost:3003
```

The dev container runs `npm install && npm run dev` on startup with the source tree bind-mounted for hot-reload.

### Local Development

Node.js 22 or later is required.

```bash
cd search-frontend

# Install dependencies
npm install

# Start dev server (proxies /api/search → http://localhost:8092)
npm run dev

# Access at http://localhost:3003
```

The Search service must be reachable on `SEARCH_API_URL` (default `http://localhost:8092`) for queries to return results.

### Production Build

```bash
npm run build      # Outputs static assets to dist/
npm run preview    # Serve dist/ locally to verify the build
```

The production `Dockerfile` uses a two-stage build: Node 25 Alpine compiles the assets, then nginx Alpine serves them with the SPA routing config in `nginx.conf`.

## Configuration

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SEARCH_API_URL` | `http://localhost:8092` | Search service base URL used by the Vite dev proxy |
| `NODE_ENV` | `development` | Standard Node environment flag |

`SEARCH_API_URL` is a **build-time / server-side variable** read by `vite.config.ts` to configure the dev proxy. It is not exposed to the browser bundle. The browser always calls `/api/search`, which nginx (production) or the Vite proxy (development) rewrites to the search service.

## Architecture

```
search-frontend/
├── src/
│   ├── main.ts                          # App entry — mounts Vue, installs router
│   ├── App.vue                          # Root layout and router-view
│   ├── style.css                        # Global styles and CSS custom properties
│   ├── router/
│   │   └── index.ts                     # Four routes: /, /search, /advanced, 404
│   ├── api/
│   │   └── search.ts                    # Axios client (baseURL /api/search)
│   ├── views/
│   │   ├── HomeView.vue                 # Landing page — search bar + suggested topics + recent searches
│   │   ├── ResultsView.vue              # Search results — sidebar/drawer filters, pagination
│   │   ├── AdvancedSearchView.vue       # Advanced query builder form
│   │   └── NotFoundView.vue             # 404 page
│   ├── components/
│   │   ├── search/
│   │   │   ├── SearchBar.vue            # Input with autocomplete dropdown
│   │   │   ├── SearchResults.vue        # Results list container
│   │   │   ├── SearchResultItem.vue     # Individual result card with highlights
│   │   │   ├── SearchResultsSkeleton.vue # Skeleton placeholder during load
│   │   │   ├── FilterSidebar.vue        # Desktop filter panel (topics, sources, quality, dates)
│   │   │   ├── FilterDrawer.vue         # Mobile filter panel (slide-out)
│   │   │   ├── FilterChips.vue          # Active filter pills shown above results
│   │   │   ├── SearchPagination.vue     # Page navigation
│   │   │   ├── EmptySearchState.vue     # No-results message
│   │   │   └── RelatedContent.vue       # Related content suggestions
│   │   └── common/
│   │       ├── EmptyState.vue           # Generic empty state
│   │       ├── ErrorAlert.vue           # Error banner
│   │       └── LoadingSpinner.vue       # Spinner (used outside skeleton contexts)
│   ├── composables/
│   │   ├── useSearch.ts                 # Central search state, API calls, URL sync
│   │   ├── useRecentSearches.ts         # localStorage recent searches (cap 10)
│   │   ├── useDebounce.ts               # Reactive debounce (default 300 ms)
│   │   └── useUrlParams.ts              # Generic URL parameter sync helper
│   ├── types/
│   │   ├── search.ts                    # SearchRequest, SearchResponse, SearchFilters, facet types
│   │   ├── api.ts                       # SearchApi interface, interceptor types
│   │   └── router.ts                    # Route meta type augmentation
│   └── utils/
│       ├── queryBuilder.ts              # Build SearchRequest payload, parse advanced form, validate
│       ├── dateFormatter.ts             # Date display helpers
│       ├── highlightHelper.ts           # Parse and sanitise Elasticsearch highlight snippets
│       └── analytics.ts                # trackEvent — fires CustomEvent for downstream integrations
├── public/
│   └── favicon.ico
├── Dockerfile                           # Two-stage production build (Node + nginx)
├── Dockerfile.dev                       # Development image with bind-mount hot-reload
├── nginx.conf                           # SPA routing, gzip, asset caching
├── Taskfile.yml                         # Task wrappers for lint, build, dev, preview
├── vite.config.ts                       # Dev proxy config, alias @/ → src/
├── eslint.config.js                     # ESLint flat config (vue + typescript-eslint)
└── tsconfig.json
```

## Development

### Linting

```bash
npm run lint        # Run ESLint
npm run lint:fix    # Auto-fix violations

# Via Taskfile
task lint
```

### Building

```bash
npm run build       # Production build to dist/
task build          # Same via Taskfile (cached; re-runs only on source changes)
```

### Testing

No automated test suite is configured. Validate changes manually via the dev server. Use the `npm run preview` command to verify production builds before shipping.

## Integration

The frontend connects to the **Search service** (port 8092 in dev, 8090 in prod behind nginx).

**Dev proxy** (`vite.config.ts`):

| Browser path | Rewrites to |
|---|---|
| `POST /api/search` | `POST http://{SEARCH_API_URL}/api/v1/search` |
| `GET /api/search` | `GET http://{SEARCH_API_URL}/api/v1/search` |
| `GET /api/search/suggest` | `GET http://{SEARCH_API_URL}/api/v1/search/suggest` |
| `GET /api/health/search` | `GET http://{SEARCH_API_URL}/health` |

**Search request shape** (sent as JSON body to `POST /api/search`):

```typescript
{
  query: string,
  filters?: {
    topics?: string[],
    content_type?: string,
    min_quality_score?: number,
    from_date?: string,        // ISO date
    to_date?: string,          // ISO date
    source_names?: string[]
  },
  pagination?: { page: number, size: number },
  sort?: { field: string, order: 'asc' | 'desc' },
  options?: { include_highlights?: boolean, include_facets?: boolean }
}
```

See `/search/README.md` for the full Search service API reference.

## Routes

| Path | View | Description |
|---|---|---|
| `/` | `HomeView` | Landing page with search bar, suggested topics, recent searches |
| `/search?q=...` | `ResultsView` | Results with facet filters, pagination, sort |
| `/advanced` | `AdvancedSearchView` | Advanced query form |
| `/*` | `NotFoundView` | 404 catch-all |
