//nolint:testpackage // Testing unexported validateIndigenousRegion requires same package access
package handlers

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

func TestValidateIndigenousRegion_Nil(t *testing.T) {
	t.Helper()
	h := &SourceHandler{}
	source := &models.Source{}

	if err := h.validateIndigenousRegion(source); err != nil {
		t.Errorf("nil region should be valid, got: %v", err)
	}
	if source.IndigenousRegion != nil {
		t.Error("nil region should remain nil")
	}
}

func TestValidateIndigenousRegion_Valid(t *testing.T) {
	t.Helper()
	h := &SourceHandler{}

	tests := []struct {
		input string
		want  string
	}{
		{"canada", "canada"},
		{"Oceania", "oceania"},
		{"LATIN-AMERICA", "latin_america"},
		{"  europe  ", "europe"},
	}

	for _, tt := range tests {
		input := tt.input
		source := &models.Source{IndigenousRegion: &input}
		if err := h.validateIndigenousRegion(source); err != nil {
			t.Errorf("input %q: unexpected error: %v", tt.input, err)
			continue
		}
		if source.IndigenousRegion == nil || *source.IndigenousRegion != tt.want {
			got := "<nil>"
			if source.IndigenousRegion != nil {
				got = *source.IndigenousRegion
			}
			t.Errorf("input %q: got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateIndigenousRegion_Invalid(t *testing.T) {
	t.Helper()
	h := &SourceHandler{}

	invalid := []string{"antartica", "north_america", "middle_east", "xyz"}
	for _, input := range invalid {
		v := input
		source := &models.Source{IndigenousRegion: &v}
		if err := h.validateIndigenousRegion(source); err == nil {
			t.Errorf("input %q: expected error, got nil", input)
		}
	}
}

func TestValidateIndigenousRegion_EmptyString(t *testing.T) {
	t.Helper()
	h := &SourceHandler{}
	empty := ""
	source := &models.Source{IndigenousRegion: &empty}

	if err := h.validateIndigenousRegion(source); err != nil {
		t.Errorf("empty string should be valid, got: %v", err)
	}
	if source.IndigenousRegion != nil {
		t.Error("empty string should normalize to nil")
	}
}
