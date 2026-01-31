# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the search-frontend service.

## Quick Reference

```bash
# Development
npm run dev           # Start dev server
npm run build         # Build for production
npm run lint          # Run ESLint
npm run preview       # Preview production build

# Or using Taskfile
task dev
task build
task lint
```

## Architecture

```
search-frontend/src/
├── App.vue              # Root component with search interface
├── main.ts              # Entry point
├── router/              # Vue Router (minimal, single-page)
├── api/                 # Search API client
├── components/
│   ├── SearchBar.vue    # Search input component
│   ├── SearchResults.vue # Results display
│   ├── ResultCard.vue   # Individual result card
│   └── Facets.vue       # Facet filters
├── composables/
│   └── useSearch.ts     # Search logic composable
├── types/               # TypeScript definitions
├── views/
│   └── SearchView.vue   # Main search page
└── utils/               # Utility functions
```

## Tech Stack

- **Vue 3** - Composition API
- **TypeScript** - Type checking
- **Vite** - Build tool
- **Tailwind CSS 4** - Styling
- **Axios** - HTTP client
- **Heroicons** - Icons

## Purpose

Public-facing search interface for querying classified content. Simpler than the dashboard - focused solely on search functionality.

## Search Flow

```
User enters query → SearchBar emits search event → useSearch composable → API call → Display results
```

## API Integration

Connects to the search service:

```typescript
// api/search.ts
export const searchApi = {
  search: (params: SearchParams) =>
    axios.get('/api/v1/search', { params }),
  suggest: (query: string) =>
    axios.get('/api/v1/search/suggest', { params: { q: query } })
};
```

## Type Definitions

```typescript
// types/search.ts
interface SearchParams {
  q: string;
  page?: number;
  size?: number;
  min_quality?: number;
  topics?: string[];
  include_facets?: boolean;
}

interface SearchResult {
  id: string;
  title: string;
  snippet: string;
  score: number;
  quality_score: number;
  topics: string[];
  source_name: string;
  published_at: string;
}

interface SearchResponse {
  query: string;
  total_hits: number;
  current_page: number;
  total_pages: number;
  hits: SearchResult[];
  facets?: Facets;
}
```

## Common Gotchas

1. **No authentication**: Public search interface, no login required.

2. **API URL**: Configure via `VITE_SEARCH_API_URL` env var.

3. **Facets optional**: Only request when filter panel is open.

4. **Debounced search**: Input is debounced to avoid excessive API calls.

## Environment Variables

```bash
VITE_SEARCH_API_URL=http://localhost:8092  # Search service URL
```

## Component Usage

**SearchBar**:
```vue
<SearchBar
  v-model="query"
  @search="handleSearch"
  :loading="isLoading"
/>
```

**SearchResults**:
```vue
<SearchResults
  :results="results"
  :total="totalHits"
  @page-change="changePage"
/>
```

## Styling

Uses Tailwind CSS 4 with `@import "tailwindcss"` syntax. Components use utility classes for styling.

## Testing

No test framework currently configured. Test manually via dev server.
