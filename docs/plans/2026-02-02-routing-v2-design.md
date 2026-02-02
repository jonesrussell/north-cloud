# Routing V2: Topic-Based, Zero-Config Content Distribution

**Date:** 2026-02-02
**Status:** Approved
**Author:** Claude + Human collaboration

## Problem Statement

The current publisher routing model requires manual creation of routes for every source→channel combination. With 334 sources in production and only 5 prototype routes, this approach cannot scale. Manual route management is not realistic.

**Current State:**
| Resource | Count |
|----------|-------|
| Sources (source-manager) | 334 |
| Sources (publisher) | 5 |
| Channels | 1 (`articles:crime`) |
| Routes | 5 |

**Core Problem:** The prototype-era routing model assumes few sources, one channel, and manual per-source configuration. The classifier now produces rich metadata (topics, quality scores, content types) that should drive routing automatically.

## Solution Overview

Replace the per-source routing model with a two-layer, topic-based routing system:

- **Layer 1 (Automatic):** Convention-based routing where every topic becomes a channel
- **Layer 2 (Custom):** Rule-based channels for consumer-specific aggregation

### Key Properties

- **Zero per-source configuration** — Wildcard discovery finds all classified indexes
- **Zero per-topic configuration** — Convention-based routing handles all topics
- **Optional custom channels** — Layer 2 rules aggregate/filter for specific consumers
- **Stateless publisher** — No sources table, no topic registry, just rules

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        PUBLISHER                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────┐                                           │
│  │  Index Discovery │  GET *_classified_content                 │
│  │  (Wildcard)      │  → discovers all classified indexes       │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐                                           │
│  │  Content Poller  │  Query all indexes for new articles       │
│  │                  │  using search_after for reliable paging   │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐     ┌─────────────────────────────┐       │
│  │  LAYER 1         │     │  For each topic in article: │       │
│  │  Auto-Routing    │ ──► │  publish to articles:{topic}│       │
│  │  (Convention)    │     └─────────────────────────────┘       │
│  └────────┬─────────┘                                           │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐     ┌─────────────────────────────┐       │
│  │  LAYER 2         │     │  For each matching rule:    │       │
│  │  Custom Channels │ ──► │  publish to custom channel  │       │
│  │  (Rule-based)    │     └─────────────────────────────┘       │
│  └──────────────────┘                                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Layer 1: Automatic Topic Routing

Every classifier topic automatically becomes a Redis channel:

```
articles:crime
articles:violent_crime
articles:property_crime
articles:local_news
articles:arts
articles:politics
...
```

**Implementation:**

```go
func (r *Router) publishLayer1(ctx context.Context, article ClassifiedArticle) error {
    for _, topic := range article.Topics {
        channel := fmt.Sprintf("articles:%s", topic)
        if err := r.redis.Publish(ctx, channel, article); err != nil {
            r.logger.Error("Layer 1 publish failed",
                "channel", channel,
                "article_id", article.ID)
        }
    }
    return nil
}
```

**Properties:**
- Zero configuration
- Predictable naming
- Full topic coverage
- New topics automatically get channels

## Layer 2: Custom Channel Routing

Consumer-defined channels with rule-based filtering:

```json
{
  "include_topics": ["violent_crime", "property_crime", "drug_crime"],
  "exclude_topics": ["criminal_justice"],
  "min_quality_score": 60,
  "content_types": ["article"]
}
```

**Rule Semantics:**
| Field | Semantics |
|-------|-----------|
| `include_topics` | Article must have at least one (OR logic). Empty = match all. |
| `exclude_topics` | Article must NOT have any of these. |
| `min_quality_score` | Article quality_score must be >= this value. |
| `content_types` | Article content_type must be in this list. Empty = match all. |

**Implementation:**

