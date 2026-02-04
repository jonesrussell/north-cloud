# Publishing Pipeline Refinement Design

**Date**: 2026-02-04
**Status**: Approved
**Scope**: Classifier location detection, crime type taxonomy, publisher channel architecture, metadata schema

## Executive Summary

This design addresses three core problems in the North Cloud → Streetcode publishing pipeline:

1. **Wrong geography**: International stories tagged as local because the classifier inherits publisher location instead of detecting story location
2. **Wrong crime type**: Peripheral crime (document releases, historical cases) mixed with active street crime
3. **Metadata bugs**: Unreliable `is_crime_related` flag causing non-crime content in crime feeds

**Solution**: A hybrid publishing model where North Cloud publishes to broad channels with rich metadata, and consumers (Streetcode) use that metadata to build properly sectioned feeds (Local → Regional → National → International → Courts → Background).

**Key changes**:
- Content-based location extraction (never inherit from publisher)
- Refined crime taxonomy with `criminal_justice` and `crime_context` sub-labels
- Dynamic Canadian city channels with validated city list
- Standardized metadata schema with controlled vocabularies

---

## Section 1: Location Detection System

**Problem**: International stories tagged as local because the classifier inherits publisher location instead of detecting story location.

**Solution**: Content-based location extraction with weighted scoring.

**Scoring Formula**:
```
score = Σ(zone_weight × specificity_bonus)
```

| Zone | Weight |
|------|--------|
| Headline | 3.0× |
| Lede (first paragraph) | 2.5× |
| Body | 1.0× |

| Specificity | Bonus |
|-------------|-------|
| City | 3 |
| Province/State | 2 |
| Country | 1 |

**Normalization**: All extracted location entities must be normalized to canonical forms before scoring (e.g., "U.S." → "United States", "Greater Sudbury" → "Sudbury").

**Specificity Rule**: If a city is detected, province/state and country mentions for that same entity do not stack; the highest-specificity level is used.

**Dominance Rule**: Winner must exceed second place by ≥30%, otherwise `location: unknown`. Multi-location stories resolve to the dominant location unless the threshold fails.

**Key Constraint**: Publisher location is **never** considered. Only content-derived entities count.

---

## Section 2: Crime Type Classification System

**Problem**: `peripheral_crime` is too broad—it lumps court proceedings with historical document releases, making feed sectioning impossible.

**Solution**: Keep the three top-level categories, add sub-labels to `peripheral_crime`.

**Taxonomy**:

| Category | Sub-label | UI Section | Priority |
|----------|-----------|------------|----------|
| `core_street_crime` | — | Local Crime | 1 (highest) |
| `peripheral_crime` | `criminal_justice` | Courts & Justice | 2 |
| `peripheral_crime` | `crime_context` | Crime Background | 3 |
| `not_crime` | — | (excluded) | — |

**Override Rule**: If a story meets the criteria for `core_street_crime`, it is classified as such regardless of additional contextual or historical elements.

**Classification Rule for `criminal_justice`** (must meet ≥2):
- Has a specific jurisdiction (court, police service)
- Contains named defendants or legal actors
- Uses criminal-justice verbs (charged, sentenced, arraigned, convicted, appeals)
- Describes an active or recent (<12 months) legal process
- Mentions a pending or newly decided outcome

**Classification Rule for `crime_context`**:
- Must positively match context signals (archival, document-driven, institutional)
- Uses context verbs (releases, reveals, declassified, publishes) OR describes archival/historical material
- Does NOT meet minimum criteria for `criminal_justice`

### Signal Reference

**Criminal Justice Verbs**: charged, arrested, arraigned, pleads, pleaded, sentenced, convicted, acquitted, appeals, investigation launched, warrant issued

**Crime Context Verbs**: releases, reveals, declassified, publishes, unsealed, revisits, reviews, commemorates, reports on

---

## Section 3: Publisher Channel Architecture

**Problem**: Need structured delivery (local → regional → national → international) without channel sprawl or mislabeled content.

**Solution**: Hybrid channel model—dynamic for Canadian cities, pre-defined for everything else.

**Channel Structure**:

| Channel | Creation | Filter Criteria |
|---------|----------|-----------------|
| `crime:local:{city}` | Dynamic | `country=canada` AND `specificity=city` AND city in validated list |
| `crime:province:{code}` | Pre-defined | `country=canada` AND `province={code}` |
| `crime:canada` | Pre-defined | `country=canada` |
| `crime:international` | Pre-defined | `country!=canada` |
| `crime:courts` | Pre-defined | `sub_label=criminal_justice` |
| `crime:context` | Pre-defined | `sub_label=crime_context` |

**Dynamic City Channel Rules**:
- Created at publish time when a story with a valid Canadian city is processed
- Only for locations in a validated Canadian cities list (StatsCan/GeoNames)
- Slugs use lowercase, hyphenated canonical forms ("Sault Ste. Marie" → `sault-ste-marie`)
- If city not in validated list → no city channel, falls back to province only
- Never created for international cities, ambiguous datelines, or non-municipal entities

**Province Channel Rule**: Province channels are populated based on province detection alone, regardless of whether a city channel is created.

