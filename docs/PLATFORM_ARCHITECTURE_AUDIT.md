# NorthCloud Platform Architecture Audit

**Date**: 2026-03-07
**Scope**: NorthCloud + all dependent projects (Waaseyaa, Minoo, Laravel aggregators)

---

## 1. Platform Surface Discovery

### Platform Surfaces

| Surface | Type | Protocol | Consumers | Auth | Documented |
|---------|------|----------|-----------|------|------------|
| **Search API** (`GET /api/v1/search`, `POST /api/v1/search`) | HTTP | REST/JSON | Minoo (via `NorthCloudSearchProvider`), Dashboard, MCP server | None (public) | Yes |
| **Redis Pub/Sub channels** (`content:*`, `crime:*`, `mining:*`, `entertainment:*`, `indigenous:*`, `coforge:*`, `rfp:*`, `social:*`) | Message broker | Redis Pub/Sub | All Laravel consumers via `northcloud-laravel` package | None (network-level) | Yes |
| **Source Manager API** (`/api/v1/sources`, `/api/v1/cities`) | HTTP | REST/JSON | Crawler (internal), Dashboard, MCP server | Public reads, JWT writes | Partial |
| **Publisher API** (`/api/v1/channels`, `/api/v1/stats/*`, `/api/v1/publish-history`) | HTTP | REST/JSON | Dashboard, MCP server | JWT | Yes |
| **Crawler API** (`/api/v1/jobs`, `/api/v1/scheduler/*`, SSE events) | HTTP | REST/JSON + SSE | Dashboard, MCP server | JWT | Yes |
| **Classifier API** (`/api/v1/classify`, `/api/v1/rules`, `/api/v1/metrics/ml-health`) | HTTP | REST/JSON | Dashboard, MCP server | JWT | Yes |
| **Index Manager API** (`/api/v1/indexes`) | HTTP | REST/JSON | Dashboard, MCP server | JWT | Partial |
| **Auth API** (`/api/v1/auth/login`) | HTTP | REST/JSON | Dashboard, MCP server | Credential-based | Minimal |
| **Pipeline API** (`/api/v1/events`, `/api/v1/funnel`) | HTTP | REST/JSON | Internal services (unauthenticated writes) | Public writes, JWT reads | Yes |
| **Social Publisher API** (`/api/v1/content`, `/api/v1/publish`, `/api/v1/accounts`) | HTTP | REST/JSON | Dashboard | JWT | Yes |
| **Click Tracker API** | HTTP | REST/JSON | Consumer frontends | Unknown | Minimal |
| **MCP Server** (stdio JSON-RPC 2.0) | stdio | JSON-RPC | Claude Code, Cursor IDE | ENV-scoped | Yes |
| **Elasticsearch indices** (`*_raw_content`, `*_classified_content`, `rfp_classified_content`) | Storage | ES REST | Classifier, Publisher, Search, Index Manager, RFP Ingestor | Network-level | Yes |
| **Grafana** (port 3000) | HTTP | Web UI + API | Operators, MCP server (alerts) | Basic auth | Minimal |
| **Nginx** (port 8443, Caddy at 443) | HTTP | Reverse proxy | All external traffic | N/A | Yes |

### Hidden/Implicit Contracts

1. **Redis message format** — The JSON structure published to Redis channels is the de facto API contract for all consumers. Field names (`id`, `title`, `body`, `canonical_url`, `quality_score`, `topics`, `publisher.channel`, `publisher.published_at`, `crime`, `mining`, `indigenous`, `coforge`, `entertainment`) are consumed directly by `northcloud-laravel` and by Minoo's `IngestServiceProvider`. Any field rename or restructure silently breaks all consumers.

2. **`northcloud-laravel` channel naming** — The package defaults to `articles:default` channel, but the publisher emits `content:*`, `crime:*`, etc. Consumers must configure `NORTHCLOUD_CHANNELS` correctly. The default is wrong for production use.

3. **Search API response shape** — Minoo's `NorthCloudSearchProvider` hardcodes the response schema: `total_hits`, `total_pages`, `current_page`, `page_size`, `took_ms`, `hits[].{id, title, url, source_name, crawled_at, quality_score, content_type, topics, score, og_image, highlight}`, `facets.{topics, content_types, sources}`. Any change to the search API response breaks Minoo.

4. **Search API URL path** — Minoo constructs `{baseUrl}/api/search?...` (note: no `/v1/`). The actual NorthCloud search endpoint is `/api/v1/search`. This may work only because nginx rewrites the path in production, or there is a redirect. This is fragile.

5. **ES index naming convention** — All services derive index names from source names at runtime (`{sanitized_name}_{raw|classified}_content`). The sanitization rules (lowercase, replace non-alphanumeric with `_`) are reimplemented in multiple services. Divergence would cause silent data loss.

6. **`content_type` field type** — Must be `text` (not `keyword`) with explicit `.keyword` sub-field. The RFP ingestor, classifier, and search service all depend on `content_type.keyword` existing. This is documented but there is no automated validation.

