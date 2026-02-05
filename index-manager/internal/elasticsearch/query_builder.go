package elasticsearch

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
)

const (
	maxPageSize     = 100
	defaultPageSize = 20
	maxQualityScore = 100
)

// DocumentQueryBuilder builds Elasticsearch queries from document query requests
type DocumentQueryBuilder struct{}

// NewDocumentQueryBuilder creates a new query builder
func NewDocumentQueryBuilder() *DocumentQueryBuilder {
	return &DocumentQueryBuilder{}
}

// Build constructs the complete Elasticsearch query from a DocumentQueryRequest
func (qb *DocumentQueryBuilder) Build(req *domain.DocumentQueryRequest) (map[string]any, error) {
	// Validate and set defaults
	if err := qb.validateRequest(req); err != nil {
		return nil, err
	}

	query := map[string]any{
		"query": qb.buildBoolQuery(req),
		"from":  (req.Pagination.Page - 1) * req.Pagination.Size,
		"size":  req.Pagination.Size,
		"sort":  qb.buildSort(req.Sort),
	}

	// Enable total hits tracking
	query["track_total_hits"] = true

	return query, nil
}

// validateRequest validates and sets defaults for the request
//
//nolint:gocognit // Complex validation logic with multiple checks
func (qb *DocumentQueryBuilder) validateRequest(req *domain.DocumentQueryRequest) error {
	// Set default pagination
	if req.Pagination == nil {
		req.Pagination = &domain.DocumentPagination{
			Page: 1,
			Size: defaultPageSize,
		}
	}

	if req.Pagination.Page < 1 {
		req.Pagination.Page = 1
	}
	if req.Pagination.Size < 1 {
		req.Pagination.Size = defaultPageSize
	}
	if req.Pagination.Size > maxPageSize {
		return fmt.Errorf("page size exceeds maximum of %d", maxPageSize)
	}

	// Set default sort
	if req.Sort == nil {
		req.Sort = &domain.DocumentSort{
			Field: "relevance",
			Order: "desc",
		}
	}

	// Validate sort field
	validFields := map[string]bool{
		"relevance":      true,
		"published_date": true,
		"crawled_at":     true,
		"quality_score":  true,
		"title":          true,
	}
	if !validFields[req.Sort.Field] {
		req.Sort.Field = "relevance"
	}

	// Validate sort order
	if req.Sort.Order != "asc" && req.Sort.Order != "desc" {
		req.Sort.Order = "desc"
	}

	// Validate filters if present
	//nolint:nestif // Complex nested validation logic
	if req.Filters != nil {
		if req.Filters.MinQualityScore < 0 || req.Filters.MinQualityScore > maxQualityScore {
			return fmt.Errorf("min_quality_score must be between 0 and %d", maxQualityScore)
		}
		if req.Filters.MaxQualityScore < 0 || req.Filters.MaxQualityScore > maxQualityScore {
			req.Filters.MaxQualityScore = maxQualityScore
		}
		if req.Filters.MinQualityScore > req.Filters.MaxQualityScore {
			return errors.New("min_quality_score cannot exceed max_quality_score")
		}

		if req.Filters.FromDate != nil && req.Filters.ToDate != nil {
			if req.Filters.FromDate.After(*req.Filters.ToDate) {
				return errors.New("from_date cannot be after to_date")
			}
		}
		if req.Filters.FromCrawledAt != nil && req.Filters.ToCrawledAt != nil {
			if req.Filters.FromCrawledAt.After(*req.Filters.ToCrawledAt) {
				return errors.New("from_crawled_at cannot be after to_crawled_at")
			}
		}
	}

	return nil
}

// buildBoolQuery constructs the bool query with must, filter, and should clauses
func (qb *DocumentQueryBuilder) buildBoolQuery(req *domain.DocumentQueryRequest) map[string]any {
	boolQuery := map[string]any{
		"must":   []any{},
		"filter": []any{},
	}

	// Multi-match query for full-text search
	if req.Query != "" && strings.TrimSpace(req.Query) != "" {
		boolQuery["must"] = []any{
			qb.buildMultiMatchQuery(req.Query),
		}
	}

	// Add filters
	if req.Filters != nil {
		filters := qb.buildFilters(req.Filters)
		if len(filters) > 0 {
			boolQuery["filter"] = filters
		}
	}

	return map[string]any{"bool": boolQuery}
}

