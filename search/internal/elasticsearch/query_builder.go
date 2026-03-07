package elasticsearch

import (
	"fmt"
	"strings"

	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/domain"
)

const (
	maxQualityScoreValue = 100
	recencyDecayFactor   = 0.5
	qualityBoostFactor   = 0.01
	topicsAggSize        = 20
	contentTypesAggSize  = 10
	sourcesAggSize       = 50
	qualityRangeLow      = 40
	qualityRangeMid      = 60
	qualityRangeHigh     = 80
	qualityRangeMax      = 101
	recipeFacetSize      = 20
	jobFacetSize         = 20
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
func (qb *QueryBuilder) Build(req *domain.SearchRequest) map[string]any {
	query := map[string]any{
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
		// Note: published_date may not exist in all documents, but Elasticsearch handles missing fields gracefully
		query["_source"] = []string{
			"id", "title", "url", "source_name",
			"published_date", "crawled_at",
			"quality_score", "content_type", "topics",
			"crime", "body", "raw_text", "og_image",
			"rfp",
		}
	}

	// Enable total hits tracking
	query["track_total_hits"] = true

	return query
}

// buildBoolQuery constructs the bool query with must, filter, and should clauses
func (qb *QueryBuilder) buildBoolQuery(req *domain.SearchRequest) map[string]any {
	boolQuery := map[string]any{
		"must":   []any{},
		"filter": []any{},
		"should": []any{},
	}

	// Multi-match query for full-text search.
	// Treat "*" as match-all (empty must clause); multi-match doesn't interpret "*" as wildcard.
	if req.Query != "" && req.Query != "*" {
		boolQuery["must"] = []any{
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

	return map[string]any{"bool": boolQuery}
}

// buildMultiMatchQuery creates a multi-match query with field boosting
func (qb *QueryBuilder) buildMultiMatchQuery(query string) map[string]any {
	boost := qb.config.DefaultBoost

	// Count words in query to adjust minimum_should_match
	words := len(strings.Fields(query))

	// For single-word queries, don't use minimum_should_match
	// For multi-word queries, use a more lenient setting
	multiMatch := map[string]any{
		"query": query,
		"fields": []string{
			"title^" + floatToString(boost.Title),
			"og_title^" + floatToString(boost.OGTitle),
			"body^" + floatToString(boost.RawText),
			"raw_text^" + floatToString(boost.RawText),
			"og_description^" + floatToString(boost.OGDescription),
			"meta_description^" + floatToString(boost.MetaDescription),
		},
		"type":      "best_fields",
		"operator":  "or",
		"fuzziness": "AUTO",
	}

	// Only add minimum_should_match for multi-word queries
	if words > 1 {
		multiMatch["minimum_should_match"] = "75%"
	}

	return map[string]any{
		"multi_match": multiMatch,
	}
}

// buildFilters constructs filter clauses.
// Validate() initializes req.Filters so Build() is safe; nil filters return no clauses.
func (qb *QueryBuilder) buildFilters(filters *domain.Filters) []any {
	if filters == nil {
		return nil
	}
	var result []any

	// Topics filter - use .keyword subfield for text fields
	if len(filters.Topics) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"topics.keyword": filters.Topics,
			},
		})
	}

	// Content type filter - use .keyword subfield
	// Note: Some indexes have content_type as text (with .keyword), others as keyword (direct)
	// Using .keyword works for text fields, but will fail for pure keyword fields
	// For now, using .keyword since existing indexes appear to be text
	if filters.ContentType != "" {
		result = append(result, map[string]any{
			"term": map[string]any{
				"content_type.keyword": filters.ContentType,
			},
		})
	}

	// Quality score range filter
	// Only add filter if there's an actual constraint (min > 0 or max < 100)
	// If both are at defaults (min=0, max=100), don't add the filter
	if filters.MinQualityScore > 0 || filters.MaxQualityScore < maxQualityScoreValue {
		qualityRange := make(map[string]any)
		if filters.MinQualityScore > 0 {
			qualityRange["gte"] = filters.MinQualityScore
		}
		if filters.MaxQualityScore < maxQualityScoreValue {
			qualityRange["lte"] = filters.MaxQualityScore
		}
		// Only add filter if we have at least one constraint
		if len(qualityRange) > 0 {
			result = append(result, map[string]any{
				"range": map[string]any{
					"quality_score": qualityRange,
				},
			})
		}
	}

	// Crime relevance filter
	if len(filters.CrimeRelevance) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"crime.relevance": filters.CrimeRelevance,
			},
		})
	}

	// Source names filter - use .keyword subfield for text fields
	if len(filters.SourceNames) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"source_name.keyword": filters.SourceNames,
			},
		})
	}

	// Date range filter - use crawled_at since published_date may not exist in all documents
	if filters.FromDate != nil || filters.ToDate != nil {
		dateRange := map[string]any{}
		if filters.FromDate != nil {
			dateRange["gte"] = filters.FromDate.Format("2006-01-02T15:04:05Z07:00")
		}
		if filters.ToDate != nil {
			dateRange["lte"] = filters.ToDate.Format("2006-01-02T15:04:05Z07:00")
		}
		// Use crawled_at as it's more reliable (always exists)
		result = append(result, map[string]any{
			"range": map[string]any{
				"crawled_at": dateRange,
			},
		})
	}

	// Recipe and job filters (extracted to stay under funlen limit)
	result = append(result, qb.buildRecipeFilters(filters)...)
	result = append(result, qb.buildJobFilters(filters)...)
	result = append(result, qb.buildRfpFilters(filters)...)

	return result
}

