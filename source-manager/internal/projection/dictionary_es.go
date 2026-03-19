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
	if unmarshalErr := json.Unmarshal([]byte(value), &parsed); unmarshalErr == nil {
		doc[key] = parsed
	}
}
