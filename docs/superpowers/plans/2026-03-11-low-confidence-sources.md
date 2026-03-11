# Low-Confidence Sources Investigation — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Investigate and resolve chronically low-confidence sources flagged by the AI Observer, adding disable-with-reason metadata to source-manager and building a reusable source diagnostic CLI tool.

**Architecture:** Two independent workstreams — (1) source-manager migration + model changes for disable metadata, (2) standalone diagnostic CLI tool in `tools/source-diagnose/` that queries ES directly. After tooling is ready, investigate the 3 kept sources and disable the 4 out-of-scope sources.

**Tech Stack:** Go 1.26+, PostgreSQL (migrations), Elasticsearch (diagnostic queries), goquery (live page comparison), testify (assertions)

**Spec:** `docs/superpowers/specs/2026-03-11-low-confidence-sources-design.md`

---

## Chunk 1: Source-Manager Disable Metadata

### Task 1: Migration — Add disable fields

**Files:**
- Create: `source-manager/migrations/015_add_disable_fields.up.sql`
- Create: `source-manager/migrations/015_add_disable_fields.down.sql`

- [ ] **Step 1: Write up migration**

```sql
ALTER TABLE sources ADD COLUMN disabled_at TIMESTAMPTZ;
ALTER TABLE sources ADD COLUMN disable_reason TEXT;
```

- [ ] **Step 2: Write down migration**

```sql
ALTER TABLE sources DROP COLUMN disabled_at;
ALTER TABLE sources DROP COLUMN disable_reason;
```

- [ ] **Step 3: Commit**

```bash
git add source-manager/migrations/015_add_disable_fields.*.sql
git commit -m "feat(source-manager): add disable_reason migration (#311)"
```

---

### Task 2: Update Source model

**Files:**
- Modify: `source-manager/internal/models/source.go`
- Modify: `source-manager/internal/models/source_test.go`

- [ ] **Step 1: Write failing test for IsDisabled helper**

Add to `source-manager/internal/models/source_test.go`:

```go
func TestSource_IsDisabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{
			name:    "enabled source is not disabled",
			enabled: true,
			want:    false,
		},
		{
			name:    "disabled source is disabled",
			enabled: false,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Source{Enabled: tt.enabled}
			assert.Equal(t, tt.want, s.IsDisabled())
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/models/ -run TestSource_IsDisabled -v`
Expected: FAIL — `s.IsDisabled undefined`

- [ ] **Step 3: Add fields and helper to Source struct**

In `source-manager/internal/models/source.go`, add fields to the `Source` struct after the existing `FeedDisableReason` field:

```go
DisabledAt    *time.Time `json:"disabled_at" db:"disabled_at"`
DisableReason *string    `json:"disable_reason" db:"disable_reason"`
```

Add the helper method:

```go
// IsDisabled returns true when the source is not enabled.
func (s Source) IsDisabled() bool {
	return !s.Enabled
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/models/ -run TestSource_IsDisabled -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/models/source.go source-manager/internal/models/source_test.go
git commit -m "feat(source-manager): add DisabledAt, DisableReason fields and IsDisabled helper (#311)"
```

---

### Task 3: Add dedicated DisableSource/EnableSource repository methods

**Files:**
- Modify: `source-manager/internal/repository/source.go`

- [ ] **Step 1: Read the repository source.go file**

Read `source-manager/internal/repository/source.go` to understand the existing `DisableFeed`/`EnableFeed` pattern (lines 625-669).

- [ ] **Step 2: Add DisableSource repository method**

Add after the `EnableFeed` method, following the same pattern:

