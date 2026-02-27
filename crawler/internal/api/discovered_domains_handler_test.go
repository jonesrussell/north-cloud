//nolint:testpackage // Testing unexported functions isValidDomainStatus, extractPathPattern, extractPath, computePathClusters
package api

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestIsValidDomainStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"active is valid", domain.DomainStatusActive, true},
		{"ignored is valid", domain.DomainStatusIgnored, true},
		{"reviewing is valid", domain.DomainStatusReviewing, true},
		{"promoted is valid", domain.DomainStatusPromoted, true},
		{"invalid string rejected", "invalid", false},
		{"empty string rejected", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isValidDomainStatus(tt.status)
			if got != tt.want {
				t.Errorf("isValidDomainStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestExtractPathPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "multi-segment yields wildcard",
			rawURL: "https://example.com/news/article/123",
			want:   "/news/*",
		},
		{
			name:   "single segment no wildcard",
			rawURL: "https://example.com/about",
			want:   "/about",
		},
		{
			name:   "root path",
			rawURL: "https://example.com/",
			want:   "/",
		},
		{
			name:   "no trailing slash",
			rawURL: "https://example.com",
			want:   "/",
		},
		{
			name:   "invalid URL returns root",
			rawURL: "://broken",
			want:   "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractPathPattern(tt.rawURL)
			if got != tt.want {
				t.Errorf("extractPathPattern(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestExtractPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "normal path",
			rawURL: "https://example.com/news/article",
			want:   "/news/article",
		},
		{
			name:   "root URL returns slash",
			rawURL: "https://example.com",
			want:   "/",
		},
		{
			name:   "root with trailing slash",
			rawURL: "https://example.com/",
			want:   "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := extractPath(tt.rawURL)
			if got != tt.want {
				t.Errorf("extractPath(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestComputePathClusters(t *testing.T) {
	t.Parallel()

	t.Run("empty input yields zero clusters", func(t *testing.T) {
		t.Parallel()

		clusters := computePathClusters(nil)
		if len(clusters) != 0 {
			t.Errorf("expected 0 clusters, got %d", len(clusters))
		}
	})

	t.Run("groups by pattern and sorts descending", func(t *testing.T) {
		t.Parallel()

		links := []*domain.DiscoveredLink{
			{URL: "https://example.com/news/article1"},
			{URL: "https://example.com/news/article2"},
			{URL: "https://example.com/news/article3"},
			{URL: "https://example.com/about"},
		}

		clusters := computePathClusters(links)

		expectedClusterCount := 2
		if len(clusters) != expectedClusterCount {
			t.Fatalf("expected %d clusters, got %d", expectedClusterCount, len(clusters))
		}

		// First cluster should be the most frequent
		expectedTopPattern := "/news/*"
		expectedTopCount := 3

		if clusters[0].Pattern != expectedTopPattern {
			t.Errorf("first cluster pattern = %q, want %q", clusters[0].Pattern, expectedTopPattern)
		}

		if clusters[0].Count != expectedTopCount {
			t.Errorf("first cluster count = %d, want %d", clusters[0].Count, expectedTopCount)
		}

		// Second cluster
		expectedSecondPattern := "/about"
		expectedSecondCount := 1

		if clusters[1].Pattern != expectedSecondPattern {
			t.Errorf("second cluster pattern = %q, want %q", clusters[1].Pattern, expectedSecondPattern)
		}

		if clusters[1].Count != expectedSecondCount {
			t.Errorf("second cluster count = %d, want %d", clusters[1].Count, expectedSecondCount)
		}
	})
}
