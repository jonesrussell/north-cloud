package domain

// IndexType represents the type of an index
type IndexType string

const (
	IndexTypeRawContent        IndexType = "raw_content"
	IndexTypeClassifiedContent IndexType = "classified_content"
	IndexTypeArticle           IndexType = "article"
	IndexTypePage              IndexType = "page"
)

// IndexStatus represents the status of an index
type IndexStatus string

const (
	IndexStatusActive   IndexStatus = "active"
	IndexStatusArchived IndexStatus = "archived"
	IndexStatusDeleted  IndexStatus = "deleted"
)

// Index represents an Elasticsearch index
type Index struct {
	Name           string    `json:"name"`
	Type           IndexType `json:"type"`
	SourceName     string    `json:"source_name,omitempty"`
	Health         string    `json:"health,omitempty"`
	Status         string    `json:"status,omitempty"`
	DocumentCount  int64     `json:"document_count,omitempty"`
	Size           string    `json:"size,omitempty"`
	MappingVersion string    `json:"mapping_version,omitempty"`
	CreatedAt      string    `json:"created_at,omitempty"`
	UpdatedAt      string    `json:"updated_at,omitempty"`
}

// CreateIndexRequest represents a request to create an index
type CreateIndexRequest struct {
	IndexName  string         `binding:"required"           json:"index_name"`
	IndexType  IndexType      `binding:"required"           json:"index_type"`
	SourceName string         `json:"source_name,omitempty"`
	Mapping    map[string]any `json:"mapping,omitempty"`
}

// BulkCreateIndexRequest represents a request to create multiple indexes
type BulkCreateIndexRequest struct {
	Indexes []CreateIndexRequest `binding:"required" json:"indexes"`
}

// BulkDeleteIndexRequest represents a request to delete multiple indexes
type BulkDeleteIndexRequest struct {
	IndexNames []string `binding:"required" json:"index_names"`
}

// IndexStats represents statistics about indexes
type IndexStats struct {
	TotalIndexes    int            `json:"total_indexes"`
	IndexesByType   map[string]int `json:"indexes_by_type"`
	TotalDocuments  int64          `json:"total_documents"`
	IndexedToday    int64          `json:"indexed_today"`
	ClusterHealth   string         `json:"cluster_health"`
	IndexesByHealth map[string]int `json:"indexes_by_health"`
}

// ListIndicesRequest holds pagination, filtering, and sorting parameters
type ListIndicesRequest struct {
	// Existing filters
	Type       string // IndexType filter
	SourceName string // Source name filter

	// New filters
	Search string // Name search filter (case-insensitive substring)
	Health string // Health filter (green, yellow, red)

	// Pagination
	Limit  int // Default: 50
	Offset int // Default: 0

	// Sorting
	SortBy    string // name, document_count, size, health (default: name)
	SortOrder string // asc, desc (default: asc)
}

// ListIndicesResponse represents paginated indices response
type ListIndicesResponse struct {
	Indices []*Index `json:"indices"`
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
}
