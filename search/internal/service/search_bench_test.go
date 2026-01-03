package service_test

import (
	"encoding/json"
	"testing"
	"time"
)

// SearchRequest represents a search query for benchmarking
type SearchRequest struct {
	Query       string
	Topics      []string
	ContentType string
	MinQuality  int
	Page        int
	PageSize    int
	SortBy      string
	SortOrder   string
}

// SearchResult represents a search result document
type SearchResult struct {
	SourceID      string              `json:"source_id"`
	URL           string              `json:"url"`
	Title         string              `json:"title"`
	Body          string              `json:"body"`
	QualityScore  int                 `json:"quality_score"`
	Topics        []string            `json:"topics"`
	ContentType   string              `json:"content_type"`
	PublishedDate time.Time           `json:"published_date"`
	Highlight     map[string][]string `json:"highlight"`
	Metadata      map[string]any      `json:"metadata"`
}

// BenchmarkFullTextSearch benchmarks multi-field search query construction
func BenchmarkFullTextSearch(b *testing.B) {
	searchReq := SearchRequest{
		Query:    "crime news police downtown",
		Page:     1,
		PageSize: 20,
		SortBy:   "_score",
	}
	_ = searchReq.SortBy

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Build multi-match query
		query := map[string]any{
			"query": map[string]any{
				"bool": map[string]any{
					"must": []any{
						map[string]any{
							"multi_match": map[string]any{
								"query":     searchReq.Query,
								"fields":    []string{"title^3", "og_tags.og:title^2", "body"},
								"type":      "best_fields",
								"fuzziness": "AUTO",
							},
						},
					},
				},
			},
			"highlight": map[string]any{
				"fields": map[string]any{
					"title": map[string]any{},
					"body":  map[string]any{},
				},
			},
			"from": (searchReq.Page - 1) * searchReq.PageSize,
			"size": searchReq.PageSize,
		}

		_, err := json.Marshal(query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFacetedSearch benchmarks search with aggregations and filters
func BenchmarkFacetedSearch(b *testing.B) {
	searchReq := SearchRequest{
		Query:       "breaking news",
		Topics:      []string{"crime", "local"},
		ContentType: "article",
		MinQuality:  70,
		Page:        1,
		PageSize:    20,
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Build complex filtered query with aggregations
		query := map[string]any{
			"query": map[string]any{
				"bool": map[string]any{
					"must": []any{
						map[string]any{
							"multi_match": map[string]any{
								"query":  searchReq.Query,
								"fields": []string{"title^3", "body"},
							},
						},
					},
					"filter": []any{
						map[string]any{
							"terms": map[string]any{
								"topics": searchReq.Topics,
							},
						},
						map[string]any{
							"term": map[string]any{
								"content_type": searchReq.ContentType,
							},
						},
						map[string]any{
							"range": map[string]any{
								"quality_score": map[string]any{
									"gte": searchReq.MinQuality,
								},
							},
						},
					},
				},
			},
			"aggs": map[string]any{
				"by_topic": map[string]any{
					"terms": map[string]any{
						"field": "topics",
						"size":  10,
					},
				},
				"by_content_type": map[string]any{
					"terms": map[string]any{
						"field": "content_type",
						"size":  5,
					},
				},
				"avg_quality": map[string]any{
					"avg": map[string]any{
						"field": "quality_score",
					},
				},
			},
			"from": (searchReq.Page - 1) * searchReq.PageSize,
			"size": searchReq.PageSize,
		}

		_, err := json.Marshal(query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSearchHighlighting benchmarks search result highlighting
func BenchmarkSearchHighlighting(b *testing.B) {
	results := make([]SearchResult, 20)
	for i := range 20 {
		results[i] = SearchResult{
			URL:   "https://example.com/article",
			Title: "Breaking News: Major Crime Event Downtown",
			Body:  "Police responded to a major crime incident in the downtown area last night. Multiple suspects were arrested following an investigation. The crime scene was secured and evidence was collected.",
			Highlight: map[string][]string{
				"title": {"Breaking News: Major <em>Crime</em> Event Downtown"},
				"body": {
					"Police responded to a major <em>crime</em> incident",
					"Multiple suspects were arrested following an investigation",
					"The <em>crime</em> scene was secured",
				},
			},
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Process highlighting for all results
		for i := range results {
			result := &results[i]
			// Extract highlighted title
			if titleHighlights, ok := result.Highlight["title"]; ok && len(titleHighlights) > 0 {
				_ = titleHighlights[0]
			}

			// Extract highlighted body snippets
			if bodyHighlights, ok := result.Highlight["body"]; ok {
				_ = bodyHighlights
			}
		}
	}
}

// BenchmarkResultSorting benchmarks sorting logic for search results
func BenchmarkResultSorting(b *testing.B) {
	sortOptions := []struct {
		field string
		order string
	}{
		{"_score", "desc"},
		{"published_date", "desc"},
		{"quality_score", "desc"},
		{"title.keyword", "asc"},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for _, sortOpt := range sortOptions {
			sort := []map[string]any{
				{
					sortOpt.field: map[string]string{
						"order": sortOpt.order,
					},
				},
			}

			_, err := json.Marshal(sort)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkPagination benchmarks pagination calculation
func BenchmarkPagination(b *testing.B) {
	testCases := []struct {
		page     int
		pageSize int
		total    int
	}{
		{1, 20, 1000},
		{5, 50, 500},
		{10, 100, 10000},
		{1, 10, 25},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for _, tc := range testCases {
			// Calculate pagination
			from := (tc.page - 1) * tc.pageSize
			size := tc.pageSize
			totalPages := (tc.total + tc.pageSize - 1) / tc.pageSize
			hasMore := tc.page < totalPages

			_ = from
			_ = size
			_ = totalPages
			_ = hasMore
		}
	}
}

// BenchmarkDateRangeFilter benchmarks date range filtering
func BenchmarkDateRangeFilter(b *testing.B) {
	now := time.Now()
	ranges := []struct {
		from time.Time
		to   time.Time
	}{
		{now.AddDate(0, 0, -7), now},  // Last 7 days
		{now.AddDate(0, -1, 0), now},  // Last month
		{now.AddDate(0, 0, -30), now}, // Last 30 days
		{now.AddDate(-1, 0, 0), now},  // Last year
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for _, r := range ranges {
			filter := map[string]any{
				"range": map[string]any{
					"published_date": map[string]any{
						"gte": r.from.Format(time.RFC3339),
						"lte": r.to.Format(time.RFC3339),
					},
				},
			}

			_, err := json.Marshal(filter)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkQueryParsing benchmarks parsing user search queries
func BenchmarkQueryParsing(b *testing.B) {
	queries := []string{
		"crime downtown",
		"breaking news police",
		"\"exact phrase search\"",
		"crime AND police",
		"robbery OR theft",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		for _, query := range queries {
			// Simple query parsing simulation
			terms := []string{}
			exact := false

			// Check for exact phrase
			if len(query) > 2 && query[0] == '"' && query[len(query)-1] == '"' {
				exact = true
				terms = append(terms, query[1:len(query)-1])
			} else {
				// Split on spaces (simplified)
				terms = append(terms, query)
			}

			_ = exact
			_ = terms
		}
	}
}
