# OPD Ingestion Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Import the Ojibwe People's Dictionary (21,358 entries) into source-manager's dictionary_entries table and optionally project to an `opd_dictionary` Elasticsearch index.

**Architecture:** Source-manager subcommand (`import-opd`) reads JSONL, validates, transforms, and bulk-upserts to PostgreSQL. A separate projection step syncs consented entries to ES. No new service — extends source-manager with CLI routing via `os.Args[1]`.

**Tech Stack:** Go 1.26+, PostgreSQL (dictionary_entries table — already exists via migration 017), Elasticsearch 8.x, Gin (existing), infrastructure packages.

**Spec:** `docs/superpowers/specs/2026-03-18-opd-ingestion-design.md`

---

## Existing Infrastructure (Already Done)

These already exist and DO NOT need to be created:
- `source-manager/migrations/017_create_dictionary_entries_table.up.sql` — table schema
- `source-manager/internal/models/dictionary_entry.go` — `DictionaryEntry` struct
- `source-manager/internal/repository/dictionary.go` — CRUD + `UpsertByContentHash`
- `source-manager/internal/handlers/dictionary.go` — HTTP handlers for `/dictionary/*`
- `source-manager/internal/bootstrap/` — `LoadConfig()`, `CreateLogger()`, `SetupDatabase()`

## File Structure — New/Modified

| File | Responsibility |
|------|----------------|
| `source-manager/migrations/019_unique_content_hash.up.sql` | Make content_hash index UNIQUE for upsert |
| `source-manager/migrations/019_unique_content_hash.down.sql` | Revert to non-unique index |
| `source-manager/main.go` | (modify) Add `os.Args[1]` subcommand routing |
| `source-manager/cmd_import_opd.go` | CLI subcommand: parse flags, bootstrap DB, run importer |
| `source-manager/internal/importer/opd.go` | JSONL reader, validator, transformer |
| `source-manager/internal/importer/opd_test.go` | Unit tests for importer |
| `source-manager/internal/importer/testdata/valid_entries.jsonl` | Fixture: 3 valid OPD entries |
| `source-manager/internal/importer/testdata/mixed_entries.jsonl` | Fixture: valid + invalid entries |
| `source-manager/internal/repository/dictionary.go` | (modify) Add `BulkUpsertEntries()` |
| `source-manager/internal/projection/dictionary_es.go` | DB -> ES projection (consent-filtered) |
| `source-manager/internal/projection/dictionary_es_test.go` | Unit tests for projection |

## Key API References

```go
// Bootstrap helpers (reuse in CLI subcommand)
bootstrap.LoadConfig() (*config.Config, error)
bootstrap.CreateLogger(cfg *config.Config, version string) (infralogger.Logger, error)
bootstrap.SetupDatabase(cfg *config.Config, log infralogger.Logger) (*database.DB, error)

// Database wrapper
db.DB() *sql.DB   // unwrap to *sql.DB for repository
db.Close() error

// Repository constructor
repository.NewDictionaryRepository(db *sql.DB, log infralogger.Logger) *DictionaryRepository
```

---

### Task 1: Add Unique Content Hash Migration

**Files:**
- Create: `source-manager/migrations/019_unique_content_hash.up.sql`
- Create: `source-manager/migrations/019_unique_content_hash.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- source-manager/migrations/019_unique_content_hash.up.sql
DROP INDEX IF EXISTS idx_dictionary_entries_hash;
CREATE UNIQUE INDEX idx_dictionary_entries_hash ON dictionary_entries(content_hash);
```

- [ ] **Step 2: Write down migration**

```sql
-- source-manager/migrations/019_unique_content_hash.down.sql
DROP INDEX IF EXISTS idx_dictionary_entries_hash;
CREATE INDEX idx_dictionary_entries_hash ON dictionary_entries(content_hash);
```

- [ ] **Step 3: Verify no duplicate prefix**

Run: `ls source-manager/migrations/ | cut -d_ -f1 | sort | uniq -d`
Expected: No output (no duplicates).

- [ ] **Step 4: Commit**

```bash
git add source-manager/migrations/019_*
git commit -m "feat(source-manager): make dictionary content_hash index unique for upsert"
```

---

### Task 2: Add BulkUpsertEntries to Dictionary Repository

**Files:**
- Modify: `source-manager/internal/repository/dictionary.go`

