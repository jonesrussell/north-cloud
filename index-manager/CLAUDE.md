# Index Manager — Developer Guide

## Quick Reference

```bash
# Daily development commands
task dev              # Start service (or use Docker: task docker:dev:up)
task test             # Run tests
task test:cover       # Run tests with coverage
task lint             # Run linter
task migrate:up       # Run pending migrations

# Useful API calls (port 8090)
curl http://localhost:8090/health
curl http://localhost:8090/api/v1/indexes
curl http://localhost:8090/api/v1/indexes/example_com_raw_content
curl http://localhost:8090/api/v1/stats
curl http://localhost:8090/api/v1/aggregations/source-health

# Provision indexes for a new source
curl -X POST http://localhost:8090/api/v1/sources/example_com/indexes \
  -H "Content-Type: application/json" \
  -d '{"index_types": ["raw_content", "classified_content"]}'

# Query documents with pagination
curl "http://localhost:8090/api/v1/indexes/example_com_classified_content/documents?query=crime&page=1&size=20"

# Delete an index (irreversible — see mappings gotcha)
curl -X DELETE http://localhost:8090/api/v1/indexes/example_com_raw_content
```

## Architecture

```
index-manager/
├── main.go                         # Entry point: bootstrap.Start()
├── config.yml                      # Service configuration
├── migrations/                     # PostgreSQL migration SQL files
└── internal/
    ├── api/
    │   ├── server.go               # infragin.ServerBuilder setup
    │   ├── routes.go               # All route definitions
    │   └── handlers.go             # HTTP handler implementations
    ├── bootstrap/
    │   ├── app.go                  # Start(): phased init (profiling→config→ES→DB→HTTP)
    │   ├── config.go               # Config loading via infraconfig
    │   ├── database.go             # PostgreSQL connection setup
    │   ├── elasticsearch.go        # ES client setup
    │   └── server.go               # HTTP server wiring (services → handler → server)
    ├── service/
    │   ├── index_service.go        # Index lifecycle operations
    │   ├── document_service.go     # Document CRUD via ES
    │   ├── aggregation_service.go  # Aggregation queries (crime, mining, source health, drift)
    │   └── aggregation_es.go       # AggregationESClient interface (for unit testing)
    ├── elasticsearch/
    │   ├── client.go               # ES client wrapper
    │   ├── index_manager.go        # Index lifecycle (create, delete, health)
    │   ├── query_builder.go        # ES query construction helpers
    │   └── mappings/
    │       ├── raw_content.go      # raw_content index mapping
    │       ├── classified_content.go  # classified_content mapping (uses extracted helpers)
    │       ├── factory.go          # Mapping factory by index type
    │       ├── mappings.go         # Shared mapping utilities
    │       ├── versions.go         # Mapping version constants
    │       └── mappings_test.go
    ├── database/                   # PostgreSQL: migrations, metadata persistence
    ├── config/                     # Config struct with env/yaml tags and defaults
    └── domain/                     # Index, Document, Aggregation domain models
```

## Key Concepts

### Index Types

| Type | Pattern | Created by | Purpose |
|------|---------|------------|---------|
| `raw_content` | `{source}_raw_content` | Crawler (or index-manager) | Crawled content, `classification_status=pending` |
| `classified_content` | `{source}_classified_content` | Classifier (or index-manager) | Enriched content with quality, topics, crime/mining fields |

Always use underscores in source names (e.g., `example_com` not `example.com`). The naming convention is `{source_name}_{type}`.

### ES Mapping Structure

**`raw_content` key fields**:
```json
{
  "url":                   { "type": "keyword" },
  "title":                 { "type": "text" },
  "raw_text":              { "type": "text" },
  "source_name":           { "type": "keyword" },
  "classification_status": { "type": "keyword" },
  "crawled_at":            { "type": "date" }
}
```

