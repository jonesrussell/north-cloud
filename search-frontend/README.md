# North Cloud Search Frontend

The search frontend application for North Cloud platform. Provides a Google-like search interface for discovering crime news and articles.

## Features

- **Full-text search**: Search across all classified content
- **Autocomplete**: Recent searches with keyboard navigation
- **Faceted filtering**: Filter by topics, sources, quality, dates
- **Advanced search**: Complex queries with multiple filters
- **Result highlighting**: See matched terms highlighted in results
- **Mobile responsive**: Works on all devices
- **Shareable URLs**: Query params sync for sharing searches

## Tech Stack

- **Vue 3**: Composition API
- **Vite 7**: Build tool with hot-reload
- **Tailwind CSS 4**: Utility-first styling
- **Vue Router 4**: Client-side routing
- **Axios**: HTTP client
- **Headless UI**: Accessible components
- **Heroicons**: Icon library

## Development

### Prerequisites

- Node.js 22+ or Docker
- Access to search-service API

### Local Development (Node.js)

```bash
# Install dependencies
npm install

# Start dev server
npm run dev

# Access at http://localhost:3003
```

### Docker Development

```bash
# From project root
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d search-frontend

# View logs
docker logs -f north-cloud-search-frontend-dev

# Access at http://localhost:3003
```

## Project Structure

```
search-frontend/
├── src/
│   ├── main.js                  # App entry point
│   ├── App.vue                  # Root component
│   ├── router/index.js          # Route definitions
│   ├── api/search.js            # Search API client
│   ├── views/                   # Page components
│   │   ├── HomeView.vue         # Landing page with search
│   │   ├── ResultsView.vue      # Search results page
│   │   ├── AdvancedSearchView.vue  # Advanced search form
│   │   └── NotFoundView.vue     # 404 page
│   ├── components/
│   │   ├── search/              # Search-specific components
│   │   └── common/              # Reusable components
│   ├── composables/             # Composition API functions
│   │   ├── useSearch.js         # Search state management
│   │   ├── useDebounce.js       # Debouncing utility
│   │   └── useUrlParams.js      # URL sync
│   └── utils/                   # Helper functions
│       ├── queryBuilder.js      # Build API payloads
│       ├── dateFormatter.js     # Date formatting
│       └── highlightHelper.js   # Highlight parsing
└── public/
    └── favicon.ico
```

## Environment Variables

- `SEARCH_API_URL`: Search service URL (default: `http://localhost:8092`)
- `NODE_ENV`: Environment mode (development/production)

## Building for Production

```bash
# Build static files
npm run build

# Preview production build
npm run preview
```

## API Integration

The app integrates with the search-service API at `/api/search`:

- **POST /api/search**: Execute search with filters
- **GET /api/search**: Simple search via query params
- **GET /health**: Health check

See `/search/README.md` for API documentation.

## Routes

- `/` - Home page with search box
- `/search?q=...&topics=...` - Search results with filters
- `/advanced` - Advanced search form
- `/*` - 404 page

## Deployment

The app is deployed as part of the North Cloud platform:

- **Production**: https://northcloud.biz/
- **Dashboard**: https://northcloud.biz/dashboard
- **API**: https://northcloud.biz/api/search

## Contributing

Follow the patterns established in the dashboard application for consistency.

## License

Proprietary - North Cloud Platform