// buildMultiMatchQuery creates a multi-match query with field boosting
func (qb *DocumentQueryBuilder) buildMultiMatchQuery(query string) map[string]any {
	return map[string]any{
		"multi_match": map[string]any{
			"query": query,
			"fields": []string{
				"title^3",
				"url^2",
				"body^1",
				"raw_text^1",
			},
			"type":      "best_fields",
			"operator":  "or",
			"fuzziness": "AUTO",
		},
	}
}

// BuildFiltersOnly returns just the filter array without wrapping in bool query
func (qb *DocumentQueryBuilder) BuildFiltersOnly(filters *domain.DocumentFilters) []any {
	if filters == nil {
		return []any{}
	}
	return qb.buildFilters(filters)
}

// buildFilters constructs filter clauses
func (qb *DocumentQueryBuilder) buildFilters(filters *domain.DocumentFilters) []any {
	var result []any

	// Basic text filters
	result = qb.appendTextFilters(result, filters)

	// Quality and topic filters
	result = qb.appendQualityTopicFilters(result, filters)

	// Date range filters
	result = qb.appendDateFilters(result, filters)

	// Crime filters
	result = qb.appendCrimeFilters(result, filters)

	// Location filters
	result = qb.appendLocationFilters(result, filters)

	// Sources filter
	if len(filters.Sources) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"source_name": filters.Sources,
			},
		})
	}

	// Legacy crime-related filter (backward compatibility)
	result = qb.appendLegacyCrimeFilter(result, filters)

	return result
}

// appendTextFilters adds title, URL, and content type filters
func (qb *DocumentQueryBuilder) appendTextFilters(result []any, filters *domain.DocumentFilters) []any {
	if filters.Title != "" {
		result = append(result, map[string]any{
			"wildcard": map[string]any{
				"title.keyword": map[string]any{
					"value":            "*" + strings.ToLower(filters.Title) + "*",
					"case_insensitive": true,
				},
			},
		})
	}
	if filters.URL != "" {
		result = append(result, map[string]any{
			"wildcard": map[string]any{
				"url.keyword": map[string]any{
					"value":            "*" + strings.ToLower(filters.URL) + "*",
					"case_insensitive": true,
				},
			},
		})
	}
	if filters.ContentType != "" {
		result = append(result, map[string]any{
			"term": map[string]any{
				"content_type.keyword": filters.ContentType,
			},
		})
	}
	return result
}

// appendQualityTopicFilters adds quality score and topics filters
func (qb *DocumentQueryBuilder) appendQualityTopicFilters(result []any, filters *domain.DocumentFilters) []any {
	if filters.MinQualityScore > 0 || (filters.MaxQualityScore > 0 && filters.MaxQualityScore < maxQualityScore) {
		qualityRange := make(map[string]any)
		if filters.MinQualityScore > 0 {
			qualityRange["gte"] = filters.MinQualityScore
		}
		if filters.MaxQualityScore > 0 && filters.MaxQualityScore < maxQualityScore {
			qualityRange["lte"] = filters.MaxQualityScore
		}
		if len(qualityRange) > 0 {
			result = append(result, map[string]any{
				"range": map[string]any{
					"quality_score": qualityRange,
				},
			})
		}
	}
	if len(filters.Topics) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"topics.keyword": filters.Topics,
			},
		})
	}
	return result
}

// appendDateFilters adds published date and crawled at filters
func (qb *DocumentQueryBuilder) appendDateFilters(result []any, filters *domain.DocumentFilters) []any {
	if filters.FromDate != nil || filters.ToDate != nil {
		dateRange := make(map[string]any)
		if filters.FromDate != nil {
			dateRange["gte"] = filters.FromDate.Format(time.RFC3339)
		}
		if filters.ToDate != nil {
			dateRange["lte"] = filters.ToDate.Format(time.RFC3339)
		}
		result = append(result, map[string]any{
			"range": map[string]any{
				"published_date": dateRange,
			},
		})
	}
	if filters.FromCrawledAt != nil || filters.ToCrawledAt != nil {
		dateRange := make(map[string]any)
		if filters.FromCrawledAt != nil {
			dateRange["gte"] = filters.FromCrawledAt.Format(time.RFC3339)
		}
		if filters.ToCrawledAt != nil {
			dateRange["lte"] = filters.ToCrawledAt.Format(time.RFC3339)
		}
		result = append(result, map[string]any{
			"range": map[string]any{
				"crawled_at": dateRange,
			},
		})
	}
	return result
}

