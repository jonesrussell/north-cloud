# Publisher Frontend Dashboard

Vue.js 3 dashboard for managing the Publisher service - sources, channels, routes, and monitoring publish activity.

## Features

- **Sources Management**: Create, edit, and manage Elasticsearch source indexes
- **Channels Management**: Configure Redis pub/sub channels for article distribution
- **Routes Management**: Set up routing rules from sources to channels with filters
- **Publishing Dashboard**: View statistics and recent publish history
- **JWT Authentication**: Secure access with token-based authentication

## Technology Stack

- Vue.js 3 (Composition API)
- Vue Router 4
- Axios (HTTP client)
- Vite (Build tool)

## Development Setup

### Prerequisites

- Node.js 18+ and npm
- Publisher API server running on port 8070

### Installation

```bash
# Install dependencies
npm install

# Start development server
npm run dev
```

The frontend will be available at `http://localhost:3003`

### API Proxy

The development server is configured to proxy API requests to `http://localhost:8070`:

```javascript
// vite.config.js
server: {
  proxy: {
    '/api': {
      target: 'http://localhost:8070',
      changeOrigin: true
    }
  }
}
```

## Production Build

```bash
# Build for production
npm run build

# Preview production build
npm run preview
```

The build output will be in the `dist/` directory.

## Project Structure

```
frontend/
├── src/
│   ├── api/
│   │   ├── client.js         # Axios client with JWT interceptors
│   │   └── publisher.js      # API methods (sources, channels, routes, stats)
│   ├── views/
│   │   ├── DashboardView.vue # Main dashboard with stats
│   │   ├── SourcesView.vue   # Sources CRUD management
│   │   ├── ChannelsView.vue  # Channels CRUD management
│   │   └── RoutesView.vue    # Routes CRUD management
│   ├── router/
│   │   └── index.js          # Vue Router configuration
│   ├── App.vue               # Main app component with navigation
│   └── main.js               # App entry point
├── index.html                # HTML template
├── vite.config.js            # Vite configuration
└── package.json              # Dependencies and scripts
```

## Environment Variables

No environment variables required for development. The API base URL is configured in the Vite proxy.

For production deployment, ensure the API server is accessible at the same domain or configure CORS appropriately.

## Authentication

The dashboard integrates with the auth service for JWT authentication:

- JWT tokens are stored in `localStorage` with key `auth_token`
- Tokens are automatically added to all API requests via Axios interceptors
- 401 responses redirect to the login page

## API Endpoints Used

### Sources
- `GET /api/v1/sources` - List sources
- `POST /api/v1/sources` - Create source
- `GET /api/v1/sources/:id` - Get source
- `PUT /api/v1/sources/:id` - Update source
- `DELETE /api/v1/sources/:id` - Delete source

### Channels
- `GET /api/v1/channels` - List channels
- `POST /api/v1/channels` - Create channel
- `GET /api/v1/channels/:id` - Get channel
- `PUT /api/v1/channels/:id` - Update channel
- `DELETE /api/v1/channels/:id` - Delete channel

### Routes
- `GET /api/v1/routes` - List routes with joined details
- `POST /api/v1/routes` - Create route
- `GET /api/v1/routes/:id` - Get route
- `PUT /api/v1/routes/:id` - Update route
- `DELETE /api/v1/routes/:id` - Delete route

### Stats
- `GET /api/v1/stats/overview?period={today|week|month|all}` - Publishing overview
- `GET /api/v1/stats/channels?since={YYYY-MM-DD}` - Per-channel stats
- `GET /api/v1/stats/routes` - Per-route stats

### Publish History
- `GET /api/v1/publish-history?limit=20&offset=0` - List publish history
- `GET /api/v1/publish-history/:article_id` - History for specific article

## Development Notes

- The UI uses minimal custom CSS (no framework like Tailwind or Bootstrap)
- All styles are scoped to components or global in App.vue
- Forms include validation and error handling
- Tables support filtering (enabled only toggle)
- Modal dialogs for create/edit operations

## Troubleshooting

### API Connection Issues

If you see connection errors:

1. Verify the API server is running: `curl http://localhost:8070/health`
2. Check Vite proxy configuration in `vite.config.js`
3. Inspect browser console for CORS errors

### Authentication Issues

If you're redirected to login unexpectedly:

1. Check that `AUTH_JWT_SECRET` environment variable matches between services
2. Verify token is in localStorage: `localStorage.getItem('auth_token')`
3. Check token expiry (tokens expire after 24 hours)

### Build Issues

If npm install fails:

```bash
# Clear npm cache and reinstall
rm -rf node_modules package-lock.json
npm cache clean --force
npm install
```

## License

Part of the North Cloud project.
