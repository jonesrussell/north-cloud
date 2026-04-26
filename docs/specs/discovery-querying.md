# Discovery & Querying Specification

> Last verified: 2026-04-22 (Phase 1B: index-manager ES mappings defer to `infrastructure/esmapping`)

Covers the search service (full-text queries) and index-manager (ES lifecycle, mappings, aggregations).

## File Map

| File | Purpose |
|------|---------|
| `search/internal/elasticsearch/query_builder.go` | ES DSL construction with field boosting |
| `search/internal/api/handlers.go` | GET/POST /api/v1/search handlers |
| `search/internal/service/search_service.go` | Search orchestration |
| `search/internal/domain/search.go` | SearchRequest, SearchResponse types |
| `index-manager/internal/bootstrap/app.go` | 6-phase startup + mapping drift check |
| `index-manager/internal/service/index_service.go` | Index CRUD, naming, metadata |
| `index-manager/internal/service/aggregation_service.go` | Crime, mining, location, overview aggregations |
| `index-manager/internal/elasticsearch/index_manager.go` | ES index operations |
| `infrastructure/esmapping/` | SSoT Elasticsearch `raw_content` / `classified_content` property maps (shared with classifier) |
| `index-manager/internal/elasticsearch/mappings/classified_content.go` | Thin wrapper: delegates to `esmapping` for classified index mapping JSON |
| `index-manager/internal/elasticsearch/mappings/raw_content.go` | Thin wrapper: delegates to `esmapping` for raw index mapping JSON |
| `index-manager/internal/elasticsearch/mappings/versions.go` | RawContentMappingVersion, ClassifiedContentMappingVersion |
| `index-manager/migrations/001_create_index_metadata.up.sql` | index_metadata + migration_history tables |
| `search/internal/telemetry/telemetry.go` | Search-specific Prometheus metrics |
| `index-manager/internal/telemetry/telemetry.go` | Index-manager-specific Prometheus metrics |
| `infrastructure/elasticsearch/client.go` | Shared ES client with retry |

## Interface Signatures

### Query Builder (`search/internal/elasticsearch/query_builder.go`)
```go
func (b *QueryBuilder) Build(req *domain.SearchRequest) map[string]any
// Returns ES query DSL with:
//   must: multi_match (title^3, og_title^2, body^1, fuzziness: AUTO)
//   filter: quality range, topics.keyword, content_type.keyword, date range
//   should: recency boost, quality boost
//   aggs: topics, content_types, sources, quality_ranges (when include_facets=true)
// FacetBucket: { key: string, label: string, count: int64 }
// label is the human-readable form of key (e.g. "local_news" → "Local News")
```

### Index Service (`index-manager/internal/service/index_service.go`)
```go
func (s *IndexService) CreateIndex(req *CreateIndexRequest) error
func (s *IndexService) DeleteIndex(indexName string) error
func (s *IndexService) ListIndexes() ([]IndexInfo, error)
func (s *IndexService) GetIndexHealth(indexName string) (string, error)
func NormalizeSourceName(name string) string  // dots→_, hyphens→_, lowercase
func GenerateIndexName(sourceName string, indexType IndexType) string  // {normalized}_{type}
```

### Aggregation Service (`index-manager/internal/service/aggregation_service.go`)
```go
func (s *AggregationService) GetCrimeAggregation(ctx, req) (*CrimeAggregation, error)
func (s *AggregationService) GetMiningAggregation(ctx, req) (*MiningAggregation, error)
func (s *AggregationService) GetLocationAggregation(ctx, req) (*LocationAggregation, error)
func (s *AggregationService) GetOverviewAggregation(ctx, req) (*OverviewAggregation, error)
func (s *AggregationService) GetSourceHealthAggregation(ctx, req) (*SourceHealthAggregation, error)
```

## Data Flow

### Search Query
```
GET/POST /api/v1/search → parse request → validate (max 500 chars, max 100 per page)
  → QueryBuilder.Build() → multi-index search across *_classified_content
  → parseSearchResponse() → faceted results with aggregations
```

**topics query param formats** (both supported):
- Comma-separated: `?topics=indigenous,crime`
- Array syntax: `?topics[]=indigenous&topics[]=crime`

### Index Lifecycle
```
Source registered → CreateIndex({source}_raw_content) + CreateIndex({source}_classified_content)
  → Track in index_metadata table (name, type, mapping_version, status)
  → Classifier creates index dynamically if not exists (dynamic mapping)
  → Mapping drift: CheckMappingVersionDrift() warns at startup
  → Reindex: POST /:index_name/migrate creates new index, copies data
```

