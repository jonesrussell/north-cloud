//nolint:testpackage // White-box test for parseFacets and recipe/job facet parsing
package service

import (
	"testing"

	"github.com/jonesrussell/north-cloud/search/internal/domain"
)

func TestParseFacets_RecipeAndJobPopulated(t *testing.T) {
	t.Helper()

	s := &SearchService{}
	aggs := map[string]aggregation{
		"recipe_cuisines": {
			Buckets: []aggregationBucket{
				{Key: "italian", DocCount: 10},
				{Key: "french", DocCount: 5},
			},
		},
		"recipe_categories": {
			Buckets: []aggregationBucket{
				{Key: "dessert", DocCount: 8},
			},
		},
		"job_types": {
			Buckets: []aggregationBucket{
				{Key: "full_time", DocCount: 20},
			},
		},
		"job_industries": {
			Buckets: []aggregationBucket{
				{Key: "technology", DocCount: 15},
			},
		},
		"job_locations": {
			Buckets: []aggregationBucket{
				{Key: "Toronto", DocCount: 12},
			},
		},
	}

	facets := s.parseFacets(aggs)
	if facets == nil {
		t.Fatal("parseFacets returned nil")
	}

	if len(facets.RecipeCuisines) != 2 {
		t.Errorf("RecipeCuisines: want 2 buckets, got %d", len(facets.RecipeCuisines))
	}
	if len(facets.RecipeCategories) != 1 {
		t.Errorf("RecipeCategories: want 1 bucket, got %d", len(facets.RecipeCategories))
	}
	if len(facets.JobTypes) != 1 {
		t.Errorf("JobTypes: want 1 bucket, got %d", len(facets.JobTypes))
	}
	if len(facets.JobIndustries) != 1 {
		t.Errorf("JobIndustries: want 1 bucket, got %d", len(facets.JobIndustries))
	}
	if len(facets.JobLocations) != 1 {
		t.Errorf("JobLocations: want 1 bucket, got %d", len(facets.JobLocations))
	}

	assertFacetBucket(t, facets.RecipeCuisines, "italian", 10)
	assertFacetBucket(t, facets.RecipeCuisines, "french", 5)
	assertFacetBucket(t, facets.JobTypes, "full_time", 20)
}

func assertFacetBucket(t *testing.T, buckets []domain.FacetBucket, key string, count int64) {
	t.Helper()
	for _, b := range buckets {
		if b.Key == key {
			if b.Count != count {
				t.Errorf("bucket %q: want count %d, got %d", key, count, b.Count)
			}
			return
		}
	}
	t.Errorf("bucket with key %q not found", key)
}

func TestFormatFacetLabel(t *testing.T) {
	t.Helper()

	cases := []struct {
		key  string
		want string
	}{
		{"local_news", "Local News"},
		{"breaking_news", "Breaking News"},
		{"real_estate", "Real Estate"},
		{"education", "Education"},
		{"crime", "Crime"},
		{"indigenous", "Indigenous"},
		{"full_time", "Full Time"},
		{"italian", "Italian"},
	}

	for _, tc := range cases {
		got := formatFacetLabel(tc.key)
		if got != tc.want {
			t.Errorf("formatFacetLabel(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestParseBuckets_SetsLabel(t *testing.T) {
	t.Helper()

	agg := aggregation{
		Buckets: []aggregationBucket{
			{Key: "local_news", DocCount: 10},
			{Key: "crime", DocCount: 5},
		},
	}

	buckets := parseBuckets(agg)

	if len(buckets) != 2 {
		t.Fatalf("want 2 buckets, got %d", len(buckets))
	}
	if buckets[0].Label != "Local News" {
		t.Errorf("bucket[0].Label = %q, want %q", buckets[0].Label, "Local News")
	}
	if buckets[1].Label != "Crime" {
		t.Errorf("bucket[1].Label = %q, want %q", buckets[1].Label, "Crime")
	}
}

func TestParseFacets_MissingRecipeJobAggsLeaveEmpty(t *testing.T) {
	t.Helper()

	s := &SearchService{}
	aggs := map[string]aggregation{
		"topics": {Buckets: []aggregationBucket{{Key: "crime", DocCount: 5}}},
	}

	facets := s.parseFacets(aggs)
	if facets == nil {
		t.Fatal("parseFacets returned nil")
	}
	if facets.RecipeCuisines != nil {
		t.Error("RecipeCuisines should be nil when agg missing")
	}
	if facets.JobTypes != nil {
		t.Error("JobTypes should be nil when agg missing")
	}
}