```go
// DisableSource marks a source as disabled with a reason.
func (r *SourceRepository) DisableSource(ctx context.Context, id, reason string) error {
	query := `
		UPDATE sources
		SET enabled = false, disabled_at = NOW(), disable_reason = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, reason)
	if err != nil {
		return fmt.Errorf("disable source: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("source not found")
	}
	return nil
}
```

- [ ] **Step 3: Add EnableSource repository method**

```go
// EnableSource clears a source's disabled state and re-enables it.
func (r *SourceRepository) EnableSource(ctx context.Context, id string) error {
	query := `
		UPDATE sources
		SET enabled = true, disabled_at = NULL, disable_reason = NULL, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("enable source: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return errors.New("source not found")
	}
	return nil
}
```

- [ ] **Step 4: Add disabled_at and disable_reason to scanSourceRow**

In `scanSourceRow` (line ~285), add `&source.DisabledAt` and `&source.DisableReason` to the `Scan()` call after `&source.UpdatedAt`. Also add the corresponding columns to all SELECT column lists that feed into `scanSourceRow` — search for `created_at, updated_at` in the file and append `, disabled_at, disable_reason` after each occurrence.

- [ ] **Step 5: Run existing repository tests**

Run: `cd source-manager && go test ./internal/repository/ -v`
Expected: PASS (existing tests still work with new nullable fields)

- [ ] **Step 6: Commit**

```bash
git add source-manager/internal/repository/source.go
git commit -m "feat(source-manager): add DisableSource/EnableSource repository methods (#311)"
```

---

### Task 4: Add disable/enable API endpoints

**Files:**
- Modify: `source-manager/internal/handlers/source.go`
- Modify: `source-manager/internal/api/router.go`

- [ ] **Step 1: Read the existing DisableFeed/EnableFeed handlers**

Read `source-manager/internal/handlers/source.go` lines 462-518 for the `DisableFeed` and `EnableFeed` handler pattern.

- [ ] **Step 2: Add DisableSource handler**

Follow the `DisableFeed` pattern exactly. Add after `EnableFeed`:

```go
// SourceDisableRequest is the request body for disabling a source.
type SourceDisableRequest struct {
	Reason string `binding:"required" json:"reason"`
}

// DisableSource marks a source as disabled with a reason.
func (h *SourceHandler) DisableSource(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source ID"})
		return
	}

	var req SourceDisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.repo.DisableSource(c.Request.Context(), id, req.Reason); err != nil {
		h.logger.Error("Failed to disable source",
			infralogger.String("source_id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable source"})
		return
	}

	h.logger.Info("Source disabled",
		infralogger.String("source_id", id),
		infralogger.String("reason", req.Reason),
	)

	c.JSON(http.StatusOK, gin.H{"message": "Source disabled", "source_id": id, "reason": req.Reason})
}
```

- [ ] **Step 3: Add EnableSource handler**

```go
// EnableSource clears a source's disabled state and re-enables it.
func (h *SourceHandler) EnableSource(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source ID"})
		return
	}

	if err := h.repo.EnableSource(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to enable source",
			infralogger.String("source_id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable source"})
		return
	}

	h.logger.Info("Source enabled", infralogger.String("source_id", id))

	c.JSON(http.StatusOK, gin.H{"message": "Source enabled", "source_id": id})
}
```

- [ ] **Step 4: Register routes in router.go**

In `source-manager/internal/api/router.go` (line ~167), add after the `feed-enable` route:
```go
sources.PATCH("/:id/disable", sourceHandler.DisableSource)
sources.PATCH("/:id/enable", sourceHandler.EnableSource)
```

- [ ] **Step 5: Write handler tests**

Add to the handler test file. Follow the existing test pattern for DisableFeed:

```go
func TestDisableSource(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	// Create a source first
	source := createTestSource(t, router)

	body := `{"reason": "out_of_scope_tech_entertainment"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/sources/"+source.ID+"/disable", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testToken)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Source disabled", resp["message"])
	assert.Equal(t, "out_of_scope_tech_entertainment", resp["reason"])
}

