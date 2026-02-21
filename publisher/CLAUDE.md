# Publisher — Developer Guide

This file provides guidance to Claude Code (claude.ai/code) when working with the publisher service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload (both API + router)
task run:api          # API server only (port 8070)
task run:router       # Router worker only
task test             # Run tests
task lint             # Run linter
task migrate:up       # Run migrations

# Common API calls
curl http://localhost:8070/api/v1/routes
curl http://localhost:8070/api/v1/routes/preview
curl http://localhost:8070/api/v1/stats/overview
curl http://localhost:8070/api/v1/publish-history?limit=10
```

## Architecture

**Two-process design** — runs as an API server and a background router worker. Both share a PostgreSQL database.

```
publisher/
├── main.go              # Multi-command entry: both/api/router
├── cmd_api.go           # REST API server
├── cmd_router.go        # Background router worker
├── internal/
│   ├── api/             # HTTP handlers (Gin)
│   ├── router/          # 8-domain routing logic
│   │   ├── service.go           # Main routing loop, fetchArticles, publishToChannel
│   │   ├── domain_topic.go      # Layer 1: automatic topic channels
│   │   ├── domain_dbchannel.go  # Layer 2: DB-backed custom channels
│   │   ├── crime.go             # Layer 3: crime classification channels
│   │   ├── location.go          # Layer 4: geographic location channels
│   │   ├── mining.go            # Layer 5: mining classification channels
│   │   ├── entertainment.go     # Layer 6: entertainment classification channels
│   │   ├── anishinaabe.go       # Layer 7: Anishinaabe classification channels
│   │   └── domain_coforge.go    # Layer 8: Coforge classification channels
│   ├── database/        # PostgreSQL repositories
│   ├── discovery/       # Elasticsearch index discovery
│   ├── models/          # Source, Channel, Route, PublishHistory
│   ├── redis/           # Redis pub/sub client
│   └── dedup/           # Deduplication tracking
└── docs/
    ├── REDIS_MESSAGE_FORMAT.md
    └── CONSUMER_GUIDE.md
