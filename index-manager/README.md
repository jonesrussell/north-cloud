# Index Manager Service

A dedicated microservice for managing Elasticsearch indexes across the North Cloud platform.

## Overview

The Index Manager service centralizes all Elasticsearch index management operations, providing a unified REST API for creating, listing, deleting, and managing indexes for all services in the North Cloud platform.

## Features

- **Index Management**: Create, list, delete, and update Elasticsearch indexes
- **Source-Based Operations**: Manage all indexes for a specific source
- **Health Checks**: Monitor index health and cluster status
- **Migration Tracking**: Track index mapping versions and migration history
- **Bulk Operations**: Perform operations on multiple indexes at once

## Index Types Supported

1. **Raw Content**: `{source}_raw_content` - Minimally-processed crawled content
2. **Classified Content**: `{source}_classified_content` - Enriched classified content
3. **Legacy Articles**: `{source}_articles` - Deprecated article format
4. **Legacy Pages**: `{source}_pages` - Deprecated page format

## Quick Start

### Prerequisites

- Go 1.25+
- PostgreSQL 16+
- Elasticsearch 9.2+

### Running Locally

```bash
# Install dependencies
go mod download

# Run migrations
psql -h localhost -p 5436 -U postgres -d index_manager -f migrations/001_create_index_metadata.sql

# Run the service
go run main.go
```

### Using Docker

```bash
# Build and run with docker-compose
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d index-manager
```

## Configuration

Configuration is managed via `config.yml`. See `config.yml.example` for all available options.

Key configuration sections:
- **Service**: Port, debug mode
- **Database**: PostgreSQL connection settings
- **Elasticsearch**: ES connection and settings
- **Index Types**: Configuration for each index type

## API Endpoints

### Index Management

- `POST /api/v1/indexes` - Create an index
- `GET /api/v1/indexes` - List all indexes (with filtering)
- `GET /api/v1/indexes/{index_name}` - Get index details
- `DELETE /api/v1/indexes/{index_name}` - Delete an index
- `PUT /api/v1/indexes/{index_name}/mapping` - Update index mapping
- `GET /api/v1/indexes/{index_name}/health` - Get index health status

### Source-Based Operations

- `POST /api/v1/sources/{source_name}/indexes` - Create all indexes for a source
- `GET /api/v1/sources/{source_name}/indexes` - List indexes for a source
- `DELETE /api/v1/sources/{source_name}/indexes` - Delete all indexes for a source

### Bulk Operations

- `POST /api/v1/indexes/bulk/create` - Create multiple indexes
- `DELETE /api/v1/indexes/bulk/delete` - Delete multiple indexes

### Health & Status

- `GET /api/v1/health` - Service health check
- `GET /api/v1/stats` - Index statistics

## Environment Variables

See `.env.example` in the root directory for all index-manager-related variables:

```bash
INDEX_MANAGER_PORT=8090
POSTGRES_INDEX_MANAGER_USER=postgres
POSTGRES_INDEX_MANAGER_PASSWORD=postgres
POSTGRES_INDEX_MANAGER_DB=index_manager
POSTGRES_INDEX_MANAGER_PORT=5436
```

## Database Schema

### index_metadata Table

Tracks metadata for all indexes including type, source, mapping version, and status.

### migration_history Table

Tracks all index migration operations including create, update, and delete operations.

See `migrations/` directory for SQL schema definitions.

## Integration

### With Crawler

The crawler can call the index-manager API to ensure indexes exist before crawling:

```bash
curl -X POST http://localhost:8090/api/v1/sources/example.com/indexes
```

### With Classifier

The classifier can ensure classified_content indexes exist:

```bash
curl -X POST http://localhost:8090/api/v1/indexes \
  -H "Content-Type: application/json" \
  -d '{
    "index_name": "example_com_classified_content",
    "index_type": "classified_content",
    "source_name": "example.com"
  }'
```

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...
```

### Building

```bash
# Build binary
go build -o bin/index-manager main.go

# Build Docker image
docker build -t north-cloud-index-manager:latest .
```

## License

Part of the North Cloud platform.