func TestEnableSource(t *testing.T) {
	router, _, cleanup := setupTestRouter(t)
	defer cleanup()

	source := createTestSource(t, router)

	// Disable first, then enable
	disableBody := `{"reason": "test"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/sources/"+source.ID+"/disable", strings.NewReader(disableBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testToken)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Now enable
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("PATCH", "/api/v1/sources/"+source.ID+"/enable", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
```

Note: Adapt test helpers (`setupTestRouter`, `createTestSource`, `testToken`) to match the existing test infrastructure. Read the handler test file first.

- [ ] **Step 6: Run handler tests**

Run: `cd source-manager && go test ./internal/handlers/ -v`
Expected: PASS

- [ ] **Step 7: Run linter**

Run: `cd source-manager && golangci-lint run`
Expected: No errors

- [ ] **Step 8: Commit**

```bash
git add source-manager/internal/handlers/source.go source-manager/internal/api/router.go
git commit -m "feat(source-manager): add disable/enable source API endpoints (#311)"
```

---

## Chunk 2: Diagnostic CLI Tool

### Task 5: Scaffold the diagnostic tool

**Files:**
- Create: `tools/source-diagnose/go.mod`
- Create: `tools/source-diagnose/main.go`

- [ ] **Step 1: Create directory and initialize Go module**

```bash
mkdir -p tools/source-diagnose
cd tools/source-diagnose
go mod init github.com/jonesrussell/north-cloud/tools/source-diagnose
```

- [ ] **Step 2: Write main.go with flag parsing**

```go
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	source := flag.String("source", "", "Source name to diagnose (required)")
	limit := flag.Int("limit", 10, "Number of recent documents to sample")
	format := flag.String("format", "table", "Output format: table or json")
	compareLive := flag.Bool("compare-live", false, "Fetch live pages and compare word counts")
	esURL := flag.String("es-url", "http://localhost:9200", "Elasticsearch URL")

	flag.Parse()

	if *source == "" {
		fmt.Fprintln(os.Stderr, "error: --source is required")
		flag.Usage()
		os.Exit(1)
	}

	cfg := Config{
		Source:      *source,
		Limit:       *limit,
		Format:      *format,
		CompareLive: *compareLive,
		ESURL:       *esURL,
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Config holds CLI configuration.
type Config struct {
	Source      string
	Limit      int
	Format     string
	CompareLive bool
	ESURL      string
}

func run(cfg Config) error {
	// TODO: implement in subsequent tasks
	fmt.Printf("Diagnosing source: %s (limit: %d, format: %s, compare-live: %v)\n",
		cfg.Source, cfg.Limit, cfg.Format, cfg.CompareLive)
	return nil
}
```

- [ ] **Step 3: Verify it compiles and runs**

Run: `cd tools/source-diagnose && go build -o /dev/null . && go run . --source "test"`
Expected: `Diagnosing source: test (limit: 10, format: table, compare-live: false)`

- [ ] **Step 4: Add to go.work**

Add `./tools/source-diagnose` to the workspace `use` block in `go.work`.

- [ ] **Step 5: Commit**

```bash
git add tools/source-diagnose/go.mod tools/source-diagnose/main.go go.work
git commit -m "feat(tools): scaffold source-diagnose CLI (#311)"
```

---

### Task 6: ES query module

**Files:**
- Create: `tools/source-diagnose/es.go`
- Create: `tools/source-diagnose/es_test.go`

- [ ] **Step 1: Write test for ES query building**

```go
package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSourceQuery(t *testing.T) {
	query, err := buildSourceQuery("Battlefords News-Optimist", 10)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal([]byte(query), &parsed)
	require.NoError(t, err)

	// Verify it has a bool filter on source_name.keyword
	boolQuery, ok := parsed["query"].(map[string]any)["bool"].(map[string]any)
	require.True(t, ok, "expected bool query")

	filter, ok := boolQuery["filter"].([]any)
	require.True(t, ok, "expected filter array")
	assert.Len(t, filter, 1)

	// Verify size
	size, ok := parsed["size"].(float64)
	require.True(t, ok)
	assert.Equal(t, float64(10), size)

	// Verify sort by date descending
	_, ok = parsed["sort"]
	assert.True(t, ok, "expected sort clause")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tools/source-diagnose && go test -run TestBuildSourceQuery -v`
Expected: FAIL — `buildSourceQuery undefined`

- [ ] **Step 3: Implement es.go**

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Document represents a classified content document from ES.
type Document struct {
	SourceName  string    `json:"source_name"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Content     string    `json:"content"`
	WordCount   int       `json:"word_count"`
	Confidence  float64   `json:"confidence"`
	Quality     int       `json:"quality_score"`
	ContentType string    `json:"content_type"`
	PublishedAt time.Time `json:"published_at"`
}

func buildSourceQuery(sourceName string, limit int) (string, error) {
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []map[string]any{
					{
						"term": map[string]any{
							"source_name.keyword": sourceName,
						},
					},
				},
			},
		},
		"size": limit,
		"sort": []map[string]any{
			{"indexed_at": map[string]any{"order": "desc"}},
		},
		"_source": []string{
			"source_name", "title", "url", "content",
			"quality_score", "confidence", "content_type", "published_at",
		},
	}

	b, err := json.Marshal(query)
	if err != nil {
		return "", fmt.Errorf("marshaling query: %w", err)
	}
	return string(b), nil
}

const httpTimeoutSeconds = 10

// fetchDocuments queries ES for recent documents from a source.
func fetchDocuments(ctx context.Context, esURL, sourceName string, limit int) ([]Document, error) {
	query, err := buildSourceQuery(sourceName, limit)
	if err != nil {
		return nil, fmt.Errorf("building query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		esURL+"/*_classified_content/_search",
		bytes.NewBufferString(query))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: httpTimeoutSeconds * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying ES: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ES returned %d: %s", resp.StatusCode, string(body))
	}

	return parseESResponse(body)
}

func parseESResponse(body []byte) ([]Document, error) {
	var result struct {
		Hits struct {
			Hits []struct {
				Source Document `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing ES response: %w", err)
	}

	docs := make([]Document, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		doc := hit.Source
		doc.WordCount = countWords(doc.Content)
		docs = append(docs, doc)
	}

	return docs, nil
}

