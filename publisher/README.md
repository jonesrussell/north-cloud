# Publisher

Multi-layer routing hub that publishes classified articles from Elasticsearch to Redis Pub/Sub channels.

## Overview

The publisher is a two-process Go service:

- **API server** (`publisher api`) — REST API for managing sources, channels, routes, and viewing publish history.
- **Router worker** (`publisher router`) — Background process that polls all `*_classified_content` Elasticsearch indexes every 30 seconds, runs each article through 8 routing domains in sequence, and publishes matching articles to Redis Pub/Sub channels.

Both processes share a PostgreSQL database for routing configuration and deduplication tracking. They can run together (`publisher both`, the default) or as separate processes.

```
{source}_classified_content (Elasticsearch)
        |
        v
   Publisher Router (polls every 30s)
        |
        v
  8-Domain Routing Pipeline
        |
        v
  Redis Pub/Sub channels (articles:*, crime:*, mining:*, ...)
        |
   +----+----+----+
   v    v    v    v
Consumer A  B  C  N
```

## Features

- 8-layer routing: automatic topic channels, custom DB-backed channels, crime classification, geographic location, mining, entertainment, Anishinaabe, Coforge
- Database-backed routing configuration (PostgreSQL) — add or modify routes without restarting the service
- Per-channel deduplication via the `publish_history` table — an article is never published to the same channel twice
- Quality filtering: each route defines a minimum quality score threshold (0-100)
- Content type filtering: only `content_type: "article"` documents are routed (pages and listings are excluded)
- Preview endpoint: see which articles would match a route before publishing
- Real-time publishing statistics and history
- Persistent cursor using `search_after` — safe to restart mid-stream

## Quick Start

### Docker Compose (recommended)

```bash
# Start all services including publisher
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# Publisher logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher

# Rebuild after code changes
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build publisher
```

### Local development

```bash
# Run both API server and router worker (default)
go run . both

# Run API server only
go run . api

# Run router worker only
go run . router

# Or via Taskfile
task run         # both
task run:api     # API only
task run:router  # router only
```

## Routing Layers

Every article is run through all 8 routing domains in sequence. An article can match zero or more domains and is published to every matched channel. Deduplication prevents re-publishing to any channel the article has already been sent to.

| Layer | Domain | Type | Channel Pattern |
|-------|--------|------|----------------|
| 1 | Topic | Automatic | `articles:{topic}` |
| 2 | DB Channels | DB-backed | Administrator-defined (e.g. `articles:crime`) |
| 3 | Crime Classification | Automatic | `crime:homepage`, `crime:category:{slug}`, `crime:courts`, `crime:context` |
| 4 | Location | Automatic | `{prefix}:local:{city}`, `{prefix}:province:{code}`, `{prefix}:canada`, `{prefix}:international` |
| 5 | Mining Classification | Automatic | `articles:mining`, `mining:core`, `mining:commodity:{slug}`, etc. |
| 6 | Entertainment Classification | Automatic | `entertainment:homepage`, `entertainment:category:{slug}`, `entertainment:peripheral` |
| 7 | Anishinaabe Classification | Automatic | `articles:anishinaabe`, `anishinaabe:category:{slug}` |
| 8 | Coforge Classification | Automatic | `coforge:core`, `coforge:audience:{slug}`, `coforge:topic:{slug}`, etc. |

### Layer 1 — Topic (automatic)

For each topic tag on an article, publishes to `articles:{topic}`. Topics handled by a dedicated ML layer are excluded from Layer 1 to prevent bypassing their relevance filters. Currently excluded: `mining` (Layer 5), `anishinaabe` (Layer 7), `coforge` (Layer 8).

### Layer 2 — DB Channels (database-backed)

Optional channels stored in the publisher's `channels` PostgreSQL table. Each channel can define include/exclude topic filters, a minimum quality score, and content type filters. Useful for consumer-specific aggregations (for example, a single `articles:crime` channel that consolidates all five crime sub-category topics).

### Layer 3 — Crime Classification (automatic)

