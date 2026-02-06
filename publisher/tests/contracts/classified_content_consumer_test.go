package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

// TestPublisherExpectedClassifiedContentFields verifies that every field the
// publisher reads from *_classified_content exists in the canonical mapping.
func TestPublisherExpectedClassifiedContentFields(t *testing.T) {
	mapping := contracts.ClassifiedContentMapping()

	requiredFields := []string{
		// Query filter and sort fields
		"content_type", "crawled_at",

		// Routing fields
		"topics", "quality_score", "is_crime_related",

		// Payload fields included in Redis messages
		"title", "url", "source_name", "raw_text", "word_count",

		// Nested objects used for routing layers
		"crime", "location", "mining",
	}

	contracts.AssertFieldsExist(t, mapping, requiredFields)
}

// TestPublisherExpectedCrimeFields verifies the crime nested object fields
// the publisher depends on for Layer 3 routing.
func TestPublisherExpectedCrimeFields(t *testing.T) {
	mapping := contracts.ClassifiedContentMapping()

	crimeFields := []string{
		"relevance", "crime_types",
		"homepage_eligible", "review_required",
	}

	contracts.AssertNestedFieldsExist(t, mapping, "crime", crimeFields)
}

// TestPublisherExpectedMiningFields verifies the mining nested object fields
// the publisher depends on for Layer 5 routing.
func TestPublisherExpectedMiningFields(t *testing.T) {
	mapping := contracts.ClassifiedContentMapping()

	miningFields := []string{
		"relevance", "mining_stage", "commodities", "location",
		"final_confidence", "review_required",
	}

	contracts.AssertNestedFieldsExist(t, mapping, "mining", miningFields)
}

// TestPublisherExpectedLocationFields verifies the location nested object
// fields the publisher depends on for Layer 4 routing.
func TestPublisherExpectedLocationFields(t *testing.T) {
	mapping := contracts.ClassifiedContentMapping()

	locationFields := []string{
		"city", "province", "country", "specificity", "confidence",
	}

	contracts.AssertNestedFieldsExist(t, mapping, "location", locationFields)
}
