# Classifier Index Naming Fix — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix silent bulk indexing failure caused by invalid ES index names derived from human-readable source names.

**Architecture:** Add `SourceIndex` field to `RawContent` populated from ES `_index` during query; use existing `GetClassifiedIndexName()` to derive classified index names; add a `SanitizeSourceName()` fallback for API-submitted content; parse bulk response body for item-level errors.

**Tech Stack:** Go, Elasticsearch 9.x go-elasticsearch/v8 client

---

### Task 1: Add `SanitizeSourceName` utility with tests

**Files:**
- Create: `classifier/internal/storage/index_naming.go`
- Create: `classifier/internal/storage/index_naming_test.go`

**Step 1: Write the failing tests**

```go
// classifier/internal/storage/index_naming_test.go
package storage

import (
	"testing"
)

func TestGetClassifiedIndexName(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{name: "valid raw index", input: "billboard_raw_content", expected: "billboard_classified_content"},
		{name: "valid with dots", input: "apnews_com_raw_content", expected: "apnews_com_classified_content"},
		{name: "empty string", input: "", wantErr: true},
		{name: "missing suffix", input: "billboard", wantErr: true},
		{name: "wrong suffix", input: "billboard_classified_content", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetClassifiedIndexName(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSanitizeSourceName(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "already valid", input: "apnews_com", expected: "apnews_com"},
		{name: "uppercase", input: "Billboard", expected: "billboard"},
		{name: "spaces", input: "Campbell River Mirror", expected: "campbell_river_mirror"},
		{name: "mixed case with spaces", input: "Manitoba Keewatinowi Okimakanak", expected: "manitoba_keewatinowi_okimakanak"},
		{name: "parentheses", input: "Awards Circuit (Variety)", expected: "awards_circuit_variety"},
		{name: "multiple spaces", input: "Some  Double  Spaced", expected: "some_double_spaced"},
		{name: "leading trailing spaces", input: "  Billboard  ", expected: "billboard"},
		{name: "special chars", input: "CNET!", expected: "cnet"},
		{name: "dots and hyphens", input: "news.com-au", expected: "news_com_au"},
		{name: "empty string", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeSourceName(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeSourceName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestClassifiedIndexForContent(t *testing.T) {
	t.Helper()

	tests := []struct {
		name        string
		sourceIndex string
		sourceName  string
		expected    string
		wantErr     bool
	}{
		{
			name:        "prefers source index",
			sourceIndex: "billboard_raw_content",
			sourceName:  "Billboard",
			expected:    "billboard_classified_content",
		},
		{
			name:        "falls back to sanitized source name",
			sourceIndex: "",
			sourceName:  "Billboard",
			expected:    "billboard_classified_content",
		},
		{
			name:        "fallback with spaces",
			sourceIndex: "",
			sourceName:  "Campbell River Mirror",
			expected:    "campbell_river_mirror_classified_content",
		},
		{
			name:        "both empty",
			sourceIndex: "",
			sourceName:  "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClassifiedIndexForContent(tt.sourceIndex, tt.sourceName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd classifier && go test ./internal/storage/ -run "TestSanitize|TestClassifiedIndex" -v`
Expected: FAIL — `SanitizeSourceName` and `ClassifiedIndexForContent` not defined.

**Step 3: Implement the functions**

Move `GetClassifiedIndexName` from `elasticsearch.go` into the new file and add the new functions:

