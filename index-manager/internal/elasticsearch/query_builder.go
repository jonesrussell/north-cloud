package elasticsearch

import (
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
func (qb *DocumentQueryBuilder) Build(req *domain.DocumentQueryRequest) (map[string]interface{}, error) {
	// Validate and set defaults
	if err := qb.validateRequest(req); err != nil {
		return nil, err
	}

	query := map[string]interface{}{
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
	if req.Filters != nil {
		if req.Filters.MinQualityScore < 0 || req.Filters.MinQualityScore > maxQualityScore {
			return fmt.Errorf("min_quality_score must be between 0 and %d", maxQualityScore)
		}
		if req.Filters.MaxQualityScore < 0 || req.Filters.MaxQualityScore > maxQualityScore {
			req.Filters.MaxQualityScore = maxQualityScore
		}
		if req.Filters.MinQualityScore > req.Filters.MaxQualityScore {
			return fmt.Errorf("min_quality_score cannot exceed max_quality_score")
		}

		if req.Filters.FromDate != nil && req.Filters.ToDate != nil {
			if req.Filters.FromDate.After(*req.Filters.ToDate) {
				return fmt.Errorf("from_date cannot be after to_date")
			}
		}
		if req.Filters.FromCrawledAt != nil && req.Filters.ToCrawledAt != nil {
			if req.Filters.FromCrawledAt.After(*req.Filters.ToCrawledAt) {
				return fmt.Errorf("from_crawled_at cannot be after to_crawled_at")
			}
		}
	}

	return nil
}

// buildBoolQuery constructs the bool query with must, filter, and should clauses
func (qb *DocumentQueryBuilder) buildBoolQuery(req *domain.DocumentQueryRequest) map[string]interface{} {
	boolQuery := map[string]interface{}{
		"must":   []interface{}{},
		"filter": []interface{}{},
	}

	// Multi-match query for full-text search
	if req.Query != "" && strings.TrimSpace(req.Query) != "" {
		boolQuery["must"] = []interface{}{
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

	return map[string]interface{}{"bool": boolQuery}
}

// buildMultiMatchQuery creates a multi-match query with field boosting
func (qb *DocumentQueryBuilder) buildMultiMatchQuery(query string) map[string]interface{} {
	return map[string]interface{}{
		"multi_match": map[string]interface{}{
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

// buildFilters constructs filter clauses
func (qb *DocumentQueryBuilder) buildFilters(filters *domain.DocumentFilters) []interface{} {
	var result []interface{}

	// Title filter (contains)
	if filters.Title != "" {
		result = append(result, map[string]interface{}{
			"wildcard": map[string]interface{}{
				"title.keyword": map[string]interface{}{
					"value":            "*" + strings.ToLower(filters.Title) + "*",
					"case_insensitive": true,
				},
			},
		})
	}

	// URL filter (contains)
	if filters.URL != "" {
		result = append(result, map[string]interface{}{
			"wildcard": map[string]interface{}{
				"url.keyword": map[string]interface{}{
					"value":            "*" + strings.ToLower(filters.URL) + "*",
					"case_insensitive": true,
				},
			},
		})
	}

	// Content type filter
	if filters.ContentType != "" {
		result = append(result, map[string]interface{}{
			"term": map[string]interface{}{
				"content_type.keyword": filters.ContentType,
			},
		})
	}

	// Quality score range filter
	if filters.MinQualityScore > 0 || filters.MaxQualityScore < maxQualityScore {
		qualityRange := make(map[string]interface{})
		if filters.MinQualityScore > 0 {
			qualityRange["gte"] = filters.MinQualityScore
		}
		if filters.MaxQualityScore < maxQualityScore {
			qualityRange["lte"] = filters.MaxQualityScore
		}
		if len(qualityRange) > 0 {
			result = append(result, map[string]interface{}{
				"range": map[string]interface{}{
					"quality_score": qualityRange,
				},
			})
		}
	}

	// Topics filter
	if len(filters.Topics) > 0 {
		result = append(result, map[string]interface{}{
			"terms": map[string]interface{}{
				"topics.keyword": filters.Topics,
			},
		})
	}

	// Published date range filter
	if filters.FromDate != nil || filters.ToDate != nil {
		dateRange := make(map[string]interface{})
		if filters.FromDate != nil {
			dateRange["gte"] = filters.FromDate.Format(time.RFC3339)
		}
		if filters.ToDate != nil {
			dateRange["lte"] = filters.ToDate.Format(time.RFC3339)
		}
		result = append(result, map[string]interface{}{
			"range": map[string]interface{}{
				"published_date": dateRange,
			},
		})
	}

	// Crawled at date range filter
	if filters.FromCrawledAt != nil || filters.ToCrawledAt != nil {
		dateRange := make(map[string]interface{})
		if filters.FromCrawledAt != nil {
			dateRange["gte"] = filters.FromCrawledAt.Format(time.RFC3339)
		}
		if filters.ToCrawledAt != nil {
			dateRange["lte"] = filters.ToCrawledAt.Format(time.RFC3339)
		}
		result = append(result, map[string]interface{}{
			"range": map[string]interface{}{
				"crawled_at": dateRange,
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

	return result
}

// buildSort constructs the sort clause
func (qb *DocumentQueryBuilder) buildSort(sort *domain.DocumentSort) []map[string]interface{} {
	var sortClauses []map[string]interface{}

	if sort == nil {
		// Default: sort by relevance descending (score)
		return []map[string]interface{}{
			{"_score": map[string]interface{}{"order": "desc"}},
		}
	}

	switch sort.Field {
	case "relevance":
		sortClauses = append(sortClauses, map[string]interface{}{
			"_score": map[string]interface{}{"order": sort.Order},
		})
	case "published_date":
		sortClauses = append(sortClauses, map[string]interface{}{
			"published_date": map[string]interface{}{
				"order":         sort.Order,
				"missing":       "_last",
				"unmapped_type": "date",
			},
		})
	case "crawled_at":
		sortClauses = append(sortClauses, map[string]interface{}{
			"crawled_at": map[string]interface{}{
				"order":         sort.Order,
				"missing":       "_last",
				"unmapped_type": "date",
			},
		})
	case "quality_score":
		sortClauses = append(sortClauses, map[string]interface{}{
			"quality_score": map[string]interface{}{
				"order":   sort.Order,
				"missing": "_last",
			},
		})
	case "title":
		sortClauses = append(sortClauses, map[string]interface{}{
			"title.keyword": map[string]interface{}{
				"order":   sort.Order,
				"missing": "_last",
			},
		})
	default:
		// Default to relevance
		sortClauses = append(sortClauses, map[string]interface{}{
			"_score": map[string]interface{}{"order": sort.Order},
		})
	}

	return sortClauses
}
