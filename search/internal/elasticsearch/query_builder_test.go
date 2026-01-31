package elasticsearch_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/domain"
	"github.com/jonesrussell/north-cloud/search/internal/elasticsearch"
)

func getTestConfig() *config.ElasticsearchConfig {
	return &config.ElasticsearchConfig{
		ClassifiedContentPattern: "*_classified_content",
		HighlightEnabled:         true,
		HighlightFragmentSize:    150,
		HighlightMaxFragments:    3,
		DefaultBoost: config.BoostConfig{
			Title:           3.0,
			OGTitle:         2.0,
			RawText:         1.0,
			OGDescription:   1.5,
			MetaDescription: 1.5,
		},
	}
}

func getDefaultSearchRequest(query string) *domain.SearchRequest {
	return &domain.SearchRequest{
		Query:   query,
		Filters: &domain.Filters{},
		Pagination: &domain.Pagination{
			Page: 1,
			Size: 10,
		},
		Sort: &domain.Sort{
			Field: "relevance",
			Order: "desc",
		},
		Options: &domain.Options{
			IncludeHighlights: false,
			IncludeFacets:     false,
		},
	}
}

func TestNewQueryBuilder(t *testing.T) {
	t.Helper()

	cfg := getTestConfig()
	qb := elasticsearch.NewQueryBuilder(cfg)
	if qb == nil {
		t.Fatal("NewQueryBuilder() returned nil")
	}
}

func TestQueryBuilder_Build_BasicQuery(t *testing.T) {
	t.Helper()

	cfg := getTestConfig()
	qb := elasticsearch.NewQueryBuilder(cfg)

	req := getDefaultSearchRequest("crime news")
	query := qb.Build(req)

	// Verify query structure
	if query == nil {
		t.Fatal("Build() returned nil")
	}

	// Should have query, from, size at minimum
	if _, ok := query["query"]; !ok {
		t.Error("Build() missing 'query' field")
	}
	if _, ok := query["from"]; !ok {
		t.Error("Build() missing 'from' field")
	}
	if _, ok := query["size"]; !ok {
		t.Error("Build() missing 'size' field")
	}
	if _, ok := query["sort"]; !ok {
		t.Error("Build() missing 'sort' field")
	}
}

func TestQueryBuilder_Build_WithFilters(t *testing.T) {
	t.Helper()

	cfg := getTestConfig()
	qb := elasticsearch.NewQueryBuilder(cfg)

	req := getDefaultSearchRequest("test")
	req.Filters = &domain.Filters{
		Topics:          []string{"crime", "local"},
		ContentType:     "article",
		MinQualityScore: 50,
	}

	query := qb.Build(req)

	// Verify query built successfully with filters
	if query == nil {
		t.Fatal("Build() returned nil for request with filters")
	}

	// Query should have a bool query with filters
	queryField, ok := query["query"].(map[string]any)
	if !ok {
		t.Fatal("Build() 'query' field not a map")
	}

	boolQuery, hasBool := queryField["bool"].(map[string]any)
	if !hasBool {
		t.Fatal("Build() query should contain 'bool' clause")
	}

	// Should have filter clause
	if _, hasFilter := boolQuery["filter"]; !hasFilter {
		t.Error("Build() with filters should have 'filter' clause")
	}
}

func TestQueryBuilder_Build_Pagination(t *testing.T) {
	t.Helper()

	cfg := getTestConfig()
	qb := elasticsearch.NewQueryBuilder(cfg)

	testCases := []struct {
		name     string
		page     int
		size     int
		wantFrom int
		wantSize int
	}{
		{"first page", 1, 10, 0, 10},
		{"second page", 2, 10, 10, 10},
		{"third page", 3, 20, 40, 20},
		{"large page", 10, 50, 450, 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := getDefaultSearchRequest("test")
			req.Pagination = &domain.Pagination{
				Page: tc.page,
				Size: tc.size,
			}

			query := qb.Build(req)

			from, ok := query["from"].(int)
			if !ok {
				t.Fatal("Build() 'from' not an int")
			}
			if from != tc.wantFrom {
				t.Errorf("Build() from = %d, want %d", from, tc.wantFrom)
			}

			size, ok := query["size"].(int)
			if !ok {
				t.Fatal("Build() 'size' not an int")
			}
			if size != tc.wantSize {
				t.Errorf("Build() size = %d, want %d", size, tc.wantSize)
			}
		})
	}
}

func TestQueryBuilder_Build_EmptyQuery(t *testing.T) {
	t.Helper()

	cfg := getTestConfig()
	qb := elasticsearch.NewQueryBuilder(cfg)

	req := getDefaultSearchRequest("")
	query := qb.Build(req)

	// Empty query should still build
	if query == nil {
		t.Fatal("Build() returned nil for empty query")
	}

	// Should still have the basic structure
	if _, ok := query["query"]; !ok {
		t.Error("Build() with empty query should still have 'query' field")
	}
}

func TestQueryBuilder_Build_WithHighlights(t *testing.T) {
	t.Helper()

	cfg := getTestConfig()
	qb := elasticsearch.NewQueryBuilder(cfg)

	req := getDefaultSearchRequest("test")
	req.Options = &domain.Options{
		IncludeHighlights: true,
		IncludeFacets:     false,
	}

	query := qb.Build(req)

	if query == nil {
		t.Fatal("Build() returned nil")
	}

	// Should have highlight configuration
	if _, ok := query["highlight"]; !ok {
		t.Error("Build() with highlights enabled should have 'highlight' field")
	}
}

func TestQueryBuilder_Build_WithFacets(t *testing.T) {
	t.Helper()

	cfg := getTestConfig()
	qb := elasticsearch.NewQueryBuilder(cfg)

	req := getDefaultSearchRequest("test")
	req.Options = &domain.Options{
		IncludeHighlights: false,
		IncludeFacets:     true,
	}

	query := qb.Build(req)

	if query == nil {
		t.Fatal("Build() returned nil")
	}

	// Should have aggregations
	if _, ok := query["aggs"]; !ok {
		t.Error("Build() with facets enabled should have 'aggs' field")
	}
}

func TestQueryBuilder_Build_SortOptions(t *testing.T) {
	t.Helper()

	cfg := getTestConfig()
	qb := elasticsearch.NewQueryBuilder(cfg)

	sortFields := []string{"relevance", "published_date", "quality_score", "crawled_at"}

	for _, field := range sortFields {
		t.Run(field, func(t *testing.T) {
			req := getDefaultSearchRequest("test")
			req.Sort = &domain.Sort{
				Field: field,
				Order: "desc",
			}

			query := qb.Build(req)

			if query == nil {
				t.Fatalf("Build() returned nil for sort field %s", field)
			}

			sortField, ok := query["sort"].([]any)
			if !ok {
				t.Fatal("Build() 'sort' should be an array")
			}

			if len(sortField) == 0 {
				t.Error("Build() 'sort' should not be empty")
			}
		})
	}
}
