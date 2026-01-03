package service

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
	SourceID      string                 `json:"source_id"`
	URL           string                 `json:"url"`
	Title         string                 `json:"title"`
	Body          string                 `json:"body"`
	QualityScore  int                    `json:"quality_score"`
	Topics        []string               `json:"topics"`
	ContentType   string                 `json:"content_type"`
	PublishedDate time.Time              `json:"published_date"`
	Highlight     map[string][]string    `json:"highlight"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// BenchmarkFullTextSearch benchmarks multi-field search query construction
func BenchmarkFullTextSearch(b *testing.B) {
	searchReq := SearchRequest{
		Query:    "crime news police downtown",
		Page:     1,
		PageSize: 20,
		SortBy:   "_score",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Build multi-match query
		query := map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"must": []interface{}{
						map[string]interface{}{
							"multi_match": map[string]interface{}{
								"query":  searchReq.Query,
								"fields": []string{"title^3", "og_tags.og:title^2", "body"},
								"type":   "best_fields",
								"fuzziness": "AUTO",
							},
						},
					},
				},
			},
			"highlight": map[string]interface{}{
				"fields": map[string]interface{}{
					"title": map[string]interface{}{},
					"body":  map[string]interface{}{},
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

	for i := 0; i < b.N; i++ {
		// Build complex filtered query with aggregations
		query := map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"must": []interface{}{
						map[string]interface{}{
							"multi_match": map[string]interface{}{
								"query":  searchReq.Query,
								"fields": []string{"title^3", "body"},
							},
						},
					},
					"filter": []interface{}{
						map[string]interface{}{
							"terms": map[string]interface{}{
								"topics": searchReq.Topics,
							},
						},
						map[string]interface{}{
							"term": map[string]interface{}{
								"content_type": searchReq.ContentType,
							},
						},
						map[string]interface{}{
							"range": map[string]interface{}{
								"quality_score": map[string]interface{}{
									"gte": searchReq.MinQuality,
								},
							},
						},
					},
				},
			},
			"aggs": map[string]interface{}{
				"by_topic": map[string]interface{}{
					"terms": map[string]interface{}{
						"field": "topics",
						"size":  10,
					},
				},
				"by_content_type": map[string]interface{}{
					"terms": map[string]interface{}{
						"field": "content_type",
						"size":  5,
					},
				},
				"avg_quality": map[string]interface{}{
					"avg": map[string]interface{}{
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
	for i := 0; i < 20; i++ {
		results[i] = SearchResult{
			URL:   "https://example.com/article",
			Title: "Breaking News: Major Crime Event Downtown",
			Body:  "Police responded to a major crime incident in the downtown area last night. Multiple suspects were arrested following an investigation. The crime scene was secured and evidence was collected.",
			Highlight: map[string][]string{
				"title": {"Breaking News: Major <em>Crime</em> Event Downtown"},
				"body":  {
					"Police responded to a major <em>crime</em> incident",
					"Multiple suspects were arrested following an investigation",
					"The <em>crime</em> scene was secured",
				},
			},
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Process highlighting for all results
		for _, result := range results {
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

	for i := 0; i < b.N; i++ {
		for _, sortOpt := range sortOptions {
			sort := []map[string]interface{}{
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

	for i := 0; i < b.N; i++ {
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
		{now.AddDate(0, 0, -7), now},    // Last 7 days
		{now.AddDate(0, -1, 0), now},    // Last month
		{now.AddDate(0, 0, -30), now},   // Last 30 days
		{now.AddDate(-1, 0, 0), now},    // Last year
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, r := range ranges {
			filter := map[string]interface{}{
				"range": map[string]interface{}{
					"published_date": map[string]interface{}{
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

	for i := 0; i < b.N; i++ {
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
