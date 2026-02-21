# Index Manager Service

A dedicated microservice for managing Elasticsearch indexes across the North Cloud platform.

## Overview

The Index Manager service centralizes all Elasticsearch index lifecycle operations, providing a unified REST API for creating, listing, deleting, and managing indexes for all services in the North Cloud platform. It also exposes aggregation endpoints used by the Dashboard to display pipeline health, crime breakdowns, mining metrics, and classification quality.

## Features

- **Index Management**: Create, list, delete, and health-check Elasticsearch indexes
- **Source-Based Operations**: Provision or tear down all indexes for a specific source in one call
- **Document Operations**: Query, retrieve, update, and delete individual documents
- **Bulk Operations**: Create or delete multiple indexes atomically (partial failures return 207)
- **Aggregations**: Crime, mining, location, overview, source health, and classification drift metrics
- **Migration Tracking**: Record mapping versions and migration history in PostgreSQL

## Index Types

| Type | Index Pattern | Purpose |
|------|---------------|---------|
| `raw_content` | `{source}_raw_content` | Crawler output — minimally processed, awaiting classification |
| `classified_content` | `{source}_classified_content` | Enriched output — quality score, topics, crime/mining fields |

Legacy indexes (`{source}_articles`, `{source}_pages`) are no longer created by this service.

## Integration

The Index Manager sits at the center of the content pipeline's storage layer:

- **Crawler** writes crawled pages to `{source}_raw_content`. It can call `POST /api/v1/sources/{source}/indexes` to ensure the index exists before writing.
- **Classifier** reads pending documents from `{source}_raw_content` and writes enriched results to `{source}_classified_content`. It can call `POST /api/v1/indexes` to provision the classified index before its first write.
- **Index Manager** holds the authoritative Elasticsearch mappings for both index types. If an index was created by the classifier with dynamic mappings (e.g., `source_name` as `text` instead of `keyword`), aggregations must use `.keyword` sub-fields. See the **Common Gotchas** section.

```
Crawler ──writes──► {source}_raw_content
                         │
             Classifier reads, classifies
                         │
                         ▼
             {source}_classified_content ◄── Index Manager owns mappings
                         │
             Publisher, Search, Dashboard read
```

## Quick Start

### Docker (recommended)

```bash
# Start the service and its dependencies
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d index-manager

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f index-manager

# Rebuild after code changes
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build index-manager
```

### Local (Go)

```bash
cd index-manager

# Install dependencies
go mod download

# Run migrations
go run cmd/migrate/main.go up

# Run the service (default port 8090)
go run main.go
```

## Configuration

Configuration is loaded from `config.yml` (copy `config.yml.example` to get started). All values can be overridden by environment variables.

Key sections:

| Section | Purpose |
|---------|---------|
| `service` | Port, debug mode, service name |
| `database` | PostgreSQL connection for metadata/migrations |
| `elasticsearch` | ES URL, credentials, timeouts |
| `index_types` | Shard/replica counts per index type |
| `logging` | Log level and format |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INDEX_MANAGER_PORT` | `8090` | HTTP listen port |
| `POSTGRES_INDEX_MANAGER_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_INDEX_MANAGER_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_INDEX_MANAGER_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_INDEX_MANAGER_PASSWORD` | _(none)_ | PostgreSQL password |
| `POSTGRES_INDEX_MANAGER_DB` | `index_manager` | PostgreSQL database name |
| `ELASTICSEARCH_URL` | `http://localhost:9200` | Elasticsearch URL |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `APP_DEBUG` | `false` | Enable Gin debug mode |

## API Endpoints

All `/api/v1/*` routes are served on port 8090. The `/health` endpoint is public.

### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Service health check |

### Index Management

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/indexes` | Create an index |
| `GET` | `/api/v1/indexes` | List all indexes (filterable, paginated) |
| `GET` | `/api/v1/indexes/:index_name` | Get index details |
| `DELETE` | `/api/v1/indexes/:index_name` | Delete an index |
| `GET` | `/api/v1/indexes/:index_name/health` | Get index health status |
| `POST` | `/api/v1/indexes/:index_name/migrate` | Migrate an index mapping |

### Document Operations

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/indexes/:index_name/documents` | Query documents (supports `query`, `page`, `size`, `sort_field`, `sort_order`) |
| `GET` | `/api/v1/indexes/:index_name/documents/:document_id` | Get a document by ID |
| `PUT` | `/api/v1/indexes/:index_name/documents/:document_id` | Update a document |
| `DELETE` | `/api/v1/indexes/:index_name/documents/:document_id` | Delete a document |
| `POST` | `/api/v1/indexes/:index_name/documents/bulk-delete` | Bulk delete documents by ID list |

### Source-Based Operations

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/sources/:source_name/indexes` | Create all indexes for a source |
| `GET` | `/api/v1/sources/:source_name/indexes` | List all indexes for a source |
| `DELETE` | `/api/v1/sources/:source_name/indexes` | Delete all indexes for a source |

### Bulk Operations

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/indexes/bulk/create` | Create multiple indexes (207 on partial failure) |
| `DELETE` | `/api/v1/indexes/bulk/delete` | Delete multiple indexes (207 on partial failure) |

### Statistics

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/stats` | Overall index statistics |

### Aggregations

| Method | Path | Query Params | Description |
|--------|------|--------------|-------------|
| `GET` | `/api/v1/aggregations/crime` | `sources[]`, `crime_relevance[]`, `crime_sub_labels[]`, `crime_types[]`, `min_quality` | Crime classification breakdown |
| `GET` | `/api/v1/aggregations/mining` | `sources[]`, `min_quality` | Mining classification breakdown |
| `GET` | `/api/v1/aggregations/location` | `sources[]`, `cities[]`, `provinces[]`, `countries[]` | Location breakdown |
| `GET` | `/api/v1/aggregations/overview` | `sources[]` | High-level content overview |
| `GET` | `/api/v1/aggregations/source-health` | _(none)_ | Per-source pipeline health (raw/classified counts, backlog, 24h delta, avg quality) |
| `GET` | `/api/v1/aggregations/classification-drift` | `hours` (default 24), `sources[]` | Raw vs classified document count gap |
| `GET` | `/api/v1/aggregations/classification-drift-timeseries` | `days` (default 7) | Drift trend over time |
| `GET` | `/api/v1/aggregations/content-type-mismatch` | `hours` (default 24) | Documents with mismatched content types |
| `GET` | `/api/v1/aggregations/suspected-misclassifications` | `hours` (default 24) | Suspected misclassified documents |

## Database Schema

### `index_metadata` Table

Tracks metadata for all managed indexes including type, source, mapping version, and status.

### `migration_history` Table

Records all index migration operations (create, update, delete) with timestamps.

See `migrations/` for the SQL schema definitions.

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Via Taskfile
task test
task test:cover
```

### Building

```bash
# Build binary
go build -o bin/index-manager main.go

# Build Docker image
docker build -t north-cloud-index-manager:latest .
```

### Linting

```bash
task lint
# or
golangci-lint run
```
