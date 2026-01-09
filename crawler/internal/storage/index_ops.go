package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// CreateIndex creates a new index with the specified mapping
func (s *Storage) CreateIndex(
	ctx context.Context,
	index string,
	mapping map[string]any,
) error {
	ctx, cancel := s.createContextWithTimeout(ctx, DefaultIndexTimeout)
	defer cancel()

	// Only add body if mapping is not empty
	// Elasticsearch allows creating an index without a body
	var res *esapi.Response
	var err error

	if len(mapping) > 0 {
		// Create index with mapping
		var buf bytes.Buffer
		if encodeErr := json.NewEncoder(&buf).Encode(mapping); encodeErr != nil {
			s.logger.Error("Failed to create index",
				infralogger.String("index", index),
				infralogger.Error(encodeErr))
			return fmt.Errorf("error encoding mapping: %w", encodeErr)
		}
		res, err = s.client.Indices.Create(
			index,
			s.client.Indices.Create.WithContext(ctx),
			s.client.Indices.Create.WithBody(&buf),
		)
	} else {
		// Create index without body (uses default settings)
		res, err = s.client.Indices.Create(
			index,
			s.client.Indices.Create.WithContext(ctx),
		)
	}
	if err != nil {
		s.logOperationError("CreateIndex", index, "", err)
		return fmt.Errorf("failed to create index: %w", err)
	}
	defer s.closeResponse(res, "CreateIndex", index, "")

	if res.IsError() {
		s.logger.Error("Failed to create index",
			infralogger.String("index", index),
			infralogger.String("error", res.String()))
		return fmt.Errorf("failed to create index: %s", res.String())
	}

	s.logger.Info("Created index", infralogger.String("index", index))
	return nil
}

// DeleteIndex deletes an index
func (s *Storage) DeleteIndex(ctx context.Context, index string) error {
	ctx, cancel := s.createContextWithTimeout(ctx, DefaultIndexTimeout)
	defer cancel()

	// Call API with []string{index} but keep index as string
	res, err := s.client.Indices.Delete(
		[]string{index},
		s.client.Indices.Delete.WithContext(ctx),
	)
	if err != nil {
		s.logOperationError("DeleteIndex", index, "", err)
		return fmt.Errorf("error deleting index: %w", err)
	}
	defer s.closeResponse(res, "DeleteIndex", index, "")

	if res.IsError() {
		s.logger.Error("Failed to delete index",
			infralogger.String("error", res.String()),
			infralogger.String("index", index))
		return fmt.Errorf("error deleting index: %s", res.String())
	}

	s.logger.Info("Deleted index", infralogger.String("index", index))
	return nil
}

// IndexExists checks if the specified index exists
func (s *Storage) IndexExists(ctx context.Context, indexName string) (bool, error) {
	ctx, cancel := s.createContextWithTimeout(ctx, DefaultTestConnectionTimeout)
	defer cancel()

	res, err := s.client.Indices.Exists([]string{indexName}, s.client.Indices.Exists.WithContext(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to check index existence: %w", err)
	}
	defer s.closeResponse(res, "IndexExists", indexName, "")

	return res.StatusCode == http.StatusOK, nil
}

// ListIndices lists all index in the cluster
func (s *Storage) ListIndices(ctx context.Context) ([]string, error) {
	res, err := s.client.Cat.Indices(
		s.client.Cat.Indices.WithContext(ctx),
		s.client.Cat.Indices.WithFormat("json"),
	)
	if err != nil {
		s.logger.Error("Failed to list index", infralogger.Error(err))
		return nil, fmt.Errorf("failed to list index: %w", err)
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			s.logger.Error("Error closing response body", infralogger.Error(closeErr))
		}
	}()

	if res.IsError() {
		s.logger.Error("Failed to list index", infralogger.String("error", res.String()))
		return nil, fmt.Errorf("error listing index: %s", res.String())
	}

	var index []struct {
		Index string `json:"index"`
	}
	if decodeErr := json.NewDecoder(res.Body).Decode(&index); decodeErr != nil {
		s.logger.Error("Failed to list index", infralogger.Error(decodeErr))
		return nil, fmt.Errorf("error decoding index: %w", decodeErr)
	}

	result := make([]string, len(index))
	for i, idx := range index {
		result[i] = idx.Index
	}

	s.logger.Info("Retrieved index list")
	return result, nil
}
