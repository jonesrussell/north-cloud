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

// buildFilters constructs filter clauses
//
//nolint:gocognit // Complex filter building with multiple conditionals
func (qb *DocumentQueryBuilder) buildFilters(filters *domain.DocumentFilters) []any {
	var result []any

	// Title filter (contains)
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

	// URL filter (contains)
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

	// Content type filter
	if filters.ContentType != "" {
		result = append(result, map[string]any{
			"term": map[string]any{
				"content_type.keyword": filters.ContentType,
			},
		})
	}

	// Quality score range filter
	if filters.MinQualityScore > 0 || filters.MaxQualityScore < maxQualityScore {
		qualityRange := make(map[string]any)
		if filters.MinQualityScore > 0 {
			qualityRange["gte"] = filters.MinQualityScore
		}
		if filters.MaxQualityScore < maxQualityScore {
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

	// Topics filter
	if len(filters.Topics) > 0 {
		result = append(result, map[string]any{
			"terms": map[string]any{
				"topics.keyword": filters.Topics,
			},
		})
	}

	// Published date range filter
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

	// Crawled at date range filter
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

	// Crime-related filter
	if filters.IsCrimeRelated != nil {
		result = append(result, map[string]any{
			"term": map[string]any{
				"is_crime_related": *filters.IsCrimeRelated,
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
