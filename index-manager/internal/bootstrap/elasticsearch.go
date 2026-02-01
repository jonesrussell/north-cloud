package bootstrap

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/index-manager/internal/config"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
)

// SetupElasticsearch creates an Elasticsearch client.
func SetupElasticsearch(cfg *config.Config) (*elasticsearch.Client, error) {
	esConfig := &elasticsearch.Config{
		URL:        cfg.Elasticsearch.URL,
		Username:   cfg.Elasticsearch.Username,
		Password:   cfg.Elasticsearch.Password,
		MaxRetries: cfg.Elasticsearch.MaxRetries,
		Timeout:    cfg.Elasticsearch.Timeout,
	}

	esClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch client: %w", err)
	}
	return esClient, nil
}
