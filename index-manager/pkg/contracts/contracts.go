// Package contracts exposes Elasticsearch mapping definitions as testable contracts.
//
// Other services import this package to verify that the fields they produce or
// consume are present in the canonical index mappings managed by index-manager.
package contracts

import (
	"testing"
)

// Mapping represents an Elasticsearch index mapping with a properties map.
type Mapping struct {
	Properties map[string]any
}

// AssertFieldsExist validates that all required fields exist in a mapping.
func AssertFieldsExist(t *testing.T, mapping Mapping, fields []string) {
	t.Helper()

	for _, field := range fields {
		if _, exists := mapping.Properties[field]; !exists {
			t.Errorf("required field %q not found in mapping (available: %v)",
				field, mapKeys(mapping.Properties))
		}
	}
}

// AssertNestedFieldsExist validates that fields exist within a nested object mapping.
// The parent field must be an object type with its own properties map.
func AssertNestedFieldsExist(t *testing.T, mapping Mapping, parent string, fields []string) {
	t.Helper()

	parentObj, exists := mapping.Properties[parent]
	if !exists {
		t.Errorf("parent field %q not found in mapping", parent)
		return
	}

	parentMap, ok := parentObj.(map[string]any)
	if !ok {
		t.Errorf("parent field %q is not a map", parent)
		return
	}

	propsRaw, exists := parentMap["properties"]
	if !exists {
		t.Errorf("parent field %q has no properties", parent)
		return
	}

	props, ok := propsRaw.(map[string]any)
	if !ok {
		t.Errorf("parent field %q properties is not a map", parent)
		return
	}

	for _, field := range fields {
		if _, found := props[field]; !found {
			t.Errorf("required field %q not found in %s.properties (available: %v)",
				field, parent, mapKeys(props))
		}
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