7. **Redis Pub/Sub fire-and-forget** — Consumers that are offline when messages are published lose those messages permanently. There is no replay mechanism, no Redis Streams fallback, and no persistent queue. Consumer reliability depends entirely on uptime.

8. **`publisher.route_id` vs `publisher.channel_id`** — The Redis message format documentation shows `route_id` but the publisher CLAUDE.md shows `channel_id`. Consumers may reference either field name.

9. **ML sidecar availability** — Classification fields (`mining`, `indigenous`, `coforge`, `entertainment`, `crime`) are only present when the corresponding ML sidecar was running at classification time. Consumers cannot distinguish "not classified" from "classified as not relevant" when the field is absent.

10. **Waaseyaa search abstraction** — Minoo depends on Waaseyaa's `SearchProviderInterface` types (`SearchRequest`, `SearchResult`, `SearchHit`, `SearchFacet`, `FacetBucket`). The contract between NorthCloud's search API and these types is owned by Minoo's `NorthCloudSearchProvider`, not by NorthCloud itself.

### High Blast Radius Changes

| Change | Impact | Affected |
|--------|--------|----------|
| Rename/remove Redis message fields (`id`, `title`, `body`, `canonical_url`, `quality_score`, `topics`) | Breaks all Laravel consumers | Streetcode, Diidjaaheer, Coforge, OreWire, Movies-of-war |
| Change search API response structure | Breaks Minoo search | Minoo |
| Change Redis channel naming convention | Breaks all consumers (silent — no errors, just no messages) | All |
| Change ES index naming pattern (`*_classified_content`) | Breaks search, publisher index discovery, RFP search integration | Search, Publisher, RFP Ingestor |
| Change `content_type` ES mapping type | Breaks search queries | Search service |
| Remove/rename classification sub-objects (`crime`, `mining`, etc.) | Breaks publisher routing layers | Publisher, all consumers of those channels |
| Change `source_name` sanitization rules | Causes index name mismatch across services | Crawler, Classifier, Publisher, Search |
| Modify auth JWT secret or algorithm | Breaks all authenticated API consumers | Dashboard, MCP server |

---

## 2. Current Architecture Map

### Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         NORTHCLOUD PLATFORM                              │
│                                                                          │
│  ┌────────────┐     ┌───────────┐     ┌────────────┐     ┌───────────┐  │
│  │   Source    │────>│  Crawler  │────>│ Classifier │────>│ Publisher │  │
│  │  Manager   │     │  :8060    │     │   :8071    │     │  :8070    │  │
│  │   :8050    │     └─────┬─────┘     └──────┬─────┘     └─────┬─────┘  │
│  └────────────┘           │                  │                 │        │
│                           ▼                  ▼                 ▼        │
│                    ┌─────────────────────────────────┐   ┌──────────┐   │
│                    │        Elasticsearch            │   │  Redis   │   │
│                    │  {src}_raw_content              │   │ Pub/Sub  │   │
│                    │  {src}_classified_content       │   │  :6379   │   │
│                    │  rfp_classified_content         │   └────┬─────┘   │
│                    └──────────┬──────────────────────┘        │         │
│                               │                              │         │
│  ┌───────────┐   ┌───────────┴───┐   ┌──────────────┐       │         │
│  │  Search   │   │ Index Manager │   │ RFP Ingestor │       │         │
│  │  :8092    │   │    :8090      │   │    :8095     │       │         │
│  └───────────┘   └───────────────┘   └──────────────┘       │         │
│                                                              │         │
│  ┌───────────┐   ┌───────────────┐   ┌──────────────┐       │         │
│  │   Auth    │   │   Pipeline    │   │   Social     │◄──────┘         │
│  │  :8040    │   │    :8075      │   │  Publisher   │                 │
│  └───────────┘   └───────────────┘   │    :8078     │                 │
│                                      └──────────────┘                 │
│  ┌───────────┐   ┌───────────────┐                                    │
│  │ Dashboard │   │ MCP Server   │                                    │
│  │  :3002    │   │  (stdio)     │                                    │
│  └───────────┘   └───────────────┘                                    │
│                                                                       │
│  ┌─── ML Sidecars ──────────────────────────────────────────────┐     │
│  │ crime-ml:8076 │ mining-ml:8077 │ coforge-ml:8078 │           │     │
│  │ entertainment-ml:8079 │ indigenous-ml:8080                   │     │
│  └──────────────────────────────────────────────────────────────┘     │
│                                                                       │
│  ┌─── Observability ────────────────────────────────────────────┐     │
│  │ Grafana:3000 │ Loki:3100 │ Alloy:12345 │ Squid Proxy        │     │
│  └──────────────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────────────────┘
                                   │
              Redis Pub/Sub        │         Search HTTP API
          ┌────────────────────────┼──────────────────────┐
          ▼                        ▼                      ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│   Streetcode     │  │   Diidjaaheer    │  │     Minoo        │