- [ ] **Step 1: Write the failing test**

```go
// source-manager/internal/repository/dictionary_bulk_test.go
package repository_test

import (
    "context"
    "testing"

    "github.com/jonesrussell/north-cloud/source-manager/internal/models"
    "github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

func TestBulkUpsertEntries_EmptySlice(t *testing.T) {
    // Verify BulkUpsertEntries handles empty input without error.
    // Uses nil DB — should return immediately before touching DB.
    repo := repository.NewDictionaryRepository(nil, nil)
    inserted, updated, err := repo.BulkUpsertEntries(context.Background(), nil)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if inserted != 0 || updated != 0 {
        t.Errorf("expected 0/0, got %d/%d", inserted, updated)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd source-manager && GOWORK=off go test ./internal/repository/... -run TestBulkUpsertEntries -v`
Expected: FAIL — method does not exist.

- [ ] **Step 3: Implement BulkUpsertEntries**

Add to `source-manager/internal/repository/dictionary.go`:

```go
// BulkUpsertEntries inserts or updates multiple dictionary entries in a single transaction.
// Returns count of inserted and updated entries. Uses content_hash for conflict detection.
func (r *DictionaryRepository) BulkUpsertEntries(ctx context.Context, entries []models.DictionaryEntry) (inserted, updated int, err error) {
    if len(entries) == 0 {
        return 0, 0, nil
    }

    tx, txErr := r.db.BeginTx(ctx, nil)
    if txErr != nil {
        return 0, 0, fmt.Errorf("begin transaction: %w", txErr)
    }
    defer func() {
        if err != nil {
            _ = tx.Rollback()
        }
    }()

    const upsertSQL = `
        INSERT INTO dictionary_entries (
            id, lemma, word_class, word_class_normalized, definitions,
            inflections, examples, word_family, media, attribution,
            license, consent_public_display, consent_ai_training,
            consent_derivative_works, content_hash, source_url,
            created_at, updated_at
        ) VALUES (
            gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9,
            $10, $11, $12, $13, $14, $15, NOW(), NOW()
        )
        ON CONFLICT (content_hash) DO UPDATE SET
            lemma = EXCLUDED.lemma,
            definitions = EXCLUDED.definitions,
            inflections = EXCLUDED.inflections,
            examples = EXCLUDED.examples,
            word_family = EXCLUDED.word_family,
            media = EXCLUDED.media,
            updated_at = NOW()
        RETURNING (xmax = 0) AS is_insert`

    for _, entry := range entries {
        var isInsert bool
        scanErr := tx.QueryRowContext(ctx, upsertSQL,
            entry.Lemma, entry.WordClass, entry.WordClassNormalized,
            entry.Definitions, entry.Inflections, entry.Examples,
            entry.WordFamily, entry.Media, entry.Attribution,
            entry.License, entry.ConsentPublicDisplay, entry.ConsentAITraining,
            entry.ConsentDerivativeWorks, entry.ContentHash, entry.SourceURL,
        ).Scan(&isInsert)
        if scanErr != nil {
            return inserted, updated, fmt.Errorf("upsert entry %q: %w", entry.Lemma, scanErr)
        }
        if isInsert {
            inserted++
        } else {
            updated++
        }
    }

    if commitErr := tx.Commit(); commitErr != nil {
        return 0, 0, fmt.Errorf("commit transaction: %w", commitErr)
    }

    return inserted, updated, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd source-manager && GOWORK=off go test ./internal/repository/... -run TestBulkUpsertEntries -v`
Expected: PASS

- [ ] **Step 5: Lint**

Run: `cd source-manager && GOWORK=off golangci-lint run ./internal/repository/...`
Expected: 0 issues

- [ ] **Step 6: Commit**

```bash
git add source-manager/internal/repository/dictionary.go source-manager/internal/repository/dictionary_bulk_test.go
git commit -m "feat(source-manager): add BulkUpsertEntries to dictionary repository"
```

---

### Task 3: Create OPD Importer Package

**Files:**
- Create: `source-manager/internal/importer/opd.go`
- Create: `source-manager/internal/importer/opd_test.go`
- Create: `source-manager/internal/importer/testdata/valid_entries.jsonl`
- Create: `source-manager/internal/importer/testdata/mixed_entries.jsonl`

- [ ] **Step 1: Create test fixtures**

