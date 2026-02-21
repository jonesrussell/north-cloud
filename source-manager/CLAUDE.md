# Source Manager — Developer Guide

This file provides guidance to Claude Code (claude.ai/code) when working with the source-manager service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload (Air)
task test             # Run tests
task lint             # Run linter
task migrate:up       # Run migrations

# Force-run lint/test bypassing task cache (matches CI)
task lint:force
task test:source-manager -f

# API (port 8050)
curl http://localhost:8050/api/v1/sources
curl http://localhost:8050/api/v1/sources/test-crawl \
  -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com", "selectors": {"article": {"title": "h1", "body": "article"}}}'
```

## Architecture

```
source-manager/
├── main.go
└── internal/
    ├── api/           # HTTP router (Gin) — route definitions and CORS config
    ├── bootstrap/     # Phased startup: profiling → config/logger → database → event publisher → server
    ├── config/        # Config struct with env-tag loading
    ├── database/      # PostgreSQL connection helpers
    ├── events/        # Redis event publisher (source created/updated/deleted)
    ├── handlers/      # HTTP handlers (SourceHandler)
    ├── importer/      # Excel bulk-import logic
    ├── metadata/      # Auto-fetch page title and selector hints from a URL
    ├── models/        # Source, SelectorConfig, City structs
    ├── repository/    # PostgreSQL source repository (CRUD)
    └── testhelpers/   # Shared test utilities
```

## Key Concepts

### Source Model

```go
type Source struct {
    ID                      string         `json:"id"`
    Name                    string         `json:"name"`                     // Unique; used to derive ES index name
    URL                     string         `json:"url"`                      // Base URL to crawl
    RateLimit               string         `json:"rate_limit"`               // Max requests per minute (string)
    MaxDepth                int            `json:"max_depth"`                // Crawl depth limit
    Time                    StringArray    `json:"time"`                     // Timezone/date format hints
    Selectors               SelectorConfig `json:"selectors"`                // CSS selectors (nested)
    Enabled                 bool           `json:"enabled"`
    FeedURL                 *string        `json:"feed_url,omitempty"`
    SitemapURL              *string        `json:"sitemap_url,omitempty"`
    IngestionMode           string         `json:"ingestion_mode"`
    FeedPollIntervalMinutes int            `json:"feed_poll_interval_minutes"`
    CreatedAt               time.Time      `json:"created_at"`
    UpdatedAt               time.Time      `json:"updated_at"`
}

// SelectorConfig is the top-level selectors object with three sub-objects.
type SelectorConfig struct {
    Article ArticleSelectors `json:"article"`
    List    ListSelectors    `json:"list"`
    Page    PageSelectors    `json:"page"`
}

// ArticleSelectors defines CSS selectors for individual article extraction.
// Key fields: Container, Title, Body, Byline, PublishedTime, JSONLD.
type ArticleSelectors struct {
    Container     string   `json:"container,omitempty"`
    Title         string   `json:"title,omitempty"`
    Body          string   `json:"body,omitempty"`
    Byline        string   `json:"byline,omitempty"`
    PublishedTime string   `json:"published_time,omitempty"`
    // ... additional fields: Intro, Link, Image, TimeAgo, Section, Category, ArticleID,
    //     JSONLD, Keywords, Description, OGTitle, OGDescription, OGImage, OGURL,
    //     OGType, OGSiteName, Canonical, Author, Exclude
}

// ListSelectors defines CSS selectors for list/index page extraction.
type ListSelectors struct {
    Container       string   `json:"container,omitempty"`
    ArticleCards    string   `json:"article_cards,omitempty"`
    ArticleList     string   `json:"article_list,omitempty"`
    ExcludeFromList []string `json:"exclude_from_list,omitempty"`
}

// PageSelectors defines CSS selectors for static page extraction.
type PageSelectors struct {
    Container   string   `json:"container,omitempty"`
    Title       string   `json:"title,omitempty"`
    Content     string   `json:"content,omitempty"`
    // ... additional fields: Description, Keywords, OGTitle, OGDescription,
    //     OGImage, OGURL, Canonical, Exclude
}
```

### Index Name Derivation

The source `name` is sanitized to produce a stable Elasticsearch index prefix. The sanitization lowercases the name and replaces spaces, dots, dashes, and other non-alphanumeric characters with underscores:

```
"Example News" → "example_news_raw_content"
"CBC.ca"       → "cbc_ca_raw_content"
"My-Source"    → "my_source_raw_content"
```

Index names are never stored in the database — they are derived at runtime by every service that needs them (crawler, classifier, publisher).

### Test Crawl

`POST /api/v1/sources/test-crawl` fetches a URL and applies given selectors without persisting anything. Use it to validate selectors before creating a source:

```bash
curl -X POST http://localhost:8050/api/v1/sources/test-crawl \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://example.com/article",
    "selectors": {
      "article": {
        "title": "h1",
        "body": "article",
        "published_time": "time[datetime]"
      }
    }
  }'