func countWords(s string) int {
	if s == "" {
		return 0
	}
	count := 0
	inWord := false
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd tools/source-diagnose && go test -run TestBuildSourceQuery -v`
Expected: PASS

- [ ] **Step 5: Add test for word counting**

```go
func TestCountWords(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single word", "hello", 1},
		{"multiple words", "hello world foo", 3},
		{"extra whitespace", "  hello   world  ", 2},
		{"newlines", "hello\nworld\nfoo", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, countWords(tt.input))
		})
	}
}
```

- [ ] **Step 6: Run all tests**

Run: `cd tools/source-diagnose && go test ./... -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add tools/source-diagnose/es.go tools/source-diagnose/es_test.go tools/source-diagnose/go.mod tools/source-diagnose/go.sum
git commit -m "feat(tools): add ES query module for source-diagnose (#311)"
```

---

### Task 7: Report module

**Files:**
- Create: `tools/source-diagnose/report.go`
- Create: `tools/source-diagnose/report_test.go`

- [ ] **Step 1: Write test for aggregate stats computation**

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeStats(t *testing.T) {
	docs := []Document{
		{Confidence: 0.55, WordCount: 200, Quality: 60},
		{Confidence: 0.70, WordCount: 500, Quality: 80},
		{Confidence: 0.45, WordCount: 100, Quality: 40},
		{Confidence: 0.65, WordCount: 300, Quality: 70},
	}

	stats := computeStats(docs)

	assert.Equal(t, 4, stats.TotalDocs)
	assert.InDelta(t, 0.5875, stats.AvgConfidence, 0.001)
	assert.InDelta(t, 50.0, stats.BorderlineRate, 0.1) // 2 of 4 below 0.6
	assert.InDelta(t, 275.0, stats.AvgWordCount, 0.1)
	assert.InDelta(t, 62.5, stats.AvgQuality, 0.1)
}

func TestComputeStats_Empty(t *testing.T) {
	stats := computeStats(nil)
	assert.Equal(t, 0, stats.TotalDocs)
	assert.Equal(t, 0.0, stats.AvgConfidence)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tools/source-diagnose && go test -run TestComputeStats -v`
