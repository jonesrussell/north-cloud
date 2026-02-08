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
		"article_section", "json_ld_data", "meta",
	}

	contracts.AssertFieldsExist(t, mapping, producedFields)
}

// TestCrawlerProducesValidJsonLdFields verifies json_ld_data nested fields.
func TestCrawlerProducesValidJsonLdFields(t *testing.T) {
	mapping := contracts.RawContentMapping()

	jsonLdFields := []string{
		"jsonld_headline", "jsonld_description", "jsonld_article_section",
		"jsonld_author", "jsonld_publisher_name", "jsonld_url", "jsonld_image_url",
		"jsonld_date_published", "jsonld_date_created", "jsonld_date_modified",
		"jsonld_word_count", "jsonld_keywords", "jsonld_raw",
	}

	contracts.AssertNestedFieldsExist(t, mapping, "json_ld_data", jsonLdFields)
}

// TestCrawlerProducesValidMetaFields verifies meta nested fields.
func TestCrawlerProducesValidMetaFields(t *testing.T) {
	mapping := contracts.RawContentMapping()

	metaFields := []string{
		"twitter_card", "twitter_site", "og_image_width", "og_image_height",
		"og_site_name", "created_at", "updated_at", "article_opinion", "article_content_tier",
	}

	contracts.AssertNestedFieldsExist(t, mapping, "meta", metaFields)
}