```

## Key Concepts

### Two-Process Design

The service binary accepts a subcommand:
- `publisher both` (default) — starts both API server and router worker in the same process
- `publisher api` — REST API only (manage sources, channels, routes)
- `publisher router` — routing worker only (polls ES, publishes to Redis)

Splitting processes is useful in production to scale the API and router independently.

### Database Schema

| Table | Purpose |
|-------|---------|
| `sources` | Elasticsearch index patterns to monitor (e.g. `example_com_classified_content`) |
| `channels` | Redis pub/sub topic definitions for Layer 2 custom channels |
| `routes` | Many-to-many source → channel mappings with filters |
| `publish_history` | Audit trail; used for per-channel deduplication |

**Route filters**:
- `min_quality_score` (0-100, default 50) — articles below threshold are skipped
- `topics[]` (optional) — articles must match at least one listed topic
- `content_type: "article"` (enforced globally) — pages and listings are never routed

### Deduplication Semantics

Deduplication is **per-channel**. The same article can be published to many different channels, but will never be published to the same channel twice. The `publish_history` table is the authoritative record.

When a publish succeeds, a history record is written atomically. If history write fails, the publish is counted as failed (conservative — avoids duplicate publishes that would be invisible to the dedup check).

### Router Flow

The routing worker runs the following steps every 30 seconds:

1. **Discover indexes** — finds all `*_classified_content` indexes (refreshed every 5 minutes)
2. **Load Layer 2 channels** — reads enabled channels with rules from PostgreSQL
3. **Fetch batch** — queries Elasticsearch using `search_after` cursor (100 articles per batch by default); only `content_type: "article"` documents are fetched
4. **Route Layer 1** — for each article topic, publishes to `articles:{topic}` (except skip-listed topics)
5. **Route Layer 2** — evaluates DB channel rules (topic filters, quality threshold, content type)
6. **Route Layer 3** — crime classification channels (`crime:homepage`, `crime:category:{slug}`, `crime:courts`, `crime:context`)
7. **Route Layer 4** — location channels for crime and entertainment content (`{prefix}:local:{city}`, `{prefix}:province:{code}`, etc.)
8. **Route Layer 5** — mining classification channels (`articles:mining`, `mining:core`, `mining:commodity:{slug}`, etc.)
9. **Route Layer 6** — entertainment classification channels (`entertainment:homepage`, `entertainment:category:{slug}`, etc.)
10. **Route Layer 7** — Anishinaabe classification channels (`articles:anishinaabe`, `anishinaabe:category:{slug}`)
11. **Route Layer 8** — Coforge classification channels (`coforge:core`, `coforge:audience:{slug}`, etc.)
12. **Deduplicate** — each candidate channel is checked against `publish_history`
13. **Publish** — sends JSON payload to Redis
14. **Record** — writes to `publish_history` for each successful publish
15. **Advance cursor** — updates `search_after` cursor in PostgreSQL; safe to restart

## Routing Layers

### Layer 1 — Topic (automatic)

**Source**: `publisher/internal/router/domain_topic.go`

Generates `articles:{topic}` for each topic tag on the article. Topics with dedicated routing layers are excluded via `layer1SkipTopics`:

| Excluded topic | Handled by |
|---------------|-----------|
| `mining` | Layer 5 (MiningDomain) |
| `anishinaabe` | Layer 7 (AnishinaabeeDomain) |
| `coforge` | Layer 8 (CoforgeDomain) |

### Layer 2 — DB Channels (database-backed)

**Source**: `publisher/internal/router/domain_dbchannel.go`

Optional. Channel definitions stored in the `channels` PostgreSQL table. Useful for aggregation channels (e.g. one `articles:crime` channel that consolidates all five crime topic tags). Add or modify channels via the API without restarting the service.

### Layer 3 — Crime Classification (automatic)

**Source**: `publisher/internal/router/crime.go`

Routes articles classified by the crime hybrid classifier. Skips articles with `crime_relevance=not_crime` or no crime object.

- `core_street_crime` + `homepage_eligible=true` → `crime:homepage`
- `core_street_crime` + `category_pages` → `crime:category:{slug}` (one per entry)
- `peripheral_crime` + `crime_sub_label=criminal_justice` → `crime:courts`
- `peripheral_crime` + other sub-label (or none) → `crime:context`

### Layer 4 — Location (automatic)

**Source**: `publisher/internal/router/location.go`

Generates geographic channels for articles with an active crime or entertainment classification and a known location. Mining is excluded — MiningDomain (Layer 5) generates its own location channels.

`{prefix}` is `crime` or `entertainment`:
- `{prefix}:local:{city}` — when specificity is `city` and city is known
- `{prefix}:province:{code}` — when province is known (lowercased)
- `{prefix}:canada` — for Canadian content
- `{prefix}:international` — for non-Canadian content

### Layer 5 — Mining Classification (automatic)

**Source**: `publisher/internal/router/mining.go`

Routes articles classified by the mining hybrid classifier. Skips articles with `mining.relevance=not_mining` or no mining object.

- `articles:mining` — catch-all (core + peripheral)
- `mining:core` — `core_mining` only
- `mining:peripheral` — `peripheral_mining` only
- `mining:commodity:{slug}` — one per commodity (underscores to hyphens)
- `mining:stage:{stage}` — when `mining.mining_stage` is not `unspecified`
- `mining:canada` — `mining.location` is `local_canada` or `national_canada`
- `mining:international` — `mining.location` is `international`

### Layer 6 — Entertainment Classification (automatic)

**Source**: `publisher/internal/router/entertainment.go`

Routes articles classified by the entertainment hybrid classifier. Skips articles with `entertainment.relevance=not_entertainment` or no entertainment object.

- `entertainment:homepage` — `core_entertainment` + `homepage_eligible=true`
- `entertainment:category:{slug}` — one per category (spaces to hyphens, lowercased)
- `entertainment:peripheral` — `peripheral_entertainment`

### Layer 7 — Anishinaabe Classification (automatic)

**Source**: `publisher/internal/router/anishinaabe.go`

Routes articles classified by the Anishinaabe/Indigenous ML classifier. Skips articles with `anishinaabe.relevance=not_anishinaabe` or no anishinaabe object.

- `articles:anishinaabe` — catch-all (core + peripheral)
- `anishinaabe:category:{slug}` — one per category (spaces to hyphens, lowercased)

### Layer 8 — Coforge Classification (automatic)

**Source**: `publisher/internal/router/domain_coforge.go`

Routes articles classified by the Coforge ML classifier. No catch-all `articles:coforge` channel — this is a product-specific domain. Skips articles with `coforge.relevance=not_relevant` or no coforge object.

- `coforge:core` — `core_coforge` relevance
- `coforge:peripheral` — `peripheral` relevance
- `coforge:audience:{slug}` — when `coforge.audience` is set
- `coforge:topic:{slug}` — one per topic (underscores to hyphens)
- `coforge:industry:{slug}` — one per industry (underscores to hyphens)

## API Reference

All `/api/v1/*` routes require JWT authentication. Health endpoints are public.

**Sources**: `GET/POST/PUT/DELETE /api/v1/sources[/:id]`

**Channels**:
- `GET/POST/PUT/DELETE /api/v1/channels[/:id]`
- `GET /api/v1/channels/:id/test-publish`

**Routes**:
- `GET/POST/PUT/DELETE /api/v1/routes[/:id]`
- `GET /api/v1/routes/preview` — preview matching articles without publishing

**History and stats**:
- `GET /api/v1/publish-history` — paginated publish history
- `GET /api/v1/stats/overview` — total published, skipped, errors
- `GET /api/v1/stats/channels` — per-channel statistics
- `GET /api/v1/articles/recent` — recently published articles

## Message Format

All routing layers produce the same message structure. The `publisher` envelope is added by `publishToChannel()` in `service.go`; all other fields come from the Elasticsearch document.

```json
{
  "publisher": {
    "channel_id": "uuid-or-null",
    "published_at": "2026-01-15T14:22:00Z",
    "channel": "articles:violent_crime"
  },
  "id": "es-document-id",
  "title": "...",
  "body": "Full text",
  "quality_score": 82,
  "topics": ["violent_crime", "local_news"],
  "content_type": "article",
  "crime_relevance": "core_street_crime",
  "crime_types": ["violent_crime"],
  "location_specificity": "local_canada",
  "homepage_eligible": true,
  "category_pages": ["violent-crime", "crime"],
  "review_required": false,
  "mining": { ... },
  "anishinaabe": { ... },
  "coforge": { ... },
  "entertainment_relevance": "...",
  "entertainment": { ... },
  "location_city": "Thunder Bay",
  "location_province": "ON",
  "location_country": "Canada",
  "location_confidence": 0.88
}
```

**Crime classification fields**:

| Field | Type | Values |
|-------|------|--------|
| `crime_relevance` | string | `core_street_crime`, `peripheral_crime`, `not_crime` |
| `crime_sub_label` | string | `criminal_justice`, `crime_context` (peripheral only) |
| `crime_types` | []string | `violent_crime`, `property_crime`, `drug_crime`, `gang_violence`, `organized_crime`, `criminal_justice`, `other_crime` |
| `location_specificity` | string | `local_canada`, `national_canada`, `international`, `not_specified` |
| `homepage_eligible` | bool | True if article qualifies for homepage display |
| `category_pages` | []string | Category slugs e.g. `["violent-crime", "crime"]` |
| `review_required` | bool | True if rules and ML disagreed |

**Mining classification fields** (`mining` object):

| Field | Type | Values |
|-------|------|--------|
| `mining.relevance` | string | `core_mining`, `peripheral_mining`, `not_mining` |
| `mining.mining_stage` | string | `exploration`, `development`, `production`, `unspecified` |
| `mining.commodities` | []string | `gold`, `copper`, `lithium`, `nickel`, `uranium`, `iron_ore`, `rare_earths`, `other` |
| `mining.location` | string | `local_canada`, `national_canada`, `international`, `not_specified` |
| `mining.final_confidence` | float | 0.0-1.0 |
| `mining.review_required` | bool | True if rules and ML disagreed |

**Entertainment classification fields**:

| Field | Type | Values |
|-------|------|--------|
| `entertainment_relevance` | string | `core_entertainment`, `peripheral_entertainment`, `not_entertainment` |
| `entertainment_categories` | []string | e.g. `["film", "music", "gaming"]` |
| `entertainment_homepage_eligible` | bool | True if article qualifies for entertainment homepage |
| `entertainment` | object | Nested: relevance, categories, final_confidence, homepage_eligible, review_required, model_version |

**Anishinaabe classification fields** (`anishinaabe` object):

| Field | Type | Values |
|-------|------|--------|
| `anishinaabe.relevance` | string | `core_anishinaabe`, `peripheral_anishinaabe`, `not_anishinaabe` |
| `anishinaabe.categories` | []string | `culture`, `language`, `governance`, `land_rights`, `education` |
| `anishinaabe.final_confidence` | float | 0.0-1.0 |
| `anishinaabe.review_required` | bool | True if rules and ML disagreed |

**Coforge classification fields** (`coforge` object):

| Field | Type | Values |
|-------|------|--------|
| `coforge.relevance` | string | `core_coforge`, `peripheral`, `not_relevant` |
| `coforge.audience` | string | Target audience |
| `coforge.topics` | []string | Topic tags |
| `coforge.industries` | []string | Industry tags |
| `coforge.final_confidence` | float | 0.0-1.0 |
| `coforge.review_required` | bool | True if rules and ML disagreed |

## Configuration

```yaml
router:
  check_interval: 5m      # PUBLISHER_ROUTER_CHECK_INTERVAL
  batch_size: 100         # PUBLISHER_ROUTER_BATCH_SIZE

database:
  # Uses POSTGRES_PUBLISHER_* env vars
```

Full environment variable reference is in the README.

## Common Gotchas

1. **Deduplication is per-channel**: The same article can publish to many different channels, but never the same channel twice. This is intentional — each channel serves a different audience.

2. **Index naming must match exactly**: The `source.index_pattern` field in the sources table must match the Elasticsearch index name character for character.

3. **Quality score range is 0-100**: Defaults to 50 if not set on a route. Articles below the route's `min_quality_score` are silently skipped.

4. **Redis Pub/Sub has no queue**: Consumers that are not subscribed at publish time miss the message permanently. Use Redis Streams if persistence is required.

5. **Router processes routes synchronously**: One article at a time through all domains. Large backlogs process slowly. Tune `PUBLISHER_ROUTER_BATCH_SIZE` and `PUBLISHER_ROUTER_CHECK_INTERVAL` for throughput.

6. **Config file is optional**: The service uses defaults if `config.yml` is missing. Environment variables always take precedence.

7. **Index not found returns empty, not an error**: The router silently returns zero articles for indexes that do not yet exist. This is normal for newly configured sources.

8. **Mining and Anishinaabe fields absent means ML sidecar was not running**: If `mining.relevance` or `anishinaabe.relevance` is absent from all documents, the relevant ML sidecar (`mining-ml`, `anishinaabe-ml`) was likely not running when the classifier processed those documents. Recreate both containers: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build mining-ml classifier` (or `anishinaabe-ml classifier`).

## Testing

```bash
# All tests
go test ./...

# With coverage
task test:coverage

# Single package
go test ./internal/router/...
```

## Code Patterns

**Continue on route errors** — a single failing route must not stop the rest of the batch:

```go
for _, route := range routes {
    if err := s.processRoute(ctx, &route); err != nil {
        s.logger.Error("Error processing route", ...)
        continue  // other routes still run
    }
}
```

**Deduplication check before publish**:

```go
published, checkErr := s.repo.CheckArticlePublished(ctx, articleID, channelName)
if checkErr != nil {
    // log and return false — cannot safely publish without knowing dedup status
    return false
}
if published {
    return false
}
```

**Adding a new routing domain**:

1. Create `internal/router/domain_{name}.go` implementing the `RoutingDomain` interface (`Name() string`, `Routes(*Article) []ChannelRoute`).
2. Append `New{Name}Domain()` to the `domains` slice in `routeArticle()` in `service.go`.
3. Add the topic to `layer1SkipTopics` in `domain_topic.go` if the domain has dedicated ML-based relevance filtering.
4. Add the new classification fields to the `Article` struct and `extractNestedFields()` if the classifier produces new fields.

## Documentation

- `publisher/docs/REDIS_MESSAGE_FORMAT.md` — full message field reference
- `publisher/docs/CONSUMER_GUIDE.md` — integration examples (Laravel, Node.js, Python)
- `ARCHITECTURE.md` — system-wide routing layer reference and Redis channel reference table
