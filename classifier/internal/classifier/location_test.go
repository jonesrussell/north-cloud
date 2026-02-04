// classifier/internal/classifier/location_test.go
package classifier_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// mockLogger implements the Logger interface for testing.
type mockLogger struct{}

func (m *mockLogger) Debug(_ string, _ ...infralogger.Field)         {}
func (m *mockLogger) Info(_ string, _ ...infralogger.Field)          {}
func (m *mockLogger) Warn(_ string, _ ...infralogger.Field)          {}
func (m *mockLogger) Error(_ string, _ ...infralogger.Field)         {}
func (m *mockLogger) Fatal(_ string, _ ...infralogger.Field)         {}
func (m *mockLogger) With(_ ...infralogger.Field) infralogger.Logger { return m }
func (m *mockLogger) Sync() error                                    { return nil }

func TestLocationClassifier_ExtractEntities(t *testing.T) {
	t.Helper()

	lc := classifier.NewLocationClassifier(&mockLogger{})

	tests := []struct {
		name       string
		text       string
		wantCities []string
	}{
		{
			name:       "single Canadian city",
			text:       "A man was arrested in Sudbury today.",
			wantCities: []string{"sudbury"},
		},
		{
			name:       "multiple Canadian cities",
			text:       "The suspect fled from Toronto to Montreal.",
			wantCities: []string{"toronto", "montreal"},
		},
		{
			name:       "US city not detected as Canadian",
			text:       "The US Justice Department in Washington announced.",
			wantCities: []string{},
		},
		{
			name:       "city with province",
			text:       "Sudbury Police in Northern Ontario responded.",
			wantCities: []string{"sudbury"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities := lc.ExtractEntities(tt.text)
			cities := extractCityNames(entities)
			if !stringSlicesEqual(cities, tt.wantCities) {
				t.Errorf("ExtractEntities() cities = %v, want %v", cities, tt.wantCities)
			}
		})
	}
}

func extractCityNames(entities []classifier.LocationEntity) []string {
	cities := make([]string, 0)
	for _, e := range entities {
		if e.EntityType == classifier.EntityTypeCity {
			cities = append(cities, e.Normalized)
		}
	}
	return cities
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLocationClassifier_Classify(t *testing.T) {
	t.Helper()

	lc := classifier.NewLocationClassifier(&mockLogger{})
	ctx := context.Background()

	tests := []struct {
		name        string
		raw         *domain.RawContent
		wantCity    string
		wantCountry string
	}{
		{
			name: "Canadian city in title",
			raw: &domain.RawContent{
				Title:   "Sudbury Police arrest suspect in downtown stabbing",
				RawText: "A man was taken into custody after the incident.",
			},
			wantCity:    "sudbury",
			wantCountry: "canada",
		},
		{
			name: "US story from Canadian publisher",
			raw: &domain.RawContent{
				Title:   "US Justice Department opens probe into police shooting",
				RawText: "The federal investigation was announced today in Washington.",
			},
			wantCity:    "",
			wantCountry: "united_states",
		},
		{
			name: "No location detected",
			raw: &domain.RawContent{
				Title:   "Breaking news today",
				RawText: "Something happened somewhere. Details are emerging.",
			},
			wantCity:    "",
			wantCountry: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lc.Classify(ctx, tt.raw)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if result.City != tt.wantCity {
				t.Errorf("Classify() city = %v, want %v", result.City, tt.wantCity)
			}
			if result.Country != tt.wantCountry {
				t.Errorf("Classify() country = %v, want %v", result.Country, tt.wantCountry)
			}
		})
	}
}
