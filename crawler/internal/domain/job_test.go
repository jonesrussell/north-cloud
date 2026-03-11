package domain_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestValidJobType(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		jobType  string
		expected bool
	}{
		{"crawl is valid", domain.JobTypeCrawl, true},
		{"leadership_scrape is valid", domain.JobTypeLeadershipScrape, true},
		{"empty is invalid", "", false},
		{"unknown is invalid", "unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.ValidJobType(tt.jobType); got != tt.expected {
				t.Errorf("ValidJobType(%q) = %v, want %v", tt.jobType, got, tt.expected)
			}
		})
	}
}