Routes articles that the crime hybrid classifier flagged. Articles with `crime_relevance=not_crime` or no crime object are skipped. Core street crime articles route to `crime:homepage` (if homepage-eligible) and per-category channels. Peripheral crime articles route to `crime:courts` or `crime:context`.

### Layer 4 — Location (automatic)

Generates geographic channels for articles that have an active crime or entertainment classification and a known location. Mining uses its own location channels in Layer 5 and is excluded here.

### Layer 5 — Mining Classification (automatic)

Routes articles that the mining hybrid classifier flagged. Articles with `mining.relevance=not_mining` or no mining object are skipped. Generates a catch-all channel, core/peripheral channels, per-commodity channels, per-stage channels, and location channels.

### Layer 6 — Entertainment Classification (automatic)

Routes articles that the entertainment hybrid classifier flagged. Articles with `entertainment.relevance=not_entertainment` or no entertainment object are skipped. Generates homepage, per-category, and peripheral channels.

### Layer 7 — Anishinaabe Classification (automatic)

Routes articles that the Anishinaabe/Indigenous ML classifier flagged. Articles with `anishinaabe.relevance=not_anishinaabe` or no anishinaabe object are skipped. Generates a catch-all channel and per-category channels.

### Layer 8 — Coforge Classification (automatic)

Routes articles that the Coforge ML classifier flagged. This is a product-specific domain with no catch-all `articles:coforge` channel. Articles with `coforge.relevance=not_relevant` or no coforge object are skipped.

## Redis Channel Reference

All channel names are consumer-agnostic — they describe content, not the consumer.

### Layer 1 — Automatic Topic Channels

| Channel | Trigger |
|---------|---------|
| `articles:news` | Article tagged `news` |
| `articles:technology` | Article tagged `technology` |
| `articles:politics` | Article tagged `politics` |
| `articles:violent_crime` | Article tagged `violent_crime` |
| `articles:property_crime` | Article tagged `property_crime` |
| `articles:drug_crime` | Article tagged `drug_crime` |
| `articles:organized_crime` | Article tagged `organized_crime` |
| `articles:criminal_justice` | Article tagged `criminal_justice` |
| `articles:{any_topic}` | Article tagged with that topic (except `mining`, `anishinaabe`, `coforge`) |

### Layer 2 — Custom DB Channels

| Channel | Typical use |
|---------|------------|
| `articles:crime` | Aggregation channel for all crime sub-category articles |
| (any name) | Consumer-specific or aggregation channel |

### Layer 3 — Crime Classification Channels

| Channel | Trigger |
|---------|---------|
| `crime:homepage` | `core_street_crime` AND `homepage_eligible=true` |
| `crime:category:{slug}` | `core_street_crime` AND matching `category_pages` entry |
| `crime:courts` | `peripheral_crime` AND `crime_sub_label=criminal_justice` |
| `crime:context` | `peripheral_crime` AND `crime_sub_label=crime_context` (or no sub-label) |

Example category channels: `crime:category:violent-crime`, `crime:category:property-crime`, `crime:category:crime`

### Layer 4 — Location Channels

Generated for crime-classified and entertainment-classified articles with a known location. `{prefix}` is `crime` or `entertainment`.

| Channel | Trigger |
|---------|---------|
| `{prefix}:local:{city}` | Location specificity is `city` and city is known |
| `{prefix}:province:{code}` | Province is known (code lowercased) |
| `{prefix}:canada` | Country is Canada |
| `{prefix}:international` | Country is not Canada |

Examples: `crime:local:thunder-bay`, `crime:province:on`, `crime:canada`, `entertainment:local:toronto`, `entertainment:canada`

### Layer 5 — Mining Channels

| Channel | Trigger |
|---------|---------|
| `articles:mining` | Any mining article (`core_mining` or `peripheral_mining`) |
| `mining:core` | `core_mining` relevance |
| `mining:peripheral` | `peripheral_mining` relevance |
| `mining:commodity:{slug}` | One per commodity in `mining.commodities` (underscores to hyphens) |
| `mining:stage:{stage}` | When `mining.mining_stage` is not `unspecified` |
| `mining:canada` | `mining.location` is `local_canada` or `national_canada` |
| `mining:international` | `mining.location` is `international` |

