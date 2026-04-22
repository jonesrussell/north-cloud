package mappings //nolint:testpackage // tests need internal access

import (
	"strings"
	"testing"
)

func TestClassifiedContentMapping_HasDrillResults(t *testing.T) {
	m := NewClassifiedContentMapping()
	s, err := m.GetJSON()
	if err != nil {
		t.Fatalf("GetJSON: %v", err)
	}

	if !strings.Contains(s, `"drill_results"`) {
		t.Error("mapping missing drill_results field in mining properties")
	}
	if !strings.Contains(s, `"extraction_method"`) {
		t.Error("mapping missing extraction_method field in mining properties")
	}
}