```go
// classifier/internal/storage/index_naming.go
package storage

import (
	"errors"
	"regexp"
	"strings"
)

// validIndexChar matches characters allowed in ES index names: lowercase alphanumeric and underscore.
var validIndexChar = regexp.MustCompile(`[^a-z0-9_]`)

// collapseUnderscores replaces runs of multiple underscores with a single one.
var collapseUnderscores = regexp.MustCompile(`_+`)

const (
	rawContentSuffix        = "_raw_content"
	classifiedContentSuffix = "_classified_content"
)

// GetClassifiedIndexName returns the classified_content index name for a raw_content index.
func GetClassifiedIndexName(rawIndex string) (string, error) {
	if !strings.HasSuffix(rawIndex, rawContentSuffix) {
		return "", errors.New("invalid raw_content index name")
	}
	return rawIndex[:len(rawIndex)-len(rawContentSuffix)] + classifiedContentSuffix, nil
}

// SanitizeSourceName converts a human-readable source name into a valid ES index prefix.
// Lowercases, replaces non-alphanumeric chars with underscore, collapses runs, trims.
func SanitizeSourceName(name string) string {
	if name == "" {
		return ""
	}
	s := strings.ToLower(strings.TrimSpace(name))
	s = validIndexChar.ReplaceAllString(s, "_")
	s = collapseUnderscores.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return s
}

// ClassifiedIndexForContent determines the classified index name for a content item.
// Prefers SourceIndex (derived from ES _index field); falls back to sanitized SourceName.
func ClassifiedIndexForContent(sourceIndex, sourceName string) (string, error) {
	if sourceIndex != "" {
		return GetClassifiedIndexName(sourceIndex)
	}
	sanitized := SanitizeSourceName(sourceName)
	if sanitized == "" {
		return "", errors.New("cannot determine classified index: both source_index and source_name are empty")
	}
	return sanitized + classifiedContentSuffix, nil
}
```

**Step 4: Remove old `GetClassifiedIndexName` from `elasticsearch.go`**

Delete lines 313-320 from `classifier/internal/storage/elasticsearch.go` (the old `GetClassifiedIndexName` function and its import of `"errors"`). The function now lives in `index_naming.go` in the same package.

**Step 5: Run tests to verify they pass**

Run: `cd classifier && go test ./internal/storage/ -run "TestGetClassified|TestSanitize|TestClassifiedIndex" -v`
Expected: PASS

**Step 6: Commit**

```
feat(classifier): add SanitizeSourceName and ClassifiedIndexForContent utilities

Extracts index naming logic into index_naming.go with full test coverage.
SanitizeSourceName handles human-readable source names (uppercase, spaces,
special chars) by converting them to valid ES index prefixes.
ClassifiedIndexForContent prefers the raw ES index name, falling back to
sanitized SourceName for API-submitted content.
```

---

### Task 2: Add `SourceIndex` field to `RawContent` and capture in `QueryRawContent`

**Files:**
- Modify: `classifier/internal/domain/raw_content.go:7-43`
- Modify: `classifier/internal/storage/elasticsearch.go:85-93`

**Step 1: Add `SourceIndex` field to `RawContent`**

In `classifier/internal/domain/raw_content.go`, add after line 11 (`SourceName`):

```go
	SourceIndex string `json:"-"` // ES index name from _index, not serialized
```

**Step 2: Capture `hit.Index` in `QueryRawContent`**

In `classifier/internal/storage/elasticsearch.go`, modify the loop at lines 86-93 to capture the index:

```go
	contents := make([]*domain.RawContent, 0, len(searchResult.Hits.Hits))
	for i := range searchResult.Hits.Hits {
		hit := &searchResult.Hits.Hits[i]
		content := hit.Source
		// Preserve the Elasticsearch document ID if not already set
		if content.ID == "" {
			content.ID = hit.ID
		}
		content.SourceIndex = hit.Index
		contents = append(contents, &content)
	}
```

**Step 3: Run existing tests**

Run: `cd classifier && go test ./internal/... -v -count=1`
Expected: PASS (no behavior change yet)

**Step 4: Commit**

```
feat(classifier): add SourceIndex field to RawContent, capture from ES _index

SourceIndex is populated during QueryRawContent from the ES hit._index field.
This provides the actual raw content index name for deriving the classified
index name, avoiding reliance on human-readable SourceName.
```

---

### Task 3: Replace index derivation in `IndexClassifiedContent` and `BulkIndexClassifiedContent`

**Files:**
- Modify: `classifier/internal/storage/elasticsearch.go:101-103,206-215`

**Step 1: Fix `IndexClassifiedContent` (line 103)**

Replace:
```go
	classifiedIndex := content.SourceName + "_classified_content"
```

With:
```go
	classifiedIndex, indexErr := ClassifiedIndexForContent(content.SourceIndex, content.SourceName)
	if indexErr != nil {
		return fmt.Errorf("cannot determine classified index for %s: %w", content.ID, indexErr)
	}
```

