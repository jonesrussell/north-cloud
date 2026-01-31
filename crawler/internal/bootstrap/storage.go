package bootstrap

import (
	"fmt"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// StorageComponents holds both storage interface and index manager.
type StorageComponents struct {
	Client       *es.Client
	Storage      types.Interface
	IndexManager types.IndexManager
}

// SetupStorage creates both storage client and storage instance.
func SetupStorage(cfg config.Interface, log infralogger.Logger) (*StorageComponents, error) {
	// Create storage client
	client, err := createStorageClient(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}

	// Create storage
	storageResult, err := storage.NewStorage(storage.StorageParams{
		Config: cfg,
		Logger: log,
		Client: client,
	})
	if err != nil {
		return nil, fmt.Errorf("create storage: %w", err)
	}

	return &StorageComponents{
		Client:       client,
		Storage:      storageResult.Storage,
		IndexManager: storageResult.IndexManager,
	}, nil
}

// createStorageClient creates an Elasticsearch client with the given config and logger.
func createStorageClient(cfg config.Interface, log infralogger.Logger) (*es.Client, error) {
	clientResult, err := storage.NewClient(storage.ClientParams{
		Config: cfg,
		Logger: log,
	})
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}
	return clientResult.Client, nil
}
