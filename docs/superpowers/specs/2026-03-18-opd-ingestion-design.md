# OPD Ingestion Pipeline Design

**Date**: 2026-03-18
**Status**: Approved
**Issue**: #326

## Overview

Ingest the Ojibwe People's Dictionary (OPD) dataset (21,358 validated entries) into North Cloud as a source-manager subcommand with dual storage: PostgreSQL (authoritative, with consent flags) and Elasticsearch (search projection, consent-filtered).

## Architectural Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Storage model | DB authoritative + ES projection | Consent flags require a DB. ES provides full-text search. Mirrors mining pipeline pattern. |
| Service ownership | Source-manager subcommand | source-manager already owns `dictionary_entries` table, CRUD repository, and API endpoints. No new service needed. |
| ES index pattern | `opd_dictionary` (separate from `*_classified_content`) | Dictionary entries are structurally different from news articles. Mixing them in general search creates noise. |

## Data Flow

### Initial Import

```
all_entries.jsonl
  -> source-manager import-opd --file <path>
    -> validate (JSON Schema draft-07)
    -> transform (sandbox schema -> dictionary_entries schema)
    -> bulk upsert to dictionary_entries table (content_hash for idempotency)
    -> project to opd_dictionary ES index (consent-filtered)
```

### Monthly Re-scrape Update

```
new_entries.jsonl
  -> source-manager import-opd --file <path> --update
    -> compare content_hash per entry
    -> skip unchanged entries
    -> upsert changed entries to DB
    -> re-project changed entries to ES
    -> log: N new, N updated, N unchanged, N failed
```

## CLI Interface

```
source-manager import-opd \
  --file data/all_entries.jsonl \
  --batch-size 500 \
  --dry-run           # validate without writing
  --update            # diff mode for re-scrapes
  --es-project        # also project to ES after DB import
```

## Source-Manager Changes

### New Files

```
source-manager/
├── cmd_import_opd.go              # CLI subcommand entry point
└── internal/
    ├── importer/
    │   ├── opd.go                 # Read JSONL, transform, validate
    │   └── opd_test.go
    └── projection/
        ├── dictionary_es.go       # DB -> ES projection (consent-filtered)
        └── dictionary_es_test.go
```

### Modified Files

```
source-manager/
├── main.go                        # Add import-opd subcommand routing via os.Args[1]
└── internal/
    └── repository/
        └── dictionary.go          # Add BulkUpsertEntries (multi-row INSERT for batch performance)
```

### Migration

```
source-manager/
└── migrations/
    └── 019_ensure_unique_content_hash.up.sql   # CREATE UNIQUE INDEX IF NOT EXISTS
    └── 019_ensure_unique_content_hash.down.sql
```

### Subcommand Architecture

`main.go` checks `os.Args[1]` for `import-opd` before calling `bootstrap.Start()`. No CLI framework needed — the subcommand has a small fixed set of flags parsed via `flag.NewFlagSet`. This matches publisher's multi-command pattern (`publisher both|api|router`).

`cmd_import_opd.go` reuses source-manager's existing config loader and DB connection setup but skips the HTTP server. It:

1. Parses CLI flags via `flag.NewFlagSet("import-opd", ...)`
2. Loads config via existing `infraconfig.LoadWithDefaults`
3. Opens DB connection via existing config
4. Creates ES client (if `--es-project`)
5. Streams JSONL file line-by-line (not loaded into memory)
6. Validates and transforms each entry
7. Batches upserts via `BulkUpsertEntries` (multi-row `INSERT ... VALUES` for performance — 500 rows per statement, wrapped in a transaction per batch)
8. Optionally projects to ES
9. Prints summary

## Elasticsearch Mapping (`opd_dictionary`)

