package repository_test

import (
	"context"
	"testing"

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
