package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

// TestSearchExpectedClassifiedContentFields verifies that every field the
// search service reads from *_classified_content exists in the canonical mapping.
func TestSearchExpectedClassifiedContentFields(t *testing.T) {
	mapping := contracts.ClassifiedContentMapping()

	// Fields used in queries, filters, sort, and source selection
	requiredFields := []string{
		// Multi-match search fields
		"title", "raw_text", "og_title", "og_description", "meta_description",

		// Source fields returned in results
		"id", "url", "source_name", "published_date", "crawled_at",
		"quality_score", "content_type", "topics", "is_crime_related",

		// Filter and aggregation fields
		"source_reputation", "confidence", "word_count",
	}

	contracts.AssertFieldsExist(t, mapping, requiredFields)
}
