package storage

import (
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
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

// GetIndexManager returns the index manager for this storage
func (s *Storage) GetIndexManager() types.IndexManager {
	return s.indexManager
}
