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