│  (Laravel 12)    │  │  (Laravel 12)    │  │  (Waaseyaa CMS)  │
│  crime channels  │  │  indigenous ch.  │  │  search API      │
└──────────────────┘  └──────────────────┘  └──────────────────┘
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│    Coforge       │  │    OreWire       │  │  Movies-of-War   │
│  (Laravel 12)    │  │  (Laravel 12)    │  │  (Laravel 12)    │
│  coforge ch.     │  │  mining ch.      │  │  entertainment   │
└──────────────────┘  └──────────────────┘  └──────────────────┘
```

### Data Flow

```
1. Source Manager defines sources (URL, CSS selectors, rate limits)
2. Crawler fetches pages on interval schedules → writes to {src}_raw_content (ES)
3. Classifier polls raw_content (classification_status=pending) every 30s
   → Runs 4-step pipeline: content_type → quality → topics → source_reputation
   → Runs optional hybrid classifiers (crime, mining, coforge, entertainment, indigenous)
   → Writes enriched doc to {src}_classified_content (ES)
   → Updates raw doc classification_status=classified
4. Publisher polls classified_content every 30s via search_after cursor
   → Routes through 11 layers (Topic, DB Channels, Crime, Location, Mining,
     Entertainment, Indigenous, Coforge, Recipe, Job, RFP)
   → Publishes JSON to Redis Pub/Sub channels
   → Records publish_history for per-channel deduplication
5. Consumers (Laravel apps, Waaseyaa) subscribe to Redis channels
   → Process messages, store in local databases
6. RFP Ingestor bypasses classifier entirely
   → Polls CanadaBuys CSV → indexes directly to rfp_classified_content
   → Search discovers it via *_classified_content wildcard
