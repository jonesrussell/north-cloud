package importer_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/importer"
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
