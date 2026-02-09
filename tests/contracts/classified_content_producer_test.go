package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

// TestClassifierProducesValidClassifiedContent verifies that every field the
// classifier writes to *_classified_content exists in the canonical mapping.
func TestClassifierProducesValidClassifiedContent(t *testing.T) {
	mapping := contracts.ClassifiedContentMapping()

	topLevelFields := []string{
		// Raw content fields carried forward
		"id", "url", "source_name", "title", "raw_text", "raw_html",
		"crawled_at", "published_date", "classification_status", "classified_at",
		"word_count",

		// Classification fields produced by the classifier
		"content_type", "content_subtype",
		"quality_score", "quality_factors",
		"topics", "topic_scores",
		"source_reputation", "source_category",
		"classifier_version", "classification_method", "model_version",
		"confidence",

		// Nested objects (verified separately below)
		"crime", "location", "mining",
	}

	contracts.AssertFieldsExist(t, mapping, topLevelFields)
}

// TestClassifierProducesValidCrimeFields verifies the crime nested object
// fields match the canonical mapping.
func TestClassifierProducesValidCrimeFields(t *testing.T) {
	mapping := contracts.ClassifiedContentMapping()

	crimeFields := []string{
		"sub_label", "primary_crime_type", "relevance",
		"crime_types", "final_confidence",
		"homepage_eligible", "review_required", "model_version",
	}

	contracts.AssertNestedFieldsExist(t, mapping, "crime", crimeFields)
}

// TestClassifierProducesValidLocationFields verifies the location nested object
// fields match the canonical mapping.
func TestClassifierProducesValidLocationFields(t *testing.T) {
	mapping := contracts.ClassifiedContentMapping()

	locationFields := []string{
		"city", "province", "country", "specificity", "confidence",
	}

	contracts.AssertNestedFieldsExist(t, mapping, "location", locationFields)
}

// TestClassifierProducesValidMiningFields verifies the mining nested object
// fields match the canonical mapping.
func TestClassifierProducesValidMiningFields(t *testing.T) {
	mapping := contracts.ClassifiedContentMapping()

	miningFields := []string{
		"relevance", "mining_stage", "commodities", "location",
		"final_confidence", "review_required", "model_version",
	}

	contracts.AssertNestedFieldsExist(t, mapping, "mining", miningFields)
}