```

### Scheduler Logic & Drift Points

- **Crawler**: Interval-based scheduler (not cron). Polls every 10s. CAS-based locking prevents duplicate execution. Adaptive scheduling doubles interval when content unchanged (cap: 24h).
- **Classifier**: Polls ES every 30s for `classification_status=pending`.
- **Publisher**: Polls ES every 30s with `search_after` cursor. Cursor stored in PostgreSQL — safe across restarts.
- **RFP Ingestor**: Polls CanadaBuys CSV every 120 minutes. HTTP 304 short-circuit.
- **Social Publisher**: Scheduler polls content table every 60s; RetryWorker every 30s.
- **Drift point**: If classifier falls behind, publisher cursor may advance past unclassified content — content could be missed. No backpressure mechanism exists.

### Entangled Areas Needing Separation

1. **ES index naming** — Derived at runtime by crawler, classifier, publisher, and search independently. No shared library or contract validation.

2. **Redis message format** — Defined implicitly by `publishToChannel()` in publisher and `extractNestedFields()`. Not versioned, not schema-validated.

3. **Classification field structure** — The `crime`, `mining`, `entertainment`, `indigenous`, `coforge` objects are defined by Go structs in the classifier, then serialized to ES, then read by the publisher. No shared schema definition.

4. **Dashboard proxies all APIs** — The Vue dashboard proxies 6+ backend services through Vite dev server / nginx. Each API has its own auth and versioning. The dashboard is tightly coupled to all service APIs simultaneously.

5. **MCP server is a thin API gateway** — It wraps all service APIs into MCP tools. Any service API change must be reflected in the MCP server's client code.

6. **`northcloud-laravel` package mixes concerns** — Redis subscriber, article processing, admin dashboard, user management, SendGrid mail, MCP routes, Inertia views, and config all live in one package.

---

## 3. Consumer Integration Map

### Streetcode (Laravel 12) — `streetcode-laravel`

| Dependency | Mechanism | Fields Used |
|-----------|-----------|-------------|
| `jonesrussell/northcloud-laravel` | Composer package | Subscriber, article model, admin views |
| Redis Pub/Sub | `articles:subscribe` command | Crime channels: `content:crime`, `crime:homepage`, `crime:category:*`, `crime:province:*`, `crime:local:*` |
| Message fields | Direct JSON access | `id`, `title`, `body`, `canonical_url`, `quality_score`, `topics`, `publisher.channel`, `publisher.published_at`, `is_crime_related`, `content_type`, `og_image` |

**Fragile points**: Depends on `northcloud-laravel` v0.1.2 (pinned to patch). Default channel `articles:default` doesn't match any publisher channel — must configure `NORTHCLOUD_CHANNELS`. No schema validation on incoming messages.

### Diidjaaheer (Laravel 12) — Indigenous content

| Dependency | Mechanism | Fields Used |
|-----------|-----------|-------------|
| `jonesrussell/northcloud-laravel` | Composer package (^0.1.2) | Same subscriber infrastructure |
| Redis Pub/Sub | Configured channels | `content:indigenous`, `indigenous:category:*` |
| Message fields | Direct JSON access | Same core fields + `indigenous.relevance`, `indigenous.categories` |

**Fragile points**: Same as Streetcode. Indigenous ML sidecar must be running for content to appear.

### Coforge (Laravel 12) — Developer/entrepreneur content

| Dependency | Mechanism | Fields Used |
|-----------|-----------|-------------|
| `jonesrussell/northcloud-laravel` | Composer package (^0.1) | Same subscriber infrastructure |
| Redis Pub/Sub | Configured channels | `coforge:core`, `coforge:peripheral`, `coforge:audience:*`, `coforge:topic:*`, `coforge:industry:*` |
| Message fields | Direct JSON access | Same core fields + `coforge.*` |

### OreWire (Laravel 12) — Mining content

| Dependency | Mechanism | Fields Used |
|-----------|-----------|-------------|
| `jonesrussell/northcloud-laravel` | Composer package (^0.1) | Same subscriber infrastructure |
| Redis Pub/Sub | Configured channels | `content:mining`, `mining:core`, `mining:commodity:*`, `mining:stage:*`, `mining:canada`, `mining:international` |
| Message fields | Direct JSON access | Same core fields + `mining.*` |

### Movies-of-War (Laravel 12) — Entertainment/war content

| Dependency | Mechanism | Fields Used |
|-----------|-----------|-------------|
| `jonesrussell/northcloud-laravel` | Composer package (dev-main) | Same subscriber infrastructure |
| `jonesrussell/x-suite-laravel` | Social publishing package | X/Twitter integration |
| Redis Pub/Sub | Configured channels | `entertainment:homepage`, `entertainment:category:film`, etc. |
| Message fields | Direct JSON access | Same core fields + `entertainment.*` |

**Fragile points**: Depends on `dev-main` of northcloud-laravel (no version pinning). Any breaking change to the package immediately affects this consumer.

### Minoo (Waaseyaa CMS) — Indigenous knowledge platform

| Dependency | Mechanism | Fields Used |
|-----------|-----------|-------------|
| **Search HTTP API** | `NorthCloudSearchProvider` → `GET {baseUrl}/api/search?...` | `total_hits`, `total_pages`, `current_page`, `page_size`, `took_ms`, `hits[].{id, title, url, source_name, crawled_at, quality_score, content_type, topics, score, og_image, highlight}`, `facets` |
| **Ingestion system** | `IngestServiceProvider` registers `ingest_log` entity type with `source=northcloud` | Payload JSON stored as `payload_raw`, mapped to entity fields |

**Fragile points**: Minoo calls `/api/search` (no `/v1/`). Hardcoded response field parsing. No error handling beyond returning `SearchResult::empty()`. Cache is in-memory only. Waaseyaa search types (`SearchHit`, `SearchFacet`) are a separate abstraction — NorthCloud doesn't know about them.

### Waaseyaa (CMS Framework) — No direct NorthCloud dependency

Waaseyaa itself does not depend on NorthCloud. It provides the `SearchProviderInterface` that Minoo's `NorthCloudSearchProvider` implements. Changes to Waaseyaa's search abstraction would affect Minoo's NorthCloud integration indirectly.

### Non-NorthCloud Projects

| Project | Stack | NorthCloud Connection |
|---------|-------|----------------------|
| `blog` | Hugo static site | None |
| `me` | Personal site | None |
| `pipelinex` | Unknown | None detected |
| `web-networks-pipeline` | Unknown | None detected |
| `northcloud-oculus` | Unknown | Likely a monitoring/observability dashboard — needs investigation |

---

## 4. Target Platform Architecture

### Proposed Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    NORTHCLOUD PLATFORM (Target)                          │
│                                                                          │
│  ┌─── Core Pipeline ────────────────────────────────────────────────┐   │
│  │                                                                   │   │
│  │  Source Registry ─► Crawler ─► Raw Store ─► Classifier ─► Store  │   │
│  │    (API + DB)       (Workers)   (ES)       (Workers)     (ES)    │   │
│  │                                                                   │   │
│  └───────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌─── Classification Layer ─────────────────────────────────────────┐   │
│  │                                                                   │   │
│  │  Core Classifier (type, quality, topic, reputation)              │   │
│  │  Plugin Classifiers: crime, mining, entertainment, indigenous,   │   │
│  │                      coforge (ML sidecars + rules)               │   │
│  │                                                                   │   │
│  └───────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌─── Distribution Layer ───────────────────────────────────────────┐   │
│  │                                                                   │   │
│  │  Publisher Router (11 routing domains)                            │   │
│  │       │                                                           │   │
│  │       ├─► Redis Pub/Sub (real-time, no persistence)              │   │
│  │       └─► Redis Streams (planned: persistent, replayable)        │   │
│  │                                                                   │   │
│  └───────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌─── Integration Layer (PUBLIC API) ───────────────────────────────┐   │
│  │                                                                   │   │
│  │  /api/v1/search    — Full-text search (public)                   │   │
│  │  /api/v1/feed      — Content feed API (planned, replaces pub/sub │   │
│  │                       for HTTP-only consumers)                    │   │
│  │  /api/v1/webhooks  — Webhook delivery (planned)                  │   │
│  │  Redis Pub/Sub     — Real-time streaming (existing)              │   │
│  │  Redis Streams     — Persistent streaming (planned)              │   │
│  │  MCP Server        — AI tool integration                         │   │
│  │                                                                   │   │
│  │  Contract Schema Registry (JSON Schema definitions)              │   │
│  │                                                                   │   │
│  └───────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌─── Operator Layer ───────────────────────────────────────────────┐   │
│  │                                                                   │   │
│  │  Dashboard (Vue 3)  — Source, channel, job management            │   │
│  │  Grafana            — System health, pipeline metrics            │   │
│  │  MCP (prod)         — AI-assisted operations                     │   │
│  │                                                                   │   │
│  └───────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│  ┌─── Ingestion Extensions ─────────────────────────────────────────┐   │
│  │                                                                   │   │
│  │  RFP Ingestor (CanadaBuys) — Direct ES ingest                   │   │
│  │  Social Publisher — Outbound social media                        │   │
│  │  Click Tracker — Engagement analytics                            │   │
│  │  Pipeline Events — Pipeline observability                        │   │
│  │                                                                   │   │
│  └───────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
                                   │
              Integration Layer    │
          ┌────────────────────────┼──────────────────────┐
          ▼                        ▼                      ▼
  ┌─────────────┐       ┌─────────────┐       ┌─────────────┐
  │ northcloud- │       │  Waaseyaa/  │       │   Custom    │
  │   laravel   │       │   Minoo     │       │  consumers  │
  │  (package)  │       │ (HTTP API)  │       │ (any lang)  │
  └─────────────┘       └─────────────┘       └─────────────┘
```

