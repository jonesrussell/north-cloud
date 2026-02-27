# Fix: Classifier Silent Bulk Indexing Failure

**Date**: 2026-02-27
**Status**: Approved
**Affects**: classifier, publisher (downstream)

## Problem

The classifier's `BulkIndexClassifiedContent` builds Elasticsearch index names from
`content.SourceName` (a human-readable field like `"Billboard"` or `"Campbell River Mirror"`).
ES requires lowercase index names with no spaces. The bulk API returns HTTP 200 even when
individual items fail, and the code only checks `res.IsError()` (HTTP-level errors), so
item-level failures are silently swallowed.

**Impact**: All content from sources with mixed-case or space-containing names is classified
but never written to ES classified indexes. The publisher cursor is stalled at 2026-02-24
because no new articles appear in classified indexes.

## Root Cause

Two bugs working together:

1. **Invalid index names** — `content.SourceName + "_classified_content"` produces names like
   `Billboard_classified_content` (rejected by ES: must be lowercase) or
   `Campbell River Mirror_classified_content` (rejected: spaces not allowed).

2. **Silent bulk errors** — ES bulk responses return HTTP 200 with `"errors": true` in the body
   when individual items fail. The code only checks `res.IsError()` which only catches HTTP 4xx/5xx.

## Design

### 1. Add `SourceIndex` field to `RawContent`

A non-serialized field that captures the ES index name (`_index`) from which the document was fetched.
`ClassifiedContent` inherits it via embedding.

```go
// domain/raw_content.go
type RawContent struct {
    // ... existing fields ...
    SourceIndex string `json:"-"` // ES index name, not serialized to ES
}
```

### 2. Capture `hit._index` in `QueryRawContent`

```go
// storage/elasticsearch.go, in QueryRawContent loop
content.SourceIndex = hit.Index
```

### 3. Replace all `SourceName + "_classified_content"` with `GetClassifiedIndexName`

Four call sites:
- `IndexClassifiedContent` (elasticsearch.go:103)
- `BulkIndexClassifiedContent` (elasticsearch.go:215)
- `OutboxWriter.Write` (outbox_writer.go:55)
- `OutboxWriter.WriteBatch` (outbox_writer.go:125)

Use existing `GetClassifiedIndexName(content.SourceIndex)` which strips `_raw_content`
and appends `_classified_content`.

Fallback when `SourceIndex` is empty (e.g. API-submitted content): sanitize `SourceName`
(lowercase, replace non-alphanumeric with underscore, collapse runs).

### 4. Add bulk response error checking

Parse the bulk response body for `"errors": true`. When found, extract individual item
errors and log them. Return an error with the count of failed items.

### 5. Regression tests

| Test | What it verifies |
|------|-----------------|
| `TestGetClassifiedIndexName` | Valid conversion and error on invalid input |
| `TestSanitizeSourceName` | Lowercase, spaces, special chars, collapse runs |
| `TestQueryRawContent_CapturesSourceIndex` | SourceIndex populated from hit._index |
| `TestBulkIndexClassifiedContent_ChecksBulkErrors` | Bulk item errors detected and returned |
| `TestBulkIndexClassifiedContent_UsesSourceIndex` | Index name derived from SourceIndex, not SourceName |
| `TestClassifiedIndexFallback_EmptySourceIndex` | Falls back to sanitized SourceName |

### 6. Data recovery (post-deploy)

Reset `classification_status` to `"pending"` for raw content items that were marked
`"classified"` but have no corresponding classified index document. This lets the poller
reclassify them with the fixed indexing logic.

```
POST *_raw_content/_update_by_query
{
  "query": { "term": { "classification_status": "classified" } },
  "script": { "source": "ctx._source.classification_status = 'pending'" }
}
```

Scope this to indexes where the source_name doesn't match the index prefix to avoid
re-processing already-correct content.
