// classifier/internal/domain/classification_test.go
package domain_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestLocationResult_GetSpecificity(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		location domain.LocationResult
		want     string
	}{
		{
			name:     "city specificity",
			location: domain.LocationResult{City: "sudbury", Province: "ON", Country: "canada"},
			want:     "city",
		},
		{
			name:     "province specificity",
			location: domain.LocationResult{Province: "ON", Country: "canada"},
			want:     "province",
		},
		{
			name:     "country specificity",
			location: domain.LocationResult{Country: "canada"},
			want:     "country",
		},
		{
			name:     "unknown specificity",
			location: domain.LocationResult{Country: "unknown"},
			want:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.location.GetSpecificity(); got != tt.want {
				t.Errorf("LocationResult.GetSpecificity() = %v, want %v", got, tt.want)
			}
		})
	}
}