### Aggregation Queries
```
All aggregations: size=0 (no docs, just aggs) + optional filters
  Crime: by_sub_label, by_relevance, by_crime_type, crime_related count
  Mining: by_relevance, by_stage, by_commodity, by_location
  Location: by_country, by_province, by_city, by_specificity
  Overview: top cities, top crime types, quality distribution (high/medium/low)
  Source health: per-source document counts, quality averages, pipeline gaps
```

## Storage / Schema

### Index Naming Convention
```
{normalized_source_name}_{type}
Examples:
  example_com_raw_content
  bbc_news_classified_content
  cbc_ca_classified_content
```

### Classified Content Mapping (key fields)
```json
{
  "content_type": "keyword",
  "quality_score": "integer",
  "topics": "keyword",
  "source_reputation": "integer",
  "crime": { "type": "object", "properties": {
    "relevance": "keyword", "sub_label": "keyword",
    "crime_types": "keyword", "final_confidence": "float",
    "homepage_eligible": "boolean"
  }},
  "mining": { "type": "object", "properties": {
    "relevance": "keyword", "mining_stage": "keyword",
    "commodities": "keyword", "final_confidence": "float"
  }},
  "location": { "type": "object", "properties": {
    "city": "keyword", "province": "keyword",
    "country": "keyword", "specificity": "keyword"
  }},
  "icp": { "type": "object", "properties": {
    "segments": { "type": "nested", "properties": {
      "segment": "keyword",
      "score": "float",
      "matched_keywords": "keyword"
    }},
    "model_version": "keyword"
  }}
}
```
Helper functions: `getCrimeMapping()`, `getMiningMapping()`, `getLocationMapping()`, etc. (funlen compliance).

### Mapping Versions
```go
RawContentMappingVersion        = "2.0.0"
ClassifiedContentMappingVersion = "2.3.0"
```

### PostgreSQL Tables (index-manager)
- **index_metadata**: index_name (UNIQUE), index_type, source_name, mapping_version, status (active|archived|deleted)
- **migration_history**: index_name, from_version, to_version, migration_type, status, error_message

## Configuration

Search:
- Port: 8092 (dev), 8090 (prod via nginx)
- `max_page_size: 100`, `default_page_size: 20`, `max_query_length: 500`
- `search_timeout: 5s`

Index-Manager:
- Port: 8090
- Mapping versions compiled in code
- Drift detection at startup (warning only)
- `/health` performs real ES cluster health and DB ping checks; returns 503 with degraded status on failure
- Health handler uses `WithElasticsearchHealthCheck()` and `WithDatabaseHealthCheck()` on the server builder

## Edge Cases

- **CRITICAL — .keyword sub-fields**: Classifier creates dynamic mappings (text fields with .keyword sub-fields). ALWAYS use `source_name.keyword`, `topics.keyword`, `content_type.keyword` in aggregations. Bare field causes ES 400 (fielddata disabled on text).
- **Mappings are immutable**: Cannot change field types in ES. Must delete + recreate index.
- **Port conflict in dev**: Both search and index-manager use 8090 internally. Docker maps search to 8092.
- **Bulk operations 207 Multi-Status**: Partial failures return 207. Check each item.
- **Only classified_content searchable**: Raw content not in search results. Check classification_status.
- **Facets expensive**: Only request with include_facets=true when UI needs them.
- **Index naming normalization**: Dots and hyphens converted to underscores. Source "bbc-news.com" → "bbc_news_com".

## Telemetry & Health Checks

Both services expose Prometheus metrics via `WithMetrics()` on the Gin server builder (`GET /metrics`). Each has a `internal/telemetry/telemetry.go` package with service-specific counters and histograms.

### Dependency-Aware Health Checks

- **Search**: `WithElasticsearchHealthCheck(pingFn)` — health endpoint pings ES cluster; returns 503 on failure. `search/main.go` passes the ES client's `Ping` function.
- **Index-Manager**: `WithElasticsearchHealthCheck(pingFn)` and `WithDatabaseHealthCheck(db)` — health endpoint checks both ES and PostgreSQL; returns 503 if either is degraded. Configured in `index-manager/internal/bootstrap/server.go` which passes both `ESPing` and `DBPing` functions.