`valid_entries.jsonl` — 3 valid OPD entries (one line each):

```jsonl
{"lemma":"makwa","word_class":"na","definitions":[{"text":"bear","language":"en"}],"inflections":{"raw":"makwag","forms":["makwag"],"stem":"makw"},"examples":[{"ojibwe":"Nimaamaa gi-waabamaan makwan.","english":"My mother saw a bear."}],"word_family":["makoons"],"media":[],"source_url":"https://ojibwe.lib.umn.edu/main-entry/makwa-na","raw_html":"<div>mock</div>"}
{"lemma":"nibi","word_class":"ni","definitions":[{"text":"water","language":"en"}],"inflections":{},"examples":[],"word_family":[],"media":[],"source_url":"https://ojibwe.lib.umn.edu/main-entry/nibi-ni","raw_html":"<div>mock2</div>"}
{"lemma":"giizhig","word_class":"na","definitions":[{"text":"sky","language":"en"},{"text":"day","language":"en"}],"inflections":{"raw":"giizhigoon","forms":["giizhigoon"],"stem":"giizhig"},"examples":[],"word_family":[],"media":[],"source_url":"https://ojibwe.lib.umn.edu/main-entry/giizhig-na","raw_html":"<div>mock3</div>"}
```

`mixed_entries.jsonl` — valid + invalid entries:

```jsonl
{"lemma":"makwa","word_class":"na","definitions":[{"text":"bear","language":"en"}],"inflections":{},"examples":[],"word_family":[],"media":[],"source_url":"https://ojibwe.lib.umn.edu/main-entry/makwa-na","raw_html":"<div>ok</div>"}
{"this is not valid json
{"definitions":[{"text":"no lemma","language":"en"}],"inflections":{},"examples":[],"word_family":[],"media":[],"source_url":"https://example.com","raw_html":"<div>missing</div>"}
```

- [ ] **Step 2: Write failing tests**

```go
// source-manager/internal/importer/opd_test.go
package importer_test

import (
    "testing"

    "github.com/jonesrussell/north-cloud/source-manager/internal/importer"
)

func TestReadOPDEntries_ValidFile(t *testing.T) {
    entries, failures, err := importer.ReadOPDFile("testdata/valid_entries.jsonl")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(entries) != 3 {
        t.Errorf("expected 3 entries, got %d", len(entries))
    }
    if len(failures) != 0 {
        t.Errorf("expected 0 failures, got %d", len(failures))
    }
    if entries[0].Lemma != "makwa" {
        t.Errorf("expected lemma 'makwa', got %q", entries[0].Lemma)
    }
    if entries[0].ContentHash == nil || *entries[0].ContentHash == "" {
        t.Error("expected content_hash to be set")
    }
}

func TestReadOPDEntries_MixedFile(t *testing.T) {
    entries, failures, err := importer.ReadOPDFile("testdata/mixed_entries.jsonl")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(entries) != 1 {
        t.Errorf("expected 1 valid entry, got %d", len(entries))
    }
    if len(failures) != 2 {
        t.Errorf("expected 2 failures, got %d", len(failures))
    }
}

func TestComputeContentHash_Deterministic(t *testing.T) {
    hash1 := importer.ComputeContentHash(`{"a":1,"b":2}`)
    hash2 := importer.ComputeContentHash(`{"a":1,"b":2}`)
    if hash1 != hash2 {
        t.Errorf("expected deterministic hash, got %q vs %q", hash1, hash2)
    }
    hash3 := importer.ComputeContentHash(`{"a":1,"b":3}`)
    if hash1 == hash3 {
        t.Error("expected different hash for different input")
    }
}

func TestComputeContentHash_Canonical(t *testing.T) {
    // Same data, different key order — should produce same hash
    hash1 := importer.ComputeContentHash(`{"b":2,"a":1}`)
    hash2 := importer.ComputeContentHash(`{"a":1,"b":2}`)
    if hash1 != hash2 {
        t.Errorf("expected canonical hash to normalize key order, got %q vs %q", hash1, hash2)
    }
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd source-manager && GOWORK=off go test ./internal/importer/... -v`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Implement the importer**

