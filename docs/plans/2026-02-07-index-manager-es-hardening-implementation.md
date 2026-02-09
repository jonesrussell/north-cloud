# Index Manager & ES Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Harden Elasticsearch index mappings by establishing single source of truth, removing dead fields, enforcing strict schema, adding versioning/migration, and improving ES settings.

**Architecture:** The index-manager owns canonical mappings. The crawler will stop defining its own mapping and defer to index-manager. Cross-service dead fields (`is_crime_related`, legacy publisher fields) are removed. Mapping versioning enables reindex-based migration for immutable ES schemas.

**Tech Stack:** Go 1.24+, Elasticsearch 8, PostgreSQL, golangci-lint

---

## Task 1: Add Missing Crawler Fields to Canonical Raw Content Mapping

The canonical mapping in index-manager is missing fields that the crawler writes: `article_section`, `json_ld_data`, and `meta` with sub-fields. Add them to the canonical mapping so it becomes the single source of truth.

**Files:**
- Modify: `index-manager/internal/elasticsearch/mappings/classified_content.go` (where `getRawContentFields()` lives)
- Test: `index-manager/internal/elasticsearch/mappings/mappings_test.go`

**Step 1: Write failing tests for new fields**

Add test cases to `mappings_test.go` that expect `article_section`, `json_ld_data`, and `meta` fields in the raw content mapping.

Update `TestGetRawContentMapping_Structure` to include the new fields in `expectedFields` and bump `expectedFieldCount` from 20 to 23 (adding `article_section`, `json_ld_data`, `meta`).

Add new test `TestGetRawContentMapping_MetaSubFields` that verifies meta has sub-properties: `twitter_card`, `twitter_site`, `og_image_width`, `og_image_height`, `og_site_name`, `created_at`, `updated_at`, `article_opinion`, `article_content_tier`.

Add new test `TestGetRawContentMapping_JsonLdDataSubFields` that verifies json_ld_data has sub-properties for the extracted fields (`jsonld_headline`, `jsonld_description`, `jsonld_article_section`, `jsonld_author`, `jsonld_publisher_name`, `jsonld_url`, `jsonld_image_url`, `jsonld_date_published`, `jsonld_date_created`, `jsonld_date_modified`, `jsonld_word_count`, `jsonld_keywords`) and `jsonld_raw` (with `enabled: false`).

**Step 2: Run tests to verify they fail**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -v -run "TestGetRawContentMapping"`
Expected: FAIL — fields `article_section`, `json_ld_data`, `meta` not found

**Step 3: Add fields to `getRawContentFields()`**

In `classified_content.go`, add to the `getRawContentFields()` function return map:

```go
"article_section": map[string]any{
    "type": "keyword",
},
"json_ld_data": map[string]any{
    "type": "object",
    "properties": getJsonLdDataFields(),
},
"meta": map[string]any{
    "type": "object",
    "properties": getMetaFields(),
},
```

Create two new helper functions in `classified_content.go` (or a new file `raw_content_helpers.go` if `classified_content.go` gets too long):

```go
func getJsonLdDataFields() map[string]any {
    return map[string]any{
        "jsonld_headline":        map[string]any{"type": "text"},
        "jsonld_description":     map[string]any{"type": "text"},
        "jsonld_article_section": map[string]any{"type": "keyword"},
        "jsonld_author":          map[string]any{"type": "text"},
        "jsonld_publisher_name":  map[string]any{"type": "text"},
        "jsonld_url":             map[string]any{"type": "keyword"},
        "jsonld_image_url":       map[string]any{"type": "keyword"},
        "jsonld_date_published":  map[string]any{"type": "date", "format": "strict_date_optional_time||epoch_millis"},
        "jsonld_date_created":    map[string]any{"type": "date", "format": "strict_date_optional_time||epoch_millis"},
        "jsonld_date_modified":   map[string]any{"type": "date", "format": "strict_date_optional_time||epoch_millis"},
        "jsonld_word_count":      map[string]any{"type": "integer"},
        "jsonld_keywords":        map[string]any{"type": "keyword"},
        "jsonld_raw":             map[string]any{"type": "object", "enabled": false},
    }
}

