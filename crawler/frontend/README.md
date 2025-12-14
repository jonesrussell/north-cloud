# GoCrawl Frontend Dashboard

Vue.js 3 dashboard interface for the GoCrawl crawler service.

## Features

- Dashboard with system health and quick statistics
- Crawl jobs management
- Performance statistics and metrics
- Responsive UI built with Tailwind CSS
- Consistent with source-manager frontend architecture

## Tech Stack

- **Vue.js 3** - Progressive JavaScript framework with Composition API
- **Vue Router 4** - Official router for Vue.js
- **Vite** - Fast build tool and dev server
- **Tailwind CSS 4** - Utility-first CSS framework
- **Axios** - HTTP client for API requests
- **Headless UI** - Unstyled, accessible UI components
- **Heroicons** - Beautiful hand-crafted SVG icons

## Development

### Prerequisites

- Node.js 18+ and npm
- GoCrawl backend running on port 8060

### Local Development

```bash
# Install dependencies
npm install

# Start dev server (http://localhost:3000)
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview

# Lint and fix
npm run lint
```

### Environment Variables

Create a `.env` file if needed:

```bash
VITE_API_URL=http://localhost:8060
```

## Project Structure

```
frontend/
├── src/
│   ├── api/
│   │   └── client.js           # API client and endpoints
│   ├── views/
│   │   ├── DashboardView.vue   # Main dashboard
│   │   ├── CrawlJobsView.vue   # Jobs management
│   │   └── StatsView.vue       # Statistics view
│   ├── App.vue                 # Root component with navigation
│   ├── main.js                 # Application entry point
│   └── style.css               # Global styles
├── index.html                  # HTML template
├── vite.config.js              # Vite configuration
└── package.json                # Dependencies
```

## API Integration

The frontend connects to the GoCrawl backend API. Update `src/api/client.js` to match your actual API endpoints:

```javascript
export const crawlerApi = {
  getHealth: () => client.get('/health'),
  listJobs: () => client.get('/api/v1/jobs'),
  getStats: () => client.get('/api/v1/stats'),
  // Add more endpoints as needed
}
```

## Docker Integration

The frontend is built and served alongside the GoCrawl backend in Docker:

- **Development**: Vite dev server runs on port 3000 with hot-reload
- **Production**: Frontend is built and served by the Go backend

See the main crawler `Dockerfile` for build configuration.

## Customization

### Adding New Views

1. Create a new component in `src/views/`
2. Add route in `src/main.js`
3. Add navigation link in `src/App.vue`

### Styling

- Uses Tailwind CSS 4 with Vite plugin
- Modify `src/style.css` for global styles
- Component-scoped styles in `.vue` files

## Notes

- API endpoints are placeholders and should be updated based on actual crawler backend
- Chart visualizations can be added using libraries like Chart.js or Apache ECharts
- Forms and modals use basic implementations - enhance with Headless UI components as needed
