package rawcontent_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/infrastructure/indigenous"
)

func TestRegionNormalizationBeforeMeta(t *testing.T) {
	t.Helper()

	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"canada", "canada", false},
		{"Canada", "canada", false},
		{"OCEANIA", "oceania", false},
		{"Latin America", "latin_america", false},
		{"latin-america", "latin_america", false},
		{"  europe  ", "europe", false},
		{"", "", false},
		{"invalid_region", "", true},
	}

	for _, tt := range tests {
		got, err := indigenous.NormalizeRegionSlug(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("NormalizeRegionSlug(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("NormalizeRegionSlug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
