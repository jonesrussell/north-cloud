package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/jonesrussell/gocrawl/internal/config"
	"github.com/jonesrussell/gocrawl/internal/logger"
	"github.com/jonesrussell/gocrawl/internal/storage/types"
)

// Constants for timeout durations
const (
	DefaultBulkIndexTimeout      = 30 * time.Second
	DefaultIndexTimeout          = 10 * time.Second
	DefaultTestConnectionTimeout = 5 * time.Second
	DefaultSearchTimeout         = 10 * time.Second
)

// StorageParams contains dependencies for creating a storage instance
type StorageParams struct {
	Config config.Interface
	Logger logger.Interface
	Client *es.Client
}

// StorageResult holds the storage instance and index manager
type StorageResult struct {
	Storage      types.Interface
	IndexManager types.IndexManager
}

// NewStorage creates a new storage instance with the given parameters.
func NewStorage(p StorageParams) (StorageResult, error) {
	// Create storage with default options
	opts := DefaultOptions()
	storage := &Storage{
		client: p.Client,
		logger: p.Logger,
		opts:   opts,
	}

	// Create index manager
	indexManager := NewElasticsearchIndexManager(p.Client, p.Logger)
	storage.indexManager = indexManager

	return StorageResult{
		Storage:      storage,
		IndexManager: indexManager,
	}, nil
}

// Storage implements the storage interface
type Storage struct {
	client       *es.Client
	logger       logger.Interface
	opts         Options
	indexManager types.IndexManager
}

// Ensure Storage implements types.Interface
var _ types.Interface = (*Storage)(nil)

// Helper function to create a context with timeout
func (s *Storage) createContextWithTimeout(
	ctx context.Context,
	timeout time.Duration,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// handleResponse handles common response processing: closing body, checking errors, and logging
func (s *Storage) handleResponse(res *esapi.Response, operation string, fields ...any) error {
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			logFields := append([]any{"error", closeErr, "operation", operation}, fields...)
			s.logger.Error("Error closing response body", logFields...)
		}
	}()

	if res.IsError() {
		logFields := append([]any{"error", res.String(), "operation", operation}, fields...)
		s.logger.Error("Elasticsearch returned error response", logFields...)
		return fmt.Errorf("elasticsearch error (%s): %s", operation, res.String())
	}

	return nil
}

// GetIndexManager returns the index manager for this storage
func (s *Storage) GetIndexManager() types.IndexManager {
	return s.indexManager
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
	defer res.Body.Close()

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
