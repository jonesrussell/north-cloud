# Data Model — Community Alert Pipeline

**Mission ID**: `01KQZC7A7SJJZ6EKHZ9JW3AZJG` (mid8: `01KQZC7A`)
**Phase**: Research (Phase 0)
**Companion**: [research.md](./research.md), [spec.md](./spec.md)

---

## Purpose

This document captures the entities, attributes, relationships, and lifecycles that the upcoming plan must respect. Field types are described, not coded; concrete Go types and ES mapping live in the plan and implementation phases.

---

## Entity Map

```
                        ┌────────────────────┐
                        │   AlertSource      │  config-time entity
                        │ (e.g., MHRN)       │
                        └─────────┬──────────┘
                                  │ 1
                                  │
                                  │ N
                        ┌─────────▼──────────┐
                        │  PollCheckpoint    │  state per source
                        │ (etag, last seen)  │  (SQLite)
                        └─────────┬──────────┘
                                  │ 1
                                  │
                                  │ N
                        ┌─────────▼──────────┐    ┌─────────────────────┐
                        │  AlertCatalogEntry │◄───┤   Alert (envelope)  │
                        │ (active set view)  │    │ (community_alert)   │
                        │ (SQLite)           │    │ (ES doc)            │
                        └────────────────────┘    └─────────┬───────────┘
                                                            │
                                                            │ 1..N
                                                            │
                                                  ┌─────────▼───────────┐
                                                  │  Revision           │
                                                  │ (revision_history)  │
                                                  └─────────────────────┘

   (transition emits)
   ┌────────────────────┐
   │  LifecycleEvent    │  Redis pub/sub payload
   │ (created/updated/  │
   │  rescinded)        │
   └────────────────────┘
```

---

## Entities

### 1. Alert (community_alert envelope)

The canonical document. One Elasticsearch document per alert. Stable across re-fetches via `id`.

| Field | Type | Cardinality | Notes |
|---|---|---|---|
| `id` | string | 1 | Stable identifier derived from canonical source URL (e.g., `safersites:20260505fentanyl`). Used as ES `_id`. |
| `category` | enum string | 1 | Discriminator. v1 supports `harm_reduction`. Future: `water`, `evacuation`, `missing_person`, `wildfire_smoke`, etc. |
| `severity` | enum string | 1 | One of `info`, `low`, `medium`, `high`, `critical`. Used by consumers for sorting and visual treatment. |
| `scope` | array of string | 1..N | Ordered list of taxonomy tokens from `jonesrussell/indigenous-taxonomy`. Tokens MAY include treaty (`treaty:1`), region (`canada:manitoba:winnipeg`), or community (`canada:manitoba:sagkeeng-fn`). Hierarchy is implicit in token slugs and resolvable via `ParentRegion`. |
| `issued_at` | timestamp (RFC 3339, UTC) | 1 | Source-provided publication time, normalized from `<pubDate>` (RSS RFC-822 with timezone). |
| `expires_at` | timestamp (RFC 3339, UTC) | 0..1 | Either source-provided (none from RSS today) or derived (default 30d from `issued_at`). Nullable. |
| `lifecycle_state` | enum string | 1 | One of `active`, `rescinded`. `expired` is implicit when `expires_at < now`. |
| `rescinded_at` | timestamp (RFC 3339, UTC) | 0..1 | Set when `lifecycle_state = rescinded`. |
| `title` | text | 1 | Short headline (e.g., "Drug Alert: Winnipeg – Fentanyl – Tue. May 5, 2026"). |
| `summary` | text | 1 | One-paragraph summary suitable for consumer display. |
| `hazard` | nested object | 1 | Category-specific structured data. See §2 for the harm-reduction shape. |
| `guidance` | array of string | 0..N | Recommended community-member actions (e.g., "use with a friend", "have naloxone"). |
| `sources` | array of `SourceAttribution` | 1..N | Provenance. See §3. |
| `revision_history` | array of `Revision` | 0..N | Append-only log of state changes. See §4. |
| `parse_quality` | enum string | 1 | One of `clean`, `degraded`, `failed`. Set by parser; consumers MAY filter. Default `clean`. |
| `crawled_at` | timestamp (RFC 3339, UTC) | 1 | When alert-crawler first persisted this alert. |
| `last_updated_at` | timestamp (RFC 3339, UTC) | 1 | Most recent state-change timestamp. |

**Relationships**: an Alert HAS-A `hazard` object (one of several discriminated shapes), HAS-MANY `sources`, HAS-MANY `revision_history` entries, HAS-MANY `scope` tokens.

### 2. Hazard (discriminated by `Alert.category`)

#### 2.1 HarmReductionHazard