```

Response (currently simulated — the handler returns a fixed stub response rather than actually crawling):
```json
{
  "articles_found": 10,
  "success_rate": 90,
  "warnings": [
    "No author selector matched on 2 articles"
  ],
  "sample_articles": [
    {
      "title": "Sample Article 1",
      "body": "This is a sample article extracted from the test crawl...",
      "url": "https://example.com/article/article-1",
      "published_date": "2026-01-02T10:00:00Z",
      "author": "John Doe",
      "quality_score": 85
    },
    {
      "title": "Sample Article 2",
      "body": "Another sample article demonstrating the crawl results...",
      "url": "https://example.com/article/article-2",
      "published_date": "2026-01-02T09:30:00Z",
      "author": "",
      "quality_score": 72
    }
  ]
}
```

### Metadata Auto-Fetch

`POST /api/v1/sources/fetch-metadata` fetches a URL and returns suggested field values (title, selector hints) to pre-populate the dashboard create-source form. Nothing is saved.

### Excel Import

`POST /api/v1/sources/import-excel` accepts a multipart Excel file and bulk-creates sources. Expected column layout:

| Name | URL | Rate Limit | Title Selector | Body Selector |
|------|-----|------------|----------------|---------------|
| Example News | https://example.com | 10 | h1.title | article.body |

## API Reference

All write endpoints require a JWT in the `Authorization: Bearer <token>` header. Read endpoints (`GET /api/v1/sources` and `GET /api/v1/cities`) are intentionally public for internal service-to-service calls.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/sources` | Public | List all sources |
| `GET` | `/api/v1/sources/:id` | JWT | Get source by ID |
| `POST` | `/api/v1/sources` | JWT | Create source |
| `PUT` | `/api/v1/sources/:id` | JWT | Update source |
| `DELETE` | `/api/v1/sources/:id` | JWT | Delete source |
| `POST` | `/api/v1/sources/test-crawl` | JWT | Preview selectors without saving |
| `POST` | `/api/v1/sources/fetch-metadata` | JWT | Auto-fetch selector hints from URL |
| `POST` | `/api/v1/sources/import-excel` | JWT | Bulk import from Excel file |
| `GET` | `/api/v1/cities` | Public | List cities from enabled sources |
| `GET` | `/health` | Public | Health check |

## Configuration

Config is loaded from `config.yml` (path overridable with `-config` flag) with environment variable overrides:

| Variable | Description |
|----------|-------------|
| `APP_DEBUG` | Enable debug mode |
| `SERVER_HOST` | Server bind host |
| `SERVER_PORT` | Server bind port (default `8050`) |
| `DB_HOST` | PostgreSQL host |
| `DB_PORT` | PostgreSQL port |
| `DB_USER` | PostgreSQL user |
| `DB_PASSWORD` | PostgreSQL password |
| `DB_NAME` | PostgreSQL database name |
| `DB_SSLMODE` | PostgreSQL SSL mode |
| `AUTH_JWT_SECRET` | Shared JWT secret (must match all other services) |
| `SOURCE_MANAGER_API_URL` | Base URL used for dynamic CORS origin derivation |

## Common Gotchas

1. **Source name must be unique**: The name is the logical key for Elasticsearch indexes (`{sanitized_name}_raw_content`). Duplicate names cause index collisions across the pipeline.

2. **Name is sanitized for ES indexes**: Spaces, dots, dashes, and other non-alphanumeric characters all become underscores; the result is lowercased. The raw name is stored in the database; sanitization happens at runtime.

3. **Selectors have defaults**: If `title`, `body`, or `published_time` selectors are omitted, the crawler falls back to common patterns (`h1`, `article`, `time[datetime]`).

4. **Rate limit is per minute**: `rate_limit: 10` means the crawler sends at most 10 requests per minute to that source.

5. **Crawler uses `source_id`**: When creating crawler jobs, supply the source's UUID (`id` field), not its name. The name is only for human readability and index derivation.

6. **Public vs protected routes**: `GET /api/v1/sources` and `GET /api/v1/cities` skip JWT validation intentionally so the crawler and publisher can call them without token management. All mutating routes (`POST`, `PUT`, `DELETE`) require a JWT.

7. **Excel import endpoint path**: The import endpoint is `/api/v1/sources/import-excel` (not `/import`). It expects a multipart form upload, not JSON.

## Testing

```bash
# Run all tests
task test

# Run with coverage report
task test:cover

# Run directly (respects GOWORK=off)
cd source-manager && go test ./...
```

Test helpers live in `internal/testhelpers/`. All helper functions call `t.Helper()` as the first statement.

## Code Patterns

### Bootstrap Phase Order

`internal/bootstrap/app.go` initialises in this order:

- **Phase 0**: Profiling server (`profiling.StartPprofServer()`)
- **Phase 1**: Config + Logger (`LoadConfig`, `CreateLogger`)
- **Phase 2**: Database (`SetupDatabase`)
- **Phase 3**: Event publisher (`SetupEventPublisher` — Redis, optional)
- **Phase 4**: HTTP Server (`SetupHTTPServer` + `server.Run()`, blocks until exit)

There is no separate Lifecycle phase; graceful shutdown is handled inside `server.Run()`. Adding a new dependency should slot into the appropriate phase and be wired through `app.go`.

### Event Publishing

Source create/update/delete handlers publish Redis events via `internal/events/Publisher`. Downstream services (e.g., the dashboard) can subscribe to receive real-time source change notifications.

### Config Loading

Use `infraconfig.GetConfigPath("config.yml")` for the default config path. The `-config` CLI flag allows overriding the path at startup, which is used by integration tests and local development with a non-default config file.
