package elasticsearch

import (
	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/domain"
)

// QueryBuilder builds Elasticsearch queries from search requests
type QueryBuilder struct {
	config *config.ElasticsearchConfig
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(cfg *config.ElasticsearchConfig) *QueryBuilder {
	return &QueryBuilder{
		config: cfg,
	}
}

// Build constructs the complete Elasticsearch query
func (qb *QueryBuilder) Build(req *domain.SearchRequest) map[string]interface{} {
	query := map[string]interface{}{
		"query": qb.buildBoolQuery(req),
		"from":  (req.Pagination.Page - 1) * req.Pagination.Size,
		"size":  req.Pagination.Size,
		"sort":  qb.buildSort(req),
	}

	// Add highlighting if enabled
	if req.Options.IncludeHighlights && qb.config.HighlightEnabled {
		query["highlight"] = qb.buildHighlight()
	}

	// Add aggregations if enabled
	if req.Options.IncludeFacets {
		query["aggs"] = qb.buildAggregations()
	}

	// Field filtering to reduce payload size
	if len(req.Options.SourceFields) > 0 {
		query["_source"] = req.Options.SourceFields
	} else {
		// Default fields to return
		query["_source"] = []string{
			"id", "title", "url", "source_name",
			"published_date", "crawled_at",
			"quality_score", "content_type", "topics",
			"is_crime_related",
		}
	}

	// Enable total hits tracking
	query["track_total_hits"] = true

	return query
}

// buildBoolQuery constructs the bool query with must, filter, and should clauses
func (qb *QueryBuilder) buildBoolQuery(req *domain.SearchRequest) map[string]interface{} {
	boolQuery := map[string]interface{}{
		"must":   []interface{}{},
		"filter": []interface{}{},
		"should": []interface{}{},
	}

	// Multi-match query for full-text search
	if req.Query != "" {
		boolQuery["must"] = []interface{}{
			qb.buildMultiMatchQuery(req.Query),
		}
	}

	// Add filters
	filters := qb.buildFilters(req.Filters)
	if len(filters) > 0 {
		boolQuery["filter"] = filters
	}

	// Add boosting for recency and quality
	shouldClauses := qb.buildBoosts()
	if len(shouldClauses) > 0 {
		boolQuery["should"] = shouldClauses
	}

	return map[string]interface{}{"bool": boolQuery}
}

// buildMultiMatchQuery creates a multi-match query with field boosting
func (qb *QueryBuilder) buildMultiMatchQuery(query string) map[string]interface{} {
	boost := qb.config.DefaultBoost

	return map[string]interface{}{
		"multi_match": map[string]interface{}{
			"query": query,
			"fields": []string{
				"title^" + floatToString(boost.Title),
				"og_title^" + floatToString(boost.OGTitle),
				"raw_text^" + floatToString(boost.RawText),
				"og_description^" + floatToString(boost.OGDescription),
				"meta_description^" + floatToString(boost.MetaDescription),
			},
			"type":                 "best_fields",
			"operator":             "or",
			"fuzziness":            "AUTO",
			"minimum_should_match": "75%",
		},
	}
}

// buildFilters constructs filter clauses
func (qb *QueryBuilder) buildFilters(filters *domain.Filters) []interface{} {
	var result []interface{}

	// Topics filter
	if len(filters.Topics) > 0 {
		result = append(result, map[string]interface{}{
			"terms": map[string]interface{}{
				"topics": filters.Topics,
			},
		})
	}

	// Content type filter
	if filters.ContentType != "" {
		result = append(result, map[string]interface{}{
			"term": map[string]interface{}{
				"content_type": filters.ContentType,
			},
		})
	}

	// Quality score range filter
	if filters.MinQualityScore > 0 || filters.MaxQualityScore < 100 {
		result = append(result, map[string]interface{}{
			"range": map[string]interface{}{
				"quality_score": map[string]interface{}{
					"gte": filters.MinQualityScore,
					"lte": filters.MaxQualityScore,
				},
			},
		})
	}

	// Crime-related filter
	if filters.IsCrimeRelated != nil {
		result = append(result, map[string]interface{}{
			"term": map[string]interface{}{
				"is_crime_related": *filters.IsCrimeRelated,
			},
		})
	}

	// Source names filter
	if len(filters.SourceNames) > 0 {
		result = append(result, map[string]interface{}{
			"terms": map[string]interface{}{
				"source_name": filters.SourceNames,
			},
		})
	}

	// Date range filter
	if filters.FromDate != nil || filters.ToDate != nil {
		dateRange := map[string]interface{}{}
		if filters.FromDate != nil {
			dateRange["gte"] = filters.FromDate.Format("2006-01-02T15:04:05Z07:00")
		}
		if filters.ToDate != nil {
			dateRange["lte"] = filters.ToDate.Format("2006-01-02T15:04:05Z07:00")
		}
		result = append(result, map[string]interface{}{
			"range": map[string]interface{}{
				"published_date": dateRange,
			},
		})
	}

	return result
}

// buildBoosts adds score boosting for recency and quality
func (qb *QueryBuilder) buildBoosts() []interface{} {
	return []interface{}{
		// Boost recent content (recency decay)
		map[string]interface{}{
			"function_score": map[string]interface{}{
				"gauss": map[string]interface{}{
					"published_date": map[string]interface{}{
						"origin": "now",
						"scale":  "30d",
						"decay":  0.5,
					},
				},
			},
		},
		// Boost high-quality content
		map[string]interface{}{
			"function_score": map[string]interface{}{
				"field_value_factor": map[string]interface{}{
					"field":    "quality_score",
					"factor":   0.01,
					"modifier": "log1p",
				},
			},
		},
	}
}

// buildSort constructs sort criteria
func (qb *QueryBuilder) buildSort(req *domain.SearchRequest) []interface{} {
	var sortCriteria []interface{}

	switch req.Sort.Field {
	case "relevance":
		sortCriteria = append(sortCriteria, map[string]interface{}{
			"_score": map[string]interface{}{
				"order": req.Sort.Order,
			},
		})
	case "published_date":
		sortCriteria = append(sortCriteria, map[string]interface{}{
			"published_date": map[string]interface{}{
				"order": req.Sort.Order,
			},
		})
	case "quality_score":
		sortCriteria = append(sortCriteria, map[string]interface{}{
			"quality_score": map[string]interface{}{
				"order": req.Sort.Order,
			},
		})
	case "crawled_at":
		sortCriteria = append(sortCriteria, map[string]interface{}{
			"crawled_at": map[string]interface{}{
				"order": req.Sort.Order,
			},
		})
	}

	// Always add secondary sort by relevance score
	if req.Sort.Field != "relevance" {
		sortCriteria = append(sortCriteria, map[string]interface{}{
			"_score": map[string]interface{}{
				"order": "desc",
			},
		})
	}

	return sortCriteria
}

// buildHighlight constructs highlight configuration
func (qb *QueryBuilder) buildHighlight() map[string]interface{} {
	return map[string]interface{}{
		"fields": map[string]interface{}{
			"title": map[string]interface{}{
				"number_of_fragments": 1,
			},
			"raw_text": map[string]interface{}{
				"fragment_size":       qb.config.HighlightFragmentSize,
				"number_of_fragments": qb.config.HighlightMaxFragments,
			},
		},
		"pre_tags":  []string{"<em>"},
		"post_tags": []string{"</em>"},
	}
}

// buildAggregations constructs faceted search aggregations
func (qb *QueryBuilder) buildAggregations() map[string]interface{} {
	return map[string]interface{}{
		"topics": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "topics",
				"size":  20,
			},
		},
		"content_types": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "content_type",
				"size":  10,
			},
		},
		"sources": map[string]interface{}{
			"terms": map[string]interface{}{
				"field": "source_name",
				"size":  50,
			},
		},
		"quality_ranges": map[string]interface{}{
			"range": map[string]interface{}{
				"field": "quality_score",
				"ranges": []map[string]interface{}{
					{"key": "0-39", "from": 0, "to": 40},
					{"key": "40-59", "from": 40, "to": 60},
					{"key": "60-79", "from": 60, "to": 80},
					{"key": "80-100", "from": 80, "to": 101},
				},
			},
		},
	}
}

// floatToString converts float64 to string for field boosting
func floatToString(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

// Helper to import fmt
import "fmt"