| Field | Type | Cardinality | Notes |
|---|---|---|---|
| `hazard_type` | enum string | 1 | One of `opioid_supply`, `stimulant_supply`, `benzo_supply`, `other`. |
| `substances` | array of string | 1..N | Substance names as published (e.g., `fentanyl`, `carfentanil`, `medetomidine`). |
| `composition` | array of `Substance` | 0..N | Lab-confirmed composition with percentages where available. |
| `visual_description` | text | 0..1 | Free-text description of pill colour, shape, packaging. |
| `lab_source` | string | 0..1 | Issuing lab (e.g., "Health Canada Drug Analysis Service"). |
| `confirmation_date` | date | 0..1 | Date of laboratory confirmation when stated. |

##### 2.1.1 Substance (sub-entity of HarmReductionHazard.composition)

| Field | Type | Notes |
|---|---|---|
| `name` | string | E.g., `medetomidine`. |
| `percentage` | float | 0..100. Optional. |
| `is_active_ingredient` | bool | True if it produces a pharmacological effect; false for cuts/fillers. |
| `note` | text | E.g., "no quantity reported due to fentanyl precursors". |

#### 2.2 (Future categories — out of mission scope)

`WaterHazard`, `EvacuationHazard`, etc. The envelope is generic enough to accommodate them via the discriminated `category` field. Concrete shapes will be defined when each source is onboarded. None are implemented in v1.

### 3. SourceAttribution

| Field | Type | Notes |
|---|---|---|
| `source_id` | string | Internal identifier of the upstream source (e.g., `mhrn`). |
| `source_name` | string | Human-readable name (e.g., "Manitoba Harm Reduction Network"). |
| `url` | string | Canonical URL of the alert at the source. |
| `attribution_text` | text | Optional free-text byline. |
| `media_links` | array of string | Optional URLs of supporting media (PDFs, photos). |

### 4. Revision

Append-only history entry on each Alert document.

| Field | Type | Notes |
|---|---|---|
| `revision_at` | timestamp (RFC 3339, UTC) | When the change occurred. |
| `revision_kind` | enum string | One of `created`, `updated`, `rescinded`, `parse_degraded`, `parse_recovered`. |
| `change_summary` | text | Human-readable. |
| `changed_fields` | array of string | Optional field-path list (e.g., `severity`, `hazard.composition`). |

### 5. AlertSource (config-time entity)

Configuration object describing one upstream source. Loaded at startup from `config.yml` and env.

| Field | Type | Notes |
|---|---|---|
| `id` | string | Stable internal identifier (e.g., `mhrn`). |
| `name` | string | Human-readable name. |
| `feed_url` | URL | Primary acquisition URL. v1 sources use RSS. |
| `acquisition_strategy` | enum string | One of `rss`, `atom`, `json`, `html`. v1 only `rss`. |
| `poll_interval` | duration | 30 to 60 minutes per spec FR-001. |
| `default_category` | enum string | Used to populate `Alert.category` for items from this source. |
| `default_scope` | array of string | Default scope tokens applied if the alert content does not specify scope. E.g., `[treaty:1, canada:manitoba]` for MHRN. |
| `default_expiry` | duration | E.g., `720h` (30 days). Per-source override of the global default. |
| `enabled` | bool | Allows operator to disable a source via env without removing config. |

### 6. PollCheckpoint (per-source state)

Persisted to SQLite. Allows conditional GET and rate-limit-friendly polling.

| Field | Type | Notes |
|---|---|---|
| `source_id` | string | PK with `feed_url`. |
| `feed_url` | string | PK with `source_id`. |
| `last_polled_at` | timestamp | When the last attempt was made. |
| `last_etag` | string | Last `ETag` returned. Sent as `If-None-Match` on next poll. |
| `last_modified` | string | Last `Last-Modified` returned. Sent as `If-Modified-Since` on next poll. |
| `last_status` | int | Last HTTP status. |
| `consecutive_failures` | int | Resets on success. Drives NFR-005 alerting threshold. |

### 7. AlertCatalogEntry (per-source active-set view)

Persisted to SQLite. Required for rescission detection (FR-008, AS-03). Each poll computes the delta between the current feed contents and this catalogue.

| Field | Type | Notes |
|---|---|---|
| `source_id` | string | PK with `alert_id`. |
| `alert_id` | string | PK with `source_id`. Same as `Alert.id`. |
| `last_seen_at` | timestamp | When the alert last appeared in the upstream feed. |
| `is_active` | bool | True while the alert is present in feed. False once detected as rescinded. |
| `content_hash` | string | Fast change-detection. Hash of the salient fields (title + composition + severity). Idempotent re-fetches do not change this. |

### 8. LifecycleEvent (Redis pub/sub payload)

Published on every state transition. Subscribers react in real time; durable consumers fall back to ES.

| Field | Type | Notes |
|---|---|---|
| `event_type` | enum string | One of `created`, `updated`, `rescinded`. |
| `event_at` | timestamp | When the alert-crawler emitted the event. |
| `alert_id` | string | The transitioning alert's stable ID. |
| `category` | string | Convenience copy of `Alert.category` for routing without re-fetch. |
| `severity` | string | Convenience copy. |
| `scope` | array of string | Convenience copy. |
| `payload` | object | Full alert envelope. Consumers MAY use this directly or re-read ES. |

