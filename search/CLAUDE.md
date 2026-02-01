# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the search service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload
task test             # Run tests
task lint             # Run linter

# API (port 8092 dev, 8090 prod)
curl "http://localhost:8092/api/v1/search?q=crime&page=1&size=20"
```

## Architecture

```
search/
├── main.go
└── internal/
    ├── api/
    │   ├── server.go      # Gin server setup
    │   ├── routes.go      # Route definitions
    │   ├── handlers.go    # HTTP handlers
    │   └── middleware.go  # CORS, logging
    ├── service/
    │   └── search_service.go  # Search orchestration
    ├── elasticsearch/
    │   ├── client.go          # ES client wrapper
    │   └── query_builder.go   # Query construction
    ├── domain/
    │   ├── search.go          # SearchRequest, SearchResponse
    │   └── content.go         # ClassifiedContent model
    └── config/
        └── config.go
```

## Search Features

- **Multi-match search**: Searches across title, body, and topics
- **Field boosting**: Title matches scored higher than body
- **Fuzzy matching**: Handles typos and variations
- **Faceted search**: Aggregate by topics, content types, sources
- **Quality filtering**: Filter by minimum quality score
- **Pagination**: Page-based navigation with configurable size

## API Endpoints

**Search** (JWT Protected):
- `GET /api/v1/search` - Full-text search
- `GET /api/v1/search/suggest` - Autocomplete suggestions

**Health** (Public):
- `GET /health` - Health check with ES status

## Search Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | required | Search query |
| `page` | int | 1 | Page number |
| `size` | int | 20 | Results per page (max 100) |
| `min_quality` | int | 0 | Minimum quality score |
| `topics` | string[] | - | Filter by topics |
| `content_type` | string | - | Filter by content type |
| `source` | string | - | Filter by source name |
| `include_facets` | bool | false | Include aggregations |

## Search Request Example

```bash
curl "http://localhost:8092/api/v1/search?q=violent+crime&page=1&size=20&min_quality=50&include_facets=true"
```

**Response**:
```json
{
  "query": "violent crime",
  "total_hits": 150,
  "current_page": 1,
  "page_size": 20,
  "total_pages": 8,
  "took_ms": 45,
  "hits": [
    {
      "id": "abc123",
      "title": "Breaking: Violent Crime Report",
      "snippet": "...highlighted text...",
      "score": 12.5,
      "quality_score": 85,
      "topics": ["violent_crime", "local"],
      "source_name": "example_com",
      "published_at": "2025-12-28T10:00:00Z"
    }
  ],
  "facets": {
    "topics": [
      { "key": "violent_crime", "count": 50 },
      { "key": "property_crime", "count": 30 }
    ],
    "content_types": [
      { "key": "article", "count": 140 }
    ]
  }
}
```

## Elasticsearch Query

The service queries all `*_classified_content` indexes:

```go
esClient.Search.WithIndex(cfg.ClassifiedContentPattern) // "*_classified_content"
```

**Query Structure**:
- `multi_match` on title (boost: 3), body (boost: 1), topics (boost: 2)
- `fuzziness: AUTO` for typo tolerance
- `highlight` on title and body fields

## Common Gotchas

1. **Searches classified_content only**: Raw content is not searchable via this service.

2. **Max query length**: Queries limited to 500 characters by default.

3. **Facets are expensive**: Only request `include_facets=true` when needed.

4. **Timeout is configurable**: Default 30 seconds per search.

5. **Port differs in dev/prod**: 8092 in dev, routed through nginx at `/api/search` in prod.

## Configuration

```yaml
service:
  port: 8092
  debug: false
  max_page_size: 100
  default_page_size: 20
  max_query_length: 500
  search_timeout: 30s

elasticsearch:
  url: http://localhost:9200
  classified_content_pattern: "*_classified_content"
```

## Testing

```bash
# Run tests
task test

# Manual search test
curl "http://localhost:8092/api/v1/search?q=test"
```