```json
{
  "mappings": {
    "properties": {
      "lemma":                  { "type": "text", "analyzer": "standard", "fields": { "keyword": { "type": "keyword" } } },
      "word_class":             { "type": "keyword" },
      "word_class_normalized":  { "type": "keyword" },
      "definitions":            { "type": "nested", "properties": {
        "text":     { "type": "text" },
        "language": { "type": "keyword" }
      }},
      "inflections":            { "type": "object", "properties": {
        "raw":    { "type": "text" },
        "forms":  { "type": "text" },
        "stem":   { "type": "keyword" }
      }},
      "examples":               { "type": "nested", "properties": {
        "ojibwe":  { "type": "text" },
        "english": { "type": "text" }
      }},
      "word_family":            { "type": "keyword" },
      "source_name":            { "type": "keyword" },
      "source_url":             { "type": "keyword" },
      "content_hash":           { "type": "keyword" },
      "consent_public_display": { "type": "boolean" },
      "attribution":            { "type": "keyword" },
      "license":                { "type": "keyword" },
      "indexed_at":             { "type": "date" }
    }
  }
}
```

Key decisions:
- `lemma` is `text` + `.keyword` for both full-text search and exact lookup
- `definitions` and `examples` are `nested` so pairs stay linked during queries
- `inflections.stem` is `keyword` for exact stem matching
- No `content_type` field — this index is dictionary-only
- `source_name` is a hardcoded constant set during projection (e.g., `"opd"`), not a DB column. The DB table uses `source_url` instead. This ES field enables future multi-dictionary filtering.
- Media/audio fields are NOT projected to ES — served from DB on detail requests
- Language-specific analyzers (Ojibwe morphology) can be added later without remapping

## Consent Enforcement

- DB stores all entries regardless of consent status (authoritative record)
- ES projection **only includes entries where `consent_public_display = true`**
- All consent flags default to `false` on import (per OPD license: CC BY-NC-SA 4.0)
- Attribution is mandatory: `"Ojibwe People's Dictionary, University of Minnesota"`
- If a consent flag changes in DB, re-projection updates or removes the ES document

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid JSON line | Skip, log to failures output, continue |
| Missing required field (lemma) | Skip, log, continue |
| DB upsert failure | Retry once, then skip and log |
| ES projection failure | Log but don't fail the import (ES is secondary) |
| Batch failure | Log failed batch, continue with next batch |
| Completion | Print summary: `{imported: N, updated: N, skipped: N, failed: N}` |

## Idempotency

- Each entry's `content_hash` is SHA-256 of a canonical JSON serialization of the entry (sorted keys, no whitespace). This is computed by the importer during transform.
- DB upsert: `ON CONFLICT (content_hash) DO UPDATE SET ...` — matches the existing `UpsertByContentHash` pattern and `idx_dictionary_entries_hash` index. A new migration ensures this index is UNIQUE if not already.
- ES uses the DB primary key as `_id` — re-projection overwrites cleanly

## Testing Plan

### Unit Tests
- JSONL parsing: valid entries, malformed JSON, missing fields
- Field validation: required fields, type checking, unicode handling
- Transform logic: sandbox schema to dictionary_entries schema
- Consent filtering in ES projection: only `consent_public_display=true` entries projected
- Content hash diffing: unchanged entries skipped, changed entries updated

### Integration Test
- Import 100 fixture entries end-to-end
- Verify DB rows match expected count and field values
- Verify ES documents match expected count (consent-filtered)
- Re-import same entries — verify 0 updates (idempotent)
- Modify 5 entries — verify only 5 updated on re-import

### Edge Cases
- Duplicate lemmas from same source
- Empty definitions array
- Missing inflections
- Unicode in Ojibwe text (double-vowels, nasals)
- Entries with `consent_public_display=false` — must not appear in ES

## Dataset Reference

- Repo: `waaseyaa/sandbox-ojibwe`
- Entries: `data/all_entries.jsonl` (21,358 validated entries)
- Failures: `data/failures.jsonl` (180 entries)
- Schemas: `specs/canonical-dictionary-schema.json`, `specs/opd-source-metadata-schema.json`
- License: CC BY-NC-SA 4.0

## Future Extensibility

This design establishes a repeatable pattern for Indigenous language datasets:

1. Prepare dataset as JSONL with canonical schema
2. Add `source-manager import-{dataset}` subcommand
3. Transform to `dictionary_entries` table
4. Project to `{dataset}_dictionary` ES index
5. Existing `/dictionary/search` API serves all datasets

Future enhancements (not in scope):
- Ojibwe-specific ES analyzer (morphological stemming)
- Cross-dictionary linking (OPD entry -> related entries in other dictionaries)
- Vector search for semantic similarity across languages
