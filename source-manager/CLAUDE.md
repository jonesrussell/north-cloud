# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the source-manager service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload
task test             # Run tests
task lint             # Run linter
task migrate:up       # Run migrations

# API (port 8050)
curl http://localhost:8050/api/v1/sources
curl http://localhost:8050/api/v1/sources/test-crawl -X POST -d '{"url": "...", "selectors": {...}}'
```

## Architecture

```
source-manager/
├── main.go
└── internal/
    ├── api/           # HTTP handlers (Gin)
    ├── repository/    # PostgreSQL source repository
    ├── models/        # Source, Selectors, City
    ├── metadata/      # Source metadata extraction
    ├── importer/      # Excel import for bulk sources
    ├── database/      # Database connection
    └── testhelpers/   # Test utilities
```

## Source Model

```go
type Source struct {
    ID        string     `json:"id"`
    Name      string     `json:"name"`      // Unique identifier
    URL       string     `json:"url"`       // Base URL to crawl
    RateLimit int        `json:"rate_limit"` // Requests per minute
    MaxDepth  int        `json:"max_depth"`  // Crawl depth
    Selectors Selectors  `json:"selectors"`  // CSS selectors
    Time      TimeConfig `json:"time"`       // Timezone, date formats
    Enabled   bool       `json:"enabled"`
}

type Selectors struct {
    Title   string `json:"title"`    // e.g., "h1.article-title"
    Body    string `json:"body"`     // e.g., "article.content"
    Date    string `json:"date"`     // e.g., "time[datetime]"
    Author  string `json:"author"`   // e.g., ".byline"
}
```

## API Endpoints (JWT Protected)

**CRUD**:
- `GET /api/v1/sources` - List all sources
- `GET /api/v1/sources/:id` - Get source by ID
- `POST /api/v1/sources` - Create source
- `PUT /api/v1/sources/:id` - Update source
- `DELETE /api/v1/sources/:id` - Delete source

**Test Crawl**:
- `POST /api/v1/sources/test-crawl` - Preview crawl without saving

**Cities**:
- `GET /api/v1/cities` - List cities (derived from enabled sources)

**Import**:
- `POST /api/v1/sources/import` - Bulk import from Excel

## Test Crawl Feature

Preview selectors before creating a source:

```bash
curl -X POST http://localhost:8050/api/v1/sources/test-crawl \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/article",
    "selectors": {
      "title": "h1",
      "body": "article",
      "date": "time[datetime]"
    }
  }'
```

**Response**:
```json
{
  "success": true,
  "extracted": {
    "title": "Article Title",
    "body": "Full article text...",
    "date": "2025-12-28"
  }
}
```

## Common Gotchas

1. **Source name must be unique**: Used as identifier for Elasticsearch indexes (e.g., `{source_name}_raw_content`).

2. **Name is sanitized for ES indexes**: Spaces, dots, dashes become underscores; all lowercase.

3. **Selectors have defaults**: If not specified, uses common patterns (`h1`, `article`, `time[datetime]`).

4. **Rate limit is per minute**: `rate_limit: 10` means max 10 requests/minute to that source.

5. **Crawler uses `source_id`**: When creating crawler jobs, you need the source's UUID, not name.

## Index Name Derivation

Source names are sanitized for Elasticsearch:

```
"Example News" → "example_news_raw_content"
"CBC.ca"       → "cbc_ca_raw_content"
"My-Source"    → "my_source_raw_content"
```

## Database Schema

**sources** table:
```sql
CREATE TABLE sources (
    id UUID PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    url TEXT NOT NULL,
    rate_limit INTEGER DEFAULT 10,
    max_depth INTEGER DEFAULT 2,
    time JSONB,
    selectors JSONB NOT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

## Excel Import Format

For bulk importing sources:

| Name | URL | Rate Limit | Title Selector | Body Selector |
|------|-----|------------|----------------|---------------|
| Example News | https://example.com | 10 | h1.title | article.body |

## Testing

```bash
# Run tests
task test

# Run with coverage
task test:cover
```
