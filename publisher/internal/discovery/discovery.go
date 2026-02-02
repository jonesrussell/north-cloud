package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const classifiedContentSuffix = "_classified_content"

// Service handles Elasticsearch index discovery
type Service struct {
	esClient *elasticsearch.Client
	logger   infralogger.Logger
	indexes  []string
	lastSync time.Time
}

// NewService creates a new discovery service
func NewService(esClient *elasticsearch.Client, logger infralogger.Logger) *Service {
	return &Service{
		esClient: esClient,
		logger:   logger,
		indexes:  []string{},
	}
}

// DiscoverIndexes fetches all classified content indexes from Elasticsearch
func (s *Service) DiscoverIndexes(ctx context.Context) ([]string, error) {
	res, err := s.esClient.Cat.Indices(
		s.esClient.Cat.Indices.WithContext(ctx),
		s.esClient.Cat.Indices.WithIndex("*"+classifiedContentSuffix),
		s.esClient.Cat.Indices.WithFormat("json"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to discover indexes: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.String())
	}

	var response []map[string]string
	if decodeErr := json.NewDecoder(res.Body).Decode(&response); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	indexes := FilterClassifiedIndexes(response)
	s.indexes = indexes
	s.lastSync = time.Now()

	s.logger.Info("Discovered classified content indexes",
		infralogger.Int("count", len(indexes)),
	)

	return indexes, nil
}

// GetIndexes returns the cached list of indexes
func (s *Service) GetIndexes() []string {
	return s.indexes
}

// GetLastSync returns the time of the last successful discovery
func (s *Service) GetLastSync() time.Time {
	return s.lastSync
}

// FilterClassifiedIndexes extracts classified content index names
func FilterClassifiedIndexes(response []map[string]string) []string {
	indexes := make([]string, 0, len(response))
	for _, item := range response {
		indexName, ok := item["index"]
		if !ok {
			continue
		}
		// Skip system indexes (start with .)
		if strings.HasPrefix(indexName, ".") {
			continue
		}
		// Only include classified content indexes
		if strings.HasSuffix(indexName, classifiedContentSuffix) {
			indexes = append(indexes, indexName)
		}
	}
	return indexes
}
