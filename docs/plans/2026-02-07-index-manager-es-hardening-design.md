# Index Manager & Elasticsearch Hardening Design

**Date**: 2026-02-07
**Status**: Approved
**Scope**: index-manager, crawler, classifier, search, publisher

## Context

The index-manager and ES index schemas have not been evaluated since initial implementation. All other services have been refactored. An audit revealed six categories of issues: duplicate mappings, dead fields, uncontrolled dynamic mapping, no versioning strategy, and minimal ES settings.

This design addresses all findings in priority order.

---

## Priority 1: Data Correctness

### 1a. Single Source of Truth for Mappings

**Problem**: The crawler defines its own raw_content mapping in `raw_content_indexer.go` that diverges from the canonical mapping in index-manager. The crawler includes `json_ld_data`, `article_section`, and extended `meta` sub-fields that the canonical mapping doesn't define. Whichever service creates the index first determines the schema.

**Changes**:
- Remove the duplicate mapping from `crawler/internal/storage/raw_content_indexer.go`
- The crawler calls the index-manager API to ensure the index exists before indexing (capability already exists)
- Update the canonical mapping in `index-manager/internal/elasticsearch/mappings/raw_content.go` to include all fields the crawler actually writes (with explicit types)

**Files**:
- `crawler/internal/storage/raw_content_indexer.go` — remove mapping definition, ensure index-manager API call before indexing
- `index-manager/internal/elasticsearch/mappings/raw_content.go` — add missing fields

### 1b. Remove `is_crime_related`

**Problem**: The mapping defines `is_crime_related` (boolean), search and publisher filter on it, but the classifier never writes it. The classifier already produces `crime.street_crime_relevance` with three-tier classification (`core_street_crime`, `peripheral_crime`, `not_crime`) which is richer than a boolean.

**Decision**: Remove `is_crime_related` entirely. Update search and publisher to filter on `crime.street_crime_relevance` directly (or the existence of the `crime` object).

**Changes**:
- Remove `is_crime_related` from all ES mappings (raw_content, classified_content)
- Update `search/internal/elasticsearch/query_builder.go` to filter on `crime.street_crime_relevance`
- Update `publisher/internal/router/service.go` to use `crime.street_crime_relevance`
- Update contract tests that reference `is_crime_related`
- Remove from `index-manager/internal/domain/document.go` if referenced

**Files**:
- `index-manager/internal/elasticsearch/mappings/raw_content.go`
- `index-manager/internal/elasticsearch/mappings/classified_content.go`
- `search/internal/elasticsearch/query_builder.go`
- `publisher/internal/router/service.go`
- `tests/contracts/` — update relevant contract tests

### 1c. Remove Legacy Publisher Fields

**Problem**: Publisher's Article struct reads `intro`, `description`, `category`, `section` — none of these exist in the mapping or are written by the classifier. They are dead code from old Drupal integration.

**Changes**:
- Remove `intro`, `description`, `category`, `section` from the publisher's Article struct
- Remove any references to these fields in publisher routing/filtering logic

**Files**:
- `publisher/internal/router/service.go` — clean up Article struct

---

## Priority 2: Schema Integrity

### 2a. Add `dynamic: strict` to All Index Mappings

**Problem**: Unmapped fields get dynamically typed by ES based on the first document. This caused the type conflicts across 49 indexes we had to delete (image, mainEntityOfPage, author, wordCount, datePublished).

**Changes**:
- Set `"dynamic": "strict"` on both raw_content and classified_content mappings
- This rejects documents with fields not in the mapping, surfacing mismatches as indexing errors

**Prerequisite**: Priority 1a must be complete first (canonical mapping must include all crawler fields).

**Files**:
- `index-manager/internal/elasticsearch/mappings/raw_content.go`
- `index-manager/internal/elasticsearch/mappings/classified_content.go`
- `index-manager/internal/elasticsearch/mappings/mappings.go` — base mapping settings

### 2b. Explicitly Map `json_ld_data` Sub-fields

**Problem**: `json_ld_data` is `"type": "object"` with no properties. Everything inside gets dynamic mapping, which is the root cause of the polymorphic type conflicts.

**Changes**:
- Define explicit mappings for extracted JSON-LD fields:
  - `jsonld_headline` → `text`
  - `jsonld_description` → `text`
  - `jsonld_article_section` → `keyword`
  - `jsonld_author` → `text`
  - `jsonld_publisher_name` → `text`
  - `jsonld_url` → `keyword`
  - `jsonld_image_url` → `keyword`
  - `jsonld_date_published`, `jsonld_date_created`, `jsonld_date_modified` → `date`
  - `jsonld_word_count` → `integer`
  - `jsonld_keywords` → `keyword` (array)
- Set `jsonld_raw` to `"type": "object", "enabled": false` — stored for retrieval but not indexed or searchable. This eliminates the entire class of polymorphic type conflicts from arbitrary JSON-LD fields.

**Files**:
- `index-manager/internal/elasticsearch/mappings/raw_content.go` — add `json_ld_data` property definitions

### 2c. Explicitly Map `meta` Sub-fields

**Problem**: The crawler writes extended `meta` sub-properties (`twitter_card`, `twitter_site`, `og_image_width`, `og_image_height`, `og_site_name`, `created_at`, `updated_at`, `article_opinion`, `article_content_tier`) that aren't in the canonical mapping.

**Changes**:
- Add explicit field definitions for all `meta` sub-fields the crawler writes:
  - `twitter_card`, `twitter_site`, `og_site_name`, `article_content_tier` → `keyword`
  - `og_image_width`, `og_image_height` → `integer`
  - `article_opinion` → `boolean`
  - `created_at`, `updated_at` → `date`

