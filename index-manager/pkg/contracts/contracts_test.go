package contracts_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/pkg/contracts"
)

func TestRawContentMappingHasProperties(t *testing.T) {
	t.Helper()

	mapping := contracts.RawContentMapping()
	if len(mapping.Properties) == 0 {
		t.Fatal("RawContentMapping returned empty properties")
	}
}

func TestClassifiedContentMappingHasProperties(t *testing.T) {
	t.Helper()

	mapping := contracts.ClassifiedContentMapping()
	if len(mapping.Properties) == 0 {
		t.Fatal("ClassifiedContentMapping returned empty properties")
	}
}

func TestClassifiedContentHasMoreFieldsThanRaw(t *testing.T) {
	t.Helper()

	raw := contracts.RawContentMapping()
	classified := contracts.ClassifiedContentMapping()

	if len(classified.Properties) <= len(raw.Properties) {
		t.Errorf("classified_content (%d fields) should have more fields than raw_content (%d fields)",
			len(classified.Properties), len(raw.Properties))
	}
}

func TestAssertFieldsExist_Passes(t *testing.T) {
	mapping := contracts.Mapping{
		Properties: map[string]any{
			"title": map[string]any{"type": "text"},
			"url":   map[string]any{"type": "keyword"},
		},
	}

	contracts.AssertFieldsExist(t, mapping, []string{"title", "url"})
}

func TestAssertNestedFieldsExist_Passes(t *testing.T) {
	mapping := contracts.Mapping{
		Properties: map[string]any{
			"crime": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"relevance": map[string]any{"type": "keyword"},
				},
			},
		},
	}

	contracts.AssertNestedFieldsExist(t, mapping, "crime", []string{"relevance"})
}