**Step 2: Fix `BulkIndexClassifiedContent` (line 215)**

Replace:
```go
		classifiedIndex := content.SourceName + "_classified_content"
```

With:
```go
		classifiedIndex, indexErr := ClassifiedIndexForContent(content.SourceIndex, content.SourceName)
		if indexErr != nil {
			return fmt.Errorf("cannot determine classified index for %s: %w", content.ID, indexErr)
		}
```

**Step 3: Run tests**

Run: `cd classifier && go test ./internal/... -v -count=1`
Expected: PASS

**Step 4: Commit**

```
fix(classifier): use ClassifiedIndexForContent instead of SourceName concatenation

Fixes the root cause of silent bulk indexing failures where human-readable
source names like "Billboard" or "Campbell River Mirror" produced invalid
ES index names (uppercase, spaces). Now derives the classified index from
the raw index name, falling back to sanitized SourceName.
```

---

### Task 4: Replace index derivation in `OutboxWriter`

**Files:**
- Modify: `classifier/internal/storage/outbox_writer.go:55,125`

**Step 1: Fix `Write` (line 55)**

Replace:
```go
	indexName := content.SourceName + "_classified_content"
```

With:
```go
	indexName, indexErr := ClassifiedIndexForContent(content.SourceIndex, content.SourceName)
	if indexErr != nil {
		return fmt.Errorf("cannot determine classified index for %s: %w", content.ID, indexErr)
	}
```

**Step 2: Fix `WriteBatch` (line 125)**

Replace:
```go
		indexName := content.SourceName + "_classified_content"
```

With:
```go
		indexName, indexErr := ClassifiedIndexForContent(content.SourceIndex, content.SourceName)
		if indexErr != nil {
			return fmt.Errorf("cannot determine classified index for %s: %w", content.ID, indexErr)
		}
```

**Step 3: Run tests**

Run: `cd classifier && go test ./internal/... -v -count=1`
Expected: PASS

**Step 4: Commit**

```
fix(classifier): use ClassifiedIndexForContent in outbox writer

Same fix as elasticsearch.go — derives index name from SourceIndex or
sanitized SourceName instead of raw SourceName concatenation.
```

---

### Task 5: Add bulk response error checking

**Files:**
- Modify: `classifier/internal/storage/elasticsearch.go:242-259`
- Modify: `classifier/internal/storage/index_naming_test.go` (add bulk error parsing test)

**Step 1: Write the failing test**

Add to `classifier/internal/storage/index_naming_test.go`:

```go
func TestParseBulkErrors(t *testing.T) {
	t.Helper()

	tests := []struct {
		name        string
		body        string
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name:    "no errors",
			body:    `{"errors":false,"items":[{"index":{"_index":"test","_id":"1","status":201}}]}`,
			wantErr: false,
		},
		{
			name:       "with item error",
			body:       `{"errors":true,"items":[{"index":{"_index":"Test_classified","_id":"1","status":400,"error":{"type":"invalid_index_name_exception","reason":"Invalid index name [Test_classified], must be lowercase"}}}]}`,
			wantErr:    true,
			wantErrMsg: "1 of 1 bulk items failed",
		},
		{
			name:       "mixed success and failure",
			body:       `{"errors":true,"items":[{"index":{"_index":"good","_id":"1","status":201}},{"index":{"_index":"Bad","_id":"2","status":400,"error":{"type":"invalid_index_name_exception","reason":"must be lowercase"}}}]}`,
			wantErr:    true,
			wantErrMsg: "1 of 2 bulk items failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkBulkResponse([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
```

Add `"strings"` to the test file imports.

**Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/storage/ -run TestParseBulkErrors -v`
Expected: FAIL — `checkBulkResponse` not defined.

**Step 3: Implement bulk error checking**

Add to `classifier/internal/storage/index_naming.go`:

```go
// bulkResponse is the minimal structure needed to check for item-level errors.
type bulkResponse struct {
	Errors bool       `json:"errors"`
	Items  []bulkItem `json:"items"`
}

// bulkItem represents one action result in a bulk response.
type bulkItem struct {
	Index bulkItemResult `json:"index"`
}

