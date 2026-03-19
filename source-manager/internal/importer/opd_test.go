package importer_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/importer"
)

func TestReadOPDEntries_ValidFile(t *testing.T) {
	t.Helper()
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
	t.Helper()
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
	t.Helper()
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
	t.Helper()
	// Same data, different key order — should produce same hash
	hash1 := importer.ComputeContentHash(`{"b":2,"a":1}`)
	hash2 := importer.ComputeContentHash(`{"a":1,"b":2}`)
	if hash1 != hash2 {
		t.Errorf("expected canonical hash to normalize key order, got %q vs %q", hash1, hash2)
	}
}
