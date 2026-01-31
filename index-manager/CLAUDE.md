# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the index-manager service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload
task test             # Run tests
task lint             # Run linter
task migrate:up       # Run migrations

# API (port 8090)
curl http://localhost:8090/api/v1/indexes
curl http://localhost:8090/api/v1/indexes/example_com_raw_content
```

## Architecture

```
index-manager/
├── cmd/httpd/main.go    # Entry point
└── internal/
    ├── api/
    │   ├── server.go    # Gin server setup
    │   ├── routes.go    # Route definitions
    │   └── handlers.go  # HTTP handlers
    ├── service/
    │   ├── index_service.go     # Index operations
    │   └── document_service.go  # Document CRUD
    ├── elasticsearch/
    │   ├── client.go            # ES client wrapper
    │   ├── index_manager.go     # Index lifecycle
    │   ├── query_builder.go     # Query construction
    │   └── mappings/            # Index mappings
    │       ├── raw_content.go
    │       ├── classified_content.go
    │       ├── article.go
    │       └── page.go
    ├── database/        # PostgreSQL (migrations, metadata)
    ├── config/          # Configuration
    └── domain/          # Index, Document models
```

## Index Types

| Type | Pattern | Purpose |
|------|---------|---------|
| `raw_content` | `{source}_raw_content` | Crawler output |
| `classified_content` | `{source}_classified_content` | Classifier output |
| `article` | `{source}_article` | Legacy (deprecated) |
| `page` | `{source}_page` | Legacy (deprecated) |

## API Endpoints (JWT Protected)

**Index Operations**:
- `GET /api/v1/indexes` - List all indexes
- `GET /api/v1/indexes/:index_name` - Get index details
- `POST /api/v1/indexes` - Create index
- `DELETE /api/v1/indexes/:index_name` - Delete index
- `GET /api/v1/indexes/:index_name/health` - Index health

**Source-Based Operations**:
- `POST /api/v1/sources/:source_name/indexes` - Create indexes for source
- `GET /api/v1/sources/:source_name/indexes` - List indexes for source
- `DELETE /api/v1/sources/:source_name/indexes` - Delete all indexes for source

**Bulk Operations**:
- `POST /api/v1/indexes/bulk/create` - Create multiple indexes
- `DELETE /api/v1/indexes/bulk/delete` - Delete multiple indexes

**Document Operations**:
- `GET /api/v1/indexes/:index_name/documents` - Query documents
- `GET /api/v1/indexes/:index_name/documents/:id` - Get document
- `PUT /api/v1/indexes/:index_name/documents/:id` - Update document
- `DELETE /api/v1/indexes/:index_name/documents/:id` - Delete document
- `POST /api/v1/indexes/:index_name/documents/bulk-delete` - Bulk delete

**Stats**:
- `GET /api/v1/stats` - Overall statistics

## Index Mappings

**raw_content** mapping (key fields):
```json
{
  "url": { "type": "keyword" },
  "title": { "type": "text" },
  "raw_text": { "type": "text" },
  "source_name": { "type": "keyword" },
  "classification_status": { "type": "keyword" },
  "crawled_at": { "type": "date" }
}
```

**classified_content** mapping (additional fields):
```json
{
  "content_type": { "type": "keyword" },
  "quality_score": { "type": "integer" },
  "topics": { "type": "keyword" },
  "source_reputation": { "type": "integer" },
  "classified_at": { "type": "date" }
}
```

## Common Gotchas

1. **Port conflict with search**: Both use 8090 by default. In dev, use different ports or nginx routing.

2. **Index naming convention**: Always use `{source_name}_{type}` pattern.

3. **Mappings are immutable**: Once created, index mappings can't be changed. Delete and recreate if needed.

4. **Bulk operations continue on error**: Partial failures return `207 Multi-Status`.

5. **Document IDs are ES-generated**: Unless explicitly provided in the request.

## Creating Indexes for a Source

```bash
# Create both raw_content and classified_content indexes
curl -X POST http://localhost:8090/api/v1/sources/example_com/indexes \
  -H "Content-Type: application/json" \
  -d '{
    "index_types": ["raw_content", "classified_content"]
  }'
```

## Query Documents

```bash
# Search with pagination
curl "http://localhost:8090/api/v1/indexes/example_com_classified_content/documents?query=crime&page=1&size=20"
```

## Configuration

```yaml
elasticsearch:
  url: http://localhost:9200
  # ES connection settings

database:
  # PostgreSQL for metadata
```

## Testing

```bash
# Run tests
task test

# Run with coverage
task test:cover
```
