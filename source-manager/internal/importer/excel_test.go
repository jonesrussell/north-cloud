package importer_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/importer"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/xuri/excelize/v2"
)

func TestSourceRowExists(t *testing.T) {
	t.Helper()
	// Verify the struct exists and has expected fields
	row := importer.SourceRow{
		Row:       2,
		Name:      "test",
		URL:       "https://example.com",
		Enabled:   true,
		RateLimit: "1s",
		MaxDepth:  2,
		Time:      `["morning"]`,
		Selectors: `{"article":{"title":"h1"}}`,
	}

	// Verify all fields are correctly set
	if row.Row != 2 {
		t.Errorf("expected Row to be 2, got %d", row.Row)
	}
	if row.Name != "test" {
		t.Errorf("expected Name to be 'test', got %s", row.Name)
	}
	if row.URL != "https://example.com" {
		t.Errorf("expected URL to be 'https://example.com', got %s", row.URL)
	}
	if !row.Enabled {
		t.Errorf("expected Enabled to be true, got %v", row.Enabled)
	}
	if row.RateLimit != "1s" {
		t.Errorf("expected RateLimit to be '1s', got %s", row.RateLimit)
	}
	if row.MaxDepth != 2 {
		t.Errorf("expected MaxDepth to be 2, got %d", row.MaxDepth)
	}
	if row.Time != `["morning"]` {
		t.Errorf("expected Time to be '[\"morning\"]', got %s", row.Time)
	}
	if row.Selectors != `{"article":{"title":"h1"}}` {
		t.Errorf("expected Selectors to be '{\"article\":{\"title\":\"h1\"}}', got %s", row.Selectors)
	}
}

func TestImportErrorExists(t *testing.T) {
	t.Helper()
	// Verify the ImportError struct exists and has expected fields
	importErr := importer.ImportError{
		Row:   5,
		Error: "invalid URL format",
	}

	if importErr.Row != 5 {
		t.Errorf("expected Row to be 5, got %d", importErr.Row)
	}
	if importErr.Error != "invalid URL format" {
		t.Errorf("expected Error to be 'invalid URL format', got %s", importErr.Error)
	}
}

