package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// Helper function to create a context with timeout
func (s *Storage) createContextWithTimeout(
	ctx context.Context,
	timeout time.Duration,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// closeResponse safely closes an Elasticsearch response body and logs any errors
// For operations that don't have a docID (like searches), pass empty string
func (s *Storage) closeResponse(res *esapi.Response, operation, index, docID string) {
	if closeErr := res.Body.Close(); closeErr != nil {
		fields := []any{
			"error", closeErr,
			"operation", operation,
		}
		if index != "" {
			fields = append(fields, "index", index)
		}
		if docID != "" {
			fields = append(fields, "doc_id", docID)
		}
		s.logger.Error("Failed to close response body", fields...)
	}
}

// logOperationError logs storage operation errors with context
func (s *Storage) logOperationError(operation, index, docID string, err error) {
	s.logger.Error("Storage operation failed",
		"operation", operation,
		"index", index,
		"doc_id", docID,
		"error", err)
}

// getURLFromDocument extracts the URL from a document
func getURLFromDocument(doc any) string {
	switch v := doc.(type) {
	case *domain.Article:
		return v.Source
	case *domain.Content:
		return v.URL
	case *domain.Page:
		return v.URL
	default:
		return ""
	}
}

// marshalJSON marshals the given value to JSON and returns an error if it fails
func marshalJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return data, nil
}

// TestConnection tests the connection to the storage backend
func (s *Storage) TestConnection(ctx context.Context) error {
	if s.client == nil {
		return errors.New("elasticsearch client is nil")
	}

	res, err := s.client.Ping(s.client.Ping.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("error pinging storage: %w", err)
	}
	defer s.closeResponse(res, "TestConnection", "", "")

	if res.IsError() {
		return fmt.Errorf("error pinging storage: %s", res.String())
	}

	return nil
}

// Close closes any resources held by the search manager.
func (s *Storage) Close() error {
	// No resources to close in this implementation
	return nil
}
