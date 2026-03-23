# Content Routing Specification

> Last verified: 2026-03-23 (migrate publisher to infrastructure/redis shared package)

Covers the publisher service: 11-layer routing pipeline, channel management, Redis publishing, and deduplication.

## File Map

| File | Purpose |
|------|---------|
| `publisher/main.go` | Entry point: `publisher both` (API + router) |
| `publisher/internal/router/service.go` | Main routing loop (Start, routeContentItem, publishToChannel) |
| `publisher/internal/router/domain.go` | RoutingDomain interface + ChannelRoute struct |
| `publisher/internal/router/domain_topic.go` | Layer 1: topic channels + layer1SkipTopics map |
| `publisher/internal/router/domain_dbchannel.go` | Layer 2: DB-backed custom channels |
| `publisher/internal/router/crime.go` | Layer 3: crime classification routing |
| `publisher/internal/router/location.go` | Layer 4: geographic location channels |
| `publisher/internal/router/mining.go` | Layer 5: mining classification routing |
| `publisher/internal/router/entertainment.go` | Layer 6: entertainment routing |
| `publisher/internal/router/indigenous.go` | Layer 7: Indigenous routing |
| `publisher/internal/router/domain_coforge.go` | Layer 8: Coforge routing |
| `publisher/internal/router/domain_recipe.go` | Layer 9: Recipe routing |
| `publisher/internal/router/domain_job.go` | Layer 10: Job routing |
| `publisher/internal/router/domain_rfp.go` | Layer 11: RFP routing |
| `publisher/internal/router/content_item.go` | ContentItem struct (all classification fields) |
| `publisher/internal/models/channel.go` | Channel, ChannelCreateRequest |
| `publisher/internal/models/rules.go` | Rules struct + Matches() |
| `publisher/internal/database/repository.go` | Channel and cursor persistence |
| `publisher/internal/database/repository_history.go` | Publish history + dedup checks |
| `publisher/internal/redis/client.go` | Redis pub/sub client |
| `publisher/internal/api/router.go` | REST API route registration |
| `publisher/internal/api/channels_handler.go` | Channel CRUD endpoints |
| `publisher/migrations/` | PostgreSQL schema (6 migrations) |
| `publisher/docs/REDIS_MESSAGE_FORMAT.md` | Published message JSON spec |
| `publisher/docs/CONSUMER_GUIDE.md` | Consumer integration guide |

## Interface Signatures

### RoutingDomain (`internal/router/domain.go`)
```go
type RoutingDomain interface {
    Name() string
    Routes(item *ContentItem) []ChannelRoute
}

type ChannelRoute struct {
    Channel   string     // Redis channel name
    ChannelID *uuid.UUID // nil for auto-generated channels
}
```

### Rules (`internal/models/rules.go`)
```go
type Rules struct {
    IncludeTopics   []string
    ExcludeTopics   []string
    MinQualityScore int
    ContentTypes    []string
}

func (r *Rules) Matches(qualityScore int, contentType string, topics []string) bool
func (r *Rules) IsEmpty() bool
```

### Router Service (`internal/router/service.go`)
```go
func (s *Service) Start(ctx context.Context) error  // Main loop
// Internal: fetchContentItems, routeContentItem, publishToChannel
```

## Data Flow

