package mappings //nolint:testpackage // tests need internal access

import (
	"encoding/json"
	"os"
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

func TestClassifiedContentMapping_HasICPFields(t *testing.T) {
	m := NewClassifiedContentMapping()
	s, err := m.GetJSON()
	if err != nil {
		t.Fatalf("GetJSON: %v", err)
	}

	for _, want := range []string{`"icp"`, `"segments"`, `"model_version"`, `"matched_keywords"`} {
		if !strings.Contains(s, want) {
			t.Errorf("mapping missing ICP field %s", want)
		}
	}
}

func TestAddICPMigrationFile(t *testing.T) {
	data, err := os.ReadFile("v015_add_icp.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var doc map[string]any
	if unmarshalErr := json.Unmarshal(data, &doc); unmarshalErr != nil {
		t.Fatalf("invalid JSON: %v", unmarshalErr)
	}

	props := doc["properties"].(map[string]any)
	icp := props["icp"].(map[string]any)
	icpProps := icp["properties"].(map[string]any)
	segments := icpProps["segments"].(map[string]any)
	if segments["type"] != "nested" {
		t.Fatalf("migration icp.segments.type = %v, want nested", segments["type"])
	}

	segmentProps := segments["properties"].(map[string]any)
	for field, wantType := range map[string]string{
		"segment":          "keyword",
		"score":            "float",
		"matched_keywords": "keyword",
	} {
		got := segmentProps[field].(map[string]any)["type"]
		if got != wantType {
			t.Errorf("migration icp.segments.%s.type = %v, want %s", field, got, wantType)
		}
	}

	if got := icpProps["model_version"].(map[string]any)["type"]; got != "keyword" {
		t.Errorf("migration icp.model_version.type = %v, want keyword", got)
	}
}
