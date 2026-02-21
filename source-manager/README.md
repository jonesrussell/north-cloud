# Source Manager

A microservice for managing content source configurations. Sources define what the pipeline crawls: the URL, CSS selectors, rate limits, and scheduling hints used by the Crawler.

## Features

- REST API for CRUD operations on sources
- PostgreSQL database storage
- Selector preview via test-crawl (no data saved)
- Metadata auto-fetch from a URL
- Bulk import from Excel spreadsheets
- City mapping for gopost integration
- Structured logging with zap
- Health check endpoint
- Graceful shutdown

## Quick Start

### Docker (Recommended)

Source Manager starts automatically with the North Cloud stack:

```bash
task docker:dev:up
```

Available at `http://localhost:8050`.

### Local Development

```bash
cp config.yml.example config.yml
# Edit config.yml with your PostgreSQL and Elasticsearch settings
go run cmd/migrate/main.go up   # Run migrations
task dev                         # Start with hot reload (Air)
# Or: go run main.go -config config.yml
```

## API Endpoints

### Sources

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/sources` | Public | List all sources |
| `GET` | `/api/v1/sources/:id` | JWT | Get source by ID |
| `POST` | `/api/v1/sources` | JWT | Create a new source |
| `PUT` | `/api/v1/sources/:id` | JWT | Update a source |
| `DELETE` | `/api/v1/sources/:id` | JWT | Delete a source |
| `POST` | `/api/v1/sources/test-crawl` | JWT | Preview selectors without saving |
| `POST` | `/api/v1/sources/fetch-metadata` | JWT | Auto-fetch title/selectors from URL |
| `POST` | `/api/v1/sources/import-excel` | JWT | Bulk import sources from Excel file |

### Cities

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/cities` | Public | List cities derived from enabled sources |

### Health

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/health` | Public | Health check endpoint |

**Note**: `GET /api/v1/sources` and `GET /api/v1/cities` are intentionally public to allow internal service-to-service calls (crawler, publisher) without JWT tokens. All write operations require a JWT.

## Configuration

Copy `config.yml.example` to `config.yml` and configure:

```yaml
debug: false
server:
  host: "0.0.0.0"
  port: 8050
database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "postgres"
  dbname: "source_manager"
```

Environment variables override config file values:

| Variable | Description |
|----------|-------------|
| `APP_DEBUG` | Debug mode |
| `SERVER_HOST` | Server host |
| `SERVER_PORT` | Server port |
| `DB_HOST` | Database host |
| `DB_PORT` | Database port |
| `DB_USER` | Database user |
| `DB_PASSWORD` | Database password |
| `DB_NAME` | Database name |
| `DB_SSLMODE` | SSL mode |
| `AUTH_JWT_SECRET` | Shared JWT secret (must match all other services) |
| `SOURCE_MANAGER_API_URL` | Base URL used for dynamic CORS origin derivation |

## Database Setup

Run migrations via the Taskfile:

```bash
task migrate:up
```

Or directly with the migrate tool:

```bash
cd source-manager && go run cmd/migrate/main.go up
```

## Source JSON Format

```json
{
  "name": "Mid-North Monitor",
  "url": "https://www.midnorthmonitor.com/category/news/local-news/",
  "rate_limit": "10",
  "max_depth": 2,
  "selectors": {
    "article": {
      "container": "article",
      "title": "h1",
      "body": ".article-body",
      "byline": ".byline",
      "published_time": "time[datetime]"
    },
    "list": {
      "container": ".article-list, main",
      "article_cards": ".article-card, article"
    },
    "page": {
      "container": "main, article",
      "title": "h1",
      "content": "main, article, .content"
    }
  },
  "enabled": true
}
```

Elasticsearch index names are derived dynamically from the source `name` at crawl time and are not stored in the database. See the Integration section below for the derivation rules.

## Architecture

```
source-manager/
├── main.go
└── internal/
    ├── api/           # HTTP router (Gin) — route definitions and CORS config
    ├── bootstrap/     # Phased startup: profiling → config/logger → database → event publisher → server
    ├── config/        # Config struct with env-tag loading
    ├── database/      # PostgreSQL connection helpers
    ├── events/        # Redis event publisher (source created/updated/deleted)
    ├── handlers/      # HTTP handlers (SourceHandler)
    ├── importer/      # Excel bulk-import logic
    ├── metadata/      # Auto-fetch page title and selector hints from a URL
    ├── models/        # Source, SelectorConfig, City structs
    ├── repository/    # PostgreSQL source repository (CRUD)
    └── testhelpers/   # Shared test utilities
```

## Running

```bash
go run main.go -config config.yml
```

## Building

```bash
go build -o bin/source-manager main.go
```

## Integration

Source Manager is the entry point for configuring what the pipeline crawls.

- **Crawler** reads `source_id` from each job to fetch source configuration (selectors, rate limits)
- **Index naming**: The source `name` is sanitized (lowercased, spaces and special characters become underscores) to derive the Elasticsearch index name: `{sanitized_name}_raw_content` and `{sanitized_name}_classified_content`
- **Publisher** uses the index pattern to discover classified content indexes to route from

Examples of name sanitization:

| Source Name | Elasticsearch Prefix |
|-------------|----------------------|
| `Example News` | `example_news` |
| `CBC.ca` | `cbc_ca` |
| `My-Source` | `my_source` |