Examples: `mining:commodity:gold`, `mining:commodity:iron-ore`, `mining:stage:exploration`, `mining:stage:production`

### Layer 6 — Entertainment Channels

| Channel | Trigger |
|---------|---------|
| `entertainment:homepage` | `core_entertainment` AND `entertainment.homepage_eligible=true` |
| `entertainment:category:{slug}` | One per entry in `entertainment.categories` |
| `entertainment:peripheral` | `peripheral_entertainment` relevance |

Examples: `entertainment:category:film`, `entertainment:category:music`, `entertainment:category:gaming`

### Layer 7 — Anishinaabe Channels

| Channel | Trigger |
|---------|---------|
| `articles:anishinaabe` | Any Anishinaabe article (`core_anishinaabe` or `peripheral_anishinaabe`) |
| `anishinaabe:category:{slug}` | One per entry in `anishinaabe.categories` |

Examples: `anishinaabe:category:culture`, `anishinaabe:category:language`, `anishinaabe:category:land-rights`

### Layer 8 — Coforge Channels

| Channel | Trigger |
|---------|---------|
| `coforge:core` | `core_coforge` relevance |
| `coforge:peripheral` | `peripheral` relevance |
| `coforge:audience:{slug}` | When `coforge.audience` is set (underscores/spaces to hyphens) |
| `coforge:topic:{slug}` | One per entry in `coforge.topics` |
| `coforge:industry:{slug}` | One per entry in `coforge.industries` |

Examples: `coforge:audience:developers`, `coforge:topic:cloud`, `coforge:topic:digital-transformation`, `coforge:industry:banking`

## API Reference

All `/api/v1/*` routes require JWT authentication (`Authorization: Bearer <token>`). Health endpoints are public.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/api/v1/sources` | List sources |
| `POST` | `/api/v1/sources` | Create source |
| `PUT` | `/api/v1/sources/:id` | Update source |
| `DELETE` | `/api/v1/sources/:id` | Delete source |
| `GET` | `/api/v1/channels` | List channels |
| `POST` | `/api/v1/channels` | Create channel |
| `PUT` | `/api/v1/channels/:id` | Update channel |
| `DELETE` | `/api/v1/channels/:id` | Delete channel |
| `GET` | `/api/v1/channels/:id/test-publish` | Test publish to channel |
| `GET` | `/api/v1/routes` | List routes (with joined source/channel names) |
| `POST` | `/api/v1/routes` | Create route |
| `PUT` | `/api/v1/routes/:id` | Update route |
| `DELETE` | `/api/v1/routes/:id` | Delete route |
| `GET` | `/api/v1/routes/preview` | Preview articles matching routes (no publishing) |
| `GET` | `/api/v1/publish-history` | Paginated publish history |
| `GET` | `/api/v1/stats/overview` | Publishing statistics |
| `GET` | `/api/v1/stats/channels` | Per-channel statistics |
| `GET` | `/api/v1/articles/recent` | Recently published articles |

## Message Format

Articles are published as JSON to Redis Pub/Sub. The `publisher` envelope is added by the router; all other fields come from the classified content document.

```json
{
  "publisher": {
    "channel_id": "uuid-or-null",
    "published_at": "2026-01-15T14:22:00Z",
    "channel": "articles:violent_crime"
  },
  "id": "es-document-id",
  "title": "Article Title",
  "body": "Full article text...",
  "canonical_url": "https://example.com/article",
  "source": "example_com",
  "published_date": "2026-01-15T12:00:00Z",
  "quality_score": 82,
  "topics": ["violent_crime", "local_news"],
  "content_type": "article",
  "content_subtype": "",
  "source_reputation": 75,
  "confidence": 0.91,
  "og_title": "Article OG Title",
  "og_description": "Description...",
  "og_image": "https://example.com/image.jpg",
  "og_url": "https://example.com/article",
  "word_count": 450,
  "crime_relevance": "core_street_crime",
  "crime_sub_label": "",
  "crime_types": ["violent_crime"],
  "location_specificity": "local_canada",
  "homepage_eligible": true,
  "category_pages": ["violent-crime", "crime"],
  "review_required": false,
  "mining": null,
  "anishinaabe": null,
  "coforge": null,
  "entertainment_relevance": "",
  "entertainment_categories": [],
  "entertainment_homepage_eligible": false,
  "entertainment": null,
  "location_city": "Thunder Bay",
  "location_province": "ON",
  "location_country": "Canada",
  "location_confidence": 0.88
}
```

See [docs/REDIS_MESSAGE_FORMAT.md](./docs/REDIS_MESSAGE_FORMAT.md) for the full field reference and [docs/CONSUMER_GUIDE.md](./docs/CONSUMER_GUIDE.md) for integration examples (Laravel, Node.js, Python).

## Configuration

### Environment Variables

#### Database

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_PUBLISHER_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PUBLISHER_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_PUBLISHER_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_PUBLISHER_PASSWORD` | — | PostgreSQL password (required) |
| `POSTGRES_PUBLISHER_DB` | `publisher` | PostgreSQL database name |

