//nolint:testpackage // testing unexported extractParamKeys
package mcp

import (
	"encoding/json"
	"sort"
	"testing"
)

func TestExtractParamKeys_ValidJSON(t *testing.T) {
	input := json.RawMessage(`{"name":"test","url":"http://example.com","limit":10}`)
	keys := extractParamKeys(input)

	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}

	sort.Strings(keys)
	expected := []string{"limit", "name", "url"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("key[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestExtractParamKeys_EmptyObject(t *testing.T) {
	input := json.RawMessage(`{}`)
	keys := extractParamKeys(input)

	if keys != nil {
		t.Errorf("expected nil for empty object, got %v", keys)
	}
}

func TestExtractParamKeys_EmptyInput(t *testing.T) {
	keys := extractParamKeys(nil)
	if keys != nil {
		t.Errorf("expected nil for nil input, got %v", keys)
	}

	keys = extractParamKeys(json.RawMessage{})
	if keys != nil {
		t.Errorf("expected nil for empty input, got %v", keys)
	}
}

func TestExtractParamKeys_InvalidJSON(t *testing.T) {
	input := json.RawMessage(`not valid json`)
	keys := extractParamKeys(input)

	if keys != nil {
		t.Errorf("expected nil for invalid JSON, got %v", keys)
	}
}

func TestExtractParamKeys_NonObject(t *testing.T) {
	input := json.RawMessage(`"just a string"`)
	keys := extractParamKeys(input)

	if keys != nil {
		t.Errorf("expected nil for non-object JSON, got %v", keys)
	}
}
