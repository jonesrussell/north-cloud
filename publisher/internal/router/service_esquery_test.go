//nolint:testpackage // White-box test for buildESQuery content_type terms
package router

import (
	"testing"
)

func TestBuildESQuery_ContentTypeTerms(t *testing.T) {
	t.Helper()

	s := &Service{}
	query := s.buildESQuery()
	if query == nil {
		t.Fatal("buildESQuery returned nil")
	}

	q, ok := query["query"].(map[string]any)
	if !ok {
		t.Fatal("query.query not a map")
	}
	boolQ, ok := q["bool"].(map[string]any)
	if !ok {
		t.Fatal("query.bool not a map")
	}
	var first map[string]any
	switch must := boolQ["must"].(type) {
	case []any:
		if len(must) == 0 {
			t.Fatal("query.bool.must empty")
		}
		first, _ = must[0].(map[string]any)
	case []map[string]any:
		if len(must) == 0 {
			t.Fatal("query.bool.must empty")
		}
		first = must[0]
	default:
		t.Fatalf("query.bool.must unexpected type %T", boolQ["must"])
	}
	if first == nil {
		t.Fatal("first must clause not a map")
	}
	terms, ok := first["terms"].(map[string]any)
	if !ok {
		t.Fatal("first must clause has no terms")
	}
	ct := terms["content_type"]
	var ctStrs []string
	switch v := ct.(type) {
	case []string:
		ctStrs = v
	case []any:
		ctStrs = make([]string, 0, len(v))
		for _, e := range v {
			if s, isStr := e.(string); isStr {
				ctStrs = append(ctStrs, s)
			}
		}
	default:
		t.Fatalf("content_type not slice: %T", ct)
	}
	want := map[string]bool{"article": true, "recipe": true, "job": true, "rfp": true}
	if len(ctStrs) != len(want) {
		t.Errorf("content_type len = %d, want %d", len(ctStrs), len(want))
	}
	for _, s := range ctStrs {
		if !want[s] {
			t.Errorf("content_type contains unexpected %q", s)
		}
	}
}
