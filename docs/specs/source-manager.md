# Source Manager Specification

> Last verified: 2026-03-19 (post-migration-019 fix)

## Purpose

Registry service for content sources, First Nations communities, and leadership data. Upstream authority for the content pipeline — crawler, publisher, and Minoo consume its data.

## File Map

| Path | Purpose |
|------|---------|
| `source-manager/main.go` | Entry point |
| `source-manager/internal/bootstrap/` | Phased startup (config → db → redis → server) |
| `source-manager/internal/api/router.go` | Route registration (public + JWT-protected) |
| `source-manager/internal/handlers/` | HTTP handlers (source, community, person, band_office) |
| `source-manager/internal/models/` | Domain types (Source, Community, Person, BandOffice) |
| `source-manager/internal/repository/` | PostgreSQL CRUD |
| `source-manager/internal/events/publisher.go` | Redis event publishing |
| `source-manager/internal/config/config.go` | Configuration struct |
| `source-manager/internal/importer/` | OPD JSONL bulk-import (validation, canonical hashing) |
| `source-manager/internal/projection/` | Dictionary ES projection (consent-filtered) |
| `source-manager/cmd_import_opd.go` | CLI subcommand: `import-opd` |
| `source-manager/migrations/` | SQL migrations (001–019) |

## Interface Signatures

### Public Endpoints (no JWT — service-to-service)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/sources` | List all sources (crawler consumption) |
| GET | `/api/v1/sources/indigenous` | Sources with indigenous_region tag |
| GET | `/api/v1/cities` | Aggregated cities from enabled sources |
| GET | `/api/v1/communities` | List communities with filters |
| GET | `/api/v1/communities/regions` | Distinct regions |
| GET | `/api/v1/communities/with-source` | Communities linked to sources |
| GET | `/api/v1/communities/nearby` | Geographic proximity search |
| GET | `/api/v1/communities/by-slug/:slug` | Lookup by slug |
| GET | `/api/v1/communities/:id` | Get community by ID |
| GET | `/api/v1/communities/:id/people` | List community leadership |
| GET | `/api/v1/communities/:id/band-office` | Band office details |
| GET | `/api/v1/people/:id` | Get person by ID |
| GET | `/health` | Health check |

### Protected Endpoints (JWT required)

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/sources` | Create source (publishes SourceCreated) |
| PUT | `/api/v1/sources/:id` | Update source (publishes SourceUpdated) |
| DELETE | `/api/v1/sources/:id` | Delete source (publishes SourceDeleted) |
| PATCH | `/api/v1/sources/:id/disable` | Disable source with reason |
| PATCH | `/api/v1/sources/:id/enable` | Re-enable source |
| PATCH | `/api/v1/sources/:id/feed-disable` | Disable feed polling |
| PATCH | `/api/v1/sources/:id/feed-enable` | Re-enable feed polling |
| POST | `/api/v1/sources/fetch-metadata` | Auto-extract title + selector hints |
| POST | `/api/v1/sources/test-crawl` | Preview selectors (stub response) |
| POST | `/api/v1/sources/import-excel` | Bulk-import from Excel |
| POST | `/api/v1/sources/import-indigenous` | Bulk-import indigenous from CSV |
| GET | `/api/v1/sources/:id` | Get source by ID |
| GET | `/api/v1/sources/by-identity` | Lookup by identity_key |
| POST | `/api/v1/communities` | Create community |
| PUT | `/api/v1/communities/:id` | Update community |
| DELETE | `/api/v1/communities/:id` | Delete community |
| PATCH | `/api/v1/communities/:id/scraped` | Update last_scraped_at |
| POST | `/api/v1/communities/link-sources` | Auto-link sources to communities |
| POST | `/api/v1/communities/:id/people` | Create person |
| PUT | `/api/v1/people/:id` | Update person |
| DELETE | `/api/v1/people/:id` | Delete person |
| POST | `/api/v1/communities/:id/band-office` | Create/update band office |
| PUT | `/api/v1/band-offices/:id` | Update band office |

## Data Flow

```
Dashboard/API → [Source Manager] → PostgreSQL (sources, communities, people, band_offices)
                                 → Redis Stream (SourceCreated/Updated/Deleted events)