```go
func (r *Router) matchesRules(article ClassifiedArticle, rules Rules) bool {
    // Fast path: empty rules match everything
    if rules.IsEmpty() {
        return true
    }

    // Quality check
    if rules.MinQualityScore > 0 && article.QualityScore < rules.MinQualityScore {
        return false
    }

    // Content type check
    if len(rules.ContentTypes) > 0 && !contains(rules.ContentTypes, article.ContentType) {
        return false
    }

    // Exclude topics check
    if hasAny(article.Topics, rules.ExcludeTopics) {
        return false
    }

    // Include topics check (empty = match all)
    if len(rules.IncludeTopics) > 0 && !hasAny(article.Topics, rules.IncludeTopics) {
        return false
    }

    return true
}
```

## Database Schema

**Clean break from prototype schema.** Drop legacy tables, create new minimal schema.

```sql
-- Migration: 004_routing_v2.sql

-- 1. Create cursor table for restart safety
CREATE TABLE IF NOT EXISTS publisher_cursor (
    id          INTEGER PRIMARY KEY DEFAULT 1,
    last_sort   JSONB NOT NULL DEFAULT '[]',
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Drop legacy tables
DROP TABLE IF EXISTS routes CASCADE;
DROP TABLE IF EXISTS sources CASCADE;

-- 3. Recreate channels with new schema
DROP TABLE IF EXISTS channels CASCADE;

CREATE TABLE channels (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,           -- Human-readable: "StreetCode Crime Feed"
    slug            VARCHAR(255) NOT NULL UNIQUE,    -- URL-safe: "streetcode_crime_feed"
    redis_channel   VARCHAR(255) NOT NULL UNIQUE,    -- Redis: "streetcode:crime_feed"
    description     TEXT,

    rules           JSONB NOT NULL DEFAULT '{}',
    rules_version   INTEGER NOT NULL DEFAULT 1,      -- For future rule evolution

    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_channels_enabled ON channels(enabled) WHERE enabled = true;

-- 4. Seed initial channel
INSERT INTO channels (name, slug, redis_channel, description, rules) VALUES (
    'StreetCode Crime Feed',
    'streetcode_crime_feed',
    'streetcode:crime_feed',
    'Aggregated crime content for StreetCode',
    '{
        "include_topics": ["violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice"],
        "exclude_topics": [],
        "min_quality_score": 50,
        "content_types": ["article"]
    }'::jsonb
);
```

## Index Discovery & Polling

**Wildcard Discovery:**

```go
func (r *Router) discoverIndexes(ctx context.Context) ([]string, error) {
    resp, err := r.es.Cat.Indices(
        r.es.Cat.Indices.WithContext(ctx),
        r.es.Cat.Indices.WithIndex("*_classified_content"),
        r.es.Cat.Indices.WithFormat("json"),
    )
    // Returns: ["www_sudbury_com_classified_content", "www_baytoday_ca_classified_content", ...]
}
```

**Polling with search_after:**

Uses `search_after` with `[classified_at, _id]` tiebreaker for reliable incremental consumption:

```go
type PollerConfig struct {
    PollInterval       time.Duration  // 30s
    DiscoveryInterval  time.Duration  // 5m
    BatchSize          int            // 100
}

func (r *Router) pollAndRoute(ctx context.Context, indexes []string) {
    query := map[string]any{
        "query": map[string]any{
            "bool": map[string]any{
                "must": []any{
                    map[string]any{"term": map[string]any{"classification_status": "classified"}},
                },
            },
        },
        "sort": []any{
            map[string]any{"classified_at": "asc"},
            map[string]any{"_id": "asc"},
        },
        "search_after": r.lastSortValues,
        "size": r.config.BatchSize,
    }

    // Loop until fewer than BatchSize results (drain the queue)
    for {
        results, _ := r.es.Search(ctx, indexes, query)

        for _, article := range results {
            r.publishLayer1(ctx, article)
            r.publishLayer2(ctx, article)
        }

        if len(results) < r.config.BatchSize {
            break
        }

        r.lastSortValues = results[len(results)-1].Sort
        r.persistCursor(ctx, r.lastSortValues)
    }
}
```

**Properties:**
- No duplicate documents (search_after guarantees ordering)
- No missed documents (cursor persisted for restart safety)
- No clock skew issues (uses document sort values, not wall clock)
- Handles spikes (loops until queue drained)

