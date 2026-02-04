package domain_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
)

func TestCrimeInfo_IsCrimeRelated(t *testing.T) {
	t.Helper()
	tests := []struct {
		name     string
		crime    *domain.CrimeInfo
		expected bool
	}{
		{"nil crime", nil, false},
		{"not_crime relevance", &domain.CrimeInfo{Relevance: "not_crime"}, false},
		{"core_street_crime relevance", &domain.CrimeInfo{Relevance: "core_street_crime"}, true},
		{"peripheral_crime relevance", &domain.CrimeInfo{Relevance: "peripheral_crime"}, true},
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

func TestDocument_ComputedIsCrimeRelated(t *testing.T) {
	t.Helper()
	tests := []struct {
		name     string
		doc      domain.Document
		expected bool
	}{
		{
			name:     "nil crime",
			doc:      domain.Document{},
			expected: false,
		},
		{
			name:     "crime related",
			doc:      domain.Document{Crime: &domain.CrimeInfo{Relevance: "core_street_crime"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.doc.ComputedIsCrimeRelated()
			if got != tt.expected {
				t.Errorf("ComputedIsCrimeRelated() = %v, want %v", got, tt.expected)
			}
		})
	}
}
