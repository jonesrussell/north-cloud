# Redis Channel Reference (v1)

All channels used by the North Cloud publisher router. Content flows through
layers 1-11 sequentially; a single item can match multiple layers.

## Layer Summary

| Layer | Domain | Trigger | Skip Condition |
|-------|--------|---------|----------------|
| L1 | Topic (auto) | Any topic tag | Topic in skip list (mining, indigenous, coforge, recipe, jobs, rfp) |
| L2 | DB Channels | Custom rules in PostgreSQL | Channel disabled or rules don't match |
| L3 | Crime | `crime_relevance` present | `crime_relevance` is `not_crime` or absent |
| L4 | Location | Crime or entertainment + known location | No active classifier or unknown location |
| L5 | Mining | `mining.relevance` present | `mining.relevance` is `not_mining` or absent |
| L6 | Entertainment | `entertainment.relevance` present | `not_entertainment` or absent |
| L7 | Indigenous | `indigenous.relevance` present | `not_indigenous` or absent |
| L8 | Coforge | `coforge.relevance` present | `not_relevant` or absent |
| L9 | Recipe | `recipe` object present | Object absent |
| L10 | Job | `job` object present | Object absent |
| L11 | RFP | `rfp` object present | Object absent |

## Channel Patterns by Layer

### Layer 1 — Topic (Auto)

Pattern: `content:{topic}`

Examples: `content:violent_crime`, `content:technology`, `content:politics`

**Skip list**: Topics with dedicated ML layers are excluded from L1 to prevent
bypassing specialized routing: `mining`, `indigenous`, `coforge`, `recipe`,
`jobs`, `rfp`.

### Layer 2 — DB Channels (Custom)

Pattern: Admin-defined (stored in `channels` table)

Each channel has:
- `slug` — unique identifier (e.g. `crime_feed`)
- `redis_channel` — actual Redis channel (e.g. `articles:crime`)
- `rules` — JSONB filter: `include_topics[]`, `exclude_topics[]`,
  `min_quality_score`, `content_types[]`

### Layer 3 — Crime

| Channel | Condition |
|---------|-----------|
| `crime:homepage` | `crime_relevance=core_street_crime` AND `homepage_eligible=true` |
| `crime:category:{slug}` | One per entry in `category_pages[]` |
| `crime:courts` | `crime_relevance=peripheral_crime` AND `crime_sub_label=criminal_justice` |
| `crime:context` | `crime_relevance=peripheral_crime` AND other/no sub-label |

### Layer 4 — Location

Prefix is `crime` or `entertainment` depending on which classifier is active.
Mining content is excluded (Layer 5 handles its own location channels).

| Channel | Condition |
|---------|-----------|
| `{prefix}:local:{city}` | City is known |
| `{prefix}:province:{code}` | Province is known (lowercased, e.g. `on`) |
| `{prefix}:canada` | Country is Canada |
| `{prefix}:international` | Country is not Canada |

### Layer 5 — Mining

| Channel | Condition |
|---------|-----------|
| `content:mining` | All core + peripheral (catch-all) |
| `mining:core` | `mining.relevance=core_mining` |
| `mining:peripheral` | `mining.relevance=peripheral_mining` |
| `mining:commodity:{slug}` | One per commodity (underscores to hyphens) |
| `mining:stage:{stage}` | When `mining_stage != unspecified` |
| `mining:canada` | `mining.location` is `local_canada` or `national_canada` |
| `mining:international` | `mining.location` is `international` |

### Layer 6 — Entertainment

| Channel | Condition |
|---------|-----------|
| `entertainment:homepage` | `core_entertainment` AND `homepage_eligible=true` |
| `entertainment:category:{slug}` | One per category (spaces to hyphens) |
| `entertainment:peripheral` | `peripheral_entertainment` |

### Layer 7 — Indigenous

| Channel | Condition |
|---------|-----------|
| `content:indigenous` | All core + peripheral (catch-all) |
| `indigenous:category:{slug}` | One per category (spaces to hyphens) |

### Layer 8 — Coforge

No catch-all `content:coforge` channel.

| Channel | Condition |
|---------|-----------|
| `coforge:core` | `coforge.relevance=core_coforge` |
| `coforge:peripheral` | `coforge.relevance=peripheral` |
| `coforge:audience:{slug}` | When `coforge.audience` is set |
| `coforge:topic:{slug}` | One per topic |
| `coforge:industry:{slug}` | One per industry |

### Layer 9 — Recipe

| Channel | Condition |
|---------|-----------|
| `content:recipes` | Catch-all |
| `recipes:category:{slug}` | When `recipe.category` is set |
| `recipes:cuisine:{slug}` | When `recipe.cuisine` is set |

### Layer 10 — Job

| Channel | Condition |
|---------|-----------|
| `content:jobs` | Catch-all |
| `jobs:type:{slug}` | When `job.employment_type` is set |
| `jobs:industry:{slug}` | When `job.industry` is set |

### Layer 11 — RFP

| Channel | Condition |
|---------|-----------|
| `content:rfps` | Catch-all |
| `rfp:country:{code}` | Per country code (lowercased) |
| `rfp:province:{code}` | Per province code (lowercased) |
| `rfp:sector:{slug}` | One per category |
| `rfp:type:{slug}` | Per `procurement_type` |

## Social Publisher (Separate Service)

The social-publisher is a standalone consumer, not a routing layer.

| Channel | Direction | Purpose |
|---------|-----------|---------|
| `social:publish` | Inbound | Receives publish requests with target platforms |
| `social:delivery-status` | Outbound | Delivery lifecycle events (created, delivered, failed) |
| `social:dead-letter` | Outbound | Permanently failed deliveries after max retries |

## Deduplication

Each `(article_id, channel_name)` pair is tracked in the `publish_history`
table. Content is never published twice to the same channel. The dedup check
runs across all layers — if L1 publishes to `content:crime` and L3 also tries,
the second publish is skipped.

## Consumer Subscriptions

| Consumer | Subscribes To | Package |
|----------|--------------|---------|
| Streetcode | `articles:crime` (L2 DB channel) | `northcloud-laravel ^0.1.2` |
| Diidjaaheer | Configured channels | `northcloud-laravel ^0.1.2` |
| Movies-of-War | Configured channels | `northcloud-laravel ^0.7` |
| Orewire | Configured channels | `northcloud-laravel` |
| Minoo | Search API (not Redis) | `waaseyaa` |
