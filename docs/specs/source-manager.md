# Source Manager Specification

> Last verified: 2026-03-22 (align go-redis v9.18.0 across all services)

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
| `source-manager/migrations/` | SQL migrations (001–018) |

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
| GET | `/api/v1/dictionary/entries` | List public dictionary entries |
| GET | `/api/v1/dictionary/entries/:id` | Get one public dictionary entry |
| GET | `/api/v1/dictionary/words/:id` | Legacy dictionary entry route |
| GET | `/api/v1/dictionary/search` | Search public dictionary entries |
| GET | `/health` | Health check |

Dictionary pagination contract:
- `GET /api/v1/dictionary/entries` and `GET /api/v1/dictionary/search` both use `limit` as the primary page size parameter.
- Legacy `size` is still accepted as a fallback alias for `limit` during client migration, and both responses still echo `size` as a legacy alias.
- `page` is 1-based. On `/dictionary/entries`, if both `page` and `offset` are provided, `page` takes precedence and `offset` is recomputed from `page * limit`.
- `/dictionary/entries` always returns both `page` and `offset`. If the caller omits `page`, the response still reports the default `page: 1` alongside the effective `offset`.

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

### sources (25 columns)

Key fields: `id` (UUID PK), `name` (UNIQUE), `url`, `rate_limit` (default '1s'), `max_depth` (default 2), `selectors` (JSONB), `enabled`, `feed_url`, `sitemap_url`, `ingestion_mode`, `render_mode` (static|dynamic), `type` (news|indigenous|government|mining|community|structured|api), `indigenous_region`, `identity_key`, `extraction_profile` (JSONB), `template_hint`, `disabled_at`, `disable_reason`, `feed_disabled_at`, `feed_disable_reason`, `data_format`, `update_frequency`, `license_type`, `attribution_text`.

**Structured source metadata** (migration 018, nullable — only used by `structured`/`api` types):
- `data_format`: json, csv, rss, html, api
- `update_frequency`: daily, weekly, monthly, realtime
- `license_type`: open, cc-by, cc-by-sa, restricted, unknown
- `attribution_text`: required attribution for hosted content

When an update sets `enabled=false`, the API requires a non-empty `disable_reason` unless the row already has one. That transition sets `disabled_at` automatically. Updating back to `enabled=true` clears `disabled_at` and `disable_reason`.

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
source-manager import-opd --file data/all_entries.jsonl [--batch-size 500] [--dry-run] [--consent-public-display]
```

Imports Ojibwe People's Dictionary entries from a JSONL file into the `dictionary_entries` table. Each line is validated, transformed, and content-hashed (SHA-256 of canonical JSON with sorted keys). Idempotent via `ON CONFLICT (content_hash)` upsert.

- **`--dry-run`**: Validate and count without DB writes (no config.yml needed)
- **`--batch-size N`**: Entries per transaction (default 500)
- **`--consent-public-display`**: Import entries as immediately public
- **Flag semantics**: Applies `consent_public_display=true` to every validated entry before DB write when enabled
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

### Production Verification: 2026-03-20

Verified manually on `jones@northcloud.one:/home/deployer/north-cloud` before closing issue `#484`.

- Safety backup created first:
  - `/home/deployer/north-cloud/backups/source-manager/source-manager_source_manager_20260320_232105.sql.gz`
- Production database state (`source_manager.dictionary_entries`):
  - `22186` total rows
  - `22186` rows with `consent_public_display = true`
  - Earliest and latest `created_at` both `2026-03-14 03:28:17.949947+00`, indicating a single completed import event
- Existing server-side OPD import artifacts:
  - `/home/deployer/north-cloud/backups/northcloud-prod-20260313-225225/opd_import.log`
  - `/home/deployer/north-cloud/backups/northcloud-prod-20260313-225225/opd_import.csv`
  - `/home/deployer/north-cloud/backups/northcloud-prod-20260313-225225/all_entries.jsonl`
- Recorded import evidence:
  - `opd_import.log` contains `Rows to insert: 22195, Skipped: 0`

Important operational note:

- The currently deployed production `source-manager` binary exposed `import-opd --help`, but did **not** expose the newer `--consent-public-display` flag.
- A fresh local dry run on `~/dev/sandbox-ojibwe/data/all_entries.jsonl` against the current importer contract returned `0 valid, 22195 failed`.
- The sandbox JSONL file currently uses fields like `word`, `pos`, and `url`, while `source-manager/internal/importer/opd.go` currently expects `lemma`, `word_class`, and `source_url`.
- Because production already had public OPD data loaded and the deployed importer lagged the newer consent flag, rerunning the import on 2026-03-20 would have been risky and was intentionally avoided.

### Migration 019

Makes `content_hash` index UNIQUE (was non-unique in migration 017) to support `ON CONFLICT` upsert.

<!-- Reviewed: 2026-03-20 — added OPD production verification findings -->