Expected: FAIL — `computeStats undefined`

- [ ] **Step 3: Implement report.go**

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

const borderlineThreshold = 0.6

// Stats holds aggregate statistics for a source's documents.
type Stats struct {
	TotalDocs      int     `json:"total_docs"`
	AvgConfidence  float64 `json:"avg_confidence"`
	BorderlineRate float64 `json:"borderline_rate"`
	AvgWordCount   float64 `json:"avg_word_count"`
	AvgQuality     float64 `json:"avg_quality"`
}

func computeStats(docs []Document) Stats {
	if len(docs) == 0 {
		return Stats{}
	}

	var totalConf float64
	var totalWords int
	var totalQuality int
	var borderline int

	for _, d := range docs {
		totalConf += d.Confidence
		totalWords += d.WordCount
		totalQuality += d.Quality
		if d.Confidence < borderlineThreshold {
			borderline++
		}
	}

	n := float64(len(docs))

	return Stats{
		TotalDocs:      len(docs),
		AvgConfidence:  totalConf / n,
		BorderlineRate: float64(borderline) / n * 100,
		AvgWordCount:   float64(totalWords) / n,
		AvgQuality:     float64(totalQuality) / n,
	}
}

// Report holds the full diagnostic report.
type Report struct {
	Source string     `json:"source"`
	Stats  Stats      `json:"stats"`
	Docs   []Document `json:"documents"`
}

func writeTable(w io.Writer, report Report) {
	fmt.Fprintf(w, "\n=== Source Diagnosis: %s ===\n\n", report.Source)

	fmt.Fprintf(w, "Summary:\n")
	fmt.Fprintf(w, "  Documents sampled:  %d\n", report.Stats.TotalDocs)
	fmt.Fprintf(w, "  Avg confidence:     %.3f\n", report.Stats.AvgConfidence)
	fmt.Fprintf(w, "  Borderline rate:    %.1f%%\n", report.Stats.BorderlineRate)
	fmt.Fprintf(w, "  Avg word count:     %.0f\n", report.Stats.AvgWordCount)
	fmt.Fprintf(w, "  Avg quality score:  %.0f\n\n", report.Stats.AvgQuality)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TITLE\tWORDS\tCONF\tQUALITY\tTYPE\tDATE")
	fmt.Fprintln(tw, "-----\t-----\t----\t-------\t----\t----")

	for _, d := range report.Docs {
		title := d.Title
		const maxTitleLen = 50
		if len(title) > maxTitleLen {
			title = title[:maxTitleLen-3] + "..."
		}
		fmt.Fprintf(tw, "%s\t%d\t%.3f\t%d\t%s\t%s\n",
			title, d.WordCount, d.Confidence, d.Quality,
			d.ContentType, d.PublishedAt.Format("2006-01-02"))
	}

	tw.Flush()
}

