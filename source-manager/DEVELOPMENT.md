# Development Guide

This guide covers setting up and running the gosources application in development mode.

## Prerequisites

- Go 1.21 or later
- PostgreSQL 15 or later
- Node.js 18 or later (for frontend)
- Task (https://taskfile.dev) - optional but recommended

## Quick Start

### 1. Install Dependencies

```bash
# Install Go dependencies
go mod download

# Install frontend dependencies
task frontend:install
# or: cd frontend && npm install

# Install Air for hot reloading (optional, will auto-install when running dev tasks)
task air:install
# or: go install github.com/air-verse/air@latest
```

### 2. Set Up Database

Create a PostgreSQL database and run migrations:

```bash
# Using Docker Compose (recommended)
task docker:up

# Or manually with psql
createdb gosources
DB_PASSWORD=yourpassword task migrate
```

### 3. Configure Application

Create a `config.yml` file (see `config.example.yml` for reference):

```yaml
server:
  port: 8050
  host: "0.0.0.0"

database:
  host: localhost
  port: 5432
  user: postgres
  password: yourpassword
  dbname: gosources
  sslmode: disable
  max_open_conns: 25
  max_idle_conns: 25
  conn_max_lifetime: 5m

logger:
  level: debug
  format: json
```

## Development Workflows

### Backend Only (with Hot Reload)

Run the Go backend with automatic reloading when files change:

```bash
task dev
# or: task dev:backend
```

This will:
- Automatically install Air if not present
- Watch for changes in `.go` files
- Rebuild and restart the server on changes
- Exclude test files, frontend, and vendor directories
- Log build errors to `build-errors.log`

The backend API will be available at: http://localhost:8050

### Frontend Only

Run the Vue.js frontend development server:

```bash
task frontend:dev
# or: cd frontend && npm run dev
```

The frontend will be available at: http://localhost:3000

### Full Stack Development

Run both backend and frontend simultaneously with hot reloading:

```bash
task dev:all
```

This will start:
- Backend API at http://localhost:8050 (with Air hot reload)
- Frontend at http://localhost:3000 (with Vite hot reload)

Press `Ctrl+C` to stop both servers.

## Air Configuration

The Air hot reload configuration is in `.air.toml`. Key settings:

- **Watched directories**: Root directory, excluding frontend/, tmp/, vendor/, etc.
- **Watched extensions**: `.go`, `.tpl`, `.tmpl`, `.html`
- **Excluded files**: `*_test.go` (tests are excluded from hot reload)
- **Build command**: `go build -o ./tmp/main ./main.go`
- **Run arguments**: `-config config.yml`
- **Temporary directory**: `tmp/` (gitignored)

### Customizing Air

Edit `.air.toml` to customize the hot reload behavior:

```toml
[build]
  # Add build flags
  cmd = "go build -tags dev -o ./tmp/main ./main.go"

  # Change the delay before rebuilding (in ms)
  delay = 1000

  # Include additional file extensions
  include_ext = ["go", "tpl", "tmpl", "html", "yaml"]
```

## Task Commands Reference

### Development

- `task dev` - Run backend with hot reload
- `task dev:backend` - Run backend with hot reload (alias)
- `task dev:all` - Run both backend and frontend
- `task frontend:dev` - Run frontend only

### Building

- `task build` - Build the backend binary
- `task frontend:build` - Build frontend for production

### Testing

- `task test` - Run all tests
- `task test:coverage` - Run tests with coverage report
- `task test:race` - Run tests with race detector

### Code Quality

- `task fmt` - Format Go code
- `task vet` - Run go vet
- `task lint` - Run golangci-lint
- `task check` - Run all quality checks (fmt, vet, lint)

### Database

- `task migrate` - Run database migrations
- `task migrate:check` - Check migration status

### Docker

- `task docker:up` - Start all services with Docker Compose
- `task docker:down` - Stop all services
- `task docker:restart` - Restart services
- `task docker:logs` - View service logs

### Utilities

- `task air:install` - Install Air hot reload tool
- `task frontend:install` - Install frontend dependencies
- `task clean` - Clean build artifacts and caches

## Project Structure

```
gosources/
├── .air.toml              # Air hot reload configuration
├── Taskfile.yml           # Task definitions
├── config.yml             # Application configuration (gitignored)
├── main.go                # Application entry point
├── internal/              # Internal packages
│   ├── api/              # API router and middleware
│   ├── config/           # Configuration management
│   ├── database/         # Database connection
│   ├── handlers/         # HTTP handlers
│   ├── logger/           # Logging
│   ├── models/           # Data models
│   └── repository/       # Data access layer
├── frontend/              # Vue.js frontend
│   ├── src/
│   │   ├── api/          # API client
│   │   ├── components/   # Vue components
│   │   ├── views/        # Page views
│   │   └── main.js       # Frontend entry point
│   └── package.json
├── scripts/               # Utility scripts
│   ├── populate_sources.go
│   └── README.md
└── tmp/                   # Air temporary build directory (gitignored)
```

## Debugging

### View Air Logs

Air outputs build and runtime logs to the console. Build errors are also saved to `build-errors.log`.

### Check Running Processes

```bash
# Check if backend is running
curl http://localhost:8050/api/v1/sources

# Check if frontend is running
curl http://localhost:3000
```

### Common Issues

**Air not found**
```bash
# Install Air manually
go install github.com/air-verse/air@latest

# Make sure $GOPATH/bin is in your PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

**Port already in use**
```bash
# Find process using port 8050
lsof -i :8050

# Kill the process
kill -9 <PID>
```

**Database connection error**
- Check PostgreSQL is running: `pg_isready`
- Verify config.yml database settings
- Check credentials: `psql -U postgres -d gosources`

## Best Practices

1. **Use Air for backend development** - Faster feedback loop with automatic rebuilds
2. **Run `task dev:all`** - Keep both frontend and backend in sync during full-stack development
3. **Run tests before committing** - Use `task test` to ensure everything works
4. **Check code quality** - Run `task check` to format and lint code
5. **Use Task commands** - They handle dependencies and setup automatically

## Next Steps

- Check out the [API Documentation](docs/api.md) for API endpoints
- See [scripts/README.md](scripts/README.md) for utility scripts
- Review source configurations at http://localhost:3000/sources
