package mapper_test

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-producer/internal/mapper"
)

// makeBaseHit returns a deep-enough hit with all top-level required fields
// populated. The caller can mutate or extend it before passing to MapHit.
func makeBaseHit(t *testing.T, contentType, id string) map[string]any {
	t.Helper()
	return map[string]any{
		"_id":           id,
		"title":         "Bridge construction services",
		"quality_score": float64(78),
		"url":           "https://canadabuys.canada.ca/example",
		"crawled_at":    "2026-04-27T05:00:00Z",
		"content_type":  contentType,
	}
}

func TestMapHit_RFP_Complete(t *testing.T) {
	hit := makeBaseHit(t, "rfp", "AbC123")
	hit["rfp"] = map[string]any{
		"organization_name": "Government of Canada",
		"province":          "ON",
		"categories":        []any{"Construction", "Infrastructure"},
		"closing_date":      "2026-05-15T17:00:00Z",
	}

	signal, err := mapper.MapHit(hit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if signal.SignalType != "rfp" {
		t.Errorf("SignalType = %q, want rfp", signal.SignalType)
	}
	if signal.ExternalID != "nc-rfp-AbC123" {
		t.Errorf("ExternalID = %q, want nc-rfp-AbC123", signal.ExternalID)
	}
	if signal.Source != "north-cloud" {
		t.Errorf("Source = %q, want north-cloud", signal.Source)
	}
	if signal.SourceURL != "https://canadabuys.canada.ca/example" {
		t.Errorf("SourceURL = %q", signal.SourceURL)
	}
	if signal.Label != "Bridge construction services" {
		t.Errorf("Label = %q", signal.Label)
	}
	if signal.Strength != 78 {
		t.Errorf("Strength = %d, want 78", signal.Strength)
	}
	if signal.OrganizationName != "Government of Canada" {
		t.Errorf("OrganizationName = %q", signal.OrganizationName)
	}
	if signal.Sector != "Construction" {
		t.Errorf("Sector = %q, want Construction (first category)", signal.Sector)
	}
	if signal.Province != "ON" {
		t.Errorf("Province = %q", signal.Province)
	}
	if signal.ExpiresAt == nil || *signal.ExpiresAt != "2026-05-15T17:00:00Z" {
		t.Errorf("ExpiresAt = %v, want pointer to 2026-05-15T17:00:00Z", signal.ExpiresAt)
	}
	if signal.Payload == nil {
		t.Error("Payload should carry the original hit, got nil")
	}
}

func TestMapHit_RFP_MissingOptionalFields(t *testing.T) {
	hit := makeBaseHit(t, "rfp", "Bare1")
	// No "rfp" subfield at all — every optional should default to "".

	signal, err := mapper.MapHit(hit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if signal.OrganizationName != "" || signal.Province != "" || signal.Sector != "" {
		t.Errorf("expected empty strings for optional RFP fields, got %+v", signal)
	}
	if signal.ExpiresAt != nil {
		t.Errorf("ExpiresAt should be nil when closing_date absent, got %v", *signal.ExpiresAt)
	}
}

func TestMapHit_RFP_EmptyCategories(t *testing.T) {
	hit := makeBaseHit(t, "rfp", "X")
	hit["rfp"] = map[string]any{"categories": []any{}}
	signal, err := mapper.MapHit(hit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if signal.Sector != "" {
		t.Errorf("Sector should be empty for empty categories, got %q", signal.Sector)
	}
}

func TestMapHit_NeedSignal_Complete(t *testing.T) {
	hit := makeBaseHit(t, "need_signal", "Sig9")
	hit["need_signal"] = map[string]any{
		"organization_name": "Acme Corp",
		"province":          "QC",
		"sector":            "IT",
		"signal_type":       "hiring_surge",
	}

	signal, err := mapper.MapHit(hit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if signal.SignalType != "hiring_surge" {
		t.Errorf("SignalType = %q, want hiring_surge", signal.SignalType)
	}
	if signal.ExternalID != "nc-sig-Sig9" {
		t.Errorf("ExternalID = %q, want nc-sig-Sig9", signal.ExternalID)
	}
	if signal.OrganizationName != "Acme Corp" {
		t.Errorf("OrganizationName = %q", signal.OrganizationName)
	}
	if signal.Sector != "IT" {
		t.Errorf("Sector = %q", signal.Sector)
	}
	if signal.Province != "QC" {
		t.Errorf("Province = %q", signal.Province)
	}
	if signal.ExpiresAt != nil {
		t.Errorf("ExpiresAt should be nil for need_signal, got %v", *signal.ExpiresAt)
	}
}

func TestMapHit_NeedSignal_MissingSignalType(t *testing.T) {
	hit := makeBaseHit(t, "need_signal", "X")
	hit["need_signal"] = map[string]any{
		"organization_name": "Acme",
	}
	_, err := mapper.MapHit(hit)
	if err == nil {
		t.Fatal("expected error for missing signal_type, got nil")
	}
	if !strings.Contains(err.Error(), "need_signal.signal_type") {
		t.Errorf("error should mention need_signal.signal_type, got %q", err.Error())
	}
}

func TestMapHit_NeedSignal_NoSubobject(t *testing.T) {
	hit := makeBaseHit(t, "need_signal", "X")
	// No "need_signal" key at all.
	_, err := mapper.MapHit(hit)
	if err == nil {
		t.Fatal("expected error when need_signal subobject missing")
	}
}

func TestMapHit_RequiredFieldErrors(t *testing.T) {
	tests := []string{"_id", "title", "quality_score", "url", "crawled_at", "content_type"}
	for _, missing := range tests {
		t.Run("missing_"+missing, func(t *testing.T) {
			hit := makeBaseHit(t, "rfp", "id1")
			delete(hit, missing)
			_, err := mapper.MapHit(hit)
			if err == nil {
				t.Fatalf("expected error when %q is missing", missing)
			}
			if !strings.Contains(err.Error(), missing) {
				t.Errorf("error should mention %q, got %q", missing, err.Error())
			}
		})
	}
}

func TestMapHit_UnsupportedContentType(t *testing.T) {
	hit := makeBaseHit(t, "blog_post", "id1")
	_, err := mapper.MapHit(hit)
	if err == nil {
		t.Fatal("expected error for unsupported content_type")
	}
	if !strings.Contains(err.Error(), "unsupported content_type") {
		t.Errorf("error should mention unsupported content_type, got %q", err.Error())
	}
}

func TestMapHit_PrefixDistinction(t *testing.T) {
	// Same _id, two content types must yield distinct external_ids.
	rfpHit := makeBaseHit(t, "rfp", "shared")
	needHit := makeBaseHit(t, "need_signal", "shared")
	needHit["need_signal"] = map[string]any{"signal_type": "growth"}

	rfpSignal, err := mapper.MapHit(rfpHit)
	if err != nil {
		t.Fatalf("rfp error: %v", err)
	}
	needSignal, err := mapper.MapHit(needHit)
	if err != nil {
		t.Fatalf("need_signal error: %v", err)
	}

	if rfpSignal.ExternalID == needSignal.ExternalID {
		t.Errorf("expected distinct ExternalIDs, both = %q", rfpSignal.ExternalID)
	}
	if rfpSignal.ExternalID != "nc-rfp-shared" {
		t.Errorf("rfp ExternalID = %q", rfpSignal.ExternalID)
	}
	if needSignal.ExternalID != "nc-sig-shared" {
		t.Errorf("need_signal ExternalID = %q", needSignal.ExternalID)
	}
}

func TestMapHit_QualityScoreAcceptsIntAndFloat(t *testing.T) {
	t.Run("float64", func(t *testing.T) {
		hit := makeBaseHit(t, "rfp", "x")
		hit["quality_score"] = float64(42)
		s, err := mapper.MapHit(hit)
		if err != nil {
			t.Fatal(err)
		}
		if s.Strength != 42 {
			t.Errorf("Strength = %d, want 42", s.Strength)
		}
	})
	t.Run("int", func(t *testing.T) {
		hit := makeBaseHit(t, "rfp", "x")
		hit["quality_score"] = 42
		s, err := mapper.MapHit(hit)
		if err != nil {
			t.Fatal(err)
		}
		if s.Strength != 42 {
			t.Errorf("Strength = %d, want 42", s.Strength)
		}
	})
	t.Run("wrong_type", func(t *testing.T) {
		hit := makeBaseHit(t, "rfp", "x")
		hit["quality_score"] = "42"
		_, err := mapper.MapHit(hit)
		if err == nil {
			t.Fatal("expected error when quality_score is string")
		}
	})
}

func TestStringFromPath(t *testing.T) {
	hit := map[string]any{
		"a": map[string]any{"b": "value"},
		"c": map[string]any{"d": 123},
	}
	if got := mapper.StringFromPath(hit, "a", "b"); got != "value" {
		t.Errorf("got %q, want value", got)
	}
	if got := mapper.StringFromPath(hit, "missing", "b"); got != "" {
		t.Errorf("missing path should yield empty, got %q", got)
	}
	if got := mapper.StringFromPath(hit, "c", "d"); got != "" {
		t.Errorf("wrong-type leaf should yield empty, got %q", got)
	}
	if got := mapper.StringFromPath(hit, "a", "b", "deeper"); got != "" {
		t.Errorf("over-walk should yield empty, got %q", got)
	}
}

func TestFirstStringInSlice(t *testing.T) {
	hit := map[string]any{
		"rfp": map[string]any{
			"categories": []any{"first", "second"},
			"empty":      []any{},
			"wrong":      "not a slice",
			"non_string": []any{42, "x"},
		},
	}
	if got := mapper.FirstStringInSlice(hit, "rfp", "categories"); got != "first" {
		t.Errorf("got %q, want first", got)
	}
	if got := mapper.FirstStringInSlice(hit, "rfp", "empty"); got != "" {
		t.Errorf("empty slice should yield empty, got %q", got)
	}
	if got := mapper.FirstStringInSlice(hit, "rfp", "wrong"); got != "" {
		t.Errorf("non-slice should yield empty, got %q", got)
	}
	if got := mapper.FirstStringInSlice(hit, "rfp", "non_string"); got != "" {
		t.Errorf("non-string head should yield empty, got %q", got)
	}
	if got := mapper.FirstStringInSlice(hit, "missing", "categories"); got != "" {
		t.Errorf("missing path should yield empty, got %q", got)
	}
}

func TestOptionalStringFromPath(t *testing.T) {
	hit := map[string]any{
		"a": map[string]any{"b": "value", "empty": ""},
	}
	if v, ok := mapper.OptionalStringFromPath(hit, "a", "b"); !ok || v != "value" {
		t.Errorf("got (%q,%v), want (value,true)", v, ok)
	}
	if _, ok := mapper.OptionalStringFromPath(hit, "a", "empty"); ok {
		t.Error("empty string should report not-found")
	}
	if _, ok := mapper.OptionalStringFromPath(hit, "a", "missing"); ok {
		t.Error("missing leaf should report not-found")
	}
}

func TestIntFromPath(t *testing.T) {
	hit := map[string]any{
		"a": map[string]any{
			"f": float64(7),
			"i": 9,
			"s": "x",
		},
	}
	if got := mapper.IntFromPath(hit, "a", "f"); got != 7 {
		t.Errorf("float64 path: got %d", got)
	}
	if got := mapper.IntFromPath(hit, "a", "i"); got != 9 {
		t.Errorf("int path: got %d", got)
	}
	if got := mapper.IntFromPath(hit, "a", "s"); got != 0 {
		t.Errorf("string leaf should yield 0, got %d", got)
	}
	if got := mapper.IntFromPath(hit, "a", "missing"); got != 0 {
		t.Errorf("missing leaf should yield 0, got %d", got)
	}
	if got := mapper.IntFromPath(hit, "missing", "i"); got != 0 {
		t.Errorf("missing root step should yield 0, got %d", got)
	}
}
