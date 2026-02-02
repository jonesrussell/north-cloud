package models_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

func TestNormalizeRateLimit(t *testing.T) {
	t.Helper()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", models.DefaultRateLimit},
		{"whitespace", "  ", models.DefaultRateLimit},
		{"bare number", "10", "10s"},
		{"bare number one", "1", "1s"},
		{"already with unit seconds", "10s", "10s"},
		{"already with unit minutes", "1m", "1m"},
		{"trimmed bare number", "  5  ", "5s"},
		{"invalid returns default", "abc", models.DefaultRateLimit},
		{"zero returns default", "0", models.DefaultRateLimit},
		{"negative returns default", "-1", models.DefaultRateLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.NormalizeRateLimit(tt.in)
			if got != tt.want {
				t.Errorf("NormalizeRateLimit(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
