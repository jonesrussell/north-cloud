# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the publisher service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload (both API + router)
task run:api          # API server only
task run:router       # Router service only
task test             # Run tests
task lint             # Run linter
task migrate:up       # Run migrations

# API (port 8070)
curl http://localhost:8070/api/v1/routes
curl http://localhost:8070/api/v1/routes/preview
```

## Architecture

**Two-Service Design** - runs as API server + background router:

```
publisher/
├── main.go              # Multi-command entry (both/api/router)
├── cmd_api.go           # REST API server
├── cmd_router.go        # Background worker
├── internal/
│   ├── api/             # HTTP handlers (Gin)
│   ├── router/          # Core routing logic
│   ├── database/        # PostgreSQL repositories
│   ├── models/          # Source, Channel, Route, PublishHistory
│   ├── redis/           # Redis pub/sub client
│   └── dedup/           # Deduplication tracking
└── docs/
    ├── REDIS_MESSAGE_FORMAT.md
    └── CONSUMER_GUIDE.md
```

## Database Schema

| Table | Purpose |
|-------|---------|
| `sources` | Elasticsearch indexes to monitor |
| `channels` | Redis pub/sub topics |
| `routes` | Many-to-many mappings with filters |
| `publish_history` | Audit trail for deduplication |

**Route Filtering**:
- `min_quality_score` (0-100, default 50)
- `topics[]` (optional - articles must match at least one)
- `content_type: "article"` (excludes pages/listings)

## Channel Model

The publisher is **topic-driven and consumer-agnostic**. Channels define *what* content is published; the publisher does not track or limit who subscribes.

- **Layer 1 (automatic topic channels)**: For each article topic, the router publishes to `articles:{topic}` (e.g. `articles:technology`, `articles:politics`, `articles:violent_crime`). These channels always exist; no DB record or configuration required. Any number of consumers may subscribe.
- **Layer 2 (custom channels)**: Optional DB-backed channels with rules (include/exclude topics, min quality, content types). Used for aggregations (e.g. one channel for all crime sub-categories). Stored in the `channels` table. Same rule: one channel can serve unlimited consumers.
- **Layer 3 (crime classification channels)**: Automatic channels based on classifier's hybrid rule + ML crime classification. Routes to `crime:homepage` (for homepage-eligible articles) and `crime:category:{type}` (for category page listings). Only articles with `crime_relevance != "not_crime"` are routed.

Name and describe channels by **content** (e.g. "Crime Feed", "Technology Feed"), not by consumer (e.g. avoid "StreetCode Crime Feed").

## Redis Pub/Sub

**Layer 1 & 2 Channel Pattern**: `articles:{topic}`
- `articles:crime:violent`
- `articles:crime:property`
- `articles:crime:drug`
- `articles:crime:organized`
- `articles:crime:justice`
- `articles:news`

**Layer 3 Crime Channel Patterns**:
- `crime:homepage` - Homepage-eligible crime articles (core_street_crime with high confidence)
- `crime:category:{type}` - Category page listings (e.g., `crime:category:violent-crime`, `crime:category:crime`)

**Message Format**:
```json
{
  "publisher": {
    "route_id": "uuid",
    "published_at": "2025-12-28T15:30:45Z",
    "channel": "articles:crime:property"
  },
  "id": "es-doc-id",
  "title": "Article Title",
  "body": "Full text",
  "quality_score": 85,
  "topics": ["crime", "local"],
  "content_type": "article",
  "crime_relevance": "core_street_crime",
  "crime_types": ["violent_crime"],
  "location_specificity": "local_canada",
  "homepage_eligible": true,
  "category_pages": ["violent-crime", "crime"],
  "review_required": false,
  ...
}
```

**Crime Classification Fields** (from classifier's hybrid rule + ML):
| Field | Type | Description |
|-------|------|-------------|
| `crime_relevance` | string | `core_street_crime`, `peripheral_crime`, or `not_crime` |
| `crime_types` | []string | Multi-label: `violent_crime`, `property_crime`, `drug_crime`, `gang_violence`, `organized_crime`, `criminal_justice`, `other_crime` |
| `location_specificity` | string | `local_canada`, `national_canada`, `international`, `not_specified` |
| `homepage_eligible` | bool | True if article qualifies for homepage display |
| `category_pages` | []string | Category page slugs (e.g., `["violent-crime", "crime"]`) |
| `review_required` | bool | True if rules and ML disagreed (needs human review) |

## API Endpoints (JWT Protected)

**Sources**: `GET/POST/PUT/DELETE /api/v1/sources[/:id]`

**Channels**:
- `GET/POST/PUT/DELETE /api/v1/channels[/:id]`
- `GET /api/v1/channels/:id/test-publish`

**Routes**:
- `GET/POST/PUT/DELETE /api/v1/routes[/:id]`
- `GET /api/v1/routes/preview` - Preview matching articles

**History & Stats**:
- `GET /api/v1/publish-history`
- `GET /api/v1/stats/overview`
- `GET /api/v1/stats/channels`
- `GET /api/v1/articles/recent`

## Common Gotchas

1. **Deduplication is per-channel**: Same article can publish to multiple channels, but not the same channel twice.

2. **Index naming must match exactly**: Route's `source.index_pattern` must match Elasticsearch index name.

3. **Quality score range**: 0-100, defaults to 50 if not specified in route.

4. **Redis Pub/Sub semantics**: Only active subscribers receive messages (no queue - use Redis Streams for persistence).

5. **Router runs synchronously**: Processes one route at a time. Large datasets can be slow.

6. **Config file optional**: Service uses defaults if config.yml missing.

7. **Index not found is OK**: Returns empty results for new sources (not an error).

## Router Flow

1. **Poll** (every 30s): Fetches articles from all discovered `*_classified_content` indexes
2. **Route Layer 1**: Publishes to `articles:{topic}` for each article topic
3. **Route Layer 2**: Applies custom channel rules (quality, content type, topics)
4. **Route Layer 3**: Generates crime channels from classification fields
5. **Dedupe**: Checks `publish_history` table (per-channel deduplication)
6. **Publish**: Sends JSON to Redis channels
7. **Record**: Writes to `publish_history` for each channel

## Configuration

```yaml
router:
  check_interval: 5m      # PUBLISHER_ROUTER_CHECK_INTERVAL
  batch_size: 100         # PUBLISHER_ROUTER_BATCH_SIZE

database:
  # Uses POSTGRES_PUBLISHER_* env vars
```

## Code Patterns

**Continue on route errors**:
```go
for _, route := range routes {
    if err := s.processRoute(ctx, &route); err != nil {
        s.logger.Error("Error processing route", ...)
        continue  // Don't stop other routes
    }
}
```

**Deduplication check**:
```go
published, _ := s.repo.CheckArticlePublished(ctx, articleID, channelName)
if published {
    continue  // Skip already published
}
```

## Documentation

- `/publisher/docs/REDIS_MESSAGE_FORMAT.md` - Message specification
- `/publisher/docs/CONSUMER_GUIDE.md` - Integration examples
