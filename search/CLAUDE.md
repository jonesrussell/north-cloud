# Search — Developer Guide

## Quick Reference

```bash
# Daily commands
task dev              # Start with hot reload (Air)
task test             # Run all tests
task lint             # Run linter
task fmt              # Format code

# Search API (dev)
curl "http://localhost:8092/api/v1/search?q=crime&page=1&size=20&min_quality=50&include_facets=true"

# Health check
curl http://localhost:8092/health
```

## Architecture

```
search/
├── main.go
└── internal/
    ├── api/
    │   ├── server.go        # Gin server setup
    │   ├── routes.go        # Route definitions
    │   ├── handlers.go      # HTTP handlers
    │   └── middleware.go    # CORS, logging
    ├── service/
    │   └── search_service.go  # Search orchestration, request validation
    ├── elasticsearch/
    │   ├── client.go          # ES client wrapper
    │   └── query_builder.go   # Elasticsearch DSL construction
    ├── domain/
    │   ├── search.go          # SearchRequest, SearchResponse types
    │   └── content.go         # ClassifiedContent model
    └── config/
        └── config.go          # Config struct and loading
```

## Key Concepts

**Multi-index search**: The service queries all `*_classified_content` Elasticsearch indexes in a single request using a wildcard pattern. Only content that has passed through the classifier pipeline is searchable — `*_raw_content` indexes are never queried.

**Multi-match query**: Full-text search runs across multiple fields simultaneously with different relevance weights (field boosting).

**Field boosting**: Title matches are weighted 3x, OG title 2x, body text 1x. This ensures headline-relevant results rank above body mentions.

**Fuzzy matching**: `fuzziness: AUTO` handles typos and minor spelling variations without requiring exact matches.

**Faceted search**: Elasticsearch aggregations return topic, source, and content-type counts alongside results. Facets are optional — only request them when the UI needs filter counts.

**Pagination**: Page-based with a hard maximum of 100 results per page. Deep pagination (high page numbers) increases ES memory pressure.

## API Reference

### POST /api/v1/search

Recommended for complex queries. Accepts a JSON body.

| Field | Type | Description |
|-------|------|-------------|
| `query` | string | Full-text search query (max 500 chars) |
| `filters.topics` | string[] | Filter by topic tags |
| `filters.content_type` | string | `article`, `page`, `video` |
| `filters.min_quality_score` | int | Minimum quality score (0-100) |
| `filters.source_names` | string[] | Filter by source name |
| `filters.from_date` | datetime | Published date range start |
| `filters.to_date` | datetime | Published date range end |
| `pagination.page` | int | Page number (default: 1) |
| `pagination.size` | int | Results per page (default: 20, max: 100) |
| `sort.field` | string | `relevance`, `published_date`, `quality_score` |
| `sort.order` | string | `asc` or `desc` |
| `options.include_highlights` | bool | Return matched text snippets |
| `options.include_facets` | bool | Return aggregation counts |

### GET /api/v1/search

Simple queries via query parameters: `q`, `page`, `size`, `min_quality`, `topics`, `content_type`, `source`, `include_facets`.

### GET /health

Public endpoint. Returns ES connection status. No authentication required.

## Configuration

`config.yml` is the primary configuration file. All values can be overridden with environment variables.

```yaml
service:
  port: 8090              # Internal port (exposed as 8092 in dev via Docker)
  max_page_size: 100
  default_page_size: 20
  max_query_length: 500
  search_timeout: "5s"

elasticsearch:
  url: "http://elasticsearch:9200"
  classified_content_pattern: "*_classified_content"
  default_boost:
    title: 3.0
    og_title: 2.0
    raw_text: 1.0
```

Key environment variables:

| Variable | Description |
|----------|-------------|
| `SEARCH_PORT` | Override service port |
| `ELASTICSEARCH_URL` | ES cluster URL |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` or `console` |

## Common Gotchas

1. **Searches classified_content only**: Raw content is not searchable via this service. If an article is missing from search results, check whether it has been classified (`classification_status` field in the raw index).

2. **Max query length**: Queries are limited to 500 characters by default. Longer queries are rejected with a validation error.

3. **Facets are expensive**: Aggregations add noticeable ES overhead. Only pass `include_facets=true` when the client actually renders filter counts.

4. **Search timeout**: Default is 5 seconds per query (configured in `config.yml` as `search_timeout`). Long-running queries beyond this threshold return a partial or empty result rather than waiting.

5. **Port differs in dev vs. prod**: The service listens on internal port 8090. In development, Docker maps this to `localhost:8092`. In production, nginx routes `/api/search` to the internal port — do not use 8092 in production configurations.

## Testing

```bash
# Unit tests
task test

# Unit tests with coverage
task test:coverage

# Run a specific test file
cd search && GOWORK=off go test ./internal/elasticsearch/... -v

# Integration test (requires running ES)
task test:integration

# Manual smoke test
curl "http://localhost:8092/api/v1/search?q=test"
```

## Code Patterns

### Elasticsearch query structure

The query builder in `internal/elasticsearch/query_builder.go` constructs a bool query that combines full-text search with filters:

```go
// Simplified example of the ES query structure produced
{
  "query": {
    "bool": {
      "must": [
        {
          "multi_match": {
            "query": "crime downtown",
            "fields": ["title^3", "og_title^2", "raw_text^1"],
            "fuzziness": "AUTO"
          }
        }
      ],
      "filter": [
        { "terms": { "topics": ["crime"] } },
        { "range": { "quality_score": { "gte": 60 } } }
      ]
    }
  },
  "highlight": {
    "fields": { "title": {}, "raw_text": {} }
  },
  "aggs": {
    "topics": { "terms": { "field": "topics", "size": 20 } }
  }
}
```

### Index wildcard

```go
// Targets all classified_content indexes across every source
esClient.Search.WithIndex(cfg.Elasticsearch.ClassifiedContentPattern)
// cfg.Elasticsearch.ClassifiedContentPattern = "*_classified_content"
```
