# rfp-ingestor/CLAUDE.md

RFP (Request for Proposals) ingestor that polls CanadaBuys CSV feeds and indexes directly to Elasticsearch, **bypassing the classifier entirely**.

> **Procurement is the canonical landing zone.** New procurement sources extend the `PortalParser` interface here rather than a parallel crawler (per [`docs/specs/lead-pipeline.md`](../docs/specs/lead-pipeline.md) and `docs/prospect-engine-plan.md` §P3).

---

## Layer Rules

The rfp-ingestor's internal packages form a strict DAG organized into 4 layers.
A package may import from its own layer or any lower layer. Never from a higher layer.

| Layer | Packages | Role |
|-------|----------|------|
| L0 | `domain`, `config` | Foundation — no internal imports |
| L1 | `feed`, `elasticsearch`, `parser` | Infrastructure / portal-specific parsing |
| L2 | `ingestor` | Processing |
| L3 | `api` | HTTP |

**Rules:**
- `domain/` must not import any other rfp-ingestor package (it is the leaf)
- All shared infrastructure imports go through `infrastructure/` (no cross-service imports)
- Lateral imports within the same layer are allowed
- Bootstrap lives in `main.go` (no dedicated bootstrap package)

---

## Architecture

```
Procurement feed (HTTPS) — CanadaBuys CSV and/or other portals (see `feeds.sources` in config)
  → feed.Fetcher (HTTP 304 short-circuit)
  → parser.PortalParser (per-source CSV/JSON → RFPDocument map)
  → elasticsearch.Indexer.BulkIndex
  → rfp_classified_content (ES index)
```

**No database.** Unlike most north-cloud services, rfp-ingestor has no Postgres dependency. State lives entirely in Elasticsearch.

**Search integration**: The index name `rfp_classified_content` matches the `*_classified_content` wildcard pattern used by the search service, so RFP documents are automatically discoverable via the search API.

---

## Directory Structure

```
rfp-ingestor/
  main.go                        # Bootstrap: config → logger → server + poll loop
  internal/
    api/server.go                # HTTP server with GET /api/v1/status
    config/config.go             # All env vars and defaults
    domain/rfp.go                # RFPDocument struct
    elasticsearch/
      mapping.go                 # RFPIndexMapping() — ES index definition
      indexer.go                 # BulkIndex + EnsureIndex
    feed/fetcher.go              # HTTP GET with ETag/304 caching
    parser/                      # PortalParser implementations (CanadaBuys, SEAO, …)
    ingestor/
      csv_parser.go              # Delegates to parser (legacy helpers)
      ingestor.go                # Orchestrates fetch → parse → index (RunOnce)
```

---

## Common Commands

```bash
task dev                          # Hot-reload dev server
task build                        # Build binary
task test                         # Run tests
task lint                         # golangci-lint

# Backfill historical records (uses archive feed URL, much larger)
./rfp-ingestor backfill

# Check ingestion status
curl http://localhost:8095/api/v1/status
```

---

## Configuration

| Env Var | Default | Notes |
|---------|---------|-------|
| `RFP_INGESTOR_PORT` | `8095` | HTTP API port |
| `APP_DEBUG` | `false` | Debug logging |
| `ELASTICSEARCH_URL` | `http://localhost:9200` | ES connection |
| `ES_RFP_INDEX` | `rfp_classified_content` | Index name — must match `*_classified_content` pattern |
| `RFP_POLL_INTERVAL_MINUTES` | `120` | How often to poll the new-notices feed |
| `CANADABUYS_NEW_URL` | CanadaBuys new-notices CSV | Active RFPs — used by regular poll cycle |
| `CANADABUYS_OPEN_URL` | CanadaBuys open-notices CSV | Currently open RFPs |
| `CANADABUYS_ARCHIVE_URL` | CanadaBuys complete archive CSV | Historical — used only by `backfill` subcommand |
| `feeds.sources` (YAML) | — | Optional multi-source mode: list of `{name, parser, urls, enabled}`; when non-empty, replaces legacy single-feed composition for `RunOnce` |
| `LOG_LEVEL` | `info` | |
| `LOG_FORMAT` | `json` | |

---

## ES Index Mapping Constraints

**`content_type` must be mapped as `text` with an explicit `.keyword` sub-field.** The search service queries `content_type.keyword` for exact filtering. ES only auto-generates `.keyword` for dynamically mapped fields — since this index has an explicit mapping, the sub-field must be declared explicitly.

```go
// elasticsearch/mapping.go
"content_type": map[string]any{
    "type": "text",
    "fields": map[string]any{
        "keyword": map[string]any{"type": "keyword", "ignore_above": 256},
    },
},
```

Do NOT change `content_type` to `type: keyword` — the search service multi-match query will break.

**`raw_text`** stores the full description so the search service's multi-match query (which searches `title`, `raw_text`, `body`) can find RFP documents.

---

## RFP Domain Fields

Key fields in `RFPDocument` (from `domain/rfp.go`):

| Field | Type | Notes |
|-------|------|-------|
| `reference_number` | keyword | Unique CanadaBuys identifier |
| `organization_name` | keyword | Procuring entity |
| `procurement_type` | keyword | e.g. "Tender Notice" |
| `categories` | keyword | GSIN / commodity codes |
| `province` | keyword | Used by publisher Layer 4 (location routing) |
| `closing_date` | keyword | ISO date string |
| `tender_status` | keyword | e.g. "Active", "Closed" |

---

## Status API

```
GET /api/v1/status
```

Response fields: `last_run`, `last_success`, `fetched`, `indexed`, `failed`, `duration_ms`, `error` (if last run failed).

---

## Feed Behavior

- **Regular poll** uses `CANADABUYS_NEW_URL` — only new tender notices. Runs every `RFP_POLL_INTERVAL_MINUTES`.
- **`backfill` subcommand** uses `CANADABUYS_ARCHIVE_URL` — full historical dataset. Run once during initial setup or index recovery.
- **HTTP 304**: If the feed hasn't changed since last fetch, the fetcher short-circuits and no documents are parsed or indexed.
- **Parse errors** don't abort the cycle — they increment the `failed` counter and log a warning.

---

## Publisher Integration

rfp-ingestor feeds **publisher Layer 11 (RFP routing)**. The publisher reads `content_type == "rfp"` from the search index and routes to the configured RFP Redis channel. rfp-ingestor itself does not publish to Redis — it only writes to ES.