```go
// source-manager/internal/importer/opd.go
package importer

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
    "sort"

    "github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

// OPDRawEntry represents a single entry from the OPD JSONL file.
type OPDRawEntry struct {
    Lemma       string          `json:"lemma"`
    WordClass   string          `json:"word_class"`
    Definitions json.RawMessage `json:"definitions"`
    Inflections json.RawMessage `json:"inflections"`
    Examples    json.RawMessage `json:"examples"`
    WordFamily  json.RawMessage `json:"word_family"`
    Media       json.RawMessage `json:"media"`
    SourceURL   string          `json:"source_url"`
    RawHTML     string          `json:"raw_html"`
}

// ImportFailure records a failed entry with its line number and reason.
type ImportFailure struct {
    Line   int    `json:"line"`
    Reason string `json:"reason"`
    Raw    string `json:"raw,omitempty"`
}

const (
    opdAttribution = "Ojibwe People's Dictionary, University of Minnesota"
    opdLicense     = "CC BY-NC-SA 4.0"
)

// ReadOPDFile reads and validates a JSONL file, returning transformed entries and failures.
func ReadOPDFile(path string) ([]models.DictionaryEntry, []ImportFailure, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, nil, fmt.Errorf("open file: %w", err)
    }
    defer f.Close()

    var entries []models.DictionaryEntry
    var failures []ImportFailure

    scanner := bufio.NewScanner(f)
    lineNum := 0

    for scanner.Scan() {
        lineNum++
        line := scanner.Text()

        entry, transformErr := transformEntry(line)
        if transformErr != nil {
            failures = append(failures, ImportFailure{
                Line:   lineNum,
                Reason: transformErr.Error(),
                Raw:    line,
            })
            continue
        }

        entries = append(entries, *entry)
    }

    if scanErr := scanner.Err(); scanErr != nil {
        return entries, failures, fmt.Errorf("read file: %w", scanErr)
    }

    return entries, failures, nil
}

func transformEntry(line string) (*models.DictionaryEntry, error) {
    var raw OPDRawEntry
    if err := json.Unmarshal([]byte(line), &raw); err != nil {
        return nil, fmt.Errorf("invalid JSON: %w", err)
    }

    if raw.Lemma == "" {
        return nil, fmt.Errorf("missing required field: lemma")
    }

    hash := ComputeContentHash(line)
    attribution := opdAttribution
    sourceURL := raw.SourceURL

    var wordClass, wordClassNorm *string
    if raw.WordClass != "" {
        wc := raw.WordClass
        wordClass = &wc
        wordClassNorm = &wc
    }

    return &models.DictionaryEntry{
        Lemma:               raw.Lemma,
        WordClass:           wordClass,
        WordClassNormalized: wordClassNorm,
        Definitions:         string(raw.Definitions),
        Inflections:         string(raw.Inflections),
        Examples:            string(raw.Examples),
        WordFamily:          string(raw.WordFamily),
        Media:               string(raw.Media),
        Attribution:         &attribution,
        License:             opdLicense,
        ContentHash:         &hash,
        SourceURL:           &sourceURL,
    }, nil
}

// ComputeContentHash returns the SHA-256 hex digest of canonical JSON (sorted keys).
func ComputeContentHash(jsonStr string) string {
    canonical := canonicalizeJSON(jsonStr)
    h := sha256.Sum256([]byte(canonical))
    return hex.EncodeToString(h[:])
}

// canonicalizeJSON parses JSON and re-serializes with sorted keys for deterministic hashing.
func canonicalizeJSON(input string) string {
    var data any
    if err := json.Unmarshal([]byte(input), &data); err != nil {
        return input // fallback to raw if unparseable
    }
    sorted := sortKeys(data)
    out, err := json.Marshal(sorted)
    if err != nil {
        return input
    }
    return string(out)
}

// sortKeys recursively sorts map keys for canonical JSON output.
func sortKeys(v any) any {
    switch val := v.(type) {
    case map[string]any:
        sorted := make(map[string]any, len(val))
        keys := make([]string, 0, len(val))
        for k := range val {
            keys = append(keys, k)
        }
        sort.Strings(keys)
        for _, k := range keys {
            sorted[k] = sortKeys(val[k])
        }
        return sorted
    case []any:
        result := make([]any, len(val))
        for i, item := range val {
            result[i] = sortKeys(item)
        }
        return result
    default:
        return v
    }
}
```

