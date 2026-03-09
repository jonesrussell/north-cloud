package indigenous_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/infrastructure/indigenous"
)

func TestIsValidRegion(t *testing.T) {
	t.Helper()

	valid := []string{"canada", "us", "latin_america", "oceania", "europe", "asia", "africa"}
	for _, r := range valid {
		if !indigenous.IsValidRegion(r) {
			t.Errorf("expected %q to be valid", r)
		}
	}

	invalid := []string{"Canada", "OCEANIA", "latin-america", "antartica", "north_america", ""}
	for _, r := range invalid {
		if indigenous.IsValidRegion(r) {
			t.Errorf("expected %q to be invalid", r)
		}
	}
}

func TestNormalizeRegionSlug_Valid(t *testing.T) {
	t.Helper()

	tests := []struct {
		input string
		want  string
	}{
		{"canada", "canada"},
		{"Canada", "canada"},
		{"OCEANIA", "oceania"},
		{"latin_america", "latin_america"},
		{"Latin America", "latin_america"},
		{"latin-america", "latin_america"},
		{"  asia  ", "asia"},
		{"LATIN-AMERICA", "latin_america"},
		{"  Europe  ", "europe"},
	}

	for _, tt := range tests {
		got, err := indigenous.NormalizeRegionSlug(tt.input)
		if err != nil {
			t.Errorf("indigenous.NormalizeRegionSlug(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("indigenous.NormalizeRegionSlug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeRegionSlug_Empty(t *testing.T) {
	t.Helper()

	for _, input := range []string{"", "  ", "   "} {
		got, err := indigenous.NormalizeRegionSlug(input)
		if err != nil {
			t.Errorf("indigenous.NormalizeRegionSlug(%q) unexpected error: %v", input, err)
		}
		if got != "" {
			t.Errorf("indigenous.NormalizeRegionSlug(%q) = %q, want empty", input, got)
		}
	}
}

func TestNormalizeRegionSlug_Invalid(t *testing.T) {
	t.Helper()

	invalid := []string{"antartica", "north_america", "middle_east", "xyz", "123"}
	for _, input := range invalid {
		_, err := indigenous.NormalizeRegionSlug(input)
		if err == nil {
			t.Errorf("indigenous.NormalizeRegionSlug(%q) expected error, got nil", input)
		}
	}
}

func TestAllowedRegionsCount(t *testing.T) {
	t.Helper()

	const expectedRegionCount = 7
	if len(indigenous.AllowedRegions) != expectedRegionCount {
		t.Errorf("indigenous.AllowedRegions has %d entries, expected %d", len(indigenous.AllowedRegions), expectedRegionCount)
	}
}
