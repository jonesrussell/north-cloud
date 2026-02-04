// classifier/internal/data/canadian_cities_test.go
package data_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/data"
)

func TestIsValidCanadianCity(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		city     string
		expected bool
	}{
		{"valid city lowercase", "sudbury", true},
		{"valid city mixed case", "Sudbury", true},
		{"valid city uppercase", "SUDBURY", true},
		{"valid major city", "toronto", true},
		{"invalid city", "new york", false},
		{"empty string", "", false},
		{"us city", "chicago", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := data.IsValidCanadianCity(tt.city)
			if result != tt.expected {
				t.Errorf("IsValidCanadianCity(%q) = %v, want %v", tt.city, result, tt.expected)
			}
		})
	}
}

func TestNormalizeCityName(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"greater prefix", "Greater Sudbury", "sudbury"},
		{"city prefix", "City of Toronto", "toronto"},
		{"sault ste marie variants", "Sault Ste. Marie", "sault-ste-marie"},
		{"st johns vs saint john", "St. John's", "st-johns"},
		{"simple lowercase", "Vancouver", "vancouver"},
		{"hyphenated", "Trois-Rivières", "trois-rivieres"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := data.NormalizeCityName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeCityName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetProvinceForCity(t *testing.T) {
	t.Helper()

	tests := []struct {
		name         string
		city         string
		wantProvince string
		wantOK       bool
	}{
		{"sudbury", "sudbury", "ON", true},
		{"toronto", "toronto", "ON", true},
		{"vancouver", "vancouver", "BC", true},
		{"montreal", "montreal", "QC", true},
		{"invalid city", "new york", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			province, ok := data.GetProvinceForCity(tt.city)
			if ok != tt.wantOK {
				t.Errorf("GetProvinceForCity(%q) ok = %v, want %v", tt.city, ok, tt.wantOK)
			}
			if province != tt.wantProvince {
				t.Errorf("GetProvinceForCity(%q) province = %q, want %q", tt.city, province, tt.wantProvince)
			}
		})
	}
}
