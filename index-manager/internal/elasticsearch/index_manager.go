package elasticsearch

import (
	"context"
	"fmt"
	"strings"
)

// IndexManager provides high-level index management operations
type IndexManager struct {
	client *Client
}

// NewIndexManager creates a new index manager
func NewIndexManager(client *Client) *IndexManager {
	return &IndexManager{
		client: client,
	}
}

// CreateIndexWithType creates an index with a specific type and mapping
func (m *IndexManager) CreateIndexWithType(ctx context.Context, indexName, indexType string, mapping any) error {
	return m.client.EnsureIndex(ctx, indexName, mapping)
}

// DeleteIndexByType deletes an index of a specific type
func (m *IndexManager) DeleteIndexByType(ctx context.Context, indexName string) error {
	return m.client.DeleteIndex(ctx, indexName)
}

// ListIndicesByType lists all indices of a specific type
func (m *IndexManager) ListIndicesByType(ctx context.Context, indexType string) ([]string, error) {
	pattern := fmt.Sprintf("*_%s", indexType)
	return m.client.ListIndices(ctx, pattern)
}

// ListIndicesBySource lists all indices for a specific source
func (m *IndexManager) ListIndicesBySource(ctx context.Context, sourceName string) ([]string, error) {
	// Normalize source name for pattern matching
	normalized := normalizeSourceName(sourceName)
	pattern := fmt.Sprintf("%s_*", normalized)
	return m.client.ListIndices(ctx, pattern)
}

// GetIndexDetails gets detailed information about an index
func (m *IndexManager) GetIndexDetails(ctx context.Context, indexName string) (*IndexInfo, error) {
	return m.client.GetIndexInfo(ctx, indexName)
}

// normalizeSourceName normalizes a source name for index naming
func normalizeSourceName(sourceName string) string {
	// Remove protocol if present
	sourceName = strings.TrimPrefix(sourceName, "http://")
	sourceName = strings.TrimPrefix(sourceName, "https://")

	// Replace dots and hyphens with underscores
	sourceName = strings.ReplaceAll(sourceName, ".", "_")
	sourceName = strings.ReplaceAll(sourceName, "-", "_")

	// Convert to lowercase
	return strings.ToLower(sourceName)
}
