package mappings

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClassifiedContentMapping_HasDrillResults(t *testing.T) {
	m := NewClassifiedContentMapping()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)

	if !strings.Contains(s, `"drill_results"`) {
		t.Error("mapping missing drill_results field in mining properties")
	}
	if !strings.Contains(s, `"extraction_method"`) {
		t.Error("mapping missing extraction_method field in mining properties")
	}
}
