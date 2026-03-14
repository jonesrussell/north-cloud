package mappings_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/elasticsearch/mappings"
)

func TestNewRawContentMapping_MetaFieldExists(t *testing.T) {
	t.Helper()

	m := mappings.NewRawContentMapping()

	if m.Mappings.Properties.Meta.Properties == nil {
		t.Fatal("expected Meta.Properties to be non-nil")
	}
}

func TestNewRawContentMapping_MetaPageType(t *testing.T) {
	t.Helper()

	m := mappings.NewRawContentMapping()

	f, ok := m.Mappings.Properties.Meta.Properties["page_type"]
	if !ok {
		t.Fatal("expected 'page_type' in Meta.Properties")
	}

	if f.Type != "text" {
		t.Errorf("expected page_type type 'text', got %q", f.Type)
	}

	kw, hasKeyword := f.Fields["keyword"]
	if !hasKeyword {
		t.Fatal("expected 'keyword' sub-field on page_type")
	}

	if kw.Type != "keyword" {
		t.Errorf("expected keyword sub-field type 'keyword', got %q", kw.Type)
	}
}

func TestNewRawContentMapping_MetaAllFields(t *testing.T) {
	t.Helper()

	m := mappings.NewRawContentMapping()
	props := m.Mappings.Properties.Meta.Properties

	expectedKeywordFields := []string{
		"article_opinion",
		"article_content_tier",
		"twitter_card",
		"twitter_site",
		"og_image_width",
		"og_image_height",
		"og_site_name",
	}

	for _, name := range expectedKeywordFields {
		f, ok := props[name]
		if !ok {
			t.Errorf("expected meta field %q to exist", name)
			continue
		}

		if f.Type != "keyword" {
			t.Errorf("expected meta field %q to have type 'keyword', got %q", name, f.Type)
		}
	}

	expectedDateFields := []string{"created_at", "updated_at"}
	for _, name := range expectedDateFields {
		f, ok := props[name]
		if !ok {
			t.Errorf("expected meta field %q to exist", name)
			continue
		}

		if f.Type != "date" {
			t.Errorf("expected meta field %q to have type 'date', got %q", name, f.Type)
		}
	}

	expectedTextFields := []string{"page_type", "detected_content_type", "indigenous_region"}
	for _, name := range expectedTextFields {
		f, ok := props[name]
		if !ok {
			t.Errorf("expected meta field %q to exist", name)
			continue
		}

		if f.Type != "text" {
			t.Errorf("expected meta field %q to have type 'text', got %q", name, f.Type)
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
	if unmarshalErr := json.Unmarshal([]byte(jsonStr), &result); unmarshalErr != nil {
		t.Fatalf("GetJSON() produced invalid JSON: %v", unmarshalErr)
	}
}
