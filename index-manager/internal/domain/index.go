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
	IndexName  string                 `json:"index_name" binding:"required"`
	IndexType  IndexType              `json:"index_type" binding:"required"`
	SourceName string                 `json:"source_name,omitempty"`
	Mapping    map[string]interface{} `json:"mapping,omitempty"`
}

// BulkCreateIndexRequest represents a request to create multiple indexes
type BulkCreateIndexRequest struct {
	Indexes []CreateIndexRequest `json:"indexes" binding:"required"`
}

// BulkDeleteIndexRequest represents a request to delete multiple indexes
type BulkDeleteIndexRequest struct {
	IndexNames []string `json:"index_names" binding:"required"`
}

// IndexStats represents statistics about indexes
type IndexStats struct {
	TotalIndexes    int            `json:"total_indexes"`
	IndexesByType   map[string]int `json:"indexes_by_type"`
	TotalDocuments  int64          `json:"total_documents"`
	ClusterHealth   string         `json:"cluster_health"`
	IndexesByHealth map[string]int `json:"indexes_by_health"`
}
