package seeder_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/seeder"
)

func TestInferCommunityType(t *testing.T) {
	t.Helper()

	tests := []struct {
		name       string
		population int
		expected   string
	}{
		{"large city", 500000, "city"},
		{"city threshold", 100000, "city"},
		{"town", 5000, "town"},
		{"town threshold", 1000, "town"},
		{"small settlement", 500, "settlement"},
		{"zero population", 0, "settlement"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := seeder.InferCommunityType(tt.population)
			if got != tt.expected {
				t.Errorf("InferCommunityType(%d) = %q, want %q", tt.population, got, tt.expected)
			}
		})
	}
}
