package domain

import "time"

// Document represents a document in Elasticsearch
type Document struct {
	ID             string                 `json:"id"`
	Title          string                 `json:"title,omitempty"`
	URL            string                 `json:"url,omitempty"`
	SourceName     string                 `json:"source_name,omitempty"`
	PublishedDate  *time.Time             `json:"published_date,omitempty"`
	CrawledAt      *time.Time             `json:"crawled_at,omitempty"`
	QualityScore   int                    `json:"quality_score,omitempty"`
	ContentType    string                 `json:"content_type,omitempty"`
	Topics         []string               `json:"topics,omitempty"`
	IsCrimeRelated bool                   `json:"is_crime_related,omitempty"`
	Body           string                 `json:"body,omitempty"`
	RawText        string                 `json:"raw_text,omitempty"`
	RawHTML        string                 `json:"raw_html,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
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
	IsCrimeRelated  *bool      `json:"is_crime_related,omitempty"`
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
	DocumentIDs []string `json:"document_ids" binding:"required"`
}