### Module Responsibilities

| Module | Current | Target | Delta |
|--------|---------|--------|-------|
| **Source Registry** | source-manager + crawler config | Unified source config with validation | Add selector validation, feed URL auto-discovery |
| **Crawler** | Monolith with scheduler, frontier, feed, proxy | Same, well-structured already | Minor: extract index-naming to shared lib |
| **Classifier** | Monolith with 5 hybrid classifiers | Same, plugin architecture already exists | Minor: formalize classifier plugin interface |
| **Publisher** | 11-layer router, Redis pub/sub output | Add Redis Streams output option | Major: add persistent delivery guarantee |
| **Search** | Simple ES query builder | Add versioned API contract | Minor: publish JSON Schema for response |
| **Integration Layer** | Implicit (Redis + HTTP) | Explicit contract layer with schema registry | **New**: JSON Schema definitions, contract tests |
| **northcloud-laravel** | Monolith package | Split into core (subscriber) + admin + mail modules | Major refactor |
| **Dashboard** | Proxies 6+ APIs, 2 SSE streams | Same, add integration health view | Minor |
| **MCP Server** | Thin API gateway | Same | Stable |

### Key Differences from Current State

1. **Schema Registry** — Formal JSON Schema definitions for Redis messages and search API responses. Published as artifacts, tested in CI.
2. **Redis Streams** — Optional persistent delivery alongside existing pub/sub. Consumers can replay missed messages.
3. **Feed API** — HTTP-based content feed for consumers that can't use Redis (Minoo). Replaces Minoo's direct search API dependency.
4. **Shared index-naming library** — Extract source name sanitization into `infrastructure/` package, used by all services.
5. **northcloud-laravel split** — Core subscriber, admin panel, and mail as separate packages.

---

## 5. Contract + Versioning Strategy

### Platform Contracts v1 (Current — Codify as-is)

