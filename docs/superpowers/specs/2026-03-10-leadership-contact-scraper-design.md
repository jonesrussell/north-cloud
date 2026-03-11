# Leadership & Contact Page Scraper Design

**Issue**: #273
**Date**: 2026-03-10
**Status**: Approved

## Overview

Extend the crawler with a CLI command that discovers leadership and contact pages on community websites, extracts structured people and band office data, and stores it via source-manager API endpoints.

## Architecture

### CLI Command

```
crawler scrape-leadership [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--community-id` | (all) | Scrape a single community |
| `--dry-run` | false | Print extracted data as JSON, skip API writes |
| `--workers` | 4 | Concurrent worker goroutines |
| `--source-manager-url` | from config | Source-manager base URL |

### Components

```
crawler/
├── cmd/
│   └── scrape_leadership.go       # CLI entry point
└── internal/
    ├── leadership/                 # EXISTING — extraction logic
    │   ├── discovery.go            # ClassifyURL(), ClassifyLinkText()
    │   ├── leaders.go              # ExtractLeaders(text) → []Leader
    │   └── contact.go              # ExtractContact(text) → Contact
    └── scraper/                    # NEW — orchestration
        ├── scraper.go              # Scraper struct, worker pool, main loop
        └── client.go               # HTTP client for source-manager API
```

### Data Flow

```
crawler scrape-leadership
  → GET /api/v1/communities?has_source=true
  → Worker pool (default 4 goroutines):
      → For each community:
          1. Fetch homepage from community.website
          2. Scan links for leadership/contact pages (ClassifyURL, ClassifyLinkText)
          3. Fetch discovered pages
          4. ExtractLeaders() → compare with GET /api/v1/communities/:id/people
             → POST /api/v1/people only if changed (verified=false, data_source="crawler", source_url=<page_url>)
          5. ExtractContact() → compare with GET /api/v1/communities/:id/band-office
             → PUT /api/v1/band-offices only if changed (verified=false, data_source="crawler", source_url=<page_url>)
          6. PATCH /api/v1/communities/:id/scraped {"last_scraped_at": "<now>"}
          7. Log results (dry-run: print JSON to stdout, skip all writes)
```

## Source-Manager Changes

### New API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/communities/:id/people` | Public | List people for a community |
| `POST` | `/api/v1/people` | JWT | Create person (sets verified=false) |
| `GET` | `/api/v1/communities/:id/band-office` | Public | Get band office for a community |
| `PUT` | `/api/v1/band-offices` | JWT | Upsert band office by community_id |
| `PATCH` | `/api/v1/communities/:id/scraped` | JWT | Update last_scraped_at |

The PATCH endpoint accepts an explicit timestamp:

```json
{ "last_scraped_at": "2026-01-01T12:00:00Z" }
```

### New Migration

```sql
-- Up
ALTER TABLE band_offices ADD COLUMN source_url TEXT;
ALTER TABLE communities ADD COLUMN last_scraped_at TIMESTAMPTZ;

-- Down
ALTER TABLE band_offices DROP COLUMN source_url;
ALTER TABLE communities DROP COLUMN last_scraped_at;
```

Both columns nullable, backwards-compatible.

## Source Attribution

All scraped records include provenance fields:

- `data_source`: `"crawler"` — identifies the ingestion method
- `source_url`: the discovered page URL (e.g., `https://community.example.com/chief-and-council`)

These fields already exist on `Person`. The `source_url` field is added to `BandOffice` via migration.

## Skip-If-Unchanged Optimization

Before writing, the scraper fetches existing records and compares:

- **People**: Compare extracted `(name, role)` tuples against existing people for the community. Skip POST if all match.
- **Band office**: Compare extracted fields (address, phone, email, etc.) against existing record. Skip PUT if identical.

This avoids unnecessary writes and keeps audit history clean.

## Concurrency

- Worker pool size configurable via `--workers` flag (default 4)
- Each worker processes one community at a time
- HTTP client uses standard rate limiting per target domain
- Worker pool uses a buffered channel for job distribution

## Error Handling

- Skip communities with no `website` URL (log info)
- Skip communities where no leadership/contact page is discovered (log warning)
- Continue on individual community failures (log error, don't abort batch)
- HTTP client retries with exponential backoff for source-manager API calls
- Summary logged at end: total communities processed, people added, band offices updated, errors

## Dry-Run Mode

With `--dry-run`, the CLI:
- Fetches and extracts data normally
- Prints results as JSON to stdout (one object per community)
- Skips all POST/PUT/PATCH calls
- Useful for validating extraction quality before committing

## Testing

- **Unit tests**: Scraper orchestrator with mocked HTTP client and source-manager API
- **Existing tests**: `internal/leadership/` already covers extraction logic
- **Integration test**: Mock source-manager server, verify correct API calls and skip-if-unchanged logic

## Future Enhancements

- #298: AI-assisted verification of scraped data
- #274: Verification queue UI
- Change detection via content hashing (deferred from v1)
- Monthly recrawl schedule via crawler scheduler
- LLM sidecar for ambiguous layouts