// buildRecipeFilters constructs filter clauses for recipe fields
func (qb *QueryBuilder) buildRecipeFilters(filters *domain.Filters) []any {
	var result []any

	if len(filters.RecipeCuisine) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{"recipe.cuisine": filters.RecipeCuisine},
		})
	}

	if len(filters.RecipeCategory) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{"recipe.category": filters.RecipeCategory},
		})
	}

	if filters.MaxPrepTime != nil {
		result = append(result, map[string]any{
			"range": map[string]any{"recipe.prep_time_minutes": map[string]any{"lte": *filters.MaxPrepTime}},
		})
	}

	if filters.MaxTotalTime != nil {
		result = append(result, map[string]any{
			"range": map[string]any{"recipe.total_time_minutes": map[string]any{"lte": *filters.MaxTotalTime}},
		})
	}

	return result
}

// buildJobFilters constructs filter clauses for job fields
func (qb *QueryBuilder) buildJobFilters(filters *domain.Filters) []any {
	var result []any

	if len(filters.JobEmploymentType) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{"job.employment_type": filters.JobEmploymentType},
		})
	}

	if len(filters.JobIndustry) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{"job.industry": filters.JobIndustry},
		})
	}

	if len(filters.JobLocation) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{"job.location": filters.JobLocation},
		})
	}

	if filters.SalaryMin != nil {
		result = append(result, map[string]any{
			"range": map[string]any{"job.salary_min": map[string]any{"gte": *filters.SalaryMin}},
		})
	}

	return result
}

// buildRfpFilters constructs filter clauses for RFP fields
func (qb *QueryBuilder) buildRfpFilters(filters *domain.Filters) []any {
	var result []any

	if filters.RfpProvince != "" {
		result = append(result, map[string]any{
			"term": map[string]any{
				"rfp.province": filters.RfpProvince,
			},
		})
	}

	if len(filters.RfpSector) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"rfp.categories": filters.RfpSector,
			},
		})
	}

	if filters.RfpClosingAfter != "" {
		result = append(result, map[string]any{
			"range": map[string]any{
				"rfp.closing_date": map[string]any{
					"gte": filters.RfpClosingAfter,
				},
			},
		})
	}

	// Filter on budget_max: find RFPs whose declared ceiling is at least RfpBudgetMin,
	// meaning the contract is large enough to be worth pursuing.
	if filters.RfpBudgetMin != nil {
		result = append(result, map[string]any{
			"range": map[string]any{
				"rfp.budget_max": map[string]any{
					"gte": *filters.RfpBudgetMin,
				},
			},
		})
	}

	if filters.RfpBudgetMax != nil {
		result = append(result, map[string]any{
			"range": map[string]any{
				"rfp.budget_max": map[string]any{
					"lte": *filters.RfpBudgetMax,
				},
			},
		})
	}

	return result
}

// buildBoosts adds score boosting for recency and quality
func (qb *QueryBuilder) buildBoosts() []any {
	// Boost recent content using crawled_at (more reliable than published_date)
	// Use crawled_at since published_date may not exist in all documents
	// Boost high-quality content
	return []any{
		map[string]any{
			"function_score": map[string]any{
				"gauss": map[string]any{
					"crawled_at": map[string]any{
						"origin": "now",
						"scale":  "30d",
						"decay":  recencyDecayFactor,
					},
				},
			},
		},
		map[string]any{
			"function_score": map[string]any{
				"field_value_factor": map[string]any{
					"field":    "quality_score",
					"factor":   qualityBoostFactor,
					"modifier": "log1p",
				},
			},
		},
	}
}