## API Changes

**Removed Endpoints:**
```
DELETE /api/v1/sources/*     # Wildcard discovery replaces this
DELETE /api/v1/routes/*      # Replaced by channel rules
```

**Kept Unchanged:**
```
GET /api/v1/publish-history  # Debugging
GET /api/v1/stats/overview   # Monitoring
```

**Evolved Endpoints:**
```
GET    /api/v1/channels              # List custom channels
POST   /api/v1/channels              # Create custom channel with rules
GET    /api/v1/channels/:id          # Get channel details
PUT    /api/v1/channels/:id          # Update channel rules
DELETE /api/v1/channels/:id          # Delete custom channel
```

**New Endpoints:**
```
GET /api/v1/channels/:id/preview     # Preview articles matching rules
GET /api/v1/topics                   # List all topics (ES aggregation)
GET /api/v1/indexes                  # List discovered indexes
```

## Dashboard Changes

Transform `/dashboard/distribution/routes` into `/dashboard/distribution/channels`:

| Old UI | New UI |
|--------|--------|
| Select source dropdown | Gone — no per-source config |
| Select channel dropdown | Channel name/slug/redis_channel inputs |
| Topics text input | Multi-select topic picker (from `/topics`) |
| Quality score slider | Quality score slider (unchanged) |
| Enabled toggle | Enabled toggle (unchanged) |
| — | Content type multi-select |
| — | Exclude topics multi-select |
| — | Live preview panel (auto-refreshes on rule changes) |

**Files to Change:**
- Delete or rename `RoutesView.vue` → `ChannelsView.vue`
- Delete `SourcesView.vue` (not needed in publisher context)
- Update `api/publisher.ts` — remove sources/routes, update channels
- Update `types/publisher.ts` — update Channel type, remove Route/Source

## Migration Plan

### Phase 1: Database Migration
1. Run migration `004_routing_v2.sql`
2. Verify tables created correctly
3. Confirm seed data present

### Phase 2: Publisher Code Changes
| Component | Change |
|-----------|--------|
| `internal/database/` | Remove `source_repository.go`, `route_repository.go`. Update `channel_repository.go`. Add `cursor_repository.go`. |
| `internal/router/` | Implement Layer 1 + Layer 2 routing. Add `search_after` polling. Add index discovery. |
| `internal/api/` | Remove sources/routes handlers. Update channels handlers. Add `/topics`, `/indexes`. |
| `cmd_router.go` | Update to use new router with wildcard discovery. |

### Phase 3: Dashboard Changes
| Component | Change |
|-----------|--------|
| `views/distribution/` | Replace RoutesView with ChannelsView |
| `api/publisher.ts` | Update API client |
| `types/publisher.ts` | Update types |

### Rollout Order
1. Merge database migration
2. Deploy updated publisher
3. Deploy updated dashboard
4. Verify Layer 1 automatic routing works
5. Create custom channels as needed

## Design Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Routing model | Topic-based, not source-based | Scales to 334+ sources without manual config |
| Layer 1 | Convention-based | Zero config, new topics "just work" |
| Layer 2 filters | Topics + quality + content_type | Covers 95% of use cases without complexity |
| Source discovery | Wildcard ES query | Publisher stays independent of source-manager |
| Polling | search_after | No duplicates, no gaps, restart-safe |
| Schema | Clean break | Only 5 prototype routes, nothing worth migrating |

## Future Considerations

These were explicitly deferred to avoid premature complexity:

- **Source reputation filtering** — Can add to rules later when consumers need it
- **Source category filtering** — Can add to rules later when categories stabilize
- **Event-driven discovery** — Wildcard polling is sufficient at current scale
- **Multiple rules per channel** — Single embedded rule is sufficient for now

## Success Criteria

1. Publisher routes articles without any per-source configuration
2. New sources automatically appear in routing within 5 minutes
3. Custom channels can filter by topic, quality, and content type
4. Dashboard allows creating/editing custom channels with live preview
5. No manual route maintenance required for ongoing operations
