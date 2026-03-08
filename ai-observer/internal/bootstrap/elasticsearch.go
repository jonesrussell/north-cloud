package bootstrap

import (
	"context"
	"fmt"

	es "github.com/elastic/go-elasticsearch/v8"
	infraes "github.com/jonesrussell/north-cloud/infrastructure/elasticsearch"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// SetupElasticsearch creates and verifies the ES client using infrastructure package.
func SetupElasticsearch(ctx context.Context, cfg Config, log logger.Logger) (*es.Client, error) {
	client, err := infraes.NewClient(ctx, infraes.Config{
		URL:      cfg.ES.URL,
		Username: cfg.ES.Username,
		Password: cfg.ES.Password,
	}, log)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch client: %w", err)
	}
	return client, nil
}