// bulkItemResult holds the status and optional error for a single bulk item.
type bulkItemResult struct {
	Index  string         `json:"_index"`
	ID     string         `json:"_id"`
	Status int            `json:"status"`
	Error  map[string]any `json:"error,omitempty"`
}

// checkBulkResponse parses an ES bulk response body and returns an error if any items failed.
func checkBulkResponse(body []byte) error {
	var resp bulkResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse bulk response: %w", err)
	}

	if !resp.Errors {
		return nil
	}

	var failedCount int
	var firstErr string
	for _, item := range resp.Items {
		if item.Index.Error != nil {
			failedCount++
			if firstErr == "" {
				reason, _ := item.Index.Error["reason"].(string)
				errType, _ := item.Index.Error["type"].(string)
				firstErr = fmt.Sprintf("index=%s id=%s type=%s reason=%s",
					item.Index.Index, item.Index.ID, errType, reason)
			}
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("%d of %d bulk items failed; first error: %s", failedCount, len(resp.Items), firstErr)
	}

	return nil
}
```

Add `"encoding/json"` and `"fmt"` to imports in `index_naming.go`.

**Step 4: Update `BulkIndexClassifiedContent` to use `checkBulkResponse`**

Replace the tail of the function (lines 242-259 in `elasticsearch.go`) with:

```go
	res, err := s.client.Bulk(
		bytes.NewReader(buf.Bytes()),
		s.client.Bulk.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("bulk request failed: %w", err)
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	if res.IsError() {
		return fmt.Errorf("bulk indexing error: %s", res.String())
	}

	// Parse response body for item-level errors (ES returns HTTP 200 even when items fail)
	bodyBytes, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return fmt.Errorf("failed to read bulk response body: %w", readErr)
	}

	return checkBulkResponse(bodyBytes)
```

Add `"io"` to `elasticsearch.go` imports.

**Step 5: Run tests**

Run: `cd classifier && go test ./internal/storage/ -run TestParseBulkErrors -v`
Expected: PASS

Run: `cd classifier && go test ./internal/... -v -count=1`
Expected: PASS

**Step 6: Commit**

```
fix(classifier): parse bulk response body for item-level ES errors