#### v1 Redis Message Contract

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "NorthCloud Published Content v1",
  "type": "object",
  "required": ["id", "title", "publisher"],
  "properties": {
    "id": { "type": "string", "description": "Elasticsearch document ID" },
    "title": { "type": "string" },
    "body": { "type": "string", "description": "Alias for raw_text" },
    "raw_text": { "type": "string" },
    "canonical_url": { "type": "string", "format": "uri" },
    "source": { "type": "string", "description": "Alias for canonical_url" },
    "published_date": { "type": "string", "format": "date-time" },
    "quality_score": { "type": "integer", "minimum": 0, "maximum": 100 },
    "topics": { "type": "array", "items": { "type": "string" } },
    "content_type": { "type": "string", "enum": ["article", "recipe", "job", "rfp"] },
    "content_subtype": { "type": "string" },
    "is_crime_related": { "type": "boolean" },
    "source_reputation": { "type": "integer", "minimum": 0, "maximum": 100 },
    "confidence": { "type": "number", "minimum": 0, "maximum": 1 },
    "og_title": { "type": "string" },
    "og_description": { "type": "string" },
    "og_image": { "type": "string" },
    "og_url": { "type": "string" },
    "word_count": { "type": "integer" },
    "publisher": {
      "type": "object",
      "required": ["channel", "published_at"],
      "properties": {
        "channel_id": { "type": ["string", "null"] },
        "channel": { "type": "string" },
        "published_at": { "type": "string", "format": "date-time" }
      }
    },
    "crime": { "type": ["object", "null"] },
    "mining": { "type": ["object", "null"] },
    "entertainment": { "type": ["object", "null"] },
    "indigenous": { "type": ["object", "null"] },
    "coforge": { "type": ["object", "null"] }
  }
}
```

#### v1 Search API Contract

Response shape for `GET /api/v1/search`:

```json
{
  "query": "string",
  "total_hits": "integer",
  "total_pages": "integer",
  "current_page": "integer",
  "page_size": "integer",
  "took_ms": "integer",
  "hits": [{
    "id": "string",
    "title": "string",
    "url": "string",
    "source_name": "string",
    "crawled_at": "datetime",
    "quality_score": "integer",
    "content_type": "string",
    "topics": ["string"],
    "score": "float",
    "og_image": "string",
    "highlight": { "body": ["string"], "raw_text": ["string"], "title": ["string"] }
  }],
  "facets": {
    "topics": [{ "key": "string", "count": "integer" }],
    "content_types": [{ "key": "string", "count": "integer" }],
    "sources": [{ "key": "string", "count": "integer" }]
  }
}
```

#### v1 Channel Naming Contract

| Pattern | Layer | Description |
|---------|-------|-------------|
| `content:{topic}` | L1/L2 | Auto-topic or DB-channel |
| `crime:{section}` | L3 | Crime classification |
| `{crime\|entertainment}:{geo}` | L4 | Location |
| `mining:{facet}` | L5 | Mining classification |
| `entertainment:{facet}` | L6 | Entertainment |
| `indigenous:{facet}` | L7 | Indigenous (also `content:indigenous`) |
| `coforge:{facet}` | L8 | Coforge |
| `content:rfps`, `rfp:{facet}` | L11 | RFPs |
| `social:publish` | N/A | Social publisher inbound |
| `social:delivery-status` | N/A | Social publisher outbound |
| `social:dead-letter` | N/A | Social publisher failures |

### v2 Contract Plan

1. **Add `_meta` envelope** to Redis messages with schema version:
   ```json
   { "_meta": { "schema": "northcloud/content/v1", "version": "1.0.0" }, ...existing fields... }
   ```

2. **Add `/api/v2/search`** with stable, documented response. Keep `/api/v1/search` unchanged. `/api/search` (no version) redirects to `/api/v1/search`.

3. **Add `/api/v1/contracts`** endpoint that returns JSON Schema definitions for all message types.

4. **Deprecation policy**: v1 endpoints and message formats supported for 6 months after v2 availability. Log warnings when v1 is used.

### Migration Path per Consumer

| Consumer | Current Integration | Migration Steps |
|----------|-------------------|-----------------|
| **Streetcode** | `northcloud-laravel` ^0.1.2, Redis pub/sub | 1. Update to northcloud-laravel v1.0 with schema validation 2. Add `_meta` envelope handling 3. Switch to Redis Streams when available |
| **Diidjaaheer** | Same | Same |
| **Coforge** | Same | Same |
| **OreWire** | Same | Same |
| **Movies-of-war** | `northcloud-laravel` dev-main | 1. Pin to stable version immediately 2. Same as above |
| **Minoo** | Direct HTTP to `/api/search` | 1. Fix URL to `/api/v1/search` 2. Add error handling for API changes 3. Add response schema validation 4. Consider using Feed API (v2) instead of search |

---

## 6. Dashboard vs Grafana Reconciliation

### Signal Classification

| Signal | Category | Current Location | Recommended |
|--------|----------|-----------------|-------------|
| **Service health** (up/down) | Platform health | Dashboard (per-service health) | Dashboard + Grafana |
| **ES cluster health** | Platform health | None | Grafana |
| **Redis connectivity** | Platform health | None | Grafana |
| **Crawler job status counts** | Platform health | Dashboard | Dashboard |
| **Crawler success rate** | Platform health | Dashboard (scheduler/metrics) | Both |
| **Classification queue depth** | Platform health | None | Grafana |
| **Publisher cursor lag** | Platform health | None | Grafana (alert) |
| **ML sidecar reachability** | Platform health | Dashboard (ml-health) | Both |
| **ML sidecar latency** | Platform health | Dashboard (ml-health) | Grafana |
| **Proxy rotation health** | Platform health | None | Grafana |
| **Source quality distribution** | Tenant/product health | Dashboard (source stats) | Dashboard |
| **Content quality trends** | Tenant/product health | None | Grafana |
| **Per-channel publish volume** | Tenant/product health | Dashboard (stats/channels) | Both |
| **Topic distribution** | Tenant/product health | Dashboard (classifier stats) | Dashboard |
| **Pipeline funnel** | Tenant/product health | Dashboard (pipeline view) | Dashboard |
| **Consumer delivery status** | Tenant/product health | None | Grafana (when Streams exist) |
| **Per-job execution logs** | Developer diagnostics | Dashboard (SSE streaming) | Dashboard |
| **Structured service logs** | Developer diagnostics | Grafana (Loki) | Grafana |
| **Squid proxy logs** | Developer diagnostics | Grafana (Loki) | Grafana |
| **ES query performance** | Developer diagnostics | None | Grafana |
| **Go pprof profiles** | Developer diagnostics | Per-service pprof endpoint | Grafana (Pyroscope) |

### Proposed Reorganization

**Dashboard (operator-facing)**:
- Source management CRUD
- Crawler job management and control
- Channel/route configuration
- Publish history and content preview
- Pipeline funnel visualization
- ML sidecar health summary
- Quick status indicators for all services

**Grafana (system health + diagnostics)**:
- Service health dashboard (UP/DOWN per service, response times)
- Pipeline throughput dashboard (crawl → classify → publish rates over time)
- Content quality dashboard (quality score distributions, spam rates)
- Infrastructure dashboard (ES cluster, Redis, Postgres metrics)
- Proxy dashboard (IP rotation, request distribution, block rates)
- Alert rules: classification queue depth > threshold, publisher cursor lag > 5m, ML sidecar down > 2m, ES disk > 80%

**Shared with consumers** (future):
- Content feed health (publish volume per channel, latency)
- Consumer connection status (when Streams exist)
- Expose as `/api/v1/platform/health` public endpoint

---

## 7. Milestones and Refactor Plan

### Milestone 1: Contract Foundation
**Goal**: Codify existing contracts. No behavior changes.

| Task | Acceptance Criteria |
|------|-------------------|
| Create JSON Schema definitions for Redis message format v1 | Schema files in `docs/contracts/v1/` |
| Create JSON Schema definition for search API response v1 | Schema file validates against actual API responses |
| Extract source name sanitization to `infrastructure/naming/` | All services import from shared package; tests pass |
| Fix Minoo search URL (`/api/search` → `/api/v1/search`) | Minoo tests pass against actual search API |
| Pin movies-of-war to stable northcloud-laravel version | `composer.json` uses `^0.x` not `dev-main` |
| Add contract test CI job | CI validates publisher output against schema |
| Document all Redis channels in a single reference | `docs/contracts/v1/channels.md` |

**Dependencies**: None. Can start immediately.
**Backwards compatibility**: 100% — no behavior changes.

### Milestone 2: Schema Validation in Publisher

**Goal**: Publisher validates output against schema before publishing.

| Task | Acceptance Criteria |
|------|-------------------|
| Add `_meta.schema` and `_meta.version` to published messages | All messages include envelope |
| Add schema validation step in `publishToChannel()` | Validation errors logged, message still published (warn mode) |
| Update `northcloud-laravel` to parse `_meta` envelope | Package handles both with and without `_meta` |
| Add `/api/v1/contracts` endpoint to publisher | Returns JSON Schema definitions |

**Dependencies**: Milestone 1.
**Backwards compatibility**: Additive only. Existing consumers unaffected.

### Milestone 3: Persistent Delivery (Redis Streams)

**Goal**: Add replay capability for consumers.

| Task | Acceptance Criteria |
|------|-------------------|
| Add Redis Streams output alongside Pub/Sub in publisher | Both mechanisms emit simultaneously |
| Add stream consumer support to `northcloud-laravel` | Package can consume from Streams or Pub/Sub |
| Add stream consumer guide to `CONSUMER_GUIDE.md` | Guide covers stream group setup and replay |
| Add stream lag monitoring to Grafana | Dashboard shows per-consumer-group lag |

**Dependencies**: Milestone 2.
**Backwards compatibility**: Pub/Sub continues working. Streams are opt-in.

### Milestone 4: northcloud-laravel Package Split

**Goal**: Reduce blast radius of package changes.

| Task | Acceptance Criteria |
|------|-------------------|
| Extract core subscriber to `northcloud-laravel-core` | Article model, subscriber, processing pipeline |
| Extract admin panel to `northcloud-laravel-admin` | Inertia views, admin controllers, user management |
| Extract mail to `northcloud-laravel-mail` | SendGrid transport |
| Update all consumers to use split packages | All tests pass |

**Dependencies**: Milestone 2 (schema changes should land first).
**Backwards compatibility**: Original package becomes a meta-package requiring all sub-packages.

### Milestone 5: Feed API for HTTP Consumers

**Goal**: Provide HTTP-based content feed for consumers that can't use Redis.

| Task | Acceptance Criteria |
|------|-------------------|
| Add `/api/v1/feed` endpoint to publisher or search | Returns paginated, filterable content feed |
| Migrate Minoo from search API to feed API | Minoo `NorthCloudSearchProvider` updated |
| Add webhook delivery option | Publisher can POST to consumer endpoints |
| Add feed API documentation | OpenAPI spec published |

**Dependencies**: Milestone 1 (contracts).
**Backwards compatibility**: New endpoints only.

### Milestone 6: Grafana Observability

**Goal**: Complete platform health visibility.

| Task | Acceptance Criteria |
|------|-------------------|
| Create pipeline throughput Grafana dashboard | Shows crawl/classify/publish rates |
| Add ES cluster health to Grafana | Disk, memory, query latency panels |
| Add classification queue depth metric | Classifier exports Prometheus metric |
| Add publisher cursor lag metric | Publisher exports Prometheus metric |
| Configure Grafana alerts for critical thresholds | Alerts fire on queue depth > 1000, cursor lag > 5m |

**Dependencies**: None. Can run in parallel with any milestone.
**Backwards compatibility**: N/A (observability only).

---

## 8. Refactor Safety Net

### Contract Tests

```
tests/contracts/
├── redis_message_v1_test.go     # Validates publisher output against JSON Schema
├── search_response_v1_test.go   # Validates search API response against schema
├── channel_naming_test.go       # Validates all routing layers produce valid channel names
├── index_naming_test.go         # Validates source name sanitization consistency
└── classification_fields_test.go # Validates classifier output matches publisher expectations
```

**Implementation**:
- Publisher: After `publishToChannel()`, validate the marshaled JSON against the v1 schema in test mode
- Search: Integration test that queries the real API and validates response shape
- Index naming: Unit tests that run the same source name through crawler, classifier, publisher, and search sanitization — all must produce identical results

### Replayable Ingestion Fixtures

```
fixtures/
├── raw_content/
│   ├── crime_article.json       # Typical crime article
│   ├── mining_article.json      # Mining industry content
│   ├── low_quality_page.json    # Below quality threshold
│   ├── entertainment_review.json
│   └── indigenous_news.json
├── classified_content/
│   ├── crime_classified.json    # Expected output after classification
│   ├── mining_classified.json
│   └── ...
├── redis_messages/
│   ├── content_crime.json       # Expected Redis message for crime channel
│   ├── mining_core.json
│   └── ...
└── search_responses/
    ├── basic_query.json         # Expected search response
    └── faceted_query.json
