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
