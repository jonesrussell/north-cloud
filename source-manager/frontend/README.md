# GoSources UI

Modern Vue.js 3 frontend for the GoSources API with Tailwind CSS 4.

## Features

- **Source Management**: Create, read, update, and delete content sources
- **City View**: View cities configured for gopost integration
- **Modern UI**: Clean, responsive design with Tailwind CSS 4
- **Real-time Updates**: Immediate feedback on all operations

## Development

### Prerequisites

- Node.js 18+ and npm

### Setup

```bash
cd frontend
npm install
```

### Run Development Server

```bash
npm run dev
```

The frontend will run on `http://localhost:3000` and proxy API requests to `http://localhost:8050`.

### Build for Production

```bash
npm run build
```

Build output will be in the `dist/` directory.

### Environment Variables

Create a `.env` file to configure the API URL:

```env
VITE_API_URL=http://localhost:8050
```

## Project Structure

```
frontend/
├── src/
│   ├── api/          # API client configuration
│   ├── components/   # Reusable Vue components
│   ├── views/        # Page components
│   ├── composables/  # Vue composables
│   ├── assets/       # Static assets
│   ├── App.vue       # Root component
│   ├── main.js       # Application entry point
│   └── style.css     # Global styles with Tailwind
├── public/           # Public assets
├── index.html        # HTML template
├── vite.config.js    # Vite configuration
└── package.json      # Dependencies
```

## API Integration

The frontend uses the GoSources REST API:

- `GET /api/v1/sources` - List all sources
- `GET /api/v1/sources/:id` - Get source by ID
- `POST /api/v1/sources` - Create source
- `PUT /api/v1/sources/:id` - Update source
- `DELETE /api/v1/sources/:id` - Delete source
- `GET /api/v1/cities` - List cities

