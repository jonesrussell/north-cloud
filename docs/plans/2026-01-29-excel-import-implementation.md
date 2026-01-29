# Excel Import Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Excel spreadsheet import capability to source-manager, allowing bulk creation/update of sources from .xlsx files.

**Architecture:** Multipart file upload → Excel parsing (excelize library) → validation of all rows → transactional upsert (all-or-nothing). New `importer` package handles parsing logic, repository gets upsert methods, handler orchestrates the flow.

**Tech Stack:** Go 1.24+, excelize/v2 for Excel parsing, PostgreSQL with ON CONFLICT upsert, Gin multipart handling

**Design Document:** Brainstorming session in conversation (2026-01-29)

---

## Phase 1: Add Excelize Dependency

### Task 1.1: Add excelize dependency to go.mod

**Files:**
- Modify: `source-manager/go.mod`

**Step 1: Add the dependency**

```bash
cd source-manager && go get github.com/xuri/excelize/v2@latest
```

**Step 2: Verify dependency added**

Run: `cd source-manager && go mod tidy && grep excelize go.mod`
Expected: Line containing `github.com/xuri/excelize/v2`

**Step 3: Commit**

```bash
git add source-manager/go.mod source-manager/go.sum
git commit -m "chore(source-manager): add excelize dependency for Excel import"
```

---

## Phase 2: Importer Package - Excel Parsing

### Task 2.1: Create importer types and constants

**Files:**
- Create: `source-manager/internal/importer/excel.go`
- Test: `source-manager/internal/importer/excel_test.go`

**Step 1: Write the failing test for SourceRow struct**

```go
// source-manager/internal/importer/excel_test.go
package importer

import (
	"testing"
)

func TestSourceRowExists(t *testing.T) {
	t.Helper()
	// Verify the struct exists and has expected fields
	row := SourceRow{
		Row:       2,
		Name:      "test",
		URL:       "https://example.com",
		Enabled:   true,
		RateLimit: "1s",
		MaxDepth:  2,
		Time:      `["morning"]`,
		Selectors: `{"article":{"title":"h1"}}`,
	}
	if row.Name != "test" {
		t.Errorf("expected Name to be 'test', got %s", row.Name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/importer/... -run TestSourceRowExists -v`
Expected: FAIL with "package importer is not in std" or similar

**Step 3: Write minimal implementation**

```go
// source-manager/internal/importer/excel.go
package importer

// Column indices for Excel spreadsheet (0-based).
const (
	colName      = 0 // Column A
	colURL       = 1 // Column B
	colEnabled   = 2 // Column C
	colRateLimit = 3 // Column D
	colMaxDepth  = 4 // Column E
	colTime      = 5 // Column F
	colSelectors = 6 // Column G

	minRequiredColumns = 7
	headerRowIndex     = 1 // Excel rows are 1-based, header is row 1
)

// SourceRow represents a parsed row from the Excel spreadsheet.
type SourceRow struct {
	Row       int    // Excel row number (for error reporting)
	Name      string
	URL       string
	Enabled   bool
	RateLimit string
	MaxDepth  int
	Time      string // Raw JSON string
	Selectors string // Raw JSON string
}

// ImportError represents a validation error for a specific row.
type ImportError struct {
	Row   int    `json:"row"`
	Error string `json:"error"`
}
```

**Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/importer/... -run TestSourceRowExists -v`
Expected: PASS

**Step 5: Commit**

```bash
git add source-manager/internal/importer/
git commit -m "feat(source-manager): add importer package with SourceRow type"
```

---

### Task 2.2: Implement row validation

**Files:**
- Modify: `source-manager/internal/importer/excel.go`
- Modify: `source-manager/internal/importer/excel_test.go`

**Step 1: Write the failing test for validateRow**

```go
// Add to source-manager/internal/importer/excel_test.go