func getMetaFields() map[string]any {
    return map[string]any{
        "twitter_card":         map[string]any{"type": "keyword"},
        "twitter_site":         map[string]any{"type": "keyword"},
        "og_image_width":       map[string]any{"type": "integer"},
        "og_image_height":      map[string]any{"type": "integer"},
        "og_site_name":         map[string]any{"type": "keyword"},
        "created_at":           map[string]any{"type": "date", "format": "strict_date_optional_time||epoch_millis"},
        "updated_at":           map[string]any{"type": "date", "format": "strict_date_optional_time||epoch_millis"},
        "article_opinion":      map[string]any{"type": "boolean"},
        "article_content_tier": map[string]any{"type": "keyword"},
    }
}
```

**Step 4: Run tests to verify they pass**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -v -run "TestGetRawContentMapping"`
Expected: PASS

**Step 5: Lint**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 6: Update contract tests for new raw_content fields**

In `tests/contracts/raw_content_producer_test.go`, add the new fields to `producedFields`:
```go
"article_section", "json_ld_data", "meta",
```

Add a new test `TestCrawlerProducesValidJsonLdFields` that verifies json_ld_data nested fields.
Add a new test `TestCrawlerProducesValidMetaFields` that verifies meta nested fields.

Run: `cd tests && go test ./contracts/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add index-manager/internal/elasticsearch/mappings/ tests/contracts/raw_content_producer_test.go
git commit -m "feat(index-manager): add article_section, json_ld_data, meta fields to canonical raw_content mapping"
```

---

## Task 2: Remove Duplicate Mapping from Crawler

Now that the canonical mapping has all fields, remove the duplicate mapping from the crawler's `raw_content_indexer.go`. The crawler should use the index-manager API (or just skip mapping creation, since indexes are created by index-manager).

**Files:**
- Modify: `crawler/internal/storage/raw_content_indexer.go`

**Step 1: Replace the inline mapping with a call to EnsureIndex without mapping**

In `EnsureRawContentIndex()` (lines 142-219 of `raw_content_indexer.go`), remove the entire `mapping := map[string]any{...}` block (lines 153-196) and the `json.Marshal(mapping)` call. Replace with a simpler approach: call `EnsureIndex` with an empty mapping string, which lets ES use the existing mapping if the index already exists.

The function becomes:
```go
func (r *RawContentIndexer) EnsureRawContentIndex(ctx context.Context, sourceName string) error {
    indexName := r.getRawContentIndexName(sourceName)

    if _, alreadyEnsured := r.ensuredIndexes.Load(indexName); alreadyEnsured {
        return nil
    }

    r.logger.Info("Ensuring raw_content index",
        infralogger.String("index", indexName),
        infralogger.String("source_name", sourceName),
    )

    indexManager := r.storage.GetIndexManager()
    err := indexManager.EnsureIndex(ctx, indexName, "")
    if err != nil {
        return fmt.Errorf("failed to ensure raw_content index: %w", err)
    }

    r.ensuredIndexes.Store(indexName, true)
    return nil
}
```

**Note**: The crawler's `EnsureIndex` already handles the "index exists" case — it's a no-op if the index is already present. The index-manager service (or its API) is responsible for creating indexes with the canonical mapping. The crawler just needs to verify the index exists.

**Step 2: Check if `json` import is still needed**

After removing the mapping definition, the `json` import may no longer be needed in `EnsureRawContentIndex`. Check if it's used elsewhere in the file. If not, remove the import.

**Step 3: Run crawler tests**

Run: `cd crawler && go test ./internal/storage/ -v`
Expected: PASS

**Step 4: Lint**

Run: `cd crawler && golangci-lint run`
Expected: No errors

**Step 5: Commit**

```bash
git add crawler/internal/storage/raw_content_indexer.go
git commit -m "refactor(crawler): remove duplicate raw_content mapping, defer to index-manager canonical"
```

---

## Task 3: Add `dynamic: strict` to Both Index Mappings

With all fields now explicitly mapped, enable `dynamic: strict` to reject documents with unmapped fields.

**Files:**
- Modify: `index-manager/internal/elasticsearch/mappings/raw_content.go`
- Modify: `index-manager/internal/elasticsearch/mappings/classified_content.go`
- Test: `index-manager/internal/elasticsearch/mappings/mappings_test.go`

**Step 1: Write failing test**

Add test `TestGetRawContentMapping_DynamicStrict` and `TestGetClassifiedContentMapping_DynamicStrict`:

```go
func TestGetRawContentMapping_DynamicStrict(t *testing.T) {
    t.Helper()
    mapping := mappings.GetRawContentMapping()
    mappingsObj := mapping["mappings"].(map[string]any)
    dynamic, exists := mappingsObj["dynamic"]
    if !exists {
        t.Fatal("raw_content mapping missing 'dynamic' setting")
    }
    if dynamic != "strict" {
        t.Errorf("dynamic = %v, want \"strict\"", dynamic)
    }
}
```

