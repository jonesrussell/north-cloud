package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

// TestClassifierExpectedRawContentFields verifies that every field the
// classifier reads from *_raw_content exists in the canonical mapping.
func TestClassifierExpectedRawContentFields(t *testing.T) {
	mapping := contracts.RawContentMapping()

	requiredFields := []string{
		"id", "url", "source_name",
		"title", "raw_text", "raw_html",
		"og_type", "og_title", "og_description",
		"meta_description", "meta_keywords",
		"crawled_at", "published_date",
		"classification_status", "classified_at",
		"word_count",
	}

	contracts.AssertFieldsExist(t, mapping, requiredFields)
}