Previously, BulkIndexClassifiedContent only checked res.IsError() which
catches HTTP-level errors (4xx/5xx). ES bulk operations return HTTP 200
even when individual items fail (e.g. invalid index name). Now parses the
response body for "errors": true and reports the count and first error.
```

---

### Task 6: Add regression test for the full pipeline (mixed-case SourceName)

**Files:**
- Modify: `classifier/internal/processor/integration_test.go`

**Step 1: Add test for SourceIndex propagation**

Add the following test to `classifier/internal/processor/integration_test.go`:

```go
func TestProcessPending_SourceIndexPropagated(t *testing.T) {
	t.Helper()

	esClient, dbClient, logger := setupTestEnvironment()

	// Set SourceIndex to simulate what QueryRawContent does
	for _, raw := range esClient.rawContent {
		raw.SourceIndex = "example_com_raw_content"
	}

	c := createTestClassifier(logger)
	bp := processor.NewBatchProcessor(c, logger, 10)
	poller := processor.NewPoller(esClient, dbClient, bp, logger, processor.PollerConfig{
		BatchSize:    100,
		PollInterval: 1 * time.Minute,
	}, nil)

	ctx := context.Background()
	err := poller.ProcessPendingExported(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify classified content inherited SourceIndex
	for _, classified := range esClient.classifiedContent {
		if classified.SourceIndex != "example_com_raw_content" {
			t.Errorf("classified content %s has SourceIndex=%q, want %q",
				classified.ID, classified.SourceIndex, "example_com_raw_content")
		}
	}
}

func TestProcessPending_MixedCaseSourceName_StillWorks(t *testing.T) {
	t.Helper()

	esClient, dbClient, logger := setupTestEnvironment()

	// Simulate what production sees: human-readable source name + valid raw index
	for _, raw := range esClient.rawContent {
		raw.SourceName = "Campbell River Mirror"
		raw.SourceIndex = "campbell_river_mirror_raw_content"
	}

	c := createTestClassifier(logger)
	bp := processor.NewBatchProcessor(c, logger, 10)
	poller := processor.NewPoller(esClient, dbClient, bp, logger, processor.PollerConfig{
		BatchSize:    100,
		PollInterval: 1 * time.Minute,
	}, nil)

	ctx := context.Background()
	err := poller.ProcessPendingExported(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(esClient.classifiedContent) == 0 {
		t.Fatal("expected classified content, got none")
	}

	// SourceIndex should propagate through to classified content
	for _, classified := range esClient.classifiedContent {
		if classified.SourceIndex != "campbell_river_mirror_raw_content" {
			t.Errorf("SourceIndex=%q, want %q", classified.SourceIndex, "campbell_river_mirror_raw_content")
		}
	}
}
```

**Step 2: Export `processPending` for testing**

The `processPending` method is unexported. Add a thin exported wrapper in `classifier/internal/processor/poller.go` for testing:

```go
// ProcessPendingExported exposes processPending for integration testing.
func (p *Poller) ProcessPendingExported(ctx context.Context) error {
	return p.processPending(ctx)
}
```

**Step 3: Run regression tests**

Run: `cd classifier && go test ./internal/processor/ -run "TestProcessPending_SourceIndex|TestProcessPending_MixedCase" -v`
Expected: PASS

Run: `cd classifier && go test ./internal/... -v -count=1`
Expected: PASS (all existing tests still pass)

**Step 4: Commit**

```
test(classifier): add regression tests for SourceIndex propagation and mixed-case names

Verifies that:
- SourceIndex from RawContent propagates through to ClassifiedContent
- Human-readable source names like "Campbell River Mirror" don't break
  the pipeline when SourceIndex is correctly populated
```

---

### Task 7: Lint, full test suite, and final commit

**Files:** None (validation only)

**Step 1: Run linter**

Run: `cd classifier && golangci-lint run`
Expected: PASS (no new violations)

**Step 2: Run full test suite with coverage**

Run: `cd classifier && go test ./... -v -count=1 -coverprofile=coverage.out`
Expected: PASS

**Step 3: Verify no regressions**

Run: `task test:classifier`
Expected: PASS

Run: `task lint:classifier`
Expected: PASS

---

### Task 8: Data recovery — reset stalled content on production

**Note:** This task runs on production after deploying the fix. Do NOT run during development.

**Step 1: Check scope of affected content**

```bash
# SSH to production
ssh jones@northcloud.one

# Count raw content marked classified but with no classified counterpart
# (items from indexes where source_name has spaces/uppercase)
docker run --rm --network=north-cloud_north-cloud-network \
  curlimages/curl:8.1.2 -s -X POST \
  'http://elasticsearch:9200/*_raw_content/_search' \
  -H 'Content-Type: application/json' \
  -d '{"size":0,"query":{"bool":{"must":[{"term":{"classification_status":"classified"}},{"range":{"crawled_at":{"gte":"2026-02-24T00:00:00Z"}}}]}}}'
```

**Step 2: Reset classification_status to pending**

```bash
docker run --rm --network=north-cloud_north-cloud-network \
  curlimages/curl:8.1.2 -s -X POST \
  'http://elasticsearch:9200/*_raw_content/_update_by_query?conflicts=proceed' \
  -H 'Content-Type: application/json' \
  -d '{
    "query": {
      "bool": {
        "must": [
          {"term": {"classification_status": "classified"}},
          {"range": {"crawled_at": {"gte": "2026-02-24T00:00:00Z"}}}
        ]
      }
    },
    "script": {
      "source": "ctx._source.classification_status = \"pending\""
    }
  }'
```

**Step 3: Verify the classifier starts processing**

Watch classifier logs for new classification activity:

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs -f classifier 2>/dev/null \
  | grep -E "Found pending|Indexing classified|Successfully indexed"
```

Expected: Classifier should start finding pending content and indexing it to correctly-named classified indexes.

**Step 4: Verify publisher starts publishing**

Watch publisher logs for routing activity:

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs -f publisher 2>/dev/null \
  | grep -E "Processing content|Published content|Batch complete"
```

Expected: Publisher should start picking up newly classified articles and publishing them.
