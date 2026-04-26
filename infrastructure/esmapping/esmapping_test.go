package esmapping_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/infrastructure/esmapping"
)

func TestRawContentIndex_StrictDynamic(t *testing.T) {
	t.Helper()
	m := esmapping.RawContentIndex(1, 1)
	dyn := m["mappings"].(map[string]any)["dynamic"]
	if dyn != "strict" {
		t.Fatalf("dynamic = %v", dyn)
	}
}

func TestClassifiedContentIndex_ContentTypeTextWithKeyword(t *testing.T) {
	t.Helper()
	m := esmapping.ClassifiedContentIndex(1, 1)
	props := m["mappings"].(map[string]any)["properties"].(map[string]any)
	ct := props["content_type"].(map[string]any)
	if ct["type"] != "text" {
		t.Fatalf("content_type.type = %v, want text", ct["type"])
	}
	fields, ok := ct["fields"].(map[string]any)
	if !ok || fields["keyword"] == nil {
		t.Fatal("content_type missing fields.keyword")
	}
}

func TestClassifiedContentIndex_ICPMapping(t *testing.T) {
	t.Helper()
	m := esmapping.ClassifiedContentIndex(1, 1)
	props := m["mappings"].(map[string]any)["properties"].(map[string]any)

	icp := props["icp"].(map[string]any)
	if icp["type"] != "object" {
		t.Fatalf("icp.type = %v, want object", icp["type"])
	}

	icpProps := icp["properties"].(map[string]any)
	segments := icpProps["segments"].(map[string]any)
	if segments["type"] != "nested" {
		t.Fatalf("icp.segments.type = %v, want nested", segments["type"])
	}

	segmentProps := segments["properties"].(map[string]any)
	for field, wantType := range map[string]string{
		"segment":          "keyword",
		"score":            "float",
		"matched_keywords": "keyword",
	} {
		got := segmentProps[field].(map[string]any)["type"]
		if got != wantType {
			t.Errorf("icp.segments.%s.type = %v, want %s", field, got, wantType)
		}
	}

	if got := icpProps["model_version"].(map[string]any)["type"]; got != "keyword" {
		t.Errorf("icp.model_version.type = %v, want keyword", got)
	}
}

func TestClassifiedContentIndex_JSONStableSnapshot(t *testing.T) {
	t.Helper()
	s, err := esmapping.ToIndentedJSON(esmapping.ClassifiedContentIndex(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, `"street_crime_relevance"`) {
		t.Error("expected crime union field street_crime_relevance")
	}
	if !strings.Contains(s, `"english_content"`) {
		t.Error("expected english_content analyzer in mapping JSON")
	}
	if !strings.Contains(s, `"icp"`) {
		t.Error("expected icp object in mapping JSON")
	}
	var tmp any
	if uerr := json.Unmarshal([]byte(s), &tmp); uerr != nil {
		t.Fatalf("invalid JSON: %v", uerr)
	}
}