**`classified_content` additional fields**:
```json
{
  "content_type":      { "type": "keyword" },
  "quality_score":     { "type": "integer" },
  "topics":            { "type": "keyword" },
  "source_reputation": { "type": "integer" },
  "classified_at":     { "type": "date" },
  "crime":             { "type": "object", "properties": { "street_crime_relevance", "crime_types", "..." } },
  "mining":            { "type": "object", "properties": { "relevance", "mining_stage", "commodities", "location", "..." } }
}
```

The `classified_content.go` mapping file uses extracted helpers (`getCrimeMapping()`, `getLocationMapping()`, `getMiningMapping()`) to stay under the 100-line `funlen` lint limit.

### Mapping Version Drift Check

On startup (`bootstrap/app.go`), `CheckMappingVersionDrift()` compares the compiled mapping versions against what was last recorded in the `index_metadata` table. A warning is logged if drift is detected — this is a signal to migrate indexes.

## API Reference

All routes are registered in `internal/api/routes.go`. There is no JWT middleware at the service level — access control is enforced externally (nginx in production).

**Index management**: `POST/GET/DELETE /api/v1/indexes`, `GET/POST /:index_name/health|migrate`

**Document operations**: `GET/PUT/DELETE /api/v1/indexes/:index_name/documents/:document_id`, `POST /bulk-delete`

**Source-based**: `POST/GET/DELETE /api/v1/sources/:source_name/indexes`

**Bulk**: `POST /api/v1/indexes/bulk/create`, `DELETE /api/v1/indexes/bulk/delete`

**Stats**: `GET /api/v1/stats`

**Aggregations**:
- `GET /api/v1/aggregations/crime` — crime classification breakdown
- `GET /api/v1/aggregations/mining` — mining classification breakdown (filter: `source`)
- `GET /api/v1/aggregations/location` — location breakdown
- `GET /api/v1/aggregations/overview` — high-level content overview
- `GET /api/v1/aggregations/source-health` — per-source pipeline health (raw/classified counts, backlog, 24h delta, avg quality)
- `GET /api/v1/aggregations/classification-drift` — raw vs classified gap (param: `hours`, `sources[]`)
- `GET /api/v1/aggregations/classification-drift-timeseries` — drift trend (param: `days`)
- `GET /api/v1/aggregations/content-type-mismatch` — mismatched content types (param: `hours`)
- `GET /api/v1/aggregations/suspected-misclassifications` — suspected misclassifications (param: `hours`)

See `internal/api/handlers.go` for full query parameter details per endpoint.

## Configuration

Configuration is loaded from `config.yml` via `infraconfig.LoadWithDefaults`. All keys can be overridden with environment variables (using the `env` struct tag).

| Env Variable | yaml key | Default | Description |
|---|---|---|---|
| `INDEX_MANAGER_PORT` | `service.port` | `8090` | HTTP listen port |
| `APP_DEBUG` | `service.debug` | `false` | Gin debug mode |
| `POSTGRES_INDEX_MANAGER_HOST` | `database.host` | `localhost` | DB host |
| `POSTGRES_INDEX_MANAGER_PORT` | `database.port` | `5432` | DB port |
| `POSTGRES_INDEX_MANAGER_USER` | `database.user` | `postgres` | DB user |
| `POSTGRES_INDEX_MANAGER_PASSWORD` | `database.password` | _(none)_ | DB password |
| `POSTGRES_INDEX_MANAGER_DB` | `database.database` | `index_manager` | DB name |
| `ELASTICSEARCH_URL` | `elasticsearch.url` | `http://localhost:9200` | ES endpoint |
| `LOG_LEVEL` | `logging.level` | `info` | Log level |
| `LOG_FORMAT` | `logging.format` | `json` | Log format |

## Common Gotchas

1. **Port conflict with search in dev**: Both index-manager and search default to port 8090. In dev, the compose file routes them to different internal ports (search gets 8092). Do not run both locally on 8090 simultaneously.

