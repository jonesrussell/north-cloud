// classifier/internal/classifier/streetcode_rules_test.go
//
//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"testing"
)

func TestStreetCodeRules_ClassifyByRules_ViolentCrime(t *testing.T) {
	t.Helper()

	tests := []struct {
		name              string
		title             string
		expectedRelevance string
		expectedTypes     []string
	}{
		{
			name:              "murder",
			title:             "Man charged with murder after stabbing",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "shooting",
			title:             "Police respond to downtown shooting",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
		{
			name:              "assault with arrest",
			title:             "Suspect arrested for assault in park",
			expectedRelevance: "core_street_crime",
			expectedTypes:     []string{"violent_crime"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != tt.expectedRelevance {
				t.Errorf("relevance: got %s, want %s", result.relevance, tt.expectedRelevance)
			}

			for _, expectedType := range tt.expectedTypes {
				found := false
				for _, actualType := range result.crimeTypes {
					if actualType == expectedType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing crime type %s in %v", expectedType, result.crimeTypes)
				}
			}
		})
	}
}

func TestStreetCodeRules_ClassifyByRules_Exclusions(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"job posting", "Full-Time Position Available"},
		{"directory", "Listings By Category"},
		{"sports", "Local Sports Updates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != "not_crime" {
				t.Errorf("expected not_crime for excluded content, got %s", result.relevance)
			}
		})
	}
}

func TestStreetCodeRules_ClassifyByRules_NotCrime(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		title string
	}{
		{"restaurant", "New restaurant opens downtown"},
		{"weather", "Weekend forecast looks sunny"},
		{"hockey", "Hockey team wins championship"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByRules(tt.title, "")

			if result.relevance != "not_crime" {
				t.Errorf("expected not_crime, got %s", result.relevance)
			}
		})
	}
}
