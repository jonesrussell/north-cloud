// Package storage provides Elasticsearch storage implementation.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	// HTTPStatusOK is the HTTP status code for successful requests.
	HTTPStatusOK = http.StatusOK
)

// ElasticsearchIndexManager implements the types.IndexManager interface using Elasticsearch.
type ElasticsearchIndexManager struct {
	client *elasticsearch.Client
	logger infralogger.Logger
}

// NewElasticsearchIndexManager creates a new Elasticsearch index manager.
func NewElasticsearchIndexManager(client *elasticsearch.Client, log infralogger.Logger) types.IndexManager {
	return &ElasticsearchIndexManager{
		client: client,
		logger: log,
	}
}

// EnsureIndex ensures an index exists with the given name and mapping.
func (m *ElasticsearchIndexManager) EnsureIndex(ctx context.Context, name string, mapping any) error {
	exists, err := m.IndexExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	mappingBytes, err := json.Marshal(mapping)
	if err != nil {
		return err
	}

	res, err := m.client.Indices.Create(
		name,
		m.client.Indices.Create.WithBody(strings.NewReader(string(mappingBytes))),
		m.client.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating index: %s", res.String())
	}

	return nil
}

// DeleteIndex deletes an index with the given name.
func (m *ElasticsearchIndexManager) DeleteIndex(ctx context.Context, name string) error {
	res, err := m.client.Indices.Delete(
		[]string{name},
		m.client.Indices.Delete.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error deleting index: %s", res.String())
	}

	return nil
}

// IndexExists checks if an index exists.
func (m *ElasticsearchIndexManager) IndexExists(ctx context.Context, name string) (bool, error) {
	res, err := m.client.Indices.Exists(
		[]string{name},
		m.client.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return res.StatusCode == HTTPStatusOK, nil
}

// UpdateMapping updates the mapping for an index.
func (m *ElasticsearchIndexManager) UpdateMapping(ctx context.Context, name string, mapping map[string]any) error {
	mappingBytes, err := json.Marshal(mapping)
	if err != nil {
		return err
	}

	res, err := m.client.Indices.PutMapping(
		[]string{name},
		strings.NewReader(string(mappingBytes)),
		m.client.Indices.PutMapping.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error updating mapping: %s", res.String())
	}

	return nil
}

// GetMapping gets the mapping for an index.
func (m *ElasticsearchIndexManager) GetMapping(ctx context.Context, name string) (map[string]any, error) {
	res, err := m.client.Indices.GetMapping(
		m.client.Indices.GetMapping.WithIndex(name),
		m.client.Indices.GetMapping.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error getting mapping: %s", res.String())
	}

	var result map[string]any
	if decodeErr := json.NewDecoder(res.Body).Decode(&result); decodeErr != nil {
		return nil, decodeErr
	}

	return result, nil
}
