package domain

import "time"

// CrimeInfo contains structured crime classification data
type CrimeInfo struct {
	SubLabel         string   `json:"sub_label,omitempty"`
	PrimaryCrimeType string   `json:"primary_crime_type,omitempty"`
	Relevance        string   `json:"relevance,omitempty"`
	CrimeTypes       []string `json:"crime_types,omitempty"`
	Confidence       float64  `json:"confidence,omitempty"`
	HomepageEligible bool     `json:"homepage_eligible,omitempty"`
	ReviewRequired   bool     `json:"review_required,omitempty"`
	ModelVersion     string   `json:"model_version,omitempty"`
}

// IsCrimeRelated returns true if this represents crime-related content
func (c *CrimeInfo) IsCrimeRelated() bool {
	if c == nil {
		return false
	}
	return c.Relevance != "not_crime" && c.Relevance != ""
}

// LocationInfo contains structured location data
type LocationInfo struct {
	City        string  `json:"city,omitempty"`
	Province    string  `json:"province,omitempty"`
	Country     string  `json:"country,omitempty"`
	Specificity string  `json:"specificity,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
}

// Document represents a document in Elasticsearch
type Document struct {
	ID            string     `json:"id"`
	Title         string     `json:"title,omitempty"`
	URL           string     `json:"url,omitempty"`
	SourceName    string     `json:"source_name,omitempty"`
	PublishedDate *time.Time `json:"published_date,omitempty"`
	CrawledAt     *time.Time `json:"crawled_at,omitempty"`
	QualityScore  int        `json:"quality_score,omitempty"`
	ContentType   string     `json:"content_type,omitempty"`
	Topics        []string   `json:"topics,omitempty"`
	Body          string     `json:"body,omitempty"`
	RawText       string     `json:"raw_text,omitempty"`
	RawHTML       string     `json:"raw_html,omitempty"`

	// Structured classification fields
	Crime    *CrimeInfo    `json:"crime,omitempty"`
	Location *LocationInfo `json:"location,omitempty"`

	// Unstructured spillover
	Meta map[string]any `json:"meta,omitempty"`
}

// DocumentQueryRequest represents a request to query documents
type DocumentQueryRequest struct {
	Query      string              `json:"query,omitempty"`
	Filters    *DocumentFilters    `json:"filters,omitempty"`
	Pagination *DocumentPagination `json:"pagination,omitempty"`
	Sort       *DocumentSort       `json:"sort,omitempty"`
}

// DocumentFilters holds filter criteria for document queries
type DocumentFilters struct {
	// Existing filters
	Title           string     `json:"title,omitempty"`
	URL             string     `json:"url,omitempty"`
	ContentType     string     `json:"content_type,omitempty"`
	MinQualityScore int        `json:"min_quality_score,omitempty"`
	MaxQualityScore int        `json:"max_quality_score,omitempty"`
	Topics          []string   `json:"topics,omitempty"`
	FromDate        *time.Time `json:"from_date,omitempty"`
	ToDate          *time.Time `json:"to_date,omitempty"`
	FromCrawledAt   *time.Time `json:"from_crawled_at,omitempty"`
	ToCrawledAt     *time.Time `json:"to_crawled_at,omitempty"`

	// Crime filters (new)
	CrimeRelevance   []string `json:"crime_relevance,omitempty"`
	CrimeSubLabels   []string `json:"crime_sub_labels,omitempty"`
	CrimeTypes       []string `json:"crime_types,omitempty"`
	HomepageEligible *bool    `json:"homepage_eligible,omitempty"`
	ReviewRequired   *bool    `json:"review_required,omitempty"`

	// Location filters (new)
	Cities      []string `json:"cities,omitempty"`
	Provinces   []string `json:"provinces,omitempty"`
	Countries   []string `json:"countries,omitempty"`
	Specificity []string `json:"specificity,omitempty"`

	// Source filter (new)
	Sources []string `json:"sources,omitempty"`
}

// DocumentPagination holds pagination parameters
type DocumentPagination struct {
	Page int `json:"page"`
	Size int `json:"size"`
}

// DocumentSort holds sorting parameters
type DocumentSort struct {
	Field string `json:"field"` // relevance, published_date, crawled_at, quality_score, title
	Order string `json:"order"` // asc, desc
}

// DocumentQueryResponse represents a paginated response of documents
type DocumentQueryResponse struct {
	Documents   []*Document `json:"documents"`
	TotalHits   int64       `json:"total_hits"`
	TotalPages  int         `json:"total_pages"`
	CurrentPage int         `json:"current_page"`
	PageSize    int         `json:"page_size"`
}

// BulkDeleteRequest represents a request to delete multiple documents
type BulkDeleteRequest struct {
	DocumentIDs []string `binding:"required" json:"document_ids"`
}