#### Elasticsearch

| Variable | Default | Description |
|----------|---------|-------------|
| `ELASTICSEARCH_URL` | `http://localhost:9200` | Elasticsearch URL |

#### Redis

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | — | Redis password (optional) |

#### API Server

| Variable | Default | Description |
|----------|---------|-------------|
| `PUBLISHER_PORT` | `8070` | HTTP port |
| `AUTH_JWT_SECRET` | — | Shared JWT secret for authentication |
| `GIN_MODE` | `debug` | Gin mode: `debug` or `release` |

#### Router Worker

| Variable | Default | Description |
|----------|---------|-------------|
| `PUBLISHER_ROUTER_CHECK_INTERVAL` | `5m` | Poll interval for checking routes |
| `PUBLISHER_ROUTER_BATCH_SIZE` | `100` | Articles to fetch per batch |

#### General

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_DEBUG` | — | Enable debug mode (`true`, `1`, or `yes`) |

## Architecture

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

## Development

```bash
# Build
task build
# Binary: ./bin/publisher

# Run tests
task test

# Run tests with coverage
task test:coverage

# Lint
task lint

# Run migrations
task migrate:up

# Format code
task fmt
```

## Integration

**Position in pipeline**: The publisher is the final stage. It reads from `{source}_classified_content` Elasticsearch indexes (written by the classifier) and publishes to Redis Pub/Sub channels.

**Consumers**: Any process that subscribes to Redis Pub/Sub channels will receive articles. The publisher does not track or limit who subscribes.

```php
// Laravel example
Redis::subscribe(['articles:crime', 'articles:news'], function ($message) {
    $article = json_decode($message, true);
    // Process article...
});
```

See [docs/CONSUMER_GUIDE.md](./docs/CONSUMER_GUIDE.md) for complete integration examples.

## Troubleshooting

**No articles published**:
1. Verify routes are enabled: `curl http://localhost:8070/api/v1/routes`
2. Check that Elasticsearch indexes exist: `curl http://localhost:9200/_cat/indices?v | grep classified_content`
3. Check router logs: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher`
4. Verify Redis is reachable: `redis-cli PING`

**Messages not received by consumer**:
1. Confirm the consumer subscribes before the publisher publishes (Pub/Sub has no queue; missed messages are lost)
2. Check channel name matches exactly: `curl http://localhost:8070/api/v1/publish-history?limit=10`
3. To test: `redis-cli SUBSCRIBE articles:crime`

**Articles published but missing ML-based channels** (e.g., no `mining:*` channels):
- Check that the relevant ML sidecar is running and its `*_ENABLED` env flag is set
- Verify the classifier container was created after the flag was added — recreate if needed: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build classifier`