```

**Usage**: Load fixture into ES, run classifier, verify output matches expected. Publish fixture, verify Redis message matches expected. Query search, verify response matches expected.

### Integration Test Harness

```bash
# End-to-end pipeline test
task test:pipeline:e2e

# Steps:
# 1. Start test ES, Redis, Postgres (docker-compose.test.yml)
# 2. Load raw_content fixture into ES
# 3. Run classifier against fixture
# 4. Verify classified_content matches expected
# 5. Run publisher against classified fixture
# 6. Verify Redis messages match expected schema
# 7. Verify search API returns expected results
# 8. Tear down
```

### Drift Detection Hooks

**Pre-commit hook** (`.claude/hooks/pre-commit`):
```bash
# If any classification struct fields changed, verify schema still matches
changed_files=$(git diff --cached --name-only)
if echo "$changed_files" | grep -q "classifier/internal/domain/"; then
  echo "Classification domain changed — running contract tests..."
  cd publisher && go test ./internal/router/... -run TestContractValidation
fi

if echo "$changed_files" | grep -q "publisher/internal/router/"; then
  echo "Publisher routing changed — running contract tests..."
  cd publisher && go test ./... -run TestRedisMessageSchema
fi

if echo "$changed_files" | grep -q "search/internal/"; then
  echo "Search service changed — running search contract tests..."
  cd search && go test ./... -run TestSearchResponseSchema