**Multi-City Rule**: Multi-city stories publish only to the dominant city channel, not multiple city channels.

**Consumer Subscription Model** (Streetcode example):
```
crime:local:sudbury     → "Your Local Area"
crime:province:on       → "Ontario"
crime:canada            → "Canada"
crime:international     → "International"
crime:courts            → "Courts & Justice"
crime:context           → "Crime Background"
```

**Deduplication**: Same article may publish to multiple channels (local + province + national). Consumer handles display deduplication.

---

## Section 4: Metadata Schema

**Problem**: Consumers need rich, consistent metadata to build UI sections, filter content, and personalize feeds.

**Solution**: Standardized metadata payload on every published message.

**Required Fields**:

```json
{
  "schema_version": "1.0",
  "id": "string",
  "title": "string",
  "body": "string",
  "canonical_url": "string",
  "published_at": "ISO8601 datetime",
  "source": {
    "name": "string",
    "url": "string"
  },

  "location": {
    "city": "string | null",
    "province": "string | null",
    "country": "string",
    "specificity": "city | province | country | unknown",
    "confidence": "float 0.0-1.0"
  },

  "crime": {
    "relevance": "core_street_crime | peripheral_crime | not_crime",
    "sub_label": "criminal_justice | crime_context | null",
    "types": ["violent_crime", "property_crime", ...],
    "homepage_eligible": "boolean",
    "confidence": "float 0.0-1.0"
  },

  "quality_score": "int 0-100",
  "content_type": "article | page | listing",

  "publisher": {
    "channel": "string",
    "published_at": "ISO8601 datetime"
  }
}
```

**Controlled Vocabularies**:
- `crime.types` must use controlled values: `violent_crime`, `property_crime`, `drug_offence`, `weapons_offence`, `sexual_offence`, `organized_crime`, `criminal_justice`. No free-form values.
- `publisher.channel` must match a canonical channel slug from Section 3.

**Field Semantics**:
- `homepage_eligible`: True only for `core_street_crime` stories with sufficient quality_score and recency to appear in the primary feed.
- `content_type`: Describes structural format, not editorial category. Articles are narrative news; pages are static informational; listings are structured multi-item posts.

**Validation Rules**:
- `location.country` is always required (never null, but may be "unknown")
- `location.specificity` must match the most specific non-null location field
- `crime.relevance` is always required for crime channels
- `crime.sub_label` is required when `relevance = peripheral_crime`, null otherwise
- `quality_score` must be 0-100 inclusive

**Backward Compatibility**:
- Legacy field `is_crime_related` (boolean) retained, derived from `crime.relevance != not_crime`
- Legacy field `crime_relevance` (string) maps to `crime.relevance`

---

## Section 5: End-to-End Publishing Flow

**Overview**: Content flows through four stages with clear handoffs and validation at each boundary.

```
[Crawler] → [Classifier] → [Publisher] → [Consumer]
```

### Stage 1: Crawler → Raw Content
**Input**: Source URL + selectors
**Output**: `{source}_raw_content` index in Elasticsearch

```
- Extracts: title, body, canonical_url, published_date
- Sets: classification_status = "pending"
- No location or crime detection at this stage
```

### Stage 2: Classifier → Enriched Content
**Input**: Raw content with `classification_status = pending`
**Output**: `{source}_classified_content` index

**Location Detection** (per Section 1):
1. Extract location entities from headline (3×), lede (2.5×), body (1×)
2. Normalize to canonical forms
3. Score by specificity (city=3, province=2, country=1)
4. Apply dominance rule (≥30% margin or `unknown`)
5. Never inherit publisher location

**Crime Classification** (per Section 2):
1. Determine top-level: `core_street_crime`, `peripheral_crime`, or `not_crime`
2. If `peripheral_crime`, apply sub-label rules for `criminal_justice` vs `crime_context`
3. Set `homepage_eligible` based on relevance + quality + recency

**Output document** includes full metadata schema (Section 4).

### Stage 3: Publisher → Redis Channels
**Input**: Classified content
**Output**: Messages to Redis pub/sub channels

**Channel Routing** (per Section 3):
1. Determine location channels:
   - If `country = canada` AND `specificity = city` AND city validated → `crime:local:{city}`
   - If `country = canada` → `crime:province:{code}` AND `crime:canada`
   - If `country != canada` → `crime:international`
   - If `country = unknown` → exclude from all geography-based channels
2. Determine crime-type channels:
   - If `sub_label = criminal_justice` → `crime:courts`
   - If `sub_label = crime_context` → `crime:context`
3. Publish to all matched channels with full metadata payload
4. Record in `publish_history` for deduplication

**Validation at publish**:
- Reject if `location.country` is null (not "unknown", but actually missing)
- Reject if `crime.relevance` is missing
- Reject if `content_type != article`

### Stage 4: Consumer → User Interface
**Input**: Redis pub/sub subscriptions
**Output**: Sectioned feed for end users

