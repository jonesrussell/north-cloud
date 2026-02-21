# North Cloud Architecture

Deep system architecture reference for the North Cloud content platform.

For day-to-day development commands, see [CLAUDE.md](./CLAUDE.md).

---

## Table of Contents

1. [Content Pipeline](#content-pipeline)
2. [Services Reference](#services-reference)
3. [ML Sidecars](#ml-sidecars)
4. [Publisher Routing Layers](#publisher-routing-layers)
5. [Elasticsearch Index Model](#elasticsearch-index-model)
6. [Redis Channel Reference](#redis-channel-reference)
7. [Go Service Bootstrap Pattern](#go-service-bootstrap-pattern)
8. [Version History](#version-history)

---

## Content Pipeline

The full content pipeline moves from source configuration through crawling, classification (including optional ML augmentation), and finally routing to Redis Pub/Sub for downstream consumers.

```mermaid
flowchart TD
    SM[Source Manager\nport 8050] -->|source config| CR[Crawler\nport 8060]
    CR -->|raw_content\nclassification_status=pending| ES_RAW[(Elasticsearch\n{source}_raw_content)]
    ES_RAW -->|polls pending| CL[Classifier\nport 8071]

    CL -->|optional HTTP call| CRIME_ML[crime-ml\nport 8076]
    CL -->|optional HTTP call| MINING_ML[mining-ml\nport 8077]
    CL -->|optional HTTP call| COFORGE_ML[coforge-ml\nport 8078]
    CL -->|optional HTTP call| ENT_ML[entertainment-ml\nport 8079]
    CL -->|optional HTTP call| ANI_ML[anishinaabe-ml\nport 8080]

    CRIME_ML -->|crime classification| CL
    MINING_ML -->|mining classification| CL
    COFORGE_ML -->|coforge classification| CL
    ENT_ML -->|entertainment classification| CL
    ANI_ML -->|anishinaabe classification| CL

    CL -->|enriched document| ES_CL[(Elasticsearch\n{source}_classified_content)]
    ES_CL -->|polls every 30s| PUB[Publisher\nport 8070]

    PUB -->|Layer 1-8 routing| REDIS[(Redis Pub/Sub\narticles:* channels)]
    REDIS -->|subscribe| C1[Consumer A\ne.g. Streetcode]
    REDIS -->|subscribe| C2[Consumer B\ne.g. Diidjaaheer]
    REDIS -->|subscribe| C3[Consumer N]
```

**Key properties**:
- The classifier marks each raw document `classification_status=classified` after processing
- All classified content is written to a separate `{source}_classified_content` index
- The publisher uses `search_after` pagination with a persistent cursor so restarts are safe
- Deduplication is per-channel: an article can appear in many channels but never twice in the same channel

---

## Services Reference

| Service | Port | Database | Description |
|---------|------|----------|-------------|
| source-manager | 8050 | postgres-source-manager | Manage content sources and CSS selector configs |
| crawler | 8060 | postgres-crawler | Interval-based web crawler with job scheduler |
| auth | 8040 | none | Username/password to JWT token issuer (24h tokens) |
| classifier | 8071 | postgres-classifier | Classifies raw content: type, quality, topics, ML sub-classifiers |
| publisher | 8070 | postgres-publisher | Polls classified content, routes to Redis via 8 layered domains |
| index-manager | 8090 | postgres-index-manager | Elasticsearch index lifecycle and document management |
| search | 8092 (dev) / 8090 (prod via nginx) | none | Full-text search across all `*_classified_content` indexes |
| dashboard | 3002 | none | Vue.js 3 management UI with JWT auth |
| click-tracker | 8093 | postgres-click-tracker | Tracks article click events for engagement analytics |
| pipeline | 8075 | postgres-pipeline | Orchestrates multi-stage content processing pipelines |
| mcp-north-cloud | stdio | none | MCP server exposing 27 tools for AI integration |
| nc-http-proxy | 8055 | none | HTTP replay proxy for deterministic crawler testing |

**Infrastructure services**:

| Service | Port(s) | Description |
|---------|---------|-------------|
| PostgreSQL | per-service | One instance per Go service |
| Redis | 6379 | Pub/Sub broker for all article routing |
| Elasticsearch | 9200 | Raw and classified content storage |
| Nginx | 80/443 | Reverse proxy and SSL termination (northcloud.biz) |
| Loki | 3100 | Log aggregation backend |
| Grafana Alloy | 12345 (debug UI) | Log collection from Docker containers |
| Grafana | 3000 | Log visualization UI |

---

## ML Sidecars

All ML sidecars are Python-based FastAPI services. They run alongside the classifier and are called over HTTP. The classifier falls back to rules-only mode if an ML sidecar is unreachable. All sidecars live under `ml-sidecars/`.

| Sidecar | Port | Env Flag | Purpose |
|---------|------|----------|---------|
| crime-ml | 8076 | `CRIME_ENABLED=true` | Crime relevance + type multi-label classification |
| mining-ml | 8077 | `MINING_ENABLED=true` | Mining relevance, stage, commodity, and location classification |
| coforge-ml | 8078 | `COFORGE_ENABLED=true` | Coforge-specific content relevance, audience, topic, and industry classification |
| entertainment-ml | 8079 | `ENTERTAINMENT_ENABLED=true` | Entertainment relevance and category classification |
| anishinaabe-ml | 8080 | `ANISHINAABE_ENABLED=true` | Anishinaabe/Indigenous content relevance and category classification |

Each sidecar implements a hybrid rules+ML decision matrix. Rules provide precision; ML provides recall. When rules and ML disagree, the article is flagged `review_required=true` and a conservative result is used.

---

## Publisher Routing Layers

The publisher's `routeArticle()` function runs every article through **8 routing domains** in sequence. Each domain is independent — an article can match zero or more domains, and is published to all matched channels (up to a per-article ceiling of 30). Deduplication is enforced per channel via the `publish_history` table.

The domain execution order is defined in `publisher/internal/router/service.go`:

```go
domains := []RoutingDomain{
    NewTopicDomain(),         // Layer 1
    NewDBChannelDomain(...),  // Layer 2
    NewCrimeDomain(),         // Layer 3
    NewLocationDomain(),      // Layer 4
    NewMiningDomain(),        // Layer 5
    NewEntertainmentDomain(), // Layer 6
    NewAnishinaabeeDomain(),  // Layer 7
    NewCoforgeDomain(),       // Layer 8
}
```

### Layer 1 — Topic (TopicDomain)

**Source**: `publisher/internal/router/domain_topic.go`

Automatic. For each topic tag on the article, publishes to `articles:{topic}`. Topics that have a dedicated routing layer are excluded from Layer 1 to prevent bypassing their ML-based relevance filters:

| Excluded topic | Handled by |
|---------------|-----------|
| `mining` | Layer 5 (MiningDomain) |
| `anishinaabe` | Layer 7 (AnishinaabeeDomain) |
| `coforge` | Layer 8 (CoforgeDomain) |

**Channel examples**:
- `articles:news`
- `articles:technology`
- `articles:politics`
- `articles:violent_crime`
- `articles:property_crime`
- `articles:drug_crime`
- `articles:organized_crime`
- `articles:criminal_justice`

### Layer 2 — DB Channels (DBChannelDomain)

**Source**: `publisher/internal/router/domain_dbchannel.go`

Optional, database-backed. Routes are stored in the `channels` table in the publisher PostgreSQL database. Channels can define include/exclude topic filters, minimum quality scores, and content type filters. This layer is used for consumer-specific aggregations (such as a single channel that consolidates all crime sub-categories).

**Channel examples** (administrator-defined, no fixed pattern):
- `articles:crime` (aggregation channel for all crime sub-categories)
- Any channel name matching `articles:{slug}` convention

### Layer 3 — Crime Classification (CrimeDomain)

**Source**: `publisher/internal/router/crime.go`

Automatic. Routes articles classified by the crime hybrid classifier. Articles with `crime_relevance=not_crime` or empty are skipped.

For `core_street_crime` articles:
- `crime:homepage` — when `homepage_eligible=true`
- `crime:category:{slug}` — one per entry in `category_pages` (e.g. `crime:category:violent-crime`, `crime:category:crime`)

For `peripheral_crime` articles:
- `crime:courts` — when `crime_sub_label=criminal_justice`
- `crime:context` — when `crime_sub_label=crime_context` or no sub-label

**Channel examples**:
- `crime:homepage`
- `crime:category:violent-crime`
- `crime:category:property-crime`
- `crime:category:crime`
- `crime:courts`
- `crime:context`

### Layer 4 — Location (LocationDomain)

**Source**: `publisher/internal/router/location.go`

Automatic. Generates geographic channels for articles with an active domain classifier (crime or entertainment) and a known location. Mining is excluded because MiningDomain (Layer 5) already generates its own location channels.

For Canadian content:
- `{prefix}:local:{city}` — when `location_specificity=city` and city is known
- `{prefix}:province:{code}` — when province is known (lowercased)
- `{prefix}:canada` — always for Canadian content

For non-Canadian content:
- `{prefix}:international`

The `{prefix}` is `crime` for crime-classified articles and `entertainment` for entertainment-classified articles.

**Channel examples**:
- `crime:local:thunder-bay`
- `crime:province:on`
- `crime:canada`
- `crime:international`
- `entertainment:local:toronto`
- `entertainment:province:bc`
- `entertainment:canada`
- `entertainment:international`

### Layer 5 — Mining Classification (MiningDomain)

**Source**: `publisher/internal/router/mining.go`

Automatic. Routes articles classified by the mining hybrid classifier. Articles with `mining.relevance=not_mining` or no mining object are skipped.

**Channel patterns**:
- `articles:mining` — catch-all for all mining articles (core + peripheral)
- `mining:core` — `core_mining` relevance only
- `mining:peripheral` — `peripheral_mining` relevance only
- `mining:commodity:{slug}` — one per commodity (underscores converted to hyphens, e.g. `mining:commodity:iron-ore`)
- `mining:stage:{stage}` — when stage is not `unspecified` (e.g. `mining:stage:exploration`)
- `mining:canada` — when `mining.location` is `local_canada` or `national_canada`
- `mining:international` — when `mining.location` is `international`

**Channel examples**:
- `articles:mining`
- `mining:core`
- `mining:peripheral`
- `mining:commodity:gold`
- `mining:commodity:copper`
- `mining:commodity:iron-ore`
- `mining:commodity:rare-earths`
- `mining:stage:exploration`
- `mining:stage:development`
- `mining:stage:production`
- `mining:canada`
- `mining:international`

### Layer 6 — Entertainment Classification (EntertainmentDomain)

**Source**: `publisher/internal/router/entertainment.go`

Automatic. Routes articles classified by the entertainment hybrid classifier. Articles with `entertainment.relevance=not_entertainment` or no entertainment object are skipped.

**Channel patterns**:
- `entertainment:homepage` — `core_entertainment` + `homepage_eligible=true`
- `entertainment:category:{slug}` — one per category (spaces converted to hyphens, lowercased)
- `entertainment:peripheral` — `peripheral_entertainment` relevance

**Channel examples**:
- `entertainment:homepage`
- `entertainment:category:film`
- `entertainment:category:music`
- `entertainment:category:gaming`
- `entertainment:category:reviews`
- `entertainment:peripheral`

### Layer 7 — Anishinaabe Classification (AnishinaabeeDomain)

**Source**: `publisher/internal/router/anishinaabe.go`

Automatic. Routes articles classified by the Anishinaabe/Indigenous ML classifier. Articles with `anishinaabe.relevance=not_anishinaabe` or no anishinaabe object are skipped.

**Channel patterns**:
- `articles:anishinaabe` — catch-all for all Anishinaabe-classified articles (core + peripheral)
- `anishinaabe:category:{slug}` — one per category (spaces converted to hyphens, lowercased)

**Channel examples**:
- `articles:anishinaabe`
- `anishinaabe:category:culture`
- `anishinaabe:category:language`
- `anishinaabe:category:governance`
- `anishinaabe:category:land-rights`
- `anishinaabe:category:education`

### Layer 8 — Coforge Classification (CoforgeDomain)

**Source**: `publisher/internal/router/domain_coforge.go`

Automatic. Routes articles classified by the Coforge ML classifier. This is a product-specific domain — it does not produce a catch-all `articles:coforge` channel. Articles with `coforge.relevance=not_relevant` or no coforge object are skipped.

**Channel patterns**:
- `coforge:core` — `core_coforge` relevance
- `coforge:peripheral` — `peripheral` relevance
- `coforge:audience:{slug}` — one per audience value (underscores/spaces to hyphens, lowercased)
- `coforge:topic:{slug}` — one per topic (underscores to hyphens, lowercased)
- `coforge:industry:{slug}` — one per industry (underscores to hyphens, lowercased)

**Channel examples**:
- `coforge:core`
- `coforge:peripheral`
- `coforge:audience:developers`
- `coforge:audience:business-analysts`
- `coforge:topic:digital-transformation`
- `coforge:topic:cloud`
- `coforge:industry:banking`
- `coforge:industry:insurance`

---

## Elasticsearch Index Model

### Raw Content Index: `{source}_raw_content`

Written by the Crawler. Read by the Classifier.

| Field | Type | Description |
|-------|------|-------------|
| `title` | string | Page title |
| `raw_text` | string | Extracted body text |
| `raw_html` | string | Original HTML |
| `canonical_url` | string | Canonical URL |
| `source_name` | keyword | Source identifier |
| `crawled_at` | date | When the crawler fetched this document |
| `classification_status` | keyword | `pending` (default) or `classified` |
| `content_hash` | keyword | Hash for deduplication |

The field `classification_status=pending` is the trigger for the classifier's poller. After the classifier processes a document it updates the status to `classified`.

### Classified Content Index: `{source}_classified_content`

Written by the Classifier. Read by the Publisher.

Includes all raw content fields plus:

| Field | Type | Description |
|-------|------|-------------|
| `content_type` | keyword | `article`, `page`, or `listing` |
| `content_subtype` | keyword | Optional sub-type |
| `quality_score` | integer | 0-100 quality score |
| `topics` | keyword[] | Rule-based topic tags (e.g. `violent_crime`, `news`) |
| `source_reputation` | integer | Source historical quality score |
| `confidence` | float | Classification confidence |
| `body` | string | Alias for `raw_text` (required by publisher) |
| `source` | string | Alias for `canonical_url` (required by publisher) |
| `og_title` | string | Open Graph title |
| `og_description` | string | Open Graph description |
| `og_image` | string | Open Graph image URL |
| `og_url` | string | Open Graph URL |
| `word_count` | integer | Body word count |
| `crime` | object | Crime classification (see below) |
| `mining` | object | Mining classification (see below) |
| `entertainment` | object | Entertainment classification (see below) |
| `anishinaabe` | object | Anishinaabe classification (see below) |
| `coforge` | object | Coforge classification (see below) |
| `location` | object | Geographic location data (see below) |

**crime object fields**:
- `street_crime_relevance`: `core_street_crime`, `peripheral_crime`, `not_crime`
- `sub_label`: `criminal_justice`, `crime_context` (peripheral only)
- `crime_types[]`: `violent_crime`, `property_crime`, `drug_crime`, `gang_violence`, `organized_crime`, `criminal_justice`, `other_crime`
- `location_specificity`: `local_canada`, `national_canada`, `international`, `not_specified`
- `homepage_eligible`: bool
- `category_pages[]`: e.g. `["violent-crime", "crime"]`
- `final_confidence`: float 0.0-1.0
- `review_required`: bool

**mining object fields**:
- `relevance`: `core_mining`, `peripheral_mining`, `not_mining`
- `mining_stage`: `exploration`, `development`, `production`, `unspecified`
- `commodities[]`: `gold`, `copper`, `lithium`, `nickel`, `uranium`, `iron_ore`, `rare_earths`, `other`
- `location`: `local_canada`, `national_canada`, `international`, `not_specified`
- `final_confidence`: float 0.0-1.0
- `review_required`: bool
- `model_version`: string

**entertainment object fields**:
- `relevance`: `core_entertainment`, `peripheral_entertainment`, `not_entertainment`
- `categories[]`: e.g. `film`, `music`, `gaming`, `reviews`
- `homepage_eligible`: bool
- `final_confidence`: float 0.0-1.0
- `review_required`: bool
- `model_version`: string

**anishinaabe object fields**:
- `relevance`: `core_anishinaabe`, `peripheral_anishinaabe`, `not_anishinaabe`
- `categories[]`: `culture`, `language`, `governance`, `land_rights`, `education`
- `final_confidence`: float 0.0-1.0
- `review_required`: bool
- `model_version`: string

**coforge object fields**:
- `relevance`: `core_coforge`, `peripheral`, `not_relevant`
- `audience`: string
- `topics[]`: string
- `industries[]`: string
- `relevance_confidence`: float 0.0-1.0
- `audience_confidence`: float 0.0-1.0
- `final_confidence`: float 0.0-1.0
- `review_required`: bool
- `model_version`: string

**location object fields**:
- `city`: string (optional)
- `province`: string (optional)
- `country`: string
- `specificity`: `city`, or broader
- `confidence`: float 0.0-1.0

### Index Naming Convention

Indexes are named after the content source slug:
- `streetcode_raw_content` / `streetcode_classified_content`
- `northernontario_raw_content` / `northernontario_classified_content`

The publisher queries all classified content using the wildcard pattern `*_classified_content`.

### Mapping Drift Note

The Classifier creates indexes with dynamic mappings (text fields). The Index Manager can define explicit mappings (keyword fields). When aggregating on fields like `source_name`, use `source_name.keyword` to target the keyword sub-field of a dynamically-mapped text field. Without this, Elasticsearch returns a 400 because fielddata is disabled on text fields by default.

---

## Redis Channel Reference

All routing is topic-driven and consumer-agnostic. Channels are named by content category, not by consumer.

### Layer 1 — Automatic Topic Channels

Pattern: `articles:{topic}`

Triggered for every topic tag on an article, except topics excluded by `layer1SkipTopics` (mining, anishinaabe, coforge).

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
| `articles:{any_topic}` | Article tagged with that topic |

### Layer 2 — Custom DB Channels

Pattern: administrator-defined (commonly `articles:{slug}`)

Channels stored in the publisher's `channels` PostgreSQL table. Route rules define quality threshold, topic filters, and content type filters.

| Channel | Typical use |
|---------|------------|
| `articles:crime` | Aggregation: all crime sub-category articles |
| (any name) | Consumer-specific or aggregation channel |

### Layer 3 — Crime Classification Channels

| Channel | Trigger |
|---------|---------|
| `crime:homepage` | `core_street_crime` AND `homepage_eligible=true` |
| `crime:category:{slug}` | `core_street_crime` AND article has matching `category_pages` entry |
| `crime:courts` | `peripheral_crime` AND `crime_sub_label=criminal_justice` |
| `crime:context` | `peripheral_crime` AND `crime_sub_label=crime_context` (or no sub-label) |

Example category channels: `crime:category:violent-crime`, `crime:category:property-crime`, `crime:category:crime`

### Layer 4 — Location Channels (crime and entertainment prefixes)

Generated for articles that have an active crime or entertainment classification and a known geographic location. Mining uses its own location channels (Layer 5) and is excluded here.

| Channel | Trigger |
|---------|---------|
| `{prefix}:local:{city}` | Location specificity is `city` and city is known |
| `{prefix}:province:{code}` | Province is known (code lowercased) |
| `{prefix}:canada` | Country is Canada |
| `{prefix}:international` | Country is not Canada |

Where `{prefix}` is `crime` or `entertainment` depending on which classifier is active.

### Layer 5 — Mining Channels

| Channel | Trigger |
|---------|---------|
| `articles:mining` | Any mining-classified article (`core_mining` or `peripheral_mining`) |
| `mining:core` | `core_mining` relevance |
| `mining:peripheral` | `peripheral_mining` relevance |
| `mining:commodity:{slug}` | One per commodity in `mining.commodities` |
| `mining:stage:{stage}` | When `mining.mining_stage` is not `unspecified` |
| `mining:canada` | `mining.location` is `local_canada` or `national_canada` |
| `mining:international` | `mining.location` is `international` |

### Layer 6 — Entertainment Channels

| Channel | Trigger |
|---------|---------|
| `entertainment:homepage` | `core_entertainment` AND `entertainment.homepage_eligible=true` |
| `entertainment:category:{slug}` | One per entry in `entertainment.categories` |
| `entertainment:peripheral` | `peripheral_entertainment` relevance |

### Layer 7 — Anishinaabe Channels

| Channel | Trigger |
|---------|---------|
| `articles:anishinaabe` | Any Anishinaabe-classified article (`core_anishinaabe` or `peripheral_anishinaabe`) |
| `anishinaabe:category:{slug}` | One per entry in `anishinaabe.categories` |

### Layer 8 — Coforge Channels

| Channel | Trigger |
|---------|---------|
| `coforge:core` | `core_coforge` relevance |
| `coforge:peripheral` | `peripheral` relevance |
| `coforge:audience:{slug}` | When `coforge.audience` is set |
| `coforge:topic:{slug}` | One per entry in `coforge.topics` |
| `coforge:industry:{slug}` | One per entry in `coforge.industries` |

---

## Go Service Bootstrap Pattern

All Go HTTP services follow one of two patterns depending on their complexity.

### Simple Pattern (auth, search)

Used for services with few dependencies. All setup is done in `main.go` via named helper functions.

```
main.go
├── main()             → calls run(), exits with its return code
├── run()              → orchestrates startup, returns 0 or 1
├── loadConfig()       → reads config.yml via infraconfig
├── createLogger()     → sets up infralogger with service name
├── setupX()           → one function per major dependency (DB, Redis, etc.)
└── runServer()        → starts HTTP server, blocks until context cancelled
```

### Complex Pattern (crawler, classifier, source-manager, index-manager)

Used for services with many dependencies or phased initialization. Setup is split into an `internal/bootstrap/` package with distinct phase modules.

```
main.go                         → thin entry point, calls bootstrap.Start()
internal/bootstrap/
├── bootstrap.go                → Start() runs all phases in order
├── phase_config.go             → load config.yml
├── phase_database.go           → connect to PostgreSQL
├── phase_storage.go            → connect to Elasticsearch / Redis
├── phase_services.go           → initialize domain services
├── phase_server.go             → register HTTP routes, create Gin engine
└── phase_lifecycle.go          → signal handling, graceful shutdown
```

### Phase Execution Order

Both patterns follow the same conceptual phase ordering:

1. **Profiling** — `profiling.StartPprofServer()` (pprof HTTP server on a debug port)
2. **Config** — `infraconfig.GetConfigPath("config.yml")` → load struct with env tag overrides
3. **Logger** — `infralogger` with `log.With(infralogger.String("service", "name"))`
4. **Database** — PostgreSQL connection with `PingContext()`
5. **Storage/Services** — Elasticsearch, Redis, domain service init
6. **Server** — Gin router, register routes, JWT middleware
7. **Lifecycle** — `signal.NotifyContext`, graceful shutdown with drain timeout

**Entry point conventions**:
- Simple: `func main() { os.Exit(run()) }`
- Complex: `func main() { if err := bootstrap.Start(); err != nil { log.Error(...); os.Exit(1) } }`

**Config path**: `infraconfig.GetConfigPath("config.yml")` resolves to the service's config directory. The source-manager additionally accepts a `-config` CLI flag to override the path.

**Exit codes**: 0 on clean shutdown, 1 on startup failure.

---

## Version History

Key architectural changes (see full history in git):

- **Anishinaabe Layer** (2026-02-16): Added anishinaabe-ml sidecar, wired Anishinaabe classifier into classifier pipeline, added Layer 7 Anishinaabe routing to publisher
- **Mining-ML Pipeline** (2026-02-05): Added mining-ml sidecar, wired hybrid mining classifier into classifier pipeline, added Layer 5 mining routing to publisher
- **Crime Sub-Category Classification** (2026-01-07): Replaced generic "crime" with 5 sub-categories (violent, property, drug, organized, justice)
- **Crawler Scheduler Refactor** (2025-12-29): Interval-based scheduling replaces cron (Migration 003)
- **Publisher Modernization** (2025-12-28): Database-backed Redis Pub/Sub routing hub
- **Dashboard Authentication** (2025-12-27): JWT-based auth with route guards
- **Raw Content Pipeline** (2025-12-23): Three-stage pipeline (raw → classify → publish)