---

## State Machines

### Alert lifecycle (per Alert.id)

```
                ┌──────────┐
   first fetch  │          │
   ───────────► │  active  │
                │          │
                └────┬─────┘
                     │
   updated content   │   issued_at...expires_at elapses
   (content_hash    │   ── no event emitted ──
   changes)         │
                     │
        ┌────────────┴───────────┐
        │                        │
        │   updated              │   absent from upstream feed
        │   ──── (loop) ─────►   │   ──────────────────────────┐
        │                        │                             │
        ▼                        ▼                             ▼
  ┌──────────┐             ┌──────────┐                  ┌──────────┐
  │  active  │ ─ updated ─►│  active  │ ─ rescinded ─►   │ rescinded│
  │ (revised)│             │ (revised)│                  │          │
  └──────────┘             └──────────┘                  └──────────┘
                                                              │
                                                              │ (terminal;
                                                              │  doc retained
                                                              │  for audit)
                                                              ▼
```

### Poll cycle (per AlertSource per cron tick)

```
   start
     │
     ▼
   load PollCheckpoint
     │
     ▼
   GET feed_url with If-None-Match: last_etag
     │
     ├─ 304 Not Modified ─► record last_polled_at, exit
     │
     ├─ 5xx / network error ─► increment consecutive_failures, exit
     │                          (if consecutive_failures >= 6: NFR-005 alarm)
     │
     ▼
   200 OK + payload
     │
     ▼
   parse feed → list of feed items
     │
     ▼
   for each item:
     compute content_hash
     lookup AlertCatalogEntry by alert_id
       ├─ not found       → create new Alert, write ES, append Revision[created], publish LifecycleEvent[created]
       ├─ hash unchanged  → idempotent, no-op (FR-006, NFR-006)
       └─ hash changed    → update Alert, append Revision[updated], publish LifecycleEvent[updated]
     mark AlertCatalogEntry.last_seen_at = now
     │
     ▼
   for each AlertCatalogEntry where is_active and last_seen_at < poll_start_time:
     mark Alert.lifecycle_state = rescinded, append Revision[rescinded], publish LifecycleEvent[rescinded]
     mark AlertCatalogEntry.is_active = false
     │
     ▼
   update PollCheckpoint (last_etag, last_polled_at, reset consecutive_failures)
     │
     ▼
   end
```

---

## Storage Layout Summary

| Storage | Purpose | Lifetime |
|---|---|---|
| Elasticsearch index `community_alerts` | Canonical Alert documents | Indefinite (audit trail; consumers filter by lifecycle/expiry) |
| Redis pub/sub channel(s) | Live LifecycleEvents | Ephemeral; not persisted |
| SQLite at `data/state.db` | PollCheckpoint, AlertCatalogEntry | Persistent across container runs via volume mount |

---

## ES Mapping Sketch (informative, plan finalizes)

```
community_alerts:
  mappings:
    properties:
      id: { type: keyword }
      category: { type: keyword }
      severity: { type: keyword }
      scope: { type: keyword }                          # array
      issued_at: { type: date }
      expires_at: { type: date }
      lifecycle_state: { type: keyword }
      rescinded_at: { type: date }
      title: { type: text, fields: { keyword: { type: keyword } } }
      summary: { type: text }
      hazard:
        type: object
        properties:
          hazard_type: { type: keyword }
          substances: { type: keyword }
          composition:
            type: nested
            properties:
              name: { type: keyword }
              percentage: { type: float }
              is_active_ingredient: { type: boolean }
              note: { type: text }
          visual_description: { type: text }
          lab_source: { type: keyword }
          confirmation_date: { type: date }
      guidance: { type: text }                          # array
      sources:
        type: nested
        properties:
          source_id: { type: keyword }
          source_name: { type: keyword }
          url: { type: keyword }
          attribution_text: { type: text }
          media_links: { type: keyword }
      revision_history:
        type: nested
        properties:
          revision_at: { type: date }
          revision_kind: { type: keyword }
          change_summary: { type: text }
          changed_fields: { type: keyword }
      parse_quality: { type: keyword }
      crawled_at: { type: date }
      last_updated_at: { type: date }
```

Note: `community_alerts` is a fresh index family (NOT `*_classified_content`). Search service will need a separate query path (Plan ticket).

---

## Constraints Carried Forward to Plan

- Stable ID generation: `${source_id}:${slug}` where slug is the URL path component (e.g., `safersites:20260505fentanyl`). Plan must define the canonicalization rule.
- Content-hash inputs for change detection: ordered list of (`title`, `severity`, sorted `substances`, sorted `composition` (name+percentage), `summary`). Plan must finalize.
- Rescission semantics rely on the Catalog being authoritative; the Catalog must be re-built at startup if missing (idempotently, by querying ES for `lifecycle_state == active` documents).
- Poll cycle latency budget (NFR-001): 95% within 60min, 99% within 120min from `issued_at` to consumer visibility.
