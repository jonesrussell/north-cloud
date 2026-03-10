package seeder_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/seeder"
)

func TestSlugify(t *testing.T) {
	t.Helper()

	tests := []struct {
		input    string
		expected string
	}{
		{"Attawapiskat First Nation", "attawapiskat-first-nation"},
		{"M'Chigeeng First Nation", "m-chigeeng-first-nation"},
		{"  Spaces Around  ", "spaces-around"},
		{"UPPERCASE", "uppercase"},
		{"already-slugged", "already-slugged"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := seeder.Slugify(tt.input)
			if got != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