Note: `json.Marshal` in Go already sorts map keys alphabetically, so `sortKeys` ensures nested maps are also sorted. The `canonicalizeJSON` function handles the round-trip.

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd source-manager && GOWORK=off go test ./internal/importer/... -v`
Expected: PASS (4 tests)

- [ ] **Step 6: Lint**

Run: `cd source-manager && GOWORK=off golangci-lint run ./internal/importer/...`
Expected: 0 issues

- [ ] **Step 7: Commit**

```bash
git add source-manager/internal/importer/
git commit -m "feat(source-manager): add OPD JSONL importer with validation and canonical hashing"
```

---

### Task 4: Add Subcommand Routing and CLI Entry Point

**Files:**
- Modify: `source-manager/main.go`
- Create: `source-manager/cmd_import_opd.go`

- [ ] **Step 1: Modify main.go to route subcommands**

```go
// source-manager/main.go
package main

import (
    "fmt"
    "os"

    "github.com/jonesrussell/north-cloud/source-manager/internal/bootstrap"
)

func main() {
    if len(os.Args) > 1 && os.Args[1] == "import-opd" {
        if err := runImportOPD(os.Args[2:]); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
        return
    }

    if err := bootstrap.Start(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

- [ ] **Step 2: Create cmd_import_opd.go**

```go
// source-manager/cmd_import_opd.go
package main

import (
    "context"
    "flag"
    "fmt"

    infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
    "github.com/jonesrussell/north-cloud/source-manager/internal/bootstrap"
    "github.com/jonesrussell/north-cloud/source-manager/internal/importer"
    "github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

const (
    defaultBatchSize    = 500
    importCommandVersion = "dev"
)

func runImportOPD(args []string) error {
    fs := flag.NewFlagSet("import-opd", flag.ExitOnError)
    filePath := fs.String("file", "", "Path to OPD JSONL file (required)")
    batchSize := fs.Int("batch-size", defaultBatchSize, "Entries per DB batch")
    dryRun := fs.Bool("dry-run", false, "Validate without writing to DB")

    if parseErr := fs.Parse(args); parseErr != nil {
        return fmt.Errorf("parse flags: %w", parseErr)
    }

    if *filePath == "" {
        fs.Usage()
        return fmt.Errorf("--file is required")
    }

    // Reuse bootstrap helpers for config, logger, DB
    cfg, cfgErr := bootstrap.LoadConfig()
    if cfgErr != nil {
        return fmt.Errorf("load config: %w", cfgErr)
    }

    log, logErr := bootstrap.CreateLogger(cfg, importCommandVersion)
    if logErr != nil {
        return fmt.Errorf("create logger: %w", logErr)
    }
    defer func() { _ = log.Sync() }()

    log.Info("Starting OPD import",
        infralogger.String("file", *filePath),
        infralogger.Int("batch_size", *batchSize),
        infralogger.Bool("dry_run", *dryRun),
    )

    // Read and validate JSONL
    entries, failures, readErr := importer.ReadOPDFile(*filePath)
    if readErr != nil {
        return fmt.Errorf("read file: %w", readErr)
    }

    log.Info("File parsed",
        infralogger.Int("valid_entries", len(entries)),
        infralogger.Int("failures", len(failures)),
    )

    for _, f := range failures {
        log.Warn("Import failure",
            infralogger.Int("line", f.Line),
            infralogger.String("reason", f.Reason),
        )
    }

    if *dryRun {
        fmt.Printf("Dry run complete: %d valid, %d failed\n", len(entries), len(failures))
        return nil
    }

    // Connect to DB
    db, dbErr := bootstrap.SetupDatabase(cfg, log)
    if dbErr != nil {
        return fmt.Errorf("connect to database: %w", dbErr)
    }
    defer func() { _ = db.Close() }()

    dictRepo := repository.NewDictionaryRepository(db.DB(), log)
    ctx := context.Background()

    // Batch upsert
    totalInserted, totalUpdated := 0, 0
    for i := 0; i < len(entries); i += *batchSize {
        end := i + *batchSize
        if end > len(entries) {
            end = len(entries)
        }

        batch := entries[i:end]
        inserted, updated, upsertErr := dictRepo.BulkUpsertEntries(ctx, batch)
        if upsertErr != nil {
            // Retry once per spec
            log.Warn("Batch failed, retrying",
                infralogger.Int("batch_start", i),
                infralogger.Error(upsertErr),
            )
            inserted, updated, upsertErr = dictRepo.BulkUpsertEntries(ctx, batch)
            if upsertErr != nil {
                log.Error("Batch failed after retry, skipping",
                    infralogger.Int("batch_start", i),
                    infralogger.Error(upsertErr),
                )
                continue
            }
        }

        totalInserted += inserted
        totalUpdated += updated
        log.Info("Batch complete",
            infralogger.Int("batch_start", i),
            infralogger.Int("inserted", inserted),
            infralogger.Int("updated", updated),
        )
    }

    fmt.Printf("Import complete: %d inserted, %d updated, %d failed\n",
        totalInserted, totalUpdated, len(failures))

    return nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd source-manager && GOWORK=off go build -o /dev/null .`
Expected: Builds without error.

- [ ] **Step 4: Verify --help works**

Run: `cd source-manager && GOWORK=off go run . import-opd --help`
Expected: Shows flag usage (--file, --batch-size, --dry-run).

- [ ] **Step 5: Verify dry-run with test fixture**

Run: `cd source-manager && GOWORK=off go run . import-opd --file internal/importer/testdata/valid_entries.jsonl --dry-run`
Expected: `Dry run complete: 3 valid, 0 failed`

- [ ] **Step 6: Verify dry-run with mixed fixture**

Run: `cd source-manager && GOWORK=off go run . import-opd --file internal/importer/testdata/mixed_entries.jsonl --dry-run`
Expected: `Dry run complete: 1 valid, 2 failed`

- [ ] **Step 7: Lint**

Run: `cd source-manager && GOWORK=off golangci-lint run .`
Expected: 0 issues

- [ ] **Step 8: Commit**

```bash
git add source-manager/main.go source-manager/cmd_import_opd.go
git commit -m "feat(source-manager): add import-opd CLI subcommand with batch upsert and retry"
```

---

### Task 5: Create ES Projection Package

**Files:**
- Create: `source-manager/internal/projection/dictionary_es.go`
- Create: `source-manager/internal/projection/dictionary_es_test.go`

- [ ] **Step 1: Write failing tests**

```go
// source-manager/internal/projection/dictionary_es_test.go
package projection_test

import (
    "testing"

    "github.com/jonesrussell/north-cloud/source-manager/internal/models"
    "github.com/jonesrussell/north-cloud/source-manager/internal/projection"
)

func TestToESDocument_ConsentTrue(t *testing.T) {
    hash := "abc123"
    entry := models.DictionaryEntry{
        ID:                   "uuid-1",
        Lemma:                "makwa",
        Definitions:          `[{"text":"bear","language":"en"}]`,
        ConsentPublicDisplay: true,
        ContentHash:          &hash,
        License:              "CC BY-NC-SA 4.0",
    }
    doc, skip := projection.ToESDocument(entry)
    if skip {
        t.Error("expected consent=true entry to not be skipped")
    }
    if doc["lemma"] != "makwa" {
        t.Errorf("expected lemma 'makwa', got %v", doc["lemma"])
    }
    if doc["source_name"] != "opd" {
        t.Errorf("expected source_name 'opd', got %v", doc["source_name"])
    }
    if doc["content_hash"] != "abc123" {
        t.Errorf("expected content_hash 'abc123', got %v", doc["content_hash"])
    }
}

func TestToESDocument_ConsentFalse(t *testing.T) {
    entry := models.DictionaryEntry{
        Lemma:                "nibi",
        ConsentPublicDisplay: false,
    }
    _, skip := projection.ToESDocument(entry)
    if !skip {
        t.Error("expected consent=false entry to be skipped")
    }
}

func TestIndexName(t *testing.T) {
    if projection.IndexName() != "opd_dictionary" {
        t.Errorf("expected 'opd_dictionary', got %q", projection.IndexName())
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd source-manager && GOWORK=off go test ./internal/projection/... -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement projection**

```go
// source-manager/internal/projection/dictionary_es.go
package projection

import (
    "encoding/json"
    "time"

    "github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

const (
    indexName  = "opd_dictionary"
    sourceName = "opd"
)

// IndexName returns the ES index name for dictionary entries.
func IndexName() string {
    return indexName
}

// ToESDocument converts a DictionaryEntry to an ES document map.
// Returns (nil, true) if the entry should be skipped (consent=false).
func ToESDocument(entry models.DictionaryEntry) (map[string]any, bool) {
    if !entry.ConsentPublicDisplay {
        return nil, true
    }

    doc := map[string]any{
        "lemma":                  entry.Lemma,
        "source_name":            sourceName,
        "consent_public_display": entry.ConsentPublicDisplay,
        "license":                entry.License,
        "indexed_at":             time.Now().UTC().Format(time.RFC3339),
    }

    if entry.WordClass != nil {
        doc["word_class"] = *entry.WordClass
    }
    if entry.WordClassNormalized != nil {
        doc["word_class_normalized"] = *entry.WordClassNormalized
    }
    if entry.ContentHash != nil {
        doc["content_hash"] = *entry.ContentHash
    }
    if entry.SourceURL != nil {
        doc["source_url"] = *entry.SourceURL
    }
    if entry.Attribution != nil {
        doc["attribution"] = *entry.Attribution
    }

    setJSONField(doc, "definitions", entry.Definitions)
    setJSONField(doc, "inflections", entry.Inflections)
    setJSONField(doc, "examples", entry.Examples)
    setJSONField(doc, "word_family", entry.WordFamily)

    return doc, false
}

func setJSONField(doc map[string]any, key, value string) {
    if value == "" {
        return
    }
    var parsed any
    if err := json.Unmarshal([]byte(value), &parsed); err == nil {
        doc[key] = parsed
    }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd source-manager && GOWORK=off go test ./internal/projection/... -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Lint**

Run: `cd source-manager && GOWORK=off golangci-lint run ./internal/projection/...`
Expected: 0 issues

- [ ] **Step 6: Commit**

```bash
git add source-manager/internal/projection/
git commit -m "feat(source-manager): add dictionary ES projection with consent filtering"
```

---

### Task 6: Final Verification, Docs, and PR

**Files:**
- Modify: `source-manager/CLAUDE.md`
- Modify: `docs/specs/source-manager.md`

- [ ] **Step 1: Run all source-manager tests**

Run: `cd source-manager && GOWORK=off go test ./... -v`
Expected: All tests pass.

- [ ] **Step 2: Run linter on entire service**

Run: `cd source-manager && GOWORK=off golangci-lint run ./...`
Expected: 0 new issues.

- [ ] **Step 3: Run CI on changed services**

Run: `task ci:changed`
Expected: All checks pass.

- [ ] **Step 4: Update source-manager CLAUDE.md**

Add to the Architecture section:
- `importer/` — OPD JSONL bulk-import logic
- `projection/` — Dictionary ES projection (consent-filtered)

Add to Quick Reference:
```bash
# Import OPD dictionary
source-manager import-opd --file data/all_entries.jsonl --batch-size 500
source-manager import-opd --file data/all_entries.jsonl --dry-run
```

- [ ] **Step 5: Update docs/specs/source-manager.md**

Add OPD ingestion section documenting the `import-opd` subcommand, dictionary pipeline, and consent model.

- [ ] **Step 6: Run drift check**

Run: `task drift:check`
Expected: All specs up to date.

- [ ] **Step 7: Commit docs**

```bash
git add source-manager/CLAUDE.md docs/specs/source-manager.md
git commit -m "docs(source-manager): document OPD import-opd subcommand and dictionary pipeline"
```

- [ ] **Step 8: Create PR**

```bash
gh pr create --title "feat(source-manager): OPD dictionary ingestion pipeline" \
  --body "$(cat <<'EOF'
## Summary
- Add `import-opd` subcommand to source-manager for bulk OPD dictionary import
- JSONL reader with validation, canonical content hashing, and batch upsert
- ES projection package with consent filtering (only `consent_public_display=true`)
- Migration 019 makes content_hash UNIQUE for idempotent upserts

Closes #326

## Test plan
- [ ] Unit tests: JSONL parsing, content hash canonicalization, consent filtering
- [ ] Dry-run with valid and mixed fixtures
- [ ] Lint and CI pass
- [ ] Manual: import 100 entries to dev DB, verify via /dictionary/search

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)" --milestone "Indigenous Data Pipeline v1"
```

---

## Dependency Order

```
Task 1 (migration) -> Task 2 (BulkUpsert) -> Task 3 (importer) -> Task 4 (CLI) -> Task 5 (ES projection) -> Task 6 (integration)
```

Tasks are strictly sequential — each builds on the previous.