Crawler → GET /sources → fetches source list → creates crawl jobs
Publisher → GET /sources, /cities → routing decisions
Minoo → GET /communities → syncs First Nations data
```

## Storage / Schema

### sources (21 columns)

Key fields: `id` (UUID PK), `name` (UNIQUE), `url`, `rate_limit` (default '1s'), `max_depth` (default 2), `selectors` (JSONB), `enabled`, `feed_url`, `sitemap_url`, `ingestion_mode`, `render_mode` (static|dynamic), `type` (news|indigenous|government|mining|community), `indigenous_region`, `identity_key`, `extraction_profile` (JSONB), `template_hint`, `disabled_at`, `disable_reason`, `feed_disabled_at`, `feed_disable_reason`.

### communities (30 columns)

Key fields: `id`, `name`, `slug` (UNIQUE), `community_type`, `province`, `region`, `inac_id` (UNIQUE), `statcan_csd` (UNIQUE), `osm_relation_id`, `wikidata_qid`, `latitude`, `longitude`, `nation`, `treaty`, `language_group`, `population`, `website`, `feed_url`, `source_id` (FK → sources), `enabled`, `last_scraped_at`.

### people (15 columns)

Key fields: `id`, `community_id` (FK, cascade delete), `name`, `slug`, `role`, `role_title`, `email`, `phone`, `term_start`, `term_end`, `is_current`.

### band_offices, people_history

Supporting tables for office contacts and leadership audit trail.

## Configuration

| Env Var | Default | Purpose |
|---------|---------|---------|
| `SERVER_PORT` | 8050 | HTTP port |
| `DB_HOST/PORT/USER/PASSWORD/NAME` | — | PostgreSQL connection |
| `AUTH_JWT_SECRET` | — | Shared JWT secret |
| `REDIS_ADDRESS` | localhost:6379 | Redis connection |
| `REDIS_EVENTS_ENABLED` | false | Feature flag for event publishing |
| `CORS_ORIGINS` | localhost:3000,:3001,:3002 | CORS allowed origins |

## Edge Cases

- **Index name derivation**: Never stored in DB — derived at runtime: lowercase name, replace non-alphanumeric with underscores. "Example News" → `example_news_raw_content`.
- **Rate limit normalization**: Bare numbers auto-suffixed with "s" (e.g., `10` → `10s`).
- **Selector defaults**: title=h1, body=article, published_time=time[datetime].
- **Source disable vs feed disable**: Independent states — a source can be enabled but its feed disabled.
- **Community external IDs**: INAC, StatCan CSD, OSM relation, Wikidata QID — all optional, unique when present.

## OPD Dictionary Ingestion

### CLI Subcommand: `import-opd`

```bash
source-manager import-opd --file data/all_entries.jsonl [--batch-size 500] [--dry-run]
```

Imports Ojibwe People's Dictionary entries from a JSONL file into the `dictionary_entries` table. Each line is validated, transformed, and content-hashed (SHA-256 of canonical JSON with sorted keys). Idempotent via `ON CONFLICT (content_hash)` upsert.

- **`--dry-run`**: Validate and count without DB writes (no config.yml needed)
- **`--batch-size N`**: Entries per transaction (default 500)
- **Retry**: Each batch retries once on failure, then skips

### Dictionary Pipeline

```
JSONL file → [importer] validates/transforms → [BulkUpsertEntries] → PostgreSQL dictionary_entries
                                              → [projection] consent filter → ES opd_dictionary index
```

### Consent Model

All consent flags default to `false`. Only entries with `consent_public_display=true` are:
- Returned by the `/dictionary/*` HTTP API
- Projected to the `opd_dictionary` ES index

### Migration 019

Makes `content_hash` index UNIQUE (was non-unique in migration 017) to support `ON CONFLICT` upsert.

<!-- Reviewed: 2026-03-18 — added OPD ingestion pipeline -->
