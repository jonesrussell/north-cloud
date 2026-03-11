package insights_test

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
)

func TestParseDeletedCount(t *testing.T) {
	body := `{"deleted": 42, "batches": 1, "version_conflicts": 0}`

	count, err := insights.ParseDeletedCountForTest(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parseDeletedCount() error = %v", err)
	}

	const expectedDeleted = 42
	if count != expectedDeleted {
		t.Errorf("expected %d deleted, got %d", expectedDeleted, count)
	}
}

func TestParseDeletedCount_Zero(t *testing.T) {
	body := `{"deleted": 0}`

	count, err := insights.ParseDeletedCountForTest(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 deleted, got %d", count)
	}
}

func TestCleaner_DeleteOld_ZeroRetention(t *testing.T) {
	c := insights.NewCleaner(nil, 0)

	count, err := c.DeleteOld(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}