func TestValidateRow(t *testing.T) {
	t.Helper()

	tests := []struct {
		name    string
		row     importer.SourceRow
		wantErr string
	}{
		{
			name: "valid row",
			row: importer.SourceRow{
				Row:       2,
				Name:      "Test Source",
				URL:       "https://example.com",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      `["morning"]`,
				Selectors: `{"article":{"title":"h1"}}`,
			},
			wantErr: "",
		},
		{
			name: "missing name",
			row: importer.SourceRow{
				Row:       2,
				Name:      "",
				URL:       "https://example.com",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  2,
			},
			wantErr: "name is required",
		},
		{
			name: "missing url",
			row: importer.SourceRow{
				Row:       2,
				Name:      "Test Source",
				URL:       "",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  2,
			},
			wantErr: "url is required",
		},
		{
			name: "invalid url scheme",
			row: importer.SourceRow{
				Row:       2,
				Name:      "Test Source",
				URL:       "ftp://example.com",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  2,
			},
			wantErr: "url must start with http:// or https://",
		},
		{
			name: "invalid time json",
			row: importer.SourceRow{
				Row:       2,
				Name:      "Test Source",
				URL:       "https://example.com",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      `invalid json`,
			},
			wantErr: "time must be a valid JSON array",
		},
		{
			name: "time not array",
			row: importer.SourceRow{
				Row:       2,
				Name:      "Test Source",
				URL:       "https://example.com",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      `{"key": "value"}`,
			},
			wantErr: "time must be a valid JSON array",
		},
		{
			name: "invalid selectors json",
			row: importer.SourceRow{
				Row:       2,
				Name:      "Test Source",
				URL:       "https://example.com",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  2,
				Selectors: `not valid json`,
			},
			wantErr: "selectors must be valid JSON",
		},
		{
			name: "negative max_depth",
			row: importer.SourceRow{
				Row:       2,
				Name:      "Test Source",
				URL:       "https://example.com",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  -1,
			},
			wantErr: "max_depth must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := importer.ValidateRow(tt.row)
			if got != tt.wantErr {
				t.Errorf("ValidateRow() = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

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
		wantErrorCount int
		wantErrorMsg   string
	}{
		{
			name: "valid file with two sources",
			rows: [][]string{
				{"Source 1", "https://example.com", "true", "1s", "2", `["morning"]`, `{"article":{"title":"h1"}}`},
				{"Source 2", "https://test.com", "false", "2s", "3", `["evening"]`, `{"article":{"body":"p"}}`},
			},
			wantRowCount:   2,
			wantErrorCount: 0,
			wantErrorMsg:   "",
		},
		{
			name: "missing name in row 2",
			rows: [][]string{
				{"", "https://example.com", "true", "1s", "2", `["morning"]`, `{}`},
			},
			wantRowCount:   0,
			wantErrorCount: 1,
			wantErrorMsg:   "name is required",
		},
		{
			name: "missing url in row 2",
			rows: [][]string{
				{"Source 1", "", "true", "1s", "2", `["morning"]`, `{}`},
			},
			wantRowCount:   0,
			wantErrorCount: 1,
			wantErrorMsg:   "url is required",
		},
		{
			name: "invalid json in time",
			rows: [][]string{
				{"Source 1", "https://example.com", "true", "1s", "2", `invalid json`, `{}`},
			},
			wantRowCount:   0,
			wantErrorCount: 1,
			wantErrorMsg:   "time must be a valid JSON array",
		},
		{
			name:           "empty file (header only)",
			rows:           [][]string{},
			wantRowCount:   0,
			wantErrorCount: 0,
			wantErrorMsg:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := createTestExcel(t, tt.rows)

			rows, errors := importer.ParseExcelFile(reader)

			if len(rows) != tt.wantRowCount {
				t.Errorf("ParseExcelFile() got %d rows, want %d", len(rows), tt.wantRowCount)
			}

			if len(errors) != tt.wantErrorCount {
				t.Errorf("ParseExcelFile() got %d errors, want %d", len(errors), tt.wantErrorCount)
			}

			if tt.wantErrorMsg != "" && len(errors) > 0 {
				if !strings.Contains(errors[0].Error, tt.wantErrorMsg) {
					t.Errorf("ParseExcelFile() error = %q, want to contain %q", errors[0].Error, tt.wantErrorMsg)
				}
			}
		})
	}
}

// validateFullRowConversion validates a fully populated source from ToSource.
func validateFullRowConversion(t *testing.T, source *models.Source) {
	t.Helper()
	if source.Name != "Test Source" {
		t.Errorf("Name = %q, want %q", source.Name, "Test Source")
	}
	if source.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", source.URL, "https://example.com")
	}
	if !source.Enabled {
		t.Error("Enabled = false, want true")
	}
	if source.RateLimit != "1s" {
		t.Errorf("RateLimit = %q, want %q", source.RateLimit, "1s")
	}
	if source.MaxDepth != 2 {
		t.Errorf("MaxDepth = %d, want %d", source.MaxDepth, 2)
	}
	validateFullRowTime(t, source)
	validateFullRowSelectors(t, source)
}

func validateFullRowTime(t *testing.T, source *models.Source) {
	t.Helper()
	expectedTimeLen := 2
	if len(source.Time) != expectedTimeLen {
		t.Errorf("Time length = %d, want %d", len(source.Time), expectedTimeLen)
	}
	if len(source.Time) >= expectedTimeLen && (source.Time[0] != "morning" || source.Time[1] != "evening") {
		t.Errorf("Time = %v, want [morning, evening]", source.Time)
	}
}

func validateFullRowSelectors(t *testing.T, source *models.Source) {
	t.Helper()
	if source.Selectors.Article.Title != "h1" {
		t.Errorf("Selectors.Article.Title = %q, want %q", source.Selectors.Article.Title, "h1")
	}
	if source.Selectors.Article.Body != "p" {
		t.Errorf("Selectors.Article.Body = %q, want %q", source.Selectors.Article.Body, "p")
	}
	if source.Selectors.List.Container != ".list" {
		t.Errorf("Selectors.List.Container = %q, want %q", source.Selectors.List.Container, ".list")
	}
}

// validateMinimalRowConversion validates a minimal source from ToSource.
func validateMinimalRowConversion(t *testing.T, source *models.Source) {
	t.Helper()
	if source.Name != "Minimal Source" {
		t.Errorf("Name = %q, want %q", source.Name, "Minimal Source")
	}
	if source.URL != "https://minimal.com" {
		t.Errorf("URL = %q, want %q", source.URL, "https://minimal.com")
	}
	if source.Enabled {
		t.Error("Enabled = true, want false (default)")
	}
	if len(source.Time) != 0 {
		t.Errorf("Time length = %d, want 0 (empty)", len(source.Time))
	}
	if source.Selectors.Article.Title != "" {
		t.Errorf("Selectors.Article.Title = %q, want empty", source.Selectors.Article.Title)
	}
}

func TestToSource(t *testing.T) {
	tests := []struct {
		name       string
		row        importer.SourceRow
		wantErr    bool
		wantErrMsg string
		validate   func(t *testing.T, source *models.Source)
	}{
		{
			name: "full row conversion",
			row: importer.SourceRow{
				Row:       2,
				Name:      "Test Source",
				URL:       "https://example.com",
				Enabled:   true,
				RateLimit: "1s",
				MaxDepth:  2,
				Time:      `["morning", "evening"]`,
				Selectors: `{"article":{"title":"h1","body":"p"},"list":{"container":".list"}}`,
			},
			wantErr:  false,
			validate: validateFullRowConversion,
		},
		{
			name: "minimal row conversion",
			row: importer.SourceRow{
				Row:  2,
				Name: "Minimal Source",
				URL:  "https://minimal.com",
			},
			wantErr:  false,
			validate: validateMinimalRowConversion,
		},
		{
			name: "invalid time json",
			row: importer.SourceRow{
				Row:  2,
				Name: "Test",
				URL:  "https://test.com",
				Time: `not valid json`,
			},
			wantErr:    true,
			wantErrMsg: "parse time",
		},
		{
			name: "invalid selectors json",
			row: importer.SourceRow{
				Row:       2,
				Name:      "Test",
				URL:       "https://test.com",
				Selectors: `{invalid}`,
			},
			wantErr:    true,
			wantErrMsg: "parse selectors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := importer.ToSource(tt.row)
			verifyToSourceResult(t, source, err, tt.wantErr, tt.wantErrMsg, tt.validate)
		})
	}
}

func verifyToSourceResult(
	t *testing.T,
	source *models.Source,
	err error,
	wantErr bool,
	wantErrMsg string,
	validate func(t *testing.T, source *models.Source),
) {
	t.Helper()

	if wantErr {
		verifyToSourceError(t, err, wantErrMsg)
		return
	}

	if err != nil {
		t.Errorf("ToSource() unexpected error = %v", err)
		return
	}

	if source == nil {
		t.Error("ToSource() returned nil source")
		return
	}

	if validate != nil {
		validate(t, source)
	}
}

func verifyToSourceError(t *testing.T, err error, wantErrMsg string) {
	t.Helper()
	if err == nil {
		t.Error("ToSource() error = nil, want error")
		return
	}
	if wantErrMsg != "" && !strings.Contains(err.Error(), wantErrMsg) {
		t.Errorf("ToSource() error = %q, want to contain %q", err.Error(), wantErrMsg)
	}
}
