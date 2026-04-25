package mappings_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/elasticsearch/mappings"
)

func metaProperties(t *testing.T, m *mappings.RawContentMapping) map[string]any {
	t.Helper()
	jsonStr, err := m.GetJSON()
	if err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	var root map[string]any
	if uerr := json.Unmarshal([]byte(jsonStr), &root); uerr != nil {
		t.Fatalf("unmarshal root: %v", uerr)
	}
	mappingsObj := root["mappings"].(map[string]any)
	props := mappingsObj["properties"].(map[string]any)
	meta := props["meta"].(map[string]any)
	return meta["properties"].(map[string]any)
}

func TestNewRawContentMapping_MetaFieldExists(t *testing.T) {
	t.Helper()
	m := mappings.NewRawContentMapping()
	metaProps := metaProperties(t, m)
	if metaProps == nil {
		t.Fatal("expected meta.properties to be non-nil")
	}
}

func TestNewRawContentMapping_MetaPageType(t *testing.T) {
	t.Helper()
	m := mappings.NewRawContentMapping()
	metaProps := metaProperties(t, m)
	pt, ok := metaProps["page_type"].(map[string]any)
	if !ok {
		t.Fatal("expected page_type object in meta.properties")
	}
	if pt["type"] != "text" {
		t.Errorf("page_type.type = %v, want text", pt["type"])
	}
	fields, ok := pt["fields"].(map[string]any)
	if !ok {
		t.Fatal("expected page_type.fields")
	}
	kw, ok := fields["keyword"].(map[string]any)
	if !ok || kw["type"] != "keyword" {
		t.Fatalf("page_type.fields.keyword = %#v", fields["keyword"])
	}
}

func TestNewRawContentMapping_MetaAllFields(t *testing.T) {
	t.Helper()
	m := mappings.NewRawContentMapping()
	props := metaProperties(t, m)

	for _, name := range []string{
		"article_opinion", "article_content_tier", "twitter_card", "twitter_site", "og_site_name",
	} {
		f, ok := props[name].(map[string]any)
		if !ok {
			t.Errorf("expected meta field %q", name)
			continue
		}
		if f["type"] != "keyword" {
			t.Errorf("meta %q type = %v, want keyword", name, f["type"])
		}
	}

	for _, name := range []string{"og_image_width", "og_image_height"} {
		f, ok := props[name].(map[string]any)
		if !ok {
			t.Errorf("expected meta field %q", name)
			continue
		}
		if f["type"] != "integer" {
			t.Errorf("meta %q type = %v, want integer (SSoT aligned with index-manager)", name, f["type"])
		}
	}

	for _, name := range []string{"created_at", "updated_at"} {
		f, ok := props[name].(map[string]any)
		if !ok {
			t.Errorf("expected meta field %q", name)
			continue
		}
		if f["type"] != "date" {
			t.Errorf("meta %q type = %v, want date", name, f["type"])
		}
	}

	for _, name := range []string{"page_type", "detected_content_type", "indigenous_region"} {
		f, ok := props[name].(map[string]any)
		if !ok {
			t.Errorf("expected meta field %q", name)
			continue
		}
		if f["type"] != "text" {
			t.Errorf("meta %q type = %v, want text", name, f["type"])
		}
	}
}

func TestNewRawContentMapping_GetJSONContainsMeta(t *testing.T) {
	t.Helper()
	m := mappings.NewRawContentMapping()
	jsonStr, err := m.GetJSON()
	if err != nil {
		t.Fatalf("GetJSON() returned error: %v", err)
	}
	if !strings.Contains(jsonStr, `"meta"`) {
		t.Error("expected GetJSON() output to contain \"meta\"")
	}
	if !strings.Contains(jsonStr, `"page_type"`) {
		t.Error("expected GetJSON() output to contain \"page_type\"")
	}
}

func TestNewRawContentMapping_GetJSONValid(t *testing.T) {
	t.Helper()
	m := mappings.NewRawContentMapping()
	jsonStr, err := m.GetJSON()
	if err != nil {
		t.Fatalf("GetJSON() returned error: %v", err)
	}
	var result map[string]any
	if uerr := json.Unmarshal([]byte(jsonStr), &result); uerr != nil {
		t.Fatalf("GetJSON() produced invalid JSON: %v", uerr)
	}
}