func writeJSON(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd tools/source-diagnose && go test -run TestComputeStats -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/source-diagnose/report.go tools/source-diagnose/report_test.go
git commit -m "feat(tools): add report module for source-diagnose (#311)"
```

---

### Task 8: Live comparison module

**Files:**
- Create: `tools/source-diagnose/compare.go`
- Create: `tools/source-diagnose/compare_test.go`

- [ ] **Step 1: Write test for extraction loss calculation**

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractionLoss(t *testing.T) {
	tests := []struct {
		name      string
		extracted int
		live      int
		wantLoss  float64
	}{
		{"no loss", 500, 500, 0.0},
		{"50% loss", 250, 500, 50.0},
		{"100% loss", 0, 500, 100.0},
		{"live is zero", 100, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loss := extractionLoss(tt.extracted, tt.live)
			assert.InDelta(t, tt.wantLoss, loss, 0.1)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tools/source-diagnose && go test -run TestExtractionLoss -v`
Expected: FAIL — `extractionLoss undefined`

- [ ] **Step 3: Implement compare.go**

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const fetchTimeoutSeconds = 15

// Comparison holds the result of comparing extracted vs live content.
type Comparison struct {
	URL            string  `json:"url"`
	ExtractedWords int     `json:"extracted_words"`
	LiveWords      int     `json:"live_words"`
	LossPercent    float64 `json:"loss_percent"`
}

func extractionLoss(extracted, live int) float64 {
	if live == 0 {
		return 0
	}
	return float64(live-extracted) / float64(live) * 100
}

// compareLive fetches a URL and counts words in the page body text.
func compareLive(ctx context.Context, doc Document) (Comparison, error) {
	client := &http.Client{Timeout: fetchTimeoutSeconds * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, doc.URL, nil)
	if err != nil {
		return Comparison{}, fmt.Errorf("creating request for %s: %w", doc.URL, err)
	}
	req.Header.Set("User-Agent", "NorthCloud-Diagnose/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return Comparison{}, fmt.Errorf("fetching %s: %w", doc.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Comparison{}, fmt.Errorf("fetching %s: HTTP %d", doc.URL, resp.StatusCode)
	}

	gDoc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Comparison{}, fmt.Errorf("parsing %s: %w", doc.URL, err)
	}

	// Remove script and style elements before extracting text
	gDoc.Find("script, style, nav, header, footer").Remove()
	bodyText := strings.TrimSpace(gDoc.Find("body").Text())
	liveWords := countWords(bodyText)

	return Comparison{
		URL:            doc.URL,
		ExtractedWords: doc.WordCount,
		LiveWords:      liveWords,
		LossPercent:    extractionLoss(doc.WordCount, liveWords),
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd tools/source-diagnose && go test -run TestExtractionLoss -v`
Expected: PASS

- [ ] **Step 5: Add goquery dependency**

Run: `cd tools/source-diagnose && go get github.com/PuerkitoBio/goquery`

- [ ] **Step 6: Commit**

```bash
git add tools/source-diagnose/compare.go tools/source-diagnose/compare_test.go tools/source-diagnose/go.mod tools/source-diagnose/go.sum
git commit -m "feat(tools): add live comparison module for source-diagnose (#311)"
```

---

### Task 9: Wire up main.go

**Files:**
- Modify: `tools/source-diagnose/main.go`

- [ ] **Step 1: Implement the run function**

Replace the `run` function in `main.go`:

```go
func run(cfg Config) error {
	ctx := context.Background()

	docs, err := fetchDocuments(ctx, cfg.ESURL, cfg.Source, cfg.Limit)
	if err != nil {
		return fmt.Errorf("fetching documents: %w", err)
	}

	if len(docs) == 0 {
		fmt.Printf("No documents found for source: %s\n", cfg.Source)
		return nil
	}

	stats := computeStats(docs)
	report := Report{
		Source: cfg.Source,
		Stats:  stats,
		Docs:   docs,
	}

	switch cfg.Format {
	case "json":
		if err := writeJSON(os.Stdout, report); err != nil {
			return fmt.Errorf("writing JSON output: %w", err)
		}
	default:
		writeTable(os.Stdout, report)
	}

	if cfg.CompareLive {
		fmt.Println("\n=== Live Comparison ===\n")
		for _, doc := range docs {
			comp, compErr := compareLive(ctx, doc)
			if compErr != nil {
				fmt.Fprintf(os.Stderr, "  SKIP %s: %v\n", doc.URL, compErr)
				continue
			}
			fmt.Printf("  %s\n    Extracted: %d words | Live: %d words | Loss: %.1f%%\n",
				comp.URL, comp.ExtractedWords, comp.LiveWords, comp.LossPercent)
		}
	}

	return nil
}
```

Update imports to include `"context"` and `"os"`.

- [ ] **Step 2: Verify it compiles**

Run: `cd tools/source-diagnose && go build -o /dev/null .`
Expected: Build succeeds

- [ ] **Step 3: Run linter**

Run: `cd tools/source-diagnose && golangci-lint run`
Expected: No errors (if golangci-lint config exists in repo root, it applies)

- [ ] **Step 4: Commit**

```bash
git add tools/source-diagnose/main.go
git commit -m "feat(tools): wire up source-diagnose CLI (#311)"
```

---

## Chunk 3: Investigation and Source Disposition

### Task 10: Disable out-of-scope sources

**Depends on:** Tasks 1-4 (source-manager changes deployed or testable locally)

- [ ] **Step 1: Run the migration locally**

Run: `task migrate:source-manager` (or manually apply migration 015)

- [ ] **Step 2: Disable the 4 sources via API**

Use the new disable endpoint for each source:

```bash
# Find source IDs first
curl -s http://localhost:8050/api/v1/sources | jq '.[] | select(.name | test("CNET|9to5Mac|Consequence|Work Remotely")) | {id, name}'

# Then disable each one (replace $ID and $TOKEN)
curl -X PATCH http://localhost:8050/api/v1/sources/$ID/disable \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "out_of_scope_tech_entertainment"}'

# For We Work Remotely:
curl -X PATCH http://localhost:8050/api/v1/sources/$ID/disable \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "wrong_content_type_job_board"}'
```

- [ ] **Step 3: Verify disabled sources are marked as disabled**

```bash
curl -s http://localhost:8050/api/v1/sources | jq '.[] | select(.name | test("CNET|9to5Mac|Consequence|Work Remotely")) | {name, enabled, disable_reason}'
```

Expected: All 4 sources show `"enabled": false` with their respective `disable_reason` values.

- [ ] **Step 4: Document the results**

Add a comment to issue #311 listing the disabled sources with their IDs and reasons.

---

### Task 11: Investigate kept sources with diagnostic tool

**Depends on:** Tasks 5-9 (diagnostic tool built)

- [ ] **Step 1: Run diagnostic on Battlefords News-Optimist**

```bash
go run tools/source-diagnose/main.go \
  --source "Battlefords News-Optimist" \
  --es-url "http://localhost:9200" \
  --compare-live \
  --limit 15
```

Capture the output. Identify: avg confidence, borderline rate, word count, extraction loss.

- [ ] **Step 2: Run diagnostic on Western Standard**

```bash
go run tools/source-diagnose/main.go \
  --source "Western Standard" \
  --es-url "http://localhost:9200" \
  --compare-live \
  --limit 15
```

- [ ] **Step 3: Run diagnostic on Waatea News**

```bash
go run tools/source-diagnose/main.go \
  --source "Waatea News" \
  --es-url "http://localhost:9200" \
  --compare-live \
  --limit 15
```

- [ ] **Step 4: Analyze root causes and fix**

Based on diagnostic output, for each source:
- If extraction loss > 30%: update CSS selectors in source-manager
- If missing metadata: check crawler extraction for title/date/description
- If content is fine but confidence is low: document as classifier training gap

Fix any selector issues found, then re-run diagnostic to confirm improvement.

- [ ] **Step 5: Document findings on issue #311**

Comment on issue #311 with:
- Diagnostic output for each source (before/after if fixes applied)
- Root cause per source
- Actions taken
- Updated borderline rates

---

### Task 12: Final verification and cleanup

- [ ] **Step 1: Run full source-manager test suite**

Run: `cd source-manager && go test ./... -v`
Expected: PASS

- [ ] **Step 2: Run full linter**

Run: `task lint:source-manager`
Expected: No errors

- [ ] **Step 3: Run diagnostic tool tests**

Run: `cd tools/source-diagnose && go test ./... -v`
Expected: PASS

- [ ] **Step 4: Verify acceptance criteria**

From issue #311:
- [ ] Each flagged source investigated with root cause identified
- [ ] Sources with extraction issues have crawler/selector fixes
- [ ] Inappropriate sources disabled with reason metadata
- [ ] Average borderline rate drops below 40% for remaining sources

- [ ] **Step 5: Close issue #311 with summary comment**

Comment on issue #311 summarizing all actions taken, then close.
