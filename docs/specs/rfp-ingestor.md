# RFP Ingestor Spec

> Last verified: 2026-04-13 (SEAO CKAN URL resolver, parser added to .layers)

## Overview

Stand-alone feed ingestor that polls procurement feeds (CanadaBuys CSV by default; optional multi-source YAML) and bulk-indexes documents to Elasticsearch. **Bypasses the classifier** — has no connection to the classifier or publisher pipeline. Documents become discoverable via the search service through the `*_classified_content` ES index wildcard.

---

## File Map

```
rfp-ingestor/
  main.go                              # Config → logger → HTTP server + poll loop
  internal/api/server.go               # GET /api/v1/status, GET /metrics
  internal/config/config.go            # Config struct + env var bindings + defaults
  internal/domain/rfp.go               # RFPDocument struct (maps to ES document)
  internal/elasticsearch/
    mapping.go                         # RFPIndexMapping() — explicit ES mapping
    indexer.go                         # EnsureIndex + BulkIndex
  internal/feed/fetcher.go             # HTTP GET with ETag/304 caching
  internal/feed/resolver.go            # URLResolver interface + SEAO CKAN implementation
  internal/parser/                     # PortalParser — CanadaBuys CSV, SEAO JSON, …
  internal/ingestor/
    csv_parser.go                      # Delegates to parser (legacy entry points)
    ingestor.go                        # RunOnce: fetch → parse → bulk index
```

---

## Feed Polling Interface

**Legacy mode** (default): three CanadaBuys CSV URLs drive behaviour when `feeds.sources` is empty in YAML:

| Feed | Env Var | Used By | Notes |
|------|---------|---------|-------|
| New notices | `CANADABUYS_NEW_URL` | Regular poll cycle | Active new tender notices |
| Open notices | `CANADABUYS_OPEN_URL` | Legacy poll | Currently open tenders (combined with new URL in multi-feed resolution) |
| Archive | `CANADABUYS_ARCHIVE_URL` | `backfill` subcommand | Full historical dataset (large) |

**Multi-source mode**: when `feeds.sources` in YAML is non-empty, each enabled source lists a `parser` name (must match a registered `PortalParser`) and one or more `urls`. `RunOnce` polls every URL per source. Sources may also set `resolver` (e.g., `"seao_ckan"`) to dynamically discover URLs before each poll cycle — static `urls` serve as fallback if resolution fails.

**URL Resolvers**: The `feed.URLResolver` interface allows feed URLs to be discovered dynamically. The `seao_ckan` resolver queries the Données Québec CKAN API (`package_show` endpoint) to find the latest `hebdo_*.json` resource, eliminating manual URL updates when SEAO publishes new weekly data.

**Poll cycle**: `RunOnce` is called at startup, then on a ticker every `RFP_POLL_INTERVAL_MINUTES` (default: 120). In legacy mode the poll uses resolved CanadaBuys URLs; in multi-source mode it iterates all configured source URLs.

**HTTP 304 short-circuit**: `feed.Fetcher` sends `If-None-Match` / `If-Modified-Since`. If feed unchanged, parse and index are skipped entirely.

**Backfill**: `./rfp-ingestor backfill` runs `RunOnce` against the archive URL once, then exits. Used during initial setup or after index recovery.

---

## ES Index Mapping

**Index name**: `rfp_classified_content`

Matches the `*_classified_content` wildcard pattern used by the search service's multi-index query. No additional configuration needed for search discoverability.

### Critical: content_type Field

`content_type` **must** be mapped as `type: text` with an explicit `.keyword` sub-field:

```json
"content_type": {
  "type": "text",
  "analyzer": "standard",
  "fields": {
    "keyword": { "type": "keyword", "ignore_above": 256 }
  }
}
```

The search service queries `content_type.keyword` for exact filtering. ES auto-generates `.keyword` only for dynamically mapped fields. This index has an explicit mapping, so the sub-field must be declared. **Never change `content_type` to `type: keyword`** — the search multi-match query breaks.

### Top-Level Fields

| Field | ES Type | Notes |
|-------|---------|-------|
| `title` | text | Tender title |
| `url` | keyword | CanadaBuys detail page URL — derived from reference number if empty in feed |
| `source_name` | keyword | Always `"canadabuys"` |
| `content_type` | text + .keyword | Always `"rfp"` |
| `quality_score` | integer | Computed during parse |
| `snippet` | text | Short description excerpt |
| `raw_text` | text | Full description — enables search multi-match |
| `topics` | keyword | Derived category tags |
| `crawled_at` | date | Ingestion timestamp |

### Nested `rfp` Object Fields

| Field | ES Type | Notes |
|-------|---------|-------|
| `rfp.reference_number` | keyword | Unique CanadaBuys ID |
| `rfp.organization_name` | keyword | Procuring entity |
| `rfp.procurement_type` | keyword | e.g. `"Tender Notice"` |
| `rfp.categories` | keyword | GSIN / UNSPSC commodity codes |
| `rfp.province` | keyword | Used by publisher L4 location routing |
| `rfp.city` | keyword | |
| `rfp.closing_date` | keyword | ISO date string |
| `rfp.tender_status` | keyword | e.g. `"Active"`, `"Closed"` |
| `rfp.description` | text | Full procurement description |
| `rfp.gsin` | keyword | Goods and Services Identification Number |

---

## Config Vars

| Env Var | Default | Required |
|---------|---------|----------|
| `RFP_INGESTOR_PORT` | `8095` | |
| `APP_DEBUG` | `false` | |
| `ELASTICSEARCH_URL` | `http://localhost:9200` | yes |
| `ES_RFP_INDEX` | `rfp_classified_content` | yes |
| `RFP_POLL_INTERVAL_MINUTES` | `120` | |
| `CANADABUYS_NEW_URL` | CanadaBuys default | |
| `CANADABUYS_OPEN_URL` | CanadaBuys default | |
| `CANADABUYS_ARCHIVE_URL` | CanadaBuys default | |
| `LOG_LEVEL` | `info` | |
| `LOG_FORMAT` | `json` | |

---

## Status API

```
GET /api/v1/status
```

```json
{
  "last_run": "2025-01-15T10:00:00Z",
  "last_success": "2025-01-15T10:00:05Z",
  "fetched": 142,
  "indexed": 138,
  "failed": 4,
  "duration_ms": 5200,
  "error": null
}
```

---

## Known Constraints

- **URL fallback**: When a CSV row has an empty `noticeURL` field, the `url` is derived from the reference number: `https://canadabuys.canada.ca/en/tender-opportunities/tender-notice/{refNumber}`. Prevents broken records with no accessible link.
- **No database dependency** — state is entirely in ES. No Postgres, no Redis.
- **No classifier integration** — rfp-ingestor never calls the classifier. Quality scoring and topic tagging happen in the active `parser` implementation during ingestion.
- **Publisher integration** — publisher Layer 11 reads `content_type == "rfp"` from the search index. rfp-ingestor itself never publishes to Redis.
- **Parse errors are non-fatal** — individual row parse failures increment `failed` counter and log a warning; the cycle continues with remaining rows.
- **Bulk size** — default 500 documents per Elasticsearch bulk request (`ES_RFP_INDEX` bulk_size config).

<!-- Reviewed: 2026-04-08 -->