**Streetcode example**:
```
Subscriptions:
  crime:local:sudbury
  crime:province:on
  crime:canada
  crime:international
  crime:courts
  crime:context

UI Sections (in priority order):
  1. "Sudbury"        ← crime:local:sudbury
  2. "Ontario"        ← crime:province:on (dedupe local)
  3. "Canada"         ← crime:canada (dedupe province)
  4. "International"  ← crime:international
  5. "Courts"         ← crime:courts
  6. "Background"     ← crime:context
```

**Consumer responsibilities**:
- Deduplicate across channels (same article in local + province + national)
- Persist messages (Redis pub/sub has no queue)
- Handle `schema_version` for forward compatibility

---

## Section 6: Error Handling, Monitoring & Observability

**Problem**: The happy path is defined, but production systems need clear failure modes, recovery paths, and drift detection.

### Classification Failures

| Failure Mode | Behavior | Recovery |
|--------------|----------|----------|
| Location extraction returns empty | Set `location.specificity = unknown`, `location.country = "unknown"` | Exclude from all geography-based channels; eligible only for `crime:courts` or `crime:context` if crime metadata valid |
| Crime classification fails | Set `crime.relevance = not_crime` | Article excluded from crime channels |
| Quality score missing | Default to `quality_score = 0` | Article filtered by quality thresholds |
| Metadata validation fails | Do not publish, log error | Manual review queue |

**Key principle**: Fail safe. When uncertain, exclude rather than misclassify.

### Incomplete Metadata Handling

| Missing Field | Action |
|---------------|--------|
| `location.country` (null) | Reject at publish, log validation error |
| `location.country` = "unknown" | Exclude from geography-based channels |
| `location.city` | Acceptable—route to province/national only |
| `crime.sub_label` when `peripheral_crime` | Default to `crime_context`, flag for review |
| `content_type` | Reject at publish |

### Misclassification Detection

**Signals to monitor**:
- Location mismatch rate: Articles where detected location differs from source location (expected for wire stories, suspicious if >30% for local sources)
- `unknown` location rate: If >20% of articles have `specificity = unknown`, classifier tuning needed
- Crime type distribution drift: Sudden shifts in `core_street_crime` vs `peripheral_crime` ratios
- `homepage_eligible` rate: Should be <50% of `core_street_crime`; higher suggests threshold is too loose

**Automated alerts**:
```
- unknown_location_rate > 20% over 24h → warn
- homepage_eligible_rate > 60% over 24h → warn
- crime_context_rate > 40% of all crime → investigate
- zero articles to crime:local:* for 6h → warn (for active sources)
```

### Channel Routing Anomalies

| Anomaly | Detection | Response |
|---------|-----------|----------|
| Channel explosion | >10 new city channels in 24h | Review city validation list |
| Empty channels | Zero publishes to expected channel for 12h | Check upstream classifier |
| Duplicate publishes | Same article_id to same channel twice | Bug in deduplication logic |
| Orphan articles | Article in `classified_content` but never published | Publisher backlog or filter mismatch |

### Consumer-Side Failures

| Failure | Detection | Recovery |
|---------|-----------|----------|
| Redis disconnect | Consumer health check fails | Auto-reconnect with exponential backoff |
| Message loss | Gap in article IDs or timestamps | Backfill from Elasticsearch (consumer responsibility) |
| Schema version mismatch | `schema_version` != expected | Log warning, attempt graceful parse, alert if critical fields missing |

### Observability Stack

**Metrics to expose** (Prometheus/Grafana):
```
classifier_articles_processed_total{status="success|failed"}
classifier_location_specificity{level="city|province|country|unknown"}
classifier_crime_relevance{type="core|peripheral|not_crime"}
publisher_messages_sent_total{channel="..."}
publisher_validation_failures_total{reason="..."}
consumer_messages_received_total{channel="..."}
consumer_deduplication_hits_total
```

**Logs** (structured JSON to Loki):
- Every classification decision with input signals
- Every publish with channel + article_id
- Every validation rejection with reason

**Dashboards**:
1. **Classifier Health**: Location/crime distribution, unknown rates, processing latency
2. **Publisher Health**: Messages per channel, validation failures, backlog size
3. **Consumer Health**: Subscription status, message lag, deduplication rate

---

## Implementation Priority

1. **Location Detection** (Section 1) - Fixes the root cause of mislabeled stories
2. **Crime Sub-labels** (Section 2) - Enables proper feed sectioning
3. **Metadata Schema** (Section 4) - Foundation for all downstream work
4. **Channel Architecture** (Section 3) - New routing logic
5. **Observability** (Section 6) - Production monitoring

---

## Key Files to Modify

| Component | Files |
|-----------|-------|
| Location Detection | `classifier/internal/classifier/location.go` (new) |
| Crime Sub-labels | `classifier/internal/classifier/crime.go` |
| Metadata Schema | `classifier/internal/domain/classification.go` |
| Channel Routing | `publisher/internal/router/service.go`, `publisher/internal/router/crime.go` |
| City Validation | `classifier/internal/data/canadian_cities.go` (new) |
| ES Mappings | `classifier/internal/elasticsearch/mappings/classified_content.go` |

---

*Design authored collaboratively via brainstorming session, 2026-02-04*