func TestValidateRow(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		row      SourceRow
		wantErr  string
	}{
		{
			name: "valid row",
			row: SourceRow{
				Row:  2,
				Name: "test-source",
				URL:  "https://example.com",
			},
			wantErr: "",
		},
		{
			name: "missing name",
			row: SourceRow{
				Row: 2,
				URL: "https://example.com",
			},
			wantErr: "name is required",
		},
		{
			name: "missing url",
			row: SourceRow{
				Row:  2,
				Name: "test-source",
			},
			wantErr: "url is required",
		},
		{
			name: "invalid url scheme",
			row: SourceRow{
				Row:  2,
				Name: "test-source",
				URL:  "ftp://example.com",
			},
			wantErr: "url must start with http:// or https://",
		},
		{
			name: "invalid time json",
			row: SourceRow{
				Row:  2,
				Name: "test-source",
				URL:  "https://example.com",
				Time: "not valid json",
			},
			wantErr: "time must be a valid JSON array",
		},
		{
			name: "time not array",
			row: SourceRow{
				Row:  2,
				Name: "test-source",
				URL:  "https://example.com",
				Time: `{"key": "value"}`,
			},
			wantErr: "time must be a valid JSON array",
		},
		{
			name: "invalid selectors json",
			row: SourceRow{
				Row:       2,
				Name:      "test-source",
				URL:       "https://example.com",
				Selectors: "not valid json",
			},
			wantErr: "selectors must be valid JSON",
		},
		{
			name: "negative max_depth",
			row: SourceRow{
				Row:      2,
				Name:     "test-source",
				URL:      "https://example.com",
				MaxDepth: -1,
			},
			wantErr: "max_depth must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateRow(tt.row)
			if tt.wantErr == "" && got != "" {
				t.Errorf("validateRow() = %q, want empty", got)
			}
			if tt.wantErr != "" && got != tt.wantErr {
				t.Errorf("validateRow() = %q, want %q", got, tt.wantErr)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/importer/... -run TestValidateRow -v`
Expected: FAIL with "undefined: validateRow"

**Step 3: Write minimal implementation**

```go
// Add to source-manager/internal/importer/excel.go

import (
	"encoding/json"
	"strings"
)

// validateRow validates a single row and returns an error message or empty string.
func validateRow(row SourceRow) string {
	// Required fields
	if strings.TrimSpace(row.Name) == "" {
		return "name is required"
	}
	if strings.TrimSpace(row.URL) == "" {
		return "url is required"
	}

	// URL must be http or https
	if !strings.HasPrefix(row.URL, "http://") && !strings.HasPrefix(row.URL, "https://") {
		return "url must start with http:// or https://"
	}

	// max_depth must be non-negative
	if row.MaxDepth < 0 {
		return "max_depth must be non-negative"
	}

	// Time must be valid JSON array if provided
	if row.Time != "" {
		var timeArr []string
		if err := json.Unmarshal([]byte(row.Time), &timeArr); err != nil {
			return "time must be a valid JSON array"
		}
	}

	// Selectors must be valid JSON object if provided
	if row.Selectors != "" {
		var selectors map[string]any
		if err := json.Unmarshal([]byte(row.Selectors), &selectors); err != nil {
			return "selectors must be valid JSON"
		}
	}

	return ""
}
```

**Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/importer/... -run TestValidateRow -v`
Expected: PASS

**Step 5: Commit**

```bash
git add source-manager/internal/importer/
git commit -m "feat(source-manager): add row validation for Excel import"
```

---

### Task 2.3: Implement ParseExcelFile function

**Files:**
- Modify: `source-manager/internal/importer/excel.go`
- Modify: `source-manager/internal/importer/excel_test.go`
- Create: `source-manager/testdata/valid_sources.xlsx` (test fixture)
- Create: `source-manager/testdata/missing_name.xlsx` (test fixture)

**Step 1: Create test fixtures**

Create test Excel files manually or via script. For now, we'll write a helper that creates them programmatically in the test.

```go
// Add to source-manager/internal/importer/excel_test.go

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

// createTestExcel creates an in-memory Excel file for testing.
func createTestExcel(t *testing.T, rows [][]string) *bytes.Reader {
	t.Helper()

	f := excelize.NewFile()
	sheetName := "Sheet1"

	// Write header
	headers := []string{"name", "url", "enabled", "rate_limit", "max_depth", "time", "selectors"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheetName, cell, h); err != nil {
			t.Fatalf("failed to set header cell: %v", err)
		}
	}

	// Write data rows
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			if err := f.SetCellValue(sheetName, cell, val); err != nil {
				t.Fatalf("failed to set cell: %v", err)
			}
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("failed to write Excel file: %v", err)
	}

	return bytes.NewReader(buf.Bytes())
}

func TestParseExcelFile(t *testing.T) {
	t.Helper()

	tests := []struct {
		name           string
		rows           [][]string
		wantRowCount   int
		wantErrCount   int
		wantFirstError string
	}{
		{
			name: "valid file with two sources",
			rows: [][]string{
				{"source-a", "https://a.com", "true", "1s", "2", `["morning"]`, `{"article":{"title":"h1"}}`},
				{"source-b", "https://b.com", "false", "500ms", "3", "", ""},
			},
			wantRowCount: 2,
			wantErrCount: 0,
		},
		{
			name: "missing name in row 2",
			rows: [][]string{
				{"", "https://a.com", "true", "1s", "2", "", ""},
			},
			wantRowCount:   0,
			wantErrCount:   1,
			wantFirstError: "name is required",
		},
		{
			name: "missing url in row 2",
			rows: [][]string{
				{"source-a", "", "true", "1s", "2", "", ""},
			},
			wantRowCount:   0,
			wantErrCount:   1,
			wantFirstError: "url is required",
		},
		{
			name: "invalid json in time",
			rows: [][]string{
				{"source-a", "https://a.com", "true", "1s", "2", "not json", ""},
			},
			wantRowCount:   0,
			wantErrCount:   1,
			wantFirstError: "time must be a valid JSON array",
		},
		{
			name:         "empty file (header only)",
			rows:         [][]string{},
			wantRowCount: 0,
			wantErrCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := createTestExcel(t, tt.rows)
			rows, errs := ParseExcelFile(reader)

			if len(rows) != tt.wantRowCount {
				t.Errorf("ParseExcelFile() got %d rows, want %d", len(rows), tt.wantRowCount)
			}
			if len(errs) != tt.wantErrCount {
				t.Errorf("ParseExcelFile() got %d errors, want %d", len(errs), tt.wantErrCount)
			}
			if tt.wantFirstError != "" && len(errs) > 0 && errs[0].Error != tt.wantFirstError {
				t.Errorf("ParseExcelFile() first error = %q, want %q", errs[0].Error, tt.wantFirstError)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/importer/... -run TestParseExcelFile -v`
Expected: FAIL with "undefined: ParseExcelFile"

**Step 3: Write minimal implementation**

```go
// Add to source-manager/internal/importer/excel.go

import (
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ParseExcelFile parses an Excel file and returns validated source rows.
// Returns all validation errors if any row fails validation.
func ParseExcelFile(reader io.Reader) ([]SourceRow, []ImportError) {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, []ImportError{{Row: 0, Error: "failed to open Excel file: " + err.Error()}}
	}
	defer f.Close()

	// Get the first sheet
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, []ImportError{{Row: 0, Error: "no sheets found in Excel file"}}
	}

	// Get all rows
	excelRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, []ImportError{{Row: 0, Error: "failed to read sheet: " + err.Error()}}
	}

	// Skip header row, need at least header
	if len(excelRows) < 1 {
		return nil, []ImportError{{Row: 0, Error: "Excel file is empty"}}
	}

	// Parse data rows (skip header at index 0)
	var rows []SourceRow
	var errors []ImportError

	for i := 1; i < len(excelRows); i++ {
		excelRow := excelRows[i]
		rowNum := i + 1 // Excel rows are 1-based

		// Skip completely empty rows
		if isEmptyRow(excelRow) {
			continue
		}

		row := parseRow(excelRow, rowNum)

		// Validate the row
		if errMsg := validateRow(row); errMsg != "" {
			errors = append(errors, ImportError{Row: rowNum, Error: errMsg})
			continue
		}

		rows = append(rows, row)
	}

	// If any validation errors, return empty rows with all errors
	if len(errors) > 0 {
		return nil, errors
	}

	return rows, nil
}

// parseRow converts an Excel row to a SourceRow.
func parseRow(cells []string, rowNum int) SourceRow {
	row := SourceRow{Row: rowNum}

	if len(cells) > colName {
		row.Name = strings.TrimSpace(cells[colName])
	}
	if len(cells) > colURL {
		row.URL = strings.TrimSpace(cells[colURL])
	}
	if len(cells) > colEnabled {
		row.Enabled = parseBool(cells[colEnabled])
	}
	if len(cells) > colRateLimit {
		row.RateLimit = strings.TrimSpace(cells[colRateLimit])
	}
	if len(cells) > colMaxDepth {
		row.MaxDepth = parseInt(cells[colMaxDepth])
	}
	if len(cells) > colTime {
		row.Time = strings.TrimSpace(cells[colTime])
	}
	if len(cells) > colSelectors {
		row.Selectors = strings.TrimSpace(cells[colSelectors])
	}

	return row
}

// parseBool parses various boolean representations.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "1", "yes", "y":
		return true
	default:
		return false
	}
}

// parseInt parses an integer, returning 0 on error.
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

// isEmptyRow checks if a row has no non-empty cells.
func isEmptyRow(cells []string) bool {
	for _, cell := range cells {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
```

**Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/importer/... -run TestParseExcelFile -v`
Expected: PASS

**Step 5: Commit**

```bash
git add source-manager/internal/importer/
git commit -m "feat(source-manager): implement Excel file parsing"
```

---

### Task 2.4: Implement ToSource conversion

**Files:**
- Modify: `source-manager/internal/importer/excel.go`
- Modify: `source-manager/internal/importer/excel_test.go`

**Step 1: Write the failing test**

```go
// Add to source-manager/internal/importer/excel_test.go

import (
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

func TestToSource(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		row     SourceRow
		wantErr bool
		check   func(*testing.T, *models.Source)
	}{
		{
			name: "full row conversion",
			row: SourceRow{
				Row:       2,
				Name:      "test-source",
				URL:       "https://example.com",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  3,
				Time:      `["morning", "evening"]`,
				Selectors: `{"article":{"title":"h1","body":".content"}}`,
			},
			wantErr: false,
			check: func(t *testing.T, s *models.Source) {
				t.Helper()
				if s.Name != "test-source" {
					t.Errorf("Name = %q, want %q", s.Name, "test-source")
				}
				if s.URL != "https://example.com" {
					t.Errorf("URL = %q, want %q", s.URL, "https://example.com")
				}
				if !s.Enabled {
					t.Error("Enabled should be true")
				}
				if s.RateLimit != "1s" {
					t.Errorf("RateLimit = %q, want %q", s.RateLimit, "1s")
				}
				if s.MaxDepth != 3 {
					t.Errorf("MaxDepth = %d, want %d", s.MaxDepth, 3)
				}
				if len(s.Time) != 2 {
					t.Errorf("Time length = %d, want 2", len(s.Time))
				}
				if s.Selectors.Article.Title != "h1" {
					t.Errorf("Selectors.Article.Title = %q, want %q", s.Selectors.Article.Title, "h1")
				}
			},
		},
		{
			name: "minimal row conversion",
			row: SourceRow{
				Row:  2,
				Name: "minimal",
				URL:  "https://minimal.com",
			},
			wantErr: false,
			check: func(t *testing.T, s *models.Source) {
				t.Helper()
				if s.Name != "minimal" {
					t.Errorf("Name = %q, want %q", s.Name, "minimal")
				}
				if len(s.Time) != 0 {
					t.Errorf("Time should be empty, got %v", s.Time)
				}
			},
		},
		{
			name: "invalid time json",
			row: SourceRow{
				Row:  2,
				Name: "test",
				URL:  "https://test.com",
				Time: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid selectors json",
			row: SourceRow{
				Row:       2,
				Name:      "test",
				URL:       "https://test.com",
				Selectors: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := ToSource(tt.row)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, source)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/importer/... -run TestToSource -v`
Expected: FAIL with "undefined: ToSource"

**Step 3: Write minimal implementation**

```go
// Add to source-manager/internal/importer/excel.go

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

// ToSource converts a validated SourceRow to a models.Source.
// This should only be called after validation passes.
func ToSource(row SourceRow) (*models.Source, error) {
	source := &models.Source{
		Name:      row.Name,
		URL:       row.URL,
		Enabled:   row.Enabled,
		RateLimit: row.RateLimit,
		MaxDepth:  row.MaxDepth,
	}

	// Parse Time JSON array
	if row.Time != "" {
		var timeArr []string
		if err := json.Unmarshal([]byte(row.Time), &timeArr); err != nil {
			return nil, fmt.Errorf("parse time: %w", err)
		}
		source.Time = models.StringArray(timeArr)
	}

	// Parse Selectors JSON object
	if row.Selectors != "" {
		var selectors models.SelectorConfig
		if err := json.Unmarshal([]byte(row.Selectors), &selectors); err != nil {
			return nil, fmt.Errorf("parse selectors: %w", err)
		}
		source.Selectors = selectors
	}

	return source, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/importer/... -run TestToSource -v`
Expected: PASS

**Step 5: Commit**

```bash
git add source-manager/internal/importer/
git commit -m "feat(source-manager): add ToSource conversion for Excel import"
```

---

## Phase 3: Repository Upsert Methods

### Task 3.1: Add UpsertSource method to repository

**Files:**
- Modify: `source-manager/internal/repository/source.go`
- Modify: `source-manager/internal/repository/source_test.go`

**Step 1: Write the failing test**

```go
// Add to source-manager/internal/repository/source_test.go

func TestSourceRepository_UpsertSource(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	t.Run("insert new source", func(t *testing.T) {
		source := &models.Source{
			Name:      "Upsert New Source",
			URL:       "https://upsert-new.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: true,
		}

		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		created, err := repo.UpsertSource(ctx, tx, source)
		require.NoError(t, err)
		assert.True(t, created, "should be created (new)")
		assert.NotEmpty(t, source.ID, "ID should be set")

		require.NoError(t, tx.Commit())
	})

	t.Run("update existing source", func(t *testing.T) {
		// First create a source
		source := &models.Source{
			Name:      "Upsert Existing Source",
			URL:       "https://upsert-existing.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h1"},
			},
			Enabled: true,
		}
		err := repo.Create(ctx, source)
		require.NoError(t, err)
		originalID := source.ID

		// Now upsert with same name but different data
		updatedSource := &models.Source{
			Name:      "Upsert Existing Source", // Same name
			URL:       "https://upsert-existing-updated.com",
			RateLimit: "2s",
			MaxDepth:  5,
			Time:      models.StringArray{"10:00"},
			Selectors: models.SelectorConfig{
				Article: models.ArticleSelectors{Title: "h2"},
			},
			Enabled: false,
		}

		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		created, err := repo.UpsertSource(ctx, tx, updatedSource)
		require.NoError(t, err)
		assert.False(t, created, "should be updated (not created)")
		assert.Equal(t, originalID, updatedSource.ID, "ID should match original")

		require.NoError(t, tx.Commit())

		// Verify the update
		fetched, err := repo.GetByID(ctx, originalID)
		require.NoError(t, err)
		assert.Equal(t, "https://upsert-existing-updated.com", fetched.URL)
		assert.Equal(t, 5, fetched.MaxDepth)
	})
}
```

**Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/repository/... -run TestSourceRepository_UpsertSource -v`
Expected: FAIL with "repo.UpsertSource undefined"

**Step 3: Write minimal implementation**

```go
// Add to source-manager/internal/repository/source.go

// UpsertSource inserts or updates a source within an existing transaction.
// Returns true if the source was created (new), false if updated (existed).
// Uses PostgreSQL's ON CONFLICT with xmax trick to determine insert vs update.
func (r *SourceRepository) UpsertSource(ctx context.Context, tx *sql.Tx, source *models.Source) (bool, error) {
	now := time.Now()

	// Generate new ID if not set (will be overwritten if exists)
	if source.ID == "" {
		source.ID = uuid.New().String()
	}
	source.CreatedAt = now
	source.UpdatedAt = now

	selectorsJSON, err := json.Marshal(source.Selectors)
	if err != nil {
		return false, fmt.Errorf("marshal selectors: %w", err)
	}

	timeJSON, err := json.Marshal(source.Time)
	if err != nil {
		return false, fmt.Errorf("marshal time: %w", err)
	}

	// Use ON CONFLICT to upsert, and xmax = 0 trick to determine if inserted
	query := `
		INSERT INTO sources (
			id, name, url, rate_limit, max_depth,
			time, selectors, enabled, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (name) DO UPDATE SET
			url = EXCLUDED.url,
			rate_limit = EXCLUDED.rate_limit,
			max_depth = EXCLUDED.max_depth,
			time = EXCLUDED.time,
			selectors = EXCLUDED.selectors,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
		RETURNING id, (xmax = 0) AS is_insert
	`

	var returnedID string
	var isInsert bool
	err = tx.QueryRowContext(ctx,
		query,
		source.ID,
		source.Name,
		source.URL,
		source.RateLimit,
		source.MaxDepth,
		timeJSON,
		selectorsJSON,
		source.Enabled,
		source.CreatedAt,
		source.UpdatedAt,
	).Scan(&returnedID, &isInsert)

	if err != nil {
		return false, fmt.Errorf("upsert source: %w", err)
	}

	// Update the source ID (may have changed if it was an update)
	source.ID = returnedID

	return isInsert, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/repository/... -run TestSourceRepository_UpsertSource -v`
Expected: PASS

**Step 5: Commit**

```bash
git add source-manager/internal/repository/
git commit -m "feat(source-manager): add UpsertSource method with ON CONFLICT"
```

---

### Task 3.2: Add UpsertSourcesTx method for batch upsert

**Files:**
- Modify: `source-manager/internal/repository/source.go`
- Modify: `source-manager/internal/repository/source_test.go`

**Step 1: Write the failing test**

```go
// Add to source-manager/internal/repository/source_test.go

func TestSourceRepository_UpsertSourcesTx(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	logger := testhelpers.NewTestLogger()
	repo := repository.NewSourceRepository(db, logger)
	ctx := context.Background()

	t.Run("batch insert all new", func(t *testing.T) {
		sources := []*models.Source{
			{
				Name:      "Batch Source 1",
				URL:       "https://batch1.com",
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      models.StringArray{"09:00"},
				Selectors: models.SelectorConfig{Article: models.ArticleSelectors{Title: "h1"}},
				Enabled:   true,
			},
			{
				Name:      "Batch Source 2",
				URL:       "https://batch2.com",
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      models.StringArray{"10:00"},
				Selectors: models.SelectorConfig{Article: models.ArticleSelectors{Title: "h1"}},
				Enabled:   true,
			},
		}

		created, updated, err := repo.UpsertSourcesTx(ctx, sources)
		require.NoError(t, err)
		assert.Equal(t, 2, created, "should create 2 sources")
		assert.Equal(t, 0, updated, "should update 0 sources")
	})

	t.Run("batch with mix of new and existing", func(t *testing.T) {
		// First create one source
		existing := &models.Source{
			Name:      "Existing Batch Source",
			URL:       "https://existing-batch.com",
			RateLimit: "1s",
			MaxDepth:  2,
			Time:      models.StringArray{"09:00"},
			Selectors: models.SelectorConfig{Article: models.ArticleSelectors{Title: "h1"}},
			Enabled:   true,
		}
		err := repo.Create(ctx, existing)
		require.NoError(t, err)

		// Now upsert a batch that includes the existing source
		sources := []*models.Source{
			{
				Name:      "Existing Batch Source", // Same name = update
				URL:       "https://existing-batch-updated.com",
				RateLimit: "2s",
				MaxDepth:  5,
				Time:      models.StringArray{"10:00"},
				Selectors: models.SelectorConfig{Article: models.ArticleSelectors{Title: "h2"}},
				Enabled:   false,
			},
			{
				Name:      "New Batch Source", // New name = insert
				URL:       "https://new-batch.com",
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      models.StringArray{"11:00"},
				Selectors: models.SelectorConfig{Article: models.ArticleSelectors{Title: "h1"}},
				Enabled:   true,
			},
		}

		created, updated, err := repo.UpsertSourcesTx(ctx, sources)
		require.NoError(t, err)
		assert.Equal(t, 1, created, "should create 1 source")
		assert.Equal(t, 1, updated, "should update 1 source")
	})

	t.Run("empty batch", func(t *testing.T) {
		created, updated, err := repo.UpsertSourcesTx(ctx, []*models.Source{})
		require.NoError(t, err)
		assert.Equal(t, 0, created)
		assert.Equal(t, 0, updated)
	})
}
```

**Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/repository/... -run TestSourceRepository_UpsertSourcesTx -v`
Expected: FAIL with "repo.UpsertSourcesTx undefined"

**Step 3: Write minimal implementation**

```go
// Add to source-manager/internal/repository/source.go

// UpsertSourcesTx upserts multiple sources in a single transaction.
// Returns the count of created and updated sources.
// If any upsert fails, the entire transaction is rolled back.
func (r *SourceRepository) UpsertSourcesTx(ctx context.Context, sources []*models.Source) (created, updated int, err error) {
	if len(sources) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.logger.Error("failed to rollback transaction",
					infralogger.Error(rbErr),
				)
			}
		}
	}()

	for _, source := range sources {
		isCreated, upsertErr := r.UpsertSource(ctx, tx, source)
		if upsertErr != nil {
			err = fmt.Errorf("upsert source %q: %w", source.Name, upsertErr)
			return 0, 0, err
		}
		if isCreated {
			created++
		} else {
			updated++
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = fmt.Errorf("commit transaction: %w", commitErr)
		return 0, 0, err
	}

	return created, updated, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/repository/... -run TestSourceRepository_UpsertSourcesTx -v`
Expected: PASS

**Step 5: Commit**

```bash
git add source-manager/internal/repository/
git commit -m "feat(source-manager): add UpsertSourcesTx for batch transactional upsert"
```

---

## Phase 4: Handler and Route

### Task 4.1: Add ImportResult type and ImportExcel handler

**Files:**
- Modify: `source-manager/internal/handlers/source.go`

**Step 1: Write the handler implementation**

```go
// Add to source-manager/internal/handlers/source.go

import (
	"strings"

	"github.com/jonesrussell/north-cloud/source-manager/internal/importer"
)

// ImportResult is the response for the import-excel endpoint.
type ImportResult struct {
	Created int                    `json:"created"`
	Updated int                    `json:"updated"`
	Errors  []importer.ImportError `json:"errors"`
}

// ImportExcel handles bulk import of sources from an Excel file.
func (h *SourceHandler) ImportExcel(c *gin.Context) {
	// 1. Extract file from multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.logger.Debug("No file in request",
			infralogger.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// 2. Validate file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
		h.logger.Debug("Invalid file extension",
			infralogger.String("filename", header.Filename),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be .xlsx format"})
		return
	}

	h.logger.Info("Processing Excel import",
		infralogger.String("filename", header.Filename),
		infralogger.Int64("size", header.Size),
	)

	// 3. Parse and validate all rows
	rows, validationErrors := importer.ParseExcelFile(file)
	if len(validationErrors) > 0 {
		h.logger.Debug("Validation errors in Excel file",
			infralogger.Int("error_count", len(validationErrors)),
		)
		c.JSON(http.StatusBadRequest, ImportResult{Errors: validationErrors})
		return
	}

	// 4. Convert to models
	sources := make([]*models.Source, 0, len(rows))
	for _, row := range rows {
		source, convErr := importer.ToSource(row)
		if convErr != nil {
			// This shouldn't happen if validation passed, but handle it
			h.logger.Error("Failed to convert row to source",
				infralogger.Int("row", row.Row),
				infralogger.Error(convErr),
			)
			c.JSON(http.StatusBadRequest, ImportResult{
				Errors: []importer.ImportError{{Row: row.Row, Error: convErr.Error()}},
			})
			return
		}
		sources = append(sources, source)
	}

	// 5. Upsert in transaction
	created, updated, err := h.repo.UpsertSourcesTx(c.Request.Context(), sources)
	if err != nil {
		h.logger.Error("Failed to import sources",
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import sources"})
		return
	}

	// 6. Log success and return
	h.logger.Info("Sources imported successfully",
		infralogger.Int("created", created),
		infralogger.Int("updated", updated),
		infralogger.String("filename", header.Filename),
	)

	c.JSON(http.StatusOK, ImportResult{
		Created: created,
		Updated: updated,
		Errors:  []importer.ImportError{},
	})
}
```

**Step 2: Run linter to verify no issues**

Run: `cd source-manager && golangci-lint run ./internal/handlers/...`
Expected: No errors

**Step 3: Commit**

```bash
git add source-manager/internal/handlers/
git commit -m "feat(source-manager): add ImportExcel handler"
```

---

### Task 4.2: Register import-excel route

**Files:**
- Modify: `source-manager/internal/api/router.go`

**Step 1: Add the route**

Find this block in `router.go`:
```go
sources := v1.Group("/sources")
sources.POST("", sourceHandler.Create)
sources.POST("/fetch-metadata", sourceHandler.FetchMetadata)
sources.POST("/test-crawl", sourceHandler.TestCrawl)
```

Add after `/test-crawl`:
```go
sources.POST("/import-excel", sourceHandler.ImportExcel)
```

**Step 2: Run linter to verify no issues**

Run: `cd source-manager && golangci-lint run ./internal/api/...`
Expected: No errors

**Step 3: Commit**

```bash
git add source-manager/internal/api/
git commit -m "feat(source-manager): register import-excel route"
```

---

## Phase 5: Integration Tests

### Task 5.1: Add handler integration test

**Files:**
- Modify: `source-manager/internal/handlers/source_test.go`

**Step 1: Write the integration test**

```go
// Add to source-manager/internal/handlers/source_test.go

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// createMultipartExcel creates a multipart form request with an Excel file.
func createMultipartExcel(t *testing.T, rows [][]string, filename string) (*bytes.Buffer, string) {
	t.Helper()

	// Create Excel file
	f := excelize.NewFile()
	sheetName := "Sheet1"
	headers := []string{"name", "url", "enabled", "rate_limit", "max_depth", "time", "selectors"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheetName, cell, h); err != nil {
			t.Fatalf("failed to set header: %v", err)
		}
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			if err := f.SetCellValue(sheetName, cell, val); err != nil {
				t.Fatalf("failed to set cell: %v", err)
			}
		}
	}

	var excelBuf bytes.Buffer
	if err := f.Write(&excelBuf); err != nil {
		t.Fatalf("failed to write Excel: %v", err)
	}

	// Create multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(excelBuf.Bytes()); err != nil {
		t.Fatalf("failed to write to form: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	return &body, writer.FormDataContentType()
}

func TestSourceHandler_ImportExcel_Integration(t *testing.T) {
	// Skip if no test DB
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires database setup similar to repository tests
	// For now, we'll mark it as a placeholder for full integration testing
	t.Skip("Integration test requires full test harness setup")
}
```

**Step 2: Run linter**

Run: `cd source-manager && golangci-lint run ./internal/handlers/...`
Expected: No errors

**Step 3: Commit**

```bash
git add source-manager/internal/handlers/
git commit -m "test(source-manager): add ImportExcel integration test scaffold"
```

---

## Phase 6: Example Template

### Task 6.1: Create example Excel template

**Files:**
- Create: `source-manager/examples/source-import-template.xlsx`

**Step 1: Create the template programmatically**

Create a small Go script or use the following to generate:

```bash
cd source-manager && go run -exec "go build -o /tmp/gentemplate" - <<'EOF'
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

func main() {
	f := excelize.NewFile()
	sheet := "Sources"
	f.SetSheetName("Sheet1", sheet)

	// Headers
	headers := []string{"name", "url", "enabled", "rate_limit", "max_depth", "time", "selectors"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Example row 1 - full
	row1 := []string{
		"example-news",
		"https://example.com/news",
		"true",
		"1s",
		"3",
		`["morning", "evening"]`,
		`{"article":{"title":"h1.headline","body":".article-content"}}`,
	}
	for i, v := range row1 {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, v)
	}

	// Example row 2 - minimal
	row2 := []string{"local-blog", "https://blog.local", "false", "500ms", "2", "", ""}
	for i, v := range row2 {
		cell, _ := excelize.CoordinatesToCellName(i+1, 3)
		f.SetCellValue(sheet, cell, v)
	}

	// Instructions sheet
	f.NewSheet("Instructions")
	instructions := []string{
		"Column Descriptions:",
		"",
		"name - Required. Unique identifier for the source",
		"url - Required. Base URL to crawl (must start with http:// or https://)",
		"enabled - Optional. true/false/1/0/yes/no (default: false)",
		"rate_limit - Optional. Delay between requests (e.g., '1s', '500ms')",
		"max_depth - Optional. Maximum crawl depth (default: 0)",
		"time - Optional. JSON array of times (e.g., '[\"morning\", \"evening\"]')",
		"selectors - Optional. JSON object with CSS selectors for article/list/page extraction",
	}
	for i, line := range instructions {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		f.SetCellValue("Instructions", cell, line)
	}

	dir := "examples"
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "source-import-template.xlsx")
	if err := f.SaveAs(path); err != nil {
		log.Fatalf("failed to save: %v", err)
	}
	log.Printf("Created %s", path)
}
EOF
```

Alternatively, create the file manually with Excel/LibreOffice with the following structure:

**Sheet: Sources**
| name | url | enabled | rate_limit | max_depth | time | selectors |
|------|-----|---------|------------|-----------|------|-----------|
| example-news | https://example.com/news | true | 1s | 3 | ["morning", "evening"] | {"article":{"title":"h1.headline"}} |
| local-blog | https://blog.local | false | 500ms | 2 | | |

**Sheet: Instructions**
- Column descriptions for each field

**Step 2: Verify file exists**

Run: `ls -la source-manager/examples/source-import-template.xlsx`
Expected: File exists

**Step 3: Commit**

```bash
git add source-manager/examples/
git commit -m "docs(source-manager): add Excel import template with instructions"
```

---

## Phase 7: Final Verification

### Task 7.1: Run all tests and linter

**Step 1: Run all source-manager tests**

Run: `cd source-manager && go test ./... -v`
Expected: All tests pass

**Step 2: Run linter**

Run: `cd source-manager && golangci-lint run`
Expected: No errors

**Step 3: Build verification**

Run: `cd source-manager && go build -o /tmp/source-manager`
Expected: Build succeeds

**Step 4: Final commit (if any cleanup needed)**

```bash
git status
# If any uncommitted changes, commit them
```

---

## Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1 | 1.1 | Add excelize dependency |
| 2 | 2.1-2.4 | Importer package: types, validation, parsing, conversion |
| 3 | 3.1-3.2 | Repository: UpsertSource and UpsertSourcesTx |
| 4 | 4.1-4.2 | Handler and route registration |
| 5 | 5.1 | Integration test scaffold |
| 6 | 6.1 | Example template |
| 7 | 7.1 | Final verification |

**Total Tasks:** 10

**Key Patterns Followed:**
- TDD approach for all new code
- Transactional all-or-nothing upsert
- Separation of concerns (importer, repository, handler)
- Consistent error handling and logging
- No magic numbers (constants defined)
- All test helpers use `t.Helper()`