Same pattern for classified_content.

**Step 2: Run tests to verify they fail**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -v -run "DynamicStrict"`
Expected: FAIL

**Step 3: Add dynamic: strict to both mappings**

In `raw_content.go`, update `GetRawContentMapping()`:
```go
"mappings": map[string]any{
    "dynamic":    "strict",
    "properties": getRawContentFields(),
},
```

In `classified_content.go`, update `GetClassifiedContentMapping()`:
```go
"mappings": map[string]any{
    "dynamic":    "strict",
    "properties": properties,
},
```

**Step 4: Run tests to verify they pass**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -v`
Expected: PASS

**Step 5: Lint**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add index-manager/internal/elasticsearch/mappings/
git commit -m "feat(index-manager): add dynamic:strict to raw_content and classified_content mappings"
```

---

## Task 4: Remove `is_crime_related` from Mappings and Contract Tests

Replace all references to `is_crime_related` with `crime.street_crime_relevance` (or `crime.relevance` — check which field name the mapping actually uses).

**Files:**
- Modify: `index-manager/internal/elasticsearch/mappings/classified_content.go`
- Modify: `index-manager/internal/elasticsearch/mappings/mappings_test.go`
- Modify: `index-manager/internal/domain/document.go`
- Modify: `tests/contracts/classified_content_producer_test.go`
- Modify: `tests/contracts/publisher_classified_content_consumer_test.go`
- Modify: `tests/contracts/search_classified_content_consumer_test.go`

**Step 1: Remove `is_crime_related` from classified_content mapping**

In `classified_content.go`, `getClassificationFields()`, remove:
```go
// Keep is_crime_related for backward compatibility (computed field)
"is_crime_related": map[string]any{
    "type": "boolean",
},
```

**Step 2: Update mappings_test.go**

In `TestGetClassifiedContentMapping_ClassificationFields`, remove `"is_crime_related"` from the `classificationFields` list.

**Step 3: Update domain/document.go**

Remove the `IsCrimeRelated` field from the `Document` struct (line 54).
Remove the `ComputedIsCrimeRelated()` method.
Remove `IsCrimeRelated` from `DocumentFilters` (line 107).
Keep the `CrimeInfo` struct and its `IsCrimeRelated()` method — those are correct.

**Step 4: Update contract tests**

In `classified_content_producer_test.go`, remove `"is_crime_related"` from `topLevelFields`.
In `publisher_classified_content_consumer_test.go`, remove `"is_crime_related"` from `requiredFields`.
In `search_classified_content_consumer_test.go`, remove `"is_crime_related"` from `requiredFields`.

**Step 5: Run all contract tests**

Run: `cd tests && go test ./contracts/ -v`
Expected: PASS

**Step 6: Run index-manager tests**

Run: `cd index-manager && go test ./... -v`
Expected: PASS

**Step 7: Lint index-manager**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 8: Commit**

```bash
git add index-manager/internal/elasticsearch/mappings/ index-manager/internal/domain/document.go tests/contracts/
git commit -m "refactor: remove is_crime_related from mappings, domain, and contract tests"
```

---

## Task 5: Update Search Service to Use `crime.relevance`

Replace `is_crime_related` filter with `crime.relevance` in the search service.

**Files:**
- Modify: `search/internal/domain/search.go`
- Modify: `search/internal/elasticsearch/query_builder.go`

**Step 1: Update domain/search.go**

In `Filters` struct (line 26), replace:
```go
IsCrimeRelated  *bool      `json:"is_crime_related,omitempty"`
```
with:
```go
CrimeRelevance []string `json:"crime_relevance,omitempty"`
```

In `SearchHit` struct (line 74), replace:
```go
IsCrimeRelated bool                `json:"is_crime_related"`
```
with:
```go
CrimeRelevance string `json:"crime_relevance,omitempty"`
```

**Step 2: Update query_builder.go**

In `buildFilters()`, replace the crime-related filter block (lines 186-192):
```go
// Crime-related filter
if filters.IsCrimeRelated != nil {
    result = append(result, map[string]any{
        "term": map[string]any{
            "is_crime_related": *filters.IsCrimeRelated,
        },
    })
}
```

With:
```go
// Crime relevance filter
if len(filters.CrimeRelevance) > 0 {
    result = append(result, map[string]any{
        "terms": map[string]any{
            "crime.relevance": filters.CrimeRelevance,
        },
    })
}
```

In `Build()`, update the default `_source` fields list (line 65):
Replace `"is_crime_related"` with `"crime"`.

**Step 3: Run search tests**

Run: `cd search && go test ./... -v`
Expected: PASS

**Step 4: Lint**

Run: `cd search && golangci-lint run`
Expected: No errors

**Step 5: Commit**

```bash
git add search/internal/domain/search.go search/internal/elasticsearch/query_builder.go
git commit -m "refactor(search): replace is_crime_related filter with crime.relevance"
```

---

## Task 6: Remove Legacy Fields from Publisher Article Struct

Remove `IsCrimeRelated`, `Intro`, `Description`, `Category`, `Section`, `Keywords` from the publisher's Article struct.

**Files:**
- Modify: `publisher/internal/router/service.go`

**Step 1: Remove dead fields from Article struct**

In `service.go`, remove from the `Article` struct (around lines 249, 282-287):
```go
IsCrimeRelated   bool     `json:"is_crime_related"`
```
and:
```go
Intro       string   `json:"intro"`
Description string   `json:"description"`
...
Category    string   `json:"category"`
Section     string   `json:"section"`
Keywords    []string `json:"keywords"`
```

**Step 2: Check for any code referencing removed fields**

Search the publisher codebase for `IsCrimeRelated`, `Intro`, `Description`, `Category`, `Section`, `Keywords` to ensure they aren't used in routing logic or message construction. If any references exist, update them.

**Step 3: Run publisher tests**

Run: `cd publisher && go test ./... -v`
Expected: PASS

**Step 4: Lint**

Run: `cd publisher && golangci-lint run`
Expected: No errors

**Step 5: Commit**

```bash
git add publisher/internal/router/service.go
git commit -m "refactor(publisher): remove is_crime_related and legacy Drupal fields from Article struct"
```

---

## Task 7: Add Mapping Version Constants

Define semantic version constants for each mapping type and use them when creating indexes.

**Files:**
- Create: `index-manager/internal/elasticsearch/mappings/versions.go`
- Modify: `index-manager/internal/service/index_service.go`
- Test: `index-manager/internal/elasticsearch/mappings/mappings_test.go`

**Step 1: Write failing test**

Add test to `mappings_test.go`:
```go
func TestMappingVersionConstants(t *testing.T) {
    t.Helper()
    if mappings.RawContentMappingVersion == "" {
        t.Error("RawContentMappingVersion is empty")
    }
    if mappings.ClassifiedContentMappingVersion == "" {
        t.Error("ClassifiedContentMappingVersion is empty")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -v -run "TestMappingVersionConstants"`
Expected: FAIL — undefined

**Step 3: Create versions.go**

Create `index-manager/internal/elasticsearch/mappings/versions.go`:
```go
package mappings

// Mapping version constants.
// Bump major for breaking changes (field type changes, removals).
// Bump minor for additions.
const (
    RawContentMappingVersion        = "2.0.0"
    ClassifiedContentMappingVersion = "2.0.0"
)
```

**Step 4: Update index_service.go to use version constants**

In `index_service.go`, `CreateIndex()`, replace hardcoded `"1.0.0"` (lines 124, 144) with the correct version constant:

```go
import "github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
```

In the migration record (line 124):
```go
ToVersion: sql.NullString{String: mappings.GetMappingVersion(string(req.IndexType)), Valid: true},
```

In the metadata save (line 144):
```go
MappingVersion: mappings.GetMappingVersion(string(req.IndexType)),
```

Add helper to `versions.go`:
```go
// GetMappingVersion returns the current mapping version for an index type.
func GetMappingVersion(indexType string) string {
    switch indexType {
    case "raw_content":
        return RawContentMappingVersion
    case "classified_content":
        return ClassifiedContentMappingVersion
    default:
        return "1.0.0"
    }
}
```

**Step 5: Run tests**

Run: `cd index-manager && go test ./... -v`
Expected: PASS

**Step 6: Lint**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 7: Commit**

```bash
git add index-manager/internal/elasticsearch/mappings/versions.go index-manager/internal/service/index_service.go
git commit -m "feat(index-manager): add semantic mapping version constants, use in index creation"
```

---

## Task 8: Add Configurable Shard/Replica Settings

Make shard and replica counts configurable per index type in the config.

**Files:**
- Modify: `index-manager/internal/config/config.go`
- Modify: `index-manager/internal/elasticsearch/mappings/raw_content.go`
- Modify: `index-manager/internal/elasticsearch/mappings/classified_content.go`
- Modify: `index-manager/internal/elasticsearch/mappings/factory.go`
- Test: `index-manager/internal/elasticsearch/mappings/mappings_test.go`

**Step 1: Add Shards and Replicas to IndexTypeConfig**

In `config.go`, add fields to `IndexTypeConfig`:
```go
type IndexTypeConfig struct {
    Suffix     string `yaml:"suffix"`
    AutoCreate bool   `yaml:"auto_create"`
    Shards     int    `yaml:"shards"`
    Replicas   int    `yaml:"replicas"`
}
```

Add defaults in the `setDefaults` function chain. Create `setIndexTypeDefaults`:
```go
func setIndexTypeDefaults(cfg *IndexTypesConfig) {
    if cfg.RawContent.Shards == 0 {
        cfg.RawContent.Shards = 1
    }
    // raw_content: replicas default 0 (transient, rebuildable)
    // Note: 0 is the desired default for RawContent.Replicas, no special handling needed

    if cfg.ClassifiedContent.Shards == 0 {
        cfg.ClassifiedContent.Shards = 1
    }
    if cfg.ClassifiedContent.Replicas == 0 {
        cfg.ClassifiedContent.Replicas = 1
    }
}
```

Call this from `setDefaults()`.

**Step 2: Update mapping functions to accept settings**

Update `GetRawContentMapping` and `GetClassifiedContentMapping` to accept shard/replica parameters:
```go
func GetRawContentMapping(shards, replicas int) map[string]any {
    return map[string]any{
        "settings": map[string]any{
            "number_of_shards":   shards,
            "number_of_replicas": replicas,
        },
        "mappings": map[string]any{
            "dynamic":    "strict",
            "properties": getRawContentFields(),
        },
    }
}
```

Same for `GetClassifiedContentMapping(shards, replicas int)`.

**Step 3: Update factory to pass settings**

Update `GetMappingForType` to accept config:
```go
func GetMappingForType(indexType string, shards, replicas int) (map[string]any, error) {
    switch indexType {
    case "raw_content":
        return GetRawContentMapping(shards, replicas), nil
    case "classified_content":
        return GetClassifiedContentMapping(shards, replicas), nil
    ...
    }
}
```

**Step 4: Update callers**

In `index_service.go`, update the `CreateIndex` call to pass config values:
```go
mapping, err = mappings.GetMappingForType(string(req.IndexType), s.getShards(req.IndexType), s.getReplicas(req.IndexType))
```

Add helper methods to `IndexService` that read from config (or use defaults).

**Step 5: Update all tests**

All tests calling `GetRawContentMapping()` need to pass `(1, 1)` or `(1, 0)`:
```go
mapping := mappings.GetRawContentMapping(1, 1)
```

**Step 6: Run tests**

Run: `cd index-manager && go test ./... -v`
Expected: PASS

**Step 7: Lint**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 8: Commit**

```bash
git add index-manager/internal/config/config.go index-manager/internal/elasticsearch/mappings/ index-manager/internal/service/index_service.go
git commit -m "feat(index-manager): configurable shard/replica settings per index type"
```

---

## Task 9: Add Custom English Analyzer for Classified Content

Add an `english_content` analyzer to classified_content indexes for better search quality.

**Files:**
- Modify: `index-manager/internal/elasticsearch/mappings/classified_content.go`
- Test: `index-manager/internal/elasticsearch/mappings/mappings_test.go`

**Step 1: Write failing test**

Add test `TestGetClassifiedContentMapping_HasEnglishAnalyzer`:
```go
func TestGetClassifiedContentMapping_HasEnglishAnalyzer(t *testing.T) {
    t.Helper()
    mapping := mappings.GetClassifiedContentMapping(1, 1)
    settings := mapping["settings"].(map[string]any)

    analysis, exists := settings["analysis"]
    if !exists {
        t.Fatal("classified_content mapping missing 'analysis' settings")
    }
    analysisMap := analysis.(map[string]any)

    analyzer, exists := analysisMap["analyzer"]
    if !exists {
        t.Fatal("missing analyzer in analysis settings")
    }
    analyzerMap := analyzer.(map[string]any)

    if _, exists := analyzerMap["english_content"]; !exists {
        t.Error("missing english_content analyzer")
    }
}
```

Add test `TestGetClassifiedContentMapping_TextFieldsUseEnglishAnalyzer` that checks `title` and `raw_text` fields use `"english_content"` analyzer:
```go
func TestGetClassifiedContentMapping_TextFieldsUseEnglishAnalyzer(t *testing.T) {
    t.Helper()
    mapping := mappings.GetClassifiedContentMapping(1, 1)
    properties := mapping["mappings"].(map[string]any)["properties"].(map[string]any)
    assertFieldHasAnalyzer(t, properties, "title", "english_content")
    assertFieldHasAnalyzer(t, properties, "raw_text", "english_content")
}
```

**Step 2: Run tests to verify they fail**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -v -run "EnglishAnalyzer"`
Expected: FAIL

**Step 3: Add analyzer to GetClassifiedContentMapping**

In `classified_content.go`, update `GetClassifiedContentMapping()`:
```go
func GetClassifiedContentMapping(shards, replicas int) map[string]any {
    properties := make(map[string]any)
    maps.Copy(properties, getRawContentFields())
    maps.Copy(properties, getClassificationFields())

    // Override text fields to use english_content analyzer for search quality
    overrideAnalyzer(properties, "title", "english_content")
    overrideAnalyzer(properties, "raw_text", "english_content")

    return map[string]any{
        "settings": map[string]any{
            "number_of_shards":   shards,
            "number_of_replicas": replicas,
            "analysis":           getEnglishAnalysisSettings(),
        },
        "mappings": map[string]any{
            "dynamic":    "strict",
            "properties": properties,
        },
    }
}

func getEnglishAnalysisSettings() map[string]any {
    return map[string]any{
        "analyzer": map[string]any{
            "english_content": map[string]any{
                "type":      "custom",
                "tokenizer": "standard",
                "filter":    []string{"lowercase", "english_stop", "english_stemmer"},
            },
        },
        "filter": map[string]any{
            "english_stop":    map[string]any{"type": "stop", "stopwords": "_english_"},
            "english_stemmer": map[string]any{"type": "stemmer", "language": "english"},
        },
    }
}

func overrideAnalyzer(properties map[string]any, field, analyzer string) {
    if fieldMap, ok := properties[field].(map[string]any); ok {
        fieldMap["analyzer"] = analyzer
    }
}
```

**Step 4: Run tests**

Run: `cd index-manager && go test ./internal/elasticsearch/mappings/ -v`
Expected: PASS

**Note**: The `TestGetRawContentMapping_FieldTypes` test checks `title` and `raw_text` use `"standard"` analyzer — this is correct for raw_content. The classified_content override only affects classified indexes.

**Step 5: Lint**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add index-manager/internal/elasticsearch/mappings/classified_content.go index-manager/internal/elasticsearch/mappings/mappings_test.go
git commit -m "feat(index-manager): add english_content analyzer for classified_content text fields"
```

---

## Task 10: Add Reindex Migration Endpoint

Add `POST /api/v1/indexes/:index_name/migrate` that creates a new index with the latest mapping and reindexes documents.

**Files:**
- Modify: `index-manager/internal/elasticsearch/client.go` — add `Reindex()` method
- Modify: `index-manager/internal/service/index_service.go` — add `MigrateIndex()` method
- Modify: `index-manager/internal/api/handlers.go` — add `MigrateIndex` handler
- Modify: `index-manager/internal/api/routes.go` — add route
- Modify: `index-manager/internal/database/migrations.go` — add `ListAllActiveMetadata()`

**Step 1: Add Reindex method to ES client**

In `client.go`, add:
```go
// Reindex copies documents from source index to destination index using the ES Reindex API.
func (c *Client) Reindex(ctx context.Context, sourceIndex, destIndex string) (int64, error) {
    body := map[string]any{
        "source": map[string]any{"index": sourceIndex},
        "dest":   map[string]any{"index": destIndex},
    }

    bodyJSON, err := json.Marshal(body)
    if err != nil {
        return 0, fmt.Errorf("failed to marshal reindex body: %w", err)
    }

    res, err := c.esClient.Reindex(
        strings.NewReader(string(bodyJSON)),
        c.esClient.Reindex.WithContext(ctx),
        c.esClient.Reindex.WithWaitForCompletion(true),
    )
    if err != nil {
        return 0, fmt.Errorf("reindex API call failed: %w", err)
    }
    defer func() { _ = res.Body.Close() }()

    if res.IsError() {
        respBody, _ := io.ReadAll(res.Body)
        return 0, fmt.Errorf("reindex returned error [%d]: %s", res.StatusCode, string(respBody))
    }

    var result struct {
        Total int64 `json:"total"`
    }
    if decodeErr := json.NewDecoder(res.Body).Decode(&result); decodeErr != nil {
        return 0, fmt.Errorf("failed to decode reindex response: %w", decodeErr)
    }

    return result.Total, nil
}
```

**Step 2: Add MigrateIndex to index service**

In `index_service.go`, add:
```go
// MigrateIndex migrates an index to the latest mapping version.
// Creates a new index with _v{version} suffix, reindexes documents, deletes old index.
func (s *IndexService) MigrateIndex(ctx context.Context, indexName string) (*domain.MigrationResult, error) {
    // 1. Get current metadata
    metadata, err := s.db.GetIndexMetadata(ctx, indexName)
    if err != nil {
        return nil, fmt.Errorf("failed to get index metadata: %w", err)
    }

    // 2. Determine target version
    targetVersion := mappings.GetMappingVersion(metadata.IndexType)
    if metadata.MappingVersion == targetVersion {
        return &domain.MigrationResult{
            IndexName: indexName,
            Status:    "up_to_date",
            Message:   fmt.Sprintf("index already at version %s", targetVersion),
        }, nil
    }

    // 3. Create new index with latest mapping
    tempName := fmt.Sprintf("%s_v%s", indexName, strings.ReplaceAll(targetVersion, ".", "_"))
    mapping, mapErr := mappings.GetMappingForType(metadata.IndexType, s.getShards(...), s.getReplicas(...))
    if mapErr != nil {
        return nil, fmt.Errorf("failed to get mapping: %w", mapErr)
    }
    if createErr := s.esClient.CreateIndex(ctx, tempName, mapping); createErr != nil {
        return nil, fmt.Errorf("failed to create new index: %w", createErr)
    }

    // 4. Reindex documents
    docCount, reindexErr := s.esClient.Reindex(ctx, indexName, tempName)
    if reindexErr != nil {
        // Clean up temp index on failure
        _ = s.esClient.DeleteIndex(ctx, tempName)
        return nil, fmt.Errorf("reindex failed: %w", reindexErr)
    }

    // 5. Delete old index
    if deleteErr := s.esClient.DeleteIndex(ctx, indexName); deleteErr != nil {
        return nil, fmt.Errorf("failed to delete old index: %w", deleteErr)
    }

    // 6. Record migration
    migration := &database.MigrationHistory{
        IndexName:     indexName,
        FromVersion:   sql.NullString{String: metadata.MappingVersion, Valid: true},
        ToVersion:     sql.NullString{String: targetVersion, Valid: true},
        MigrationType: "reindex",
        Status:        "completed",
        CreatedAt:     time.Now(),
        CompletedAt:   sql.NullTime{Time: time.Now(), Valid: true},
    }
    _ = s.db.RecordMigration(ctx, migration)

    // 7. Update metadata for new index
    metadata.IndexName = tempName
    metadata.MappingVersion = targetVersion
    _ = s.db.SaveIndexMetadata(ctx, metadata)

    return &domain.MigrationResult{
        IndexName:      tempName,
        FromVersion:    metadata.MappingVersion,
        ToVersion:      targetVersion,
        DocumentCount:  docCount,
        Status:         "completed",
    }, nil
}
```

**Step 3: Add MigrationResult to domain**

In `index-manager/internal/domain/index.go`, add:
```go
// MigrationResult represents the result of an index migration
type MigrationResult struct {
    IndexName     string `json:"index_name"`
    FromVersion   string `json:"from_version,omitempty"`
    ToVersion     string `json:"to_version,omitempty"`
    DocumentCount int64  `json:"document_count,omitempty"`
    Status        string `json:"status"`
    Message       string `json:"message,omitempty"`
}
```

**Step 4: Add handler and route**

In `handlers.go`, add `MigrateIndex` handler:
```go
func (h *Handler) MigrateIndex(c *gin.Context) {
    indexName := c.Param("index_name")
    result, err := h.indexService.MigrateIndex(c.Request.Context(), indexName)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, result)
}
```

In `routes.go`, add after the health route:
```go
indexes.POST("/:index_name/migrate", handler.MigrateIndex)
```

**Step 5: Run tests**

Run: `cd index-manager && go test ./... -v`
Expected: PASS

**Step 6: Lint**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 7: Commit**

```bash
git add index-manager/internal/
git commit -m "feat(index-manager): add reindex-based migration endpoint POST /indexes/:name/migrate"
```

---

## Task 11: Add Startup Version Drift Warnings

On startup, check all tracked indexes and log warnings for outdated mapping versions.

**Files:**
- Modify: `index-manager/internal/bootstrap/elasticsearch.go`
- Modify: `index-manager/internal/database/migrations.go` — add `ListAllActiveMetadata()`

**Step 1: Add ListAllActiveMetadata to database**

In `migrations.go`, add:
```go
// ListAllActiveMetadata returns all active index metadata records.
func (c *Connection) ListAllActiveMetadata(ctx context.Context) ([]*IndexMetadata, error) {
    query := `
        SELECT id, index_name, index_type, source_name, mapping_version, created_at, updated_at, status
        FROM index_metadata
        WHERE status = 'active'
        ORDER BY index_name
    `
    rows, err := c.DB.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to list all index metadata: %w", err)
    }
    defer func() { _ = rows.Close() }()
    return scanIndexMetadataRows(rows)
}
```

**Step 2: Add drift check function**

In `bootstrap/elasticsearch.go`, add:
```go
// CheckMappingVersionDrift logs warnings for indexes whose mapping version
// is behind the current version constants.
func CheckMappingVersionDrift(db *database.Connection, log infralogger.Logger) {
    ctx := context.Background()
    allMetadata, err := db.ListAllActiveMetadata(ctx)
    if err != nil {
        log.Warn("Failed to check mapping version drift", infralogger.Error(err))
        return
    }

    for _, meta := range allMetadata {
        currentVersion := mappings.GetMappingVersion(meta.IndexType)
        if meta.MappingVersion != currentVersion {
            log.Warn("Index mapping version drift detected",
                infralogger.String("index_name", meta.IndexName),
                infralogger.String("current_version", meta.MappingVersion),
                infralogger.String("latest_version", currentVersion),
                infralogger.String("index_type", meta.IndexType),
            )
        }
    }
}
```

**Step 3: Wire into startup**

Find where the index-manager starts up (likely `main.go` or a bootstrap `Start()` function) and call `CheckMappingVersionDrift()` after the database connection is established.

**Step 4: Run tests**

Run: `cd index-manager && go test ./... -v`
Expected: PASS

**Step 5: Lint**

Run: `cd index-manager && golangci-lint run`
Expected: No errors

**Step 6: Commit**

```bash
git add index-manager/internal/bootstrap/elasticsearch.go index-manager/internal/database/migrations.go
git commit -m "feat(index-manager): add startup mapping version drift warnings"
```

---

## Task 12: Final Validation

Run all linters and tests across all affected services.

**Step 1: Run all tests**

```bash
cd index-manager && go test ./... -v
cd crawler && go test ./... -v
cd search && go test ./... -v
cd publisher && go test ./... -v
cd tests && go test ./contracts/ -v
```

**Step 2: Run all linters**

```bash
cd index-manager && golangci-lint run
cd crawler && golangci-lint run
cd search && golangci-lint run
cd publisher && golangci-lint run
```

**Step 3: Verify contract tests pass**

All 5 contract test files should pass:
- `raw_content_producer_test.go` — includes new `article_section`, `json_ld_data`, `meta` fields
- `raw_content_consumer_test.go` — unchanged
- `classified_content_producer_test.go` — `is_crime_related` removed
- `publisher_classified_content_consumer_test.go` — `is_crime_related` removed
- `search_classified_content_consumer_test.go` — `is_crime_related` removed

**Step 4: Update CLAUDE.md files if needed**

Update `index-manager/CLAUDE.md` to reflect:
- New fields in raw_content mapping
- `dynamic: strict` on both mappings
- `is_crime_related` removal
- Version constants
- Migration endpoint
- Configurable shard/replica settings
- English analyzer for classified_content

---

## Summary of Changes by Service

| Service | Changes |
|---------|---------|
| **index-manager** | Add fields to canonical mapping, `dynamic: strict`, remove `is_crime_related`, version constants, configurable settings, english analyzer, migration endpoint, startup drift check |
| **crawler** | Remove duplicate mapping from `raw_content_indexer.go` |
| **search** | Replace `is_crime_related` filter with `crime.relevance` |
| **publisher** | Remove `IsCrimeRelated` and legacy Drupal fields from Article struct |
| **tests/contracts** | Update contract tests for field changes |
