# Search Service

Full-text search microservice for the North Cloud platform. Provides Google-like search across all classified content with relevance ranking, advanced filtering, faceted search, and pagination.

## Features

- **Full-text search** across title, body text, OG tags, and metadata
- **Relevance ranking** with configurable field boosting
- **Advanced filtering** by topics, content type, quality score, date ranges, and source
- **Faceted search** with aggregations for topics, sources, and content types
- **Search highlighting** to show matched text snippets
- **Pagination** with configurable page sizes
- **Multi-field sorting** (relevance, date, quality score)
- **Public API** (no authentication required for MVP)

## Quick Start

### Development

```bash
# Run locally (requires Go 1.26+)
cd search
task run

# Or use Docker
task docker:run
```

### Production

```bash
docker compose -f docker-compose.base.yml up -d search
```

## Integration

The search service queries all `*_classified_content` Elasticsearch indexes — only content that has passed through the classifier pipeline is searchable. Raw content (`*_raw_content`) is not exposed.

- **Upstream dependency**: Classifier must have processed and written to `{source}_classified_content` before articles appear in search results.
- **Production access**: Routed through nginx at `/api/search` (maps to internal port 8090).
- **Development access**: Exposed directly on `http://localhost:8092`.

## API Documentation

### Base URLs

- **Production**: `https://northcloud.biz/api/search`
- **Development**: `http://localhost:8092/api/v1/search`

### Endpoints

#### 1. Search

**POST /api/v1/search** (recommended for complex queries)

**Request Body**:
```json
{
  "query": "crime downtown",
  "filters": {
    "topics": ["crime", "local_news"],
    "content_type": "article",
    "min_quality_score": 60,
    "from_date": "2024-01-01T00:00:00Z",
    "to_date": "2024-12-31T23:59:59Z"
  },
  "pagination": {
    "page": 1,
    "size": 20
  },
  "sort": {
    "field": "relevance",
    "order": "desc"
  },
  "options": {
    "include_highlights": true,
    "include_facets": true
  }
}
```

**GET /api/v1/search** (simple queries via query parameters)

```bash
curl "http://localhost:8092/api/v1/search?q=crime&topics=crime&page=1&size=20&sort=relevance"
```

**Response**:
```json
{
  "query": "crime downtown",
  "total_hits": 1523,
  "total_pages": 77,
  "current_page": 1,
  "page_size": 20,
  "took_ms": 45,
  "hits": [
    {
      "id": "abc123",
      "title": "Downtown crime rates drop significantly",
      "url": "https://example.com/article/123",
      "published_date": "2024-12-15T10:30:00Z",
      "quality_score": 85,
      "topics": ["crime", "local_news"],
      "score": 12.5,
      "highlight": {
        "title": ["Downtown <em>crime</em> rates drop"],
        "body": ["...reduction in <em>downtown</em> <em>crime</em>..."]
      }
    }
  ],
  "facets": {
    "topics": [{"key": "crime", "count": 856}],
    "sources": [{"key": "example_com", "count": 500}]
  }
}
```

#### 2. Health Check

**GET /health**

```bash
curl http://localhost:8092/health
```

**Response**:
```json
{
  "status": "healthy",
  "timestamp": "2024-12-26T20:00:00Z",
  "version": "1.0.0",
  "dependencies": {
    "elasticsearch": "healthy"
  }
}
```

## Configuration

Configuration is loaded from `config.yml` with environment variable overrides.

### config.yml

```yaml
service:
  port: 8090
  debug: true
  max_page_size: 100
  default_page_size: 20

elasticsearch:
  url: "http://elasticsearch:9200"
  classified_content_pattern: "*_classified_content"

  default_boost:
    title: 3.0
    og_title: 2.0
    raw_text: 1.0
```

### Environment Variables

```bash
SEARCH_PORT=8090
SEARCH_DEBUG=true
ELASTICSEARCH_URL=http://elasticsearch:9200
LOG_LEVEL=info
LOG_FORMAT=json
```

## Search Query Parameters

### Filters

- `topics` (array): Filter by topics (e.g., `["crime", "local_news"]`)
- `content_type` (string): Filter by content type (`article`, `page`, `video`)
- `min_quality_score` (int): Minimum quality score (0-100)
- `max_quality_score` (int): Maximum quality score (0-100)
- `is_crime_related` (bool): Filter crime-related content
- `source_names` (array): Filter by source names
- `from_date` (datetime): Start date for published_date range
- `to_date` (datetime): End date for published_date range

### Pagination

- `page` (int): Page number (default: 1)
- `size` (int): Results per page (default: 20, max: 100)

### Sorting

- `field` (string): Sort field (`relevance`, `published_date`, `quality_score`, `crawled_at`)
- `order` (string): Sort order (`asc`, `desc`)

### Options

- `include_highlights` (bool): Include matched text snippets (default: true)
- `include_facets` (bool): Include aggregations (default: true)
- `source_fields` (array): Specific fields to return

## Development

### Prerequisites

- Go 1.26+
- Docker & Docker Compose
- Task (task runner)
- Elasticsearch 9.x

### Setup

```bash
# Install dependencies
go mod download

# Install development tools
task install:tools

# Run tests
task test

# Run with coverage
task test:coverage

# Format code
task fmt

# Lint code
task lint
```

### Docker Development

```bash
# Build development image
task docker:build:dev

# Run in Docker
task docker:run

# View logs
task docker:logs

# Stop service
task docker:stop
```

### Hot Reload (Air)

```bash
# Start with hot-reload
task dev
```

## Testing

### Unit Tests

```bash
task test:unit
```

### Integration Tests

```bash
task test:integration
```

### Manual Testing

```bash
# Health check
task health

# Sample search
task search
```

## Architecture

### Components

- **API Layer** (`internal/api`): Gin HTTP server, handlers, middleware
- **Service Layer** (`internal/service`): Business logic orchestration
- **Elasticsearch Layer** (`internal/elasticsearch`): Query builder, ES client
- **Domain Layer** (`internal/domain`): Models (SearchRequest, SearchResponse)
- **Config Layer** (`internal/config`): Configuration management

### Search Flow

1. **Request** — API handler parses JSON body or query parameters
2. **Validation** — Service validates request and applies defaults
3. **Query Building** — Query builder constructs Elasticsearch DSL
4. **Execution** — ES client executes search against `*_classified_content` indexes
5. **Parsing** — Service parses ES response into domain models
6. **Response** — API handler returns JSON response

### Elasticsearch Query Strategy

- **Multi-match** across title (3x boost), og_title (2x), raw_text (1x)
- **Bool query** combining full-text search with filters
- **Boosting** for recency (30-day decay) and quality (log scale)
- **Aggregations** for faceted search
- **Highlighting** for matched text snippets

## Performance

- **Target latency**: p95 < 200ms
- **Throughput**: 100+ concurrent requests
- **Page size limit**: Max 100 results per page
- **Search timeout**: 5s (configurable)
- **HTTP timeouts**: 30s read, 60s write

## Troubleshooting

### Service won't start

```bash
# Check Elasticsearch connection
curl http://localhost:9200/_cluster/health

# Check logs
task docker:logs

# Verify configuration
cat config.yml
```

### No search results

- Verify Elasticsearch has `*_classified_content` indexes
- Check index pattern in configuration
- Ensure content is classified (not just raw_content)
- Try broader query or remove filters

### Slow searches

- Check Elasticsearch cluster health
- Review query complexity (too many filters?)
- Consider reducing page size
- Check Elasticsearch query performance in Kibana