### Routing Pipeline
```
ES *_classified_content → Router (30s poll, batch=100, search_after cursor)

For each ContentItem, evaluate 11 layers sequentially:

Layer 1 (TopicDomain):
  For each topic NOT in layer1SkipTopics:
    → publish to content:{topic}
  Skip topics: mining, indigenous, coforge, recipe, jobs, rfp

Layer 2 (DBChannelDomain):
  For each enabled channel in database:
    If channel.Rules.Matches(item):
      → publish to channel.RedisChannel

Layer 3 (CrimeDomain):
  If crime.relevance != "not_crime" and != "":
    If homepage_eligible → crime:homepage
    For each category_page → crime:category:{slug}
    If sub_label == "criminal_justice" → crime:courts
    If relevance == "peripheral_crime" → crime:context

Layer 4 (LocationDomain):
  If active crime or entertainment result has location:
    If city → {prefix}:local:{city}
    If province → {prefix}:province:{code}
    If country == "Canada" → {prefix}:canada
    Else → {prefix}:international

Layer 5 (MiningDomain):
  If mining.relevance != "not_mining":
    → content:mining
    If relevance == "core_mining" → mining:core
    For each commodity → mining:commodity:{slug}
    If mining_stage → mining:stage:{stage}
    Location routing (mining:canada / mining:international)

Layer 6 (EntertainmentDomain):
  If entertainment.relevance != "not_entertainment":
    If homepage_eligible → entertainment:homepage
    For each category → entertainment:category:{slug}
    If peripheral → entertainment:peripheral

Layer 7 (IndigenousDomain):
  If indigenous.relevance != "not_indigenous":
    → content:indigenous
    For each category → indigenous:category:{slug}
    If region is present → indigenous:region:{slug}

Layer 8 (CoforgeDomain):
  If coforge.relevance != "not_relevant":
    If core → coforge:core, else → coforge:peripheral
    For each audience → coforge:audience:{slug}
    For each topic → coforge:topic:{slug}
    For each industry → coforge:industry:{slug}

Layer 9 (RecipeDomain):
  If content_type == "recipe" and recipe result present:
    → content:recipes

Layer 10 (JobDomain):
  If content_type == "job" and job result present:
    → content:jobs

Layer 11 (RFPDomain):
  If content_type == "rfp" or rfp result present:
    → content:rfps
    Per country → rfp:country:{code}
    Per province → rfp:province:{code}
    Per category → rfp:sector:{slug}
    Per procurement type → rfp:type:{slug}
```

### Publishing Flow
```
For each matched channel:
  1. Check dedup: SELECT EXISTS(... WHERE article_id=$1 AND channel_name=$2)
  2. If already published → skip
  3. Redis PUBLISH channel message_json
  4. INSERT into publish_history (article_id, channel_name, published_at)
  5. Continue on error (one failed channel doesn't stop others)
```

### Layer 1 Skip Topics (CRITICAL)
```go
var layer1SkipTopics = map[string]bool{
    "mining":     true,  // Layer 5 ML filter
    "indigenous": true,  // Layer 7 ML filter
    "coforge":    true,  // Layer 8 ML filter
    "recipe":     true,
    "jobs":       true,
    "rfp":        true,  // Layer 11 RFP filter
}
```
Topics in this map MUST be skipped to prevent bypassing ML classification filters.

## Storage / Schema

### Redis Message Format
```json
{
  "publisher": {
    "channel_id": "uuid-or-null",
    "published_at": "2026-01-15T14:22:00Z",
    "channel": "content:violent_crime"
  },
  "id": "es-document-id",
  "title": "Article Title",
  "body": "Full article text",
  "source": "https://example.com/article",
  "published_date": "2026-01-15T12:00:00Z",
  "quality_score": 82,
  "topics": ["violent_crime"],
  "content_type": "article",
  "crime_relevance": "core_street_crime",
  "homepage_eligible": true,
  "mining": { "relevance": "...", "commodities": [...] },
  "indigenous": { ... },
  "entertainment": { ... }
}
```

### PostgreSQL Tables
- **channels**: id (UUID), name, slug (UNIQUE), redis_channel (UNIQUE), description, rules (JSONB), rules_version, enabled
- **publish_history**: id (UUID), article_id, channel_name, article_title, article_url, published_at, quality_score, topics (TEXT[])
  - Index: `(article_id, channel_name)` — dedup key
- **publisher_cursor**: id=1, last_sort (JSONB), updated_at — search_after pagination state

## Configuration

- `PUBLISHER_PORT` (default: 8070)
- `PUBLISHER_ROUTER_POLL_INTERVAL` (default: 30s) — content polling frequency
- `PUBLISHER_ROUTER_DISCOVERY_INTERVAL` (default: 5m) — ES index discovery frequency
- `PUBLISHER_ROUTER_BATCH_SIZE` (default: 100)
- `ELASTICSEARCH_URL`, `REDIS_ADDR`, `AUTH_JWT_SECRET`

## Edge Cases

- **Dedup is per-channel**: Same content publishes to many channels but never twice to the same channel.
- **No pub/sub persistence**: Consumers missing at publish time lose messages. Use Redis Streams if persistence needed.
- **Router processes synchronously**: One item through all domains before the next. Tune batch size for throughput.
- **Nil nested objects**: Always check `item.Mining == nil` before accessing fields. Return nil from Routes() when domain doesn't apply.
- **Cursor persistence**: search_after cursor saved to DB. Safe across restarts. If cursor invalid (deleted index), resets to beginning.
- **Slug normalization**: Underscores → hyphens in channel slugs.

<\!-- Reviewed: 2026-03-18 — go.mod dependency update only, no spec changes needed -->