// appendLegacyCrimeFilter adds backward-compatible is_crime_related filter
func (qb *DocumentQueryBuilder) appendLegacyCrimeFilter(result []any, filters *domain.DocumentFilters) []any {
	if filters.IsCrimeRelated != nil && len(filters.CrimeRelevance) == 0 {
		if *filters.IsCrimeRelated {
			result = append(result, map[string]any{
				"terms": map[string]any{
					"crime.relevance": []string{"core_street_crime", "peripheral_crime"},
				},
			})
		} else {
			result = append(result, map[string]any{
				"term": map[string]any{
					"crime.relevance": "not_crime",
				},
			})
		}
	}
	return result
}

// appendCrimeFilters adds crime-related filters to the result slice
func (qb *DocumentQueryBuilder) appendCrimeFilters(result []any, filters *domain.DocumentFilters) []any {
	// Crime relevance filter
	if len(filters.CrimeRelevance) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"crime.relevance": filters.CrimeRelevance,
			},
		})
	}

	// Crime sub-labels filter
	if len(filters.CrimeSubLabels) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"crime.sub_label": filters.CrimeSubLabels,
			},
		})
	}

	// Crime types filter
	if len(filters.CrimeTypes) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"crime.crime_types": filters.CrimeTypes,
			},
		})
	}

	// Homepage eligible filter
	if filters.HomepageEligible != nil {
		result = append(result, map[string]any{
			"term": map[string]any{
				"crime.homepage_eligible": *filters.HomepageEligible,
			},
		})
	}

	// Review required filter
	if filters.ReviewRequired != nil {
		result = append(result, map[string]any{
			"term": map[string]any{
				"crime.review_required": *filters.ReviewRequired,
			},
		})
	}

	return result
}

// appendLocationFilters adds location-related filters to the result slice
func (qb *DocumentQueryBuilder) appendLocationFilters(result []any, filters *domain.DocumentFilters) []any {
	// Cities filter
	if len(filters.Cities) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"location.city": filters.Cities,
			},
		})
	}

	// Provinces filter
	if len(filters.Provinces) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"location.province": filters.Provinces,
			},
		})
	}

	// Countries filter
	if len(filters.Countries) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"location.country": filters.Countries,
			},
		})
	}

	// Specificity filter
	if len(filters.Specificity) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"location.specificity": filters.Specificity,
			},
		})
	}

	return result
}

// buildSort constructs the sort clause
func (qb *DocumentQueryBuilder) buildSort(sort *domain.DocumentSort) []map[string]any {
	var sortClauses []map[string]any

	if sort == nil {
		// Default: sort by relevance descending (score)
		return []map[string]any{
			{"_score": map[string]any{"order": "desc"}},
		}
	}

	switch sort.Field {
	case "relevance":
		sortClauses = append(sortClauses, map[string]any{
			"_score": map[string]any{"order": sort.Order},
		})
	case "published_date":
		sortClauses = append(sortClauses, map[string]any{
			"published_date": map[string]any{
				"order":         sort.Order,
				"missing":       "_last",
				"unmapped_type": "date",
			},
		})
	case "crawled_at":
		sortClauses = append(sortClauses, map[string]any{
			"crawled_at": map[string]any{
				"order":         sort.Order,
				"missing":       "_last",
				"unmapped_type": "date",
			},
		})
	case "quality_score":
		sortClauses = append(sortClauses, map[string]any{
			"quality_score": map[string]any{
				"order":   sort.Order,
				"missing": "_last",
			},
		})
	case "title":
		sortClauses = append(sortClauses, map[string]any{
			"title.keyword": map[string]any{
				"order":   sort.Order,
				"missing": "_last",
			},
		})
	default:
		// Default to relevance
		sortClauses = append(sortClauses, map[string]any{
			"_score": map[string]any{"order": sort.Order},
		})
	}

	return sortClauses
}
