package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

// TestCrawlerProducesValidRawContent verifies that every field the crawler
// writes to *_raw_content exists in the canonical mapping.
func TestCrawlerProducesValidRawContent(t *testing.T) {
	mapping := contracts.RawContentMapping()

	producedFields := []string{
		"id", "url", "source_name",
		"title", "raw_text", "raw_html",
		"og_type", "og_title", "og_description", "og_image", "og_url",
		"meta_description", "meta_keywords", "canonical_url",
		"author",
		"crawled_at", "published_date",
		"classification_status",
		"word_count",
	}

	contracts.AssertFieldsExist(t, mapping, producedFields)
}
