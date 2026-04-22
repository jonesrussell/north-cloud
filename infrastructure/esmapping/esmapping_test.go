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
	var tmp any
	if uerr := json.Unmarshal([]byte(s), &tmp); uerr != nil {
		t.Fatalf("invalid JSON: %v", uerr)
	}
}
