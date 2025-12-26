package domain

import (
	"fmt"
	"time"
)

// SearchRequest represents a search query request
type SearchRequest struct {
	Query      string      `json:"query"`
	Filters    *Filters    `json:"filters,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Sort       *Sort       `json:"sort,omitempty"`
	Options    *Options    `json:"options,omitempty"`
}

// Filters holds search filter criteria
type Filters struct {
	Topics           []string   `json:"topics,omitempty"`
	ContentType      string     `json:"content_type,omitempty"`
	MinQualityScore  int        `json:"min_quality_score,omitempty"`
	MaxQualityScore  int        `json:"max_quality_score,omitempty"`
	IsCrimeRelated   *bool      `json:"is_crime_related,omitempty"`
	SourceNames      []string   `json:"source_names,omitempty"`
	FromDate         *time.Time `json:"from_date,omitempty"`
	ToDate           *time.Time `json:"to_date,omitempty"`
}

// Pagination holds pagination parameters
type Pagination struct {
	Page int `json:"page"`
	Size int `json:"size"`
}

// Sort holds sorting parameters
type Sort struct {
	Field string `json:"field"` // relevance, published_date, quality_score, crawled_at
	Order string `json:"order"` // asc, desc
}

// Options holds optional search features
type Options struct {
	IncludeHighlights bool     `json:"include_highlights,omitempty"`
	IncludeFacets     bool     `json:"include_facets,omitempty"`
	SourceFields      []string `json:"source_fields,omitempty"`
}

// SearchResponse represents a search result response
type SearchResponse struct {
	Query       string          `json:"query"`
	TotalHits   int64           `json:"total_hits"`
	TotalPages  int             `json:"total_pages"`
	CurrentPage int             `json:"current_page"`
	PageSize    int             `json:"page_size"`
	TookMs      int64           `json:"took_ms"`
	Hits        []*SearchHit    `json:"hits"`
	Facets      *Facets         `json:"facets,omitempty"`
}

// SearchHit represents a single search result
type SearchHit struct {
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	URL           string                 `json:"url"`
	SourceName    string                 `json:"source_name"`
	PublishedDate *time.Time             `json:"published_date,omitempty"`
	CrawledAt     *time.Time             `json:"crawled_at,omitempty"`
	QualityScore  int                    `json:"quality_score"`
	ContentType   string                 `json:"content_type"`
	Topics        []string               `json:"topics,omitempty"`
	IsCrimeRelated bool                  `json:"is_crime_related"`
	Score         float64                `json:"score"` // Relevance score
	Highlight     map[string][]string    `json:"highlight,omitempty"`
	Snippet       string                 `json:"snippet,omitempty"`
}

// Facets holds faceted search aggregations
type Facets struct {
	Topics        []FacetBucket `json:"topics,omitempty"`
	ContentTypes  []FacetBucket `json:"content_types,omitempty"`
	Sources       []FacetBucket `json:"sources,omitempty"`
	QualityRanges []FacetBucket `json:"quality_ranges,omitempty"`
}

// FacetBucket represents a single facet bucket
type FacetBucket struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// Validate validates the search request
func (req *SearchRequest) Validate(maxPageSize, defaultPageSize, maxQueryLength int) error {
	// Validate query length
	if len(req.Query) > maxQueryLength {
		return fmt.Errorf("query length exceeds maximum of %d characters", maxQueryLength)
	}

	// Set defaults and validate pagination
	if req.Pagination == nil {
		req.Pagination = &Pagination{
			Page: 1,
			Size: defaultPageSize,
		}
	} else {
		if req.Pagination.Page < 1 {
			req.Pagination.Page = 1
		}
		if req.Pagination.Size < 1 {
			req.Pagination.Size = defaultPageSize
		}
		if req.Pagination.Size > maxPageSize {
			return fmt.Errorf("page size exceeds maximum of %d", maxPageSize)
		}
	}

	// Set default filters
	if req.Filters == nil {
		req.Filters = &Filters{
			MaxQualityScore: 100,
		}
	} else {
		// Validate quality score range
		if req.Filters.MinQualityScore < 0 || req.Filters.MinQualityScore > 100 {
			return fmt.Errorf("min_quality_score must be between 0 and 100")
		}
		if req.Filters.MaxQualityScore < 0 || req.Filters.MaxQualityScore > 100 {
			req.Filters.MaxQualityScore = 100
		}
		if req.Filters.MinQualityScore > req.Filters.MaxQualityScore {
			return fmt.Errorf("min_quality_score cannot exceed max_quality_score")
		}

		// Validate date range
		if req.Filters.FromDate != nil && req.Filters.ToDate != nil {
			if req.Filters.FromDate.After(*req.Filters.ToDate) {
				return fmt.Errorf("from_date cannot be after to_date")
			}
		}
	}

	// Set default sort
	if req.Sort == nil {
		req.Sort = &Sort{
			Field: "relevance",
			Order: "desc",
		}
	} else {
		// Validate sort field
		validFields := map[string]bool{
			"relevance":      true,
			"published_date": true,
			"quality_score":  true,
			"crawled_at":     true,
		}
		if !validFields[req.Sort.Field] {
			return fmt.Errorf("invalid sort field: %s", req.Sort.Field)
		}

		// Validate sort order
		if req.Sort.Order != "asc" && req.Sort.Order != "desc" {
			req.Sort.Order = "desc"
		}
	}

	// Set default options
	if req.Options == nil {
		req.Options = &Options{
			IncludeHighlights: true,
			IncludeFacets:     true,
		}
	}

	return nil
}

// HealthStatus represents the health status of the service
type HealthStatus struct {
	Status       string                 `json:"status"`
	Timestamp    time.Time              `json:"timestamp"`
	Version      string                 `json:"version"`
	Dependencies map[string]string      `json:"dependencies"`
}