fi
```

**CI spec drift detector** (already exists — extend it):
- Add contract schema validation to the existing drift detector
- Flag when Go struct fields change but JSON Schema is not updated
- Flag when new routing layers are added but channel documentation is not updated

### CI Recommendations

1. **Add contract test stage** to GitHub Actions (runs after unit tests, before deploy)
2. **Add integration test stage** with fixture replay (weekly scheduled run)
3. **Add schema validation** to publisher CI — every PR that touches publisher must pass schema tests
4. **Add consumer compatibility check** — test `northcloud-laravel` against publisher output fixtures
5. **Add Minoo search compatibility check** — test Minoo's `NorthCloudSearchProvider` against search fixtures
6. **Pin all consumer dependencies** — no `dev-main` or `*` versions in production composer.json files

---

## Appendix: Repository Inventory

| Repo | Stack | NorthCloud Role | Integration |
|------|-------|----------------|-------------|
| `north-cloud` | Go monorepo | Platform | Core |
| `northcloud-laravel` | PHP/Laravel package | SDK | Redis subscriber, admin UI, mail |
| `streetcode-laravel` | Laravel 12 + Vue | Consumer | Crime content aggregator |
| `diidjaaheer` | Laravel 12 + Vue | Consumer | Indigenous content aggregator |
| `coforge` | Laravel 12 + Vue | Consumer | Developer/entrepreneur content |
| `orewire-laravel` | Laravel 12 + Vue | Consumer | Mining content aggregator |
| `movies-of-war.com` | Laravel 12 + Vue | Consumer | Entertainment/war film content |
| `waaseyaa` | PHP monorepo (CMS) | Framework | Provides search abstraction for Minoo |
| `minoo` | PHP (Waaseyaa CMS) | Consumer | Indigenous knowledge platform, search API |
| `northcloud-oculus` | Unknown | Unknown | Needs investigation |
| `blog` | Hugo | None | Personal blog |
| `me` | Static site | None | Personal site |
| `pipelinex` | Unknown | None | Unrelated |
| `web-networks-pipeline` | Unknown | None | Unrelated |
