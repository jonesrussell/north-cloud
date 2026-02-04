package domain

import (
	"testing"
)

func TestCrimeInfo_IsCrimeRelated(t *testing.T) {
	t.Helper()
	tests := []struct {
		name     string
		crime    *CrimeInfo
		expected bool
	}{
		{"nil crime", nil, false},
		{"not_crime relevance", &CrimeInfo{Relevance: "not_crime"}, false},
		{"core_street_crime relevance", &CrimeInfo{Relevance: "core_street_crime"}, true},
		{"peripheral_crime relevance", &CrimeInfo{Relevance: "peripheral_crime"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.crime.IsCrimeRelated()
			if got != tt.expected {
				t.Errorf("IsCrimeRelated() = %v, want %v", got, tt.expected)
			}
		})
	}
}
