// Package archive provides HTML archiving functionality using MinIO object storage.
package archive

import (
	"context"
	"time"
)

// UploadTask represents a task to upload HTML and metadata to MinIO.
type UploadTask struct {
	// HTML is the response body to archive
	HTML []byte
	// URL is the full request URL
	URL string
	// SourceName is the source identifier (e.g., "example_com")
	SourceName string
	// StatusCode is the HTTP response status code
	StatusCode int
	// Headers are the response headers
	Headers map[string]string
	// Timestamp is the crawl time
	Timestamp time.Time
	// Ctx is the request context
	Ctx context.Context
}

// HTMLMetadata represents metadata about an archived HTML document.
type HTMLMetadata struct {
	// URL is the full page URL
	URL string `json:"url"`
	// URLHash is the SHA-256 hash of the URL (8 characters)
	URLHash string `json:"url_hash"`
	// SourceName is the source identifier
	SourceName string `json:"source_name"`
	// CrawledAt is the crawl timestamp
	CrawledAt time.Time `json:"crawled_at"`
	// StatusCode is the HTTP response status code
	StatusCode int `json:"status_code"`
	// ContentType is the response Content-Type header
	ContentType string `json:"content_type,omitempty"`
	// ContentLength is the HTML size in bytes
	ContentLength int64 `json:"content_length"`
	// Headers are selected response headers
	Headers map[string]string `json:"headers,omitempty"`
	// ESIndex is the Elasticsearch index name (e.g., "example_com_raw_content")
	ESIndex string `json:"es_index,omitempty"`
	// ESDocumentID is the Elasticsearch document ID
	ESDocumentID string `json:"es_document_id,omitempty"`
	// ObjectKey is the MinIO object path
	ObjectKey string `json:"object_key"`
}