**Files**:
- `index-manager/internal/elasticsearch/mappings/raw_content.go` — add `meta` property definitions

---

## Priority 3: Mapping Versioning & Migration

### 3a. Semantic Mapping Versions

**Problem**: `mapping_version` is hardcoded to `"1.0.0"` with no mechanism to track or update it.

**Changes**:
- Define version constants in the mappings package:
  - `RawContentMappingVersion = "2.0.0"`
  - `ClassifiedContentMappingVersion = "2.0.0"`
- Bump these when mappings change
- Store in `index_metadata` table on index creation
- Use semantic versioning: major bump for breaking changes (field type changes, removals), minor for additions

**Files**:
- `index-manager/internal/elasticsearch/mappings/` — version constants
- `index-manager/internal/service/index_service.go` — use version constants

### 3b. Reindex-Based Migration Endpoint

**Problem**: ES mappings are immutable — you can't change field types on existing indexes. The only path is create-new → reindex → swap.

**Changes**:
- Add endpoint: `POST /api/v1/indexes/:index_name/migrate`
  - Checks if index mapping version is behind current version
  - Creates new index with `_v{version}` suffix and latest mapping
  - Reindexes documents from old index to new
  - Deletes old index, creates alias or renames
  - Records migration in `migration_history` table (from_version, to_version, status)
- Sequential reindex is acceptable at current scale (~200 raw, ~250 classified)
- No alias swapping automation or zero-downtime orchestration needed

**Files**:
- `index-manager/internal/api/handlers.go` — new migration handler
- `index-manager/internal/api/routes.go` — new route
- `index-manager/internal/service/index_service.go` — migration logic
- `index-manager/internal/elasticsearch/client.go` — reindex operation

### 3c. Startup Version Drift Warnings

**Changes**:
- On index-manager startup, query `index_metadata` for all indexes
- Compare each index's `mapping_version` against current version constants
- Log warnings for any indexes with outdated versions
- No automatic migration — operator decides when to run it

**Files**:
- `index-manager/internal/bootstrap/elasticsearch.go` — add drift check after ES client init

---

## Priority 4: ES Settings & Performance

### 4a. Configurable Shard & Replica Settings

**Problem**: Every index gets hardcoded `shards: 1, replicas: 1` regardless of use case.

**Changes**:
- Make shard/replica counts configurable per index type in `config.yml`:
  ```yaml
  index_types:
    raw_content:
      shards: 1
      replicas: 0    # transient staging data, rebuildable via crawler
    classified_content:
      shards: 1
      replicas: 1    # search/publisher reads benefit from replica
  ```
- Pass settings from config to mapping factory when creating indexes

**Files**:
- `index-manager/internal/config/config.go` — add shard/replica fields to index type config
- `index-manager/internal/elasticsearch/mappings/factory.go` — accept settings parameter
- `index-manager/internal/elasticsearch/mappings/mappings.go` — configurable settings in base mapping

### 4b. Custom Text Analyzer for Classified Content

**Problem**: All `text` fields use ES default `standard` analyzer — no stemming, no stop words, no language-aware tokenization.

**Changes**:
- Define a custom `english_content` analyzer on classified_content indexes:
  ```json
  {
    "analysis": {
      "analyzer": {
        "english_content": {
          "type": "custom",
          "tokenizer": "standard",
          "filter": ["lowercase", "english_stop", "english_stemmer"]
        }
      },
      "filter": {
        "english_stop": { "type": "stop", "stopwords": "_english_" },
        "english_stemmer": { "type": "stemmer", "language": "english" }
      }
    }
  }
  ```
- Apply to `title`, `raw_text`, and `body` fields in classified_content mapping
- Raw content indexes do not need this (not searched directly)

**Files**:
- `index-manager/internal/elasticsearch/mappings/classified_content.go` — add analyzer settings and apply to text fields

### 4c. Index Lifecycle Management (Deferred)

Not implementing ILM at this time. Current index counts (~450 total) don't warrant automatic rollover, warm/cold tiers, or retention policies. The versioning infrastructure from Priority 3 provides the foundation to add ILM later if needed.

---

## Implementation Order

The priorities have dependencies that dictate sequencing:

1. **P1a** (single source of truth) — must come first, everything depends on canonical mappings
2. **P2b + P2c** (explicit json_ld_data + meta mappings) — extends the canonical mapping
3. **P2a** (dynamic: strict) — only safe after all fields are explicitly mapped
4. **P1b** (remove is_crime_related) — cross-service change, independent of mapping structure
5. **P1c** (remove legacy publisher fields) — small, independent cleanup
6. **P3a** (version constants) — needed before migration endpoint
7. **P4a + P4b** (settings + analyzer) — mapping enhancements, include in version bump
8. **P3b** (migration endpoint) — needed to migrate existing indexes to v2.0.0
9. **P3c** (startup drift check) — final piece, uses version infrastructure

After all code changes: run migration endpoint on all existing indexes to bring them to v2.0.0.

---

## Testing Strategy

- Update contract tests in `tests/contracts/` to reflect new field expectations
- Unit tests for new normalization functions (already done for JSON-LD)
- Unit tests for migration endpoint logic
- Integration test: create index with v1 mapping, migrate to v2, verify documents preserved
- Lint all modified services: `task lint:index-manager`, `task lint:crawler`, `task lint:search`, `task lint:publisher`

## Rollback

If issues arise after deploying v2.0.0 mappings:
- Old indexes still exist until explicitly deleted
- Migration endpoint records from/to versions — can be reversed
- `dynamic: strict` can be relaxed back to `dynamic: true` per-index via ES API
