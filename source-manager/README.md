# GoSources API

A microservice for managing content sources configuration with REST API.

## Features

- REST API for CRUD operations on sources
- PostgreSQL database storage
- City mapping for gopost integration
- Structured logging with zap
- Health check endpoint
- Graceful shutdown

## API Endpoints

### Sources

- `POST /api/v1/sources` - Create a new source
- `GET /api/v1/sources` - List all sources
- `GET /api/v1/sources/:id` - Get source by ID
- `PUT /api/v1/sources/:id` - Update a source
- `DELETE /api/v1/sources/:id` - Delete a source

### Cities (for gopost integration)

- `GET /api/v1/cities` - Get all enabled cities with their configurations

### Health

- `GET /health` - Health check endpoint

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
  dbname: "gosources"
```

Environment variables override config file values:
- `APP_DEBUG` - Debug mode
- `SERVER_HOST` - Server host
- `SERVER_PORT` - Server port
- `DB_HOST` - Database host
- `DB_PORT` - Database port
- `DB_USER` - Database user
- `DB_PASSWORD` - Database password
- `DB_NAME` - Database name
- `DB_SSLMODE` - SSL mode

## Database Setup

Run the migration to create the sources table:

```bash
psql -U postgres -d gosources -f migrations/001_create_sources_table.sql
```

Or using docker:

```bash
docker exec -i postgres psql -U postgres -d gosources < migrations/001_create_sources_table.sql
```

## Source JSON Format

```json
{
  "name": "Mid-North Monitor",
  "url": "https://www.midnorthmonitor.com/category/news/local-news/",
  "article_index": "midnorthmonitor_articles",
  "page_index": "midnorthmonitor_pages",
  "rate_limit": "1s",
  "max_depth": 2,
  "time": ["11:45", "23:45"],
  "selectors": {
    "article": {
      "container": "article.article-card",
      "title": "h1",
      "body": ".article-body",
      "exclude": [".ad", "nav"]
    },
    "list": {
      "container": ".feed-section",
      "article_cards": "article.article-card"
    },
    "page": {
      "container": "main",
      "title": "h1"
    }
  },
  "city_name": "sudbury_com",
  "group_id": "550e8400-e29b-41d4-a716-446655440000",
  "enabled": true
}
```

## Running

```bash
go run main.go -config config.yml
```

## Building

```bash
go build -o bin/gosources main.go
```