2. **Index naming convention**: Always use `{source_name}_{type}` with underscores. Source names must use underscores (e.g., `example_com`), not dots or hyphens, because ES index names cannot contain dots in all contexts.

3. **Mappings are immutable**: Once an Elasticsearch index is created, its mapping cannot be changed in place. To update a mapping, delete the index and recreate it (`POST /:index_name/migrate` handles this). Deleting an index destroys all data — the crawler must re-crawl to repopulate `raw_content`, and the classifier must re-run for `classified_content`.

4. **Dynamic vs explicit mapping drift**: The classifier creates indexes on the fly with ES dynamic mappings. In dynamic mappings, `source_name` becomes type `text` (with a `.keyword` sub-field), whereas index-manager's explicit mappings define it as pure `keyword`. When running aggregations on dynamically-mapped indexes, use `source_name.keyword` to target the keyword sub-field. Using the bare `source_name` field on a dynamically-mapped index causes ES to return a 400 error because fielddata is disabled for text fields by default. See `fetchClassifiedAggregations` in `aggregation_service.go` for the correct pattern. This caused a production bug where source health aggregations silently returned empty results.

5. **Bulk operations continue on error**: Partial failures in `POST /api/v1/indexes/bulk/create` and `DELETE /api/v1/indexes/bulk/delete` return `207 Multi-Status`. The response body contains both `created`/`deleted` and `errors` arrays. Check the status code before assuming all operations succeeded.

6. **Document IDs are ES-generated**: Unless explicitly provided in the `PUT` request, document IDs are assigned by Elasticsearch. Retrieve the ID from the create response before attempting to reference a document by ID.

7. **`source-health` aggregation uses `.keyword` sub-fields**: The aggregation queries both raw and classified indexes. Classified indexes created by the classifier use dynamic mappings, so the aggregation explicitly targets `source_name.keyword`. Do not change this to bare `source_name` — it breaks on dynamically-mapped indexes.

8. **Mapping version drift logged but not blocking**: `CheckMappingVersionDrift()` only logs a warning on startup; it does not prevent the service from starting. If you update a mapping definition in `internal/elasticsearch/mappings/`, bump the version constant in `versions.go` and run `POST /:index_name/migrate` for each affected index.

## Testing

```bash
# Run all tests
task test

# Run with coverage report
task test:cover

# Or directly
cd index-manager && go test ./...
cd index-manager && go test -cover ./...
```

**Mock pattern**: `AggregationESClient` is an interface defined in `aggregation_es.go`. `aggregation_service.go` depends on this interface rather than the concrete ES client, enabling unit tests to inject a mock. See `aggregation_service_test.go` for examples covering valid responses, ES errors, malformed JSON, null values, and empty result sets.

`IndexService` tests in `index_service_test.go` follow the same pattern with mock ES and DB dependencies.

`query_builder_test.go` tests the ES query construction helpers independently of any ES connection.

## Code Patterns

### Creating an index via the service layer

```go
req := &domain.CreateIndexRequest{
    IndexName:  "example_com_raw_content",
    IndexType:  domain.IndexTypeRawContent,
    SourceName: "example_com",
}
index, err := indexService.CreateIndex(ctx, req)
if err != nil {
    return fmt.Errorf("create index: %w", err)
}
```

### Querying documents with pagination

```go
req := &domain.DocumentQueryRequest{
    Query: "crime",
    Pagination: &domain.DocumentPagination{Page: 1, Size: 20},
    Sort: &domain.DocumentSort{Field: "relevance", Order: "desc"},
}
resp, err := documentService.QueryDocuments(ctx, "example_com_classified_content", req)
if err != nil {
    return fmt.Errorf("query documents: %w", err)
}
```

### Aggregation with source filter

```go
req := &domain.AggregationRequest{
    Filters: &domain.DocumentFilters{
        Sources: []string{"example_com"},
    },
}
result, err := aggregationService.GetCrimeAggregation(ctx, req)
```
