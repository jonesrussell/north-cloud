package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// ElasticsearchStorage implements storage operations for the classifier
type ElasticsearchStorage struct {
	client *es.Client
}

// NewElasticsearchStorage creates a new Elasticsearch storage instance
func NewElasticsearchStorage(client *es.Client) *ElasticsearchStorage {
	return &ElasticsearchStorage{
		client: client,
	}
}

// QueryRawContent queries for raw content with specified classification status
// This matches the ElasticsearchClient interface expected by the Poller
func (s *ElasticsearchStorage) QueryRawContent(ctx context.Context, status string, batchSize int) ([]*domain.RawContent, error) {
	// Query all *_raw_content indices
	indexPattern := "*_raw_content"
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"classification_status": status,
			},
		},
		"size": batchSize,
		"sort": []map[string]interface{}{
			{
				"crawled_at": map[string]interface{}{
					"order": "asc",
				},
			},
		},
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(indexPattern),
		s.client.Search.WithBody(bytes.NewReader(queryBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error searching: %s", res.String())
	}

	var searchResult struct {
		Hits struct {
			Hits []struct {
				Index  string             `json:"_index"`
				ID     string             `json:"_id"`
				Source domain.RawContent `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&searchResult); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	contents := make([]*domain.RawContent, 0, len(searchResult.Hits.Hits))
	for _, hit := range searchResult.Hits.Hits {
		content := hit.Source
		// Preserve the Elasticsearch document ID if not already set
		if content.ID == "" {
			content.ID = hit.ID
		}
		contents = append(contents, &content)
	}

	return contents, nil
}

// IndexClassifiedContent indexes enriched classified content
// This matches the ElasticsearchClient interface expected by the Poller
func (s *ElasticsearchStorage) IndexClassifiedContent(ctx context.Context, content *domain.ClassifiedContent) error {
	// Determine the classified index name from the source
	classifiedIndex := content.SourceName + "_classified_content"

	// Ensure publisher compatibility aliases are set
	content.Body = content.RawText
	content.Source = content.URL

	// Update classification status
	content.ClassificationStatus = domain.StatusClassified
	now := time.Now()
	content.ClassifiedAt = &now

	docBytes, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	res, err := s.client.Index(
		classifiedIndex,
		bytes.NewReader(docBytes),
		s.client.Index.WithContext(ctx),
		s.client.Index.WithDocumentID(content.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error indexing document: %s", res.String())
	}

	return nil
}

// UpdateRawContentStatus updates the classification_status field in raw_content
// This matches the ElasticsearchClient interface expected by the Poller
func (s *ElasticsearchStorage) UpdateRawContentStatus(ctx context.Context, contentID string, status string, classifiedAt time.Time) error {
	// We need to find which index this document is in
	// For now, we'll update across all *_raw_content indices
	// In production, you might want to track index per content ID

	update := map[string]interface{}{
		"doc": map[string]interface{}{
			"classification_status": status,
			"classified_at":         classifiedAt,
		},
	}

	updateBytes, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	// Try to update in all *_raw_content indices
	// The document will only exist in one of them
	indices, err := s.ListRawContentIndices(ctx)
	if err != nil {
		return fmt.Errorf("failed to list indices: %w", err)
	}

	var lastErr error
	for _, index := range indices {
		res, err := s.client.Update(
			index,
			contentID,
			bytes.NewReader(updateBytes),
			s.client.Update.WithContext(ctx),
		)
		if err != nil {
			lastErr = err
			continue
		}
		defer res.Body.Close()

		if !res.IsError() {
			// Successfully updated
			return nil
		}
		// If we get a 404, the document wasn't in this index - that's expected
		// Continue to next index
		lastErr = fmt.Errorf("error updating document: %s", res.String())
	}

	if lastErr != nil {
		return fmt.Errorf("failed to update document in any index: %w", lastErr)
	}

	return nil
}

// BulkIndexClassifiedContent indexes multiple classified content items
// This matches the ElasticsearchClient interface expected by the Poller
func (s *ElasticsearchStorage) BulkIndexClassifiedContent(ctx context.Context, contents []*domain.ClassifiedContent) error {
	if len(contents) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, content := range contents {
		// Determine the classified index name from the source
		classifiedIndex := content.SourceName + "_classified_content"

		// Create bulk index action
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": classifiedIndex,
				"_id":    content.ID,
			},
		}

		if err := json.NewEncoder(&buf).Encode(meta); err != nil {
			return fmt.Errorf("failed to encode meta: %w", err)
		}

		if err := json.NewEncoder(&buf).Encode(content); err != nil {
			return fmt.Errorf("failed to encode content: %w", err)
		}
	}

	res, err := s.client.Bulk(
		bytes.NewReader(buf.Bytes()),
		s.client.Bulk.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("bulk request failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk indexing error: %s", res.String())
	}

	return nil
}

// ListRawContentIndices lists all *_raw_content indices
func (s *ElasticsearchStorage) ListRawContentIndices(ctx context.Context) ([]string, error) {
	res, err := s.client.Indices.Get(
		[]string{"*_raw_content"},
		s.client.Indices.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list indices: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error listing indices: %s", res.String())
	}

	var indices map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	result := make([]string, 0, len(indices))
	for index := range indices {
		result = append(result, index)
	}

	return result, nil
}

// TestConnection tests the connection to Elasticsearch
func (s *ElasticsearchStorage) TestConnection(ctx context.Context) error {
	res, err := s.client.Info()
	if err != nil {
		return fmt.Errorf("failed to connect to Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error response from Elasticsearch: %s", res.String())
	}

	return nil
}

// GetClassifiedIndexName returns the classified_content index name for a raw_content index
func GetClassifiedIndexName(rawIndex string) (string, error) {
	if len(rawIndex) < 12 || rawIndex[len(rawIndex)-12:] != "_raw_content" {
		return "", errors.New("invalid raw_content index name")
	}
	// Replace "_raw_content" with "_classified_content"
	return rawIndex[:len(rawIndex)-12] + "_classified_content", nil
}