// buildSort constructs sort criteria
func (qb *QueryBuilder) buildSort(req *domain.SearchRequest) []any {
	var sortCriteria []any

	switch req.Sort.Field {
	case "relevance":
		sortCriteria = append(sortCriteria, map[string]any{
			"_score": map[string]any{
				"order": req.Sort.Order,
			},
		})
	case "published_date":
		// Use crawled_at instead since published_date may not exist in all documents
		// This is more reliable for sorting
		sortCriteria = append(sortCriteria, map[string]any{
			"crawled_at": map[string]any{
				"order": req.Sort.Order,
			},
		})
	case "quality_score":
		sortCriteria = append(sortCriteria, map[string]any{
			"quality_score": map[string]any{
				"order": req.Sort.Order,
			},
		})
	case "crawled_at":
		sortCriteria = append(sortCriteria, map[string]any{
			"crawled_at": map[string]any{
				"order": req.Sort.Order,
			},
		})
	default:
		// Default to relevance if field is unknown
		sortCriteria = append(sortCriteria, map[string]any{
			"_score": map[string]any{
				"order": "desc",
			},
		})
	}

	// Always add secondary sort by relevance score
	if req.Sort.Field != "relevance" {
		sortCriteria = append(sortCriteria, map[string]any{
			"_score": map[string]any{
				"order": "desc",
			},
		})
	}

	return sortCriteria
}

// buildHighlight constructs highlight configuration
func (qb *QueryBuilder) buildHighlight() map[string]any {
	return map[string]any{
		"fields": map[string]any{
			"title": map[string]any{
				"number_of_fragments": 1,
			},
			"body": map[string]any{
				"fragment_size":       qb.config.HighlightFragmentSize,
				"number_of_fragments": qb.config.HighlightMaxFragments,
			},
			"raw_text": map[string]any{
				"fragment_size":       qb.config.HighlightFragmentSize,
				"number_of_fragments": qb.config.HighlightMaxFragments,
			},
		},
		"pre_tags":  []string{"<em>"},
		"post_tags": []string{"</em>"},
	}
}

// buildAggregations constructs faceted search aggregations
func (qb *QueryBuilder) buildAggregations() map[string]any {
	return map[string]any{
		"topics": map[string]any{
			"terms": map[string]any{
				"field": "topics.keyword",
				"size":  topicsAggSize,
			},
		},
		// content_type aggregation - use .keyword subfield
		// Note: Some indexes have content_type as text (with .keyword), others as keyword (direct)
		// Using .keyword works for text fields (which is what existing indexes have)
		"content_types": map[string]any{
			"terms": map[string]any{
				"field": "content_type.keyword",
				"size":  contentTypesAggSize,
			},
		},
		"sources": map[string]any{
			"terms": map[string]any{
				"field": "source_name.keyword",
				"size":  sourcesAggSize,
			},
		},
		"quality_ranges": map[string]any{
			"range": map[string]any{
				"field": "quality_score",
				"ranges": []map[string]any{
					{"key": "0-39", "from": 0, "to": qualityRangeLow},
					{"key": "40-59", "from": qualityRangeLow, "to": qualityRangeMid},
					{"key": "60-79", "from": qualityRangeMid, "to": qualityRangeHigh},
					{"key": "80-100", "from": qualityRangeHigh, "to": qualityRangeMax},
				},
			},
		},
		// Recipe facets
		"recipe_cuisines": map[string]any{
			"terms": map[string]any{
				"field": "recipe.cuisine",
				"size":  recipeFacetSize,
			},
		},
		"recipe_categories": map[string]any{
			"terms": map[string]any{
				"field": "recipe.category",
				"size":  recipeFacetSize,
			},
		},
		// Job facets
		"job_types": map[string]any{
			"terms": map[string]any{
				"field": "job.employment_type",
				"size":  jobFacetSize,
			},
		},
		"job_industries": map[string]any{
			"terms": map[string]any{
				"field": "job.industry",
				"size":  jobFacetSize,
			},
		},
		"job_locations": map[string]any{
			"terms": map[string]any{
				"field": "job.location",
				"size":  jobFacetSize,
			},
		},
	}
}

// floatToString converts float64 to string for field boosting
func floatToString(f float64) string {
	return fmt.Sprintf("%.1f", f)
}
