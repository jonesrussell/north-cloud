// Package storage provides Elasticsearch storage implementation.
package storage

import (
	"context"
	"errors"
	"fmt"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/elasticsearch"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	esclient "github.com/north-cloud/infrastructure/elasticsearch"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// ClientParams contains dependencies for creating the Elasticsearch client
type ClientParams struct {
	Config config.Interface
	Logger logger.Interface
}

// ClientResult contains the Elasticsearch client
type ClientResult struct {
	Client *es.Client
}

// NewClient creates a new Elasticsearch client using the standardized infrastructure client
func NewClient(p ClientParams) (ClientResult, error) {
	ctx := context.Background()

	// Get Elasticsearch config
	esConfig := p.Config.GetElasticsearchConfig()
	if esConfig == nil {
		return ClientResult{}, errors.New("elasticsearch configuration is required")
	}

	// Log the addresses being used for debugging
	if len(esConfig.Addresses) > 0 {
		p.Logger.Debug("Connecting to Elasticsearch", "addresses", esConfig.Addresses)
	}

	// Get the first address (standardized client uses single URL)
	url := elasticsearch.DefaultAddresses
	if len(esConfig.Addresses) > 0 {
		url = esConfig.Addresses[0]
	}

	// Create infrastructure logger adapter
	var infLog infralogger.Logger
	if p.Logger != nil {
		// Create a basic logger for the standardized client
		// The crawler's logger interface is different, so we'll create a new one
		var err error
		infLog, err = infralogger.New(infralogger.Config{
			Level:  "info",
			Format: "json",
		})
		if err != nil {
			// Continue without logger if creation fails
			infLog = nil
		}
	}

	// Map crawler config to standardized config
	esCfg := esclient.Config{
		URL:         url,
		Username:    esConfig.Username,
		Password:    esConfig.Password,
		APIKey:      esConfig.APIKey,
		CloudID:     esConfig.Cloud.ID,
		CloudAPIKey: esConfig.Cloud.APIKey,
		MaxRetries:  esConfig.Retry.MaxRetries,
	}

	// Map TLS config if present
	if esConfig.TLS != nil && esConfig.TLS.Enabled {
		esCfg.TLS = &esclient.TLSConfig{
			Enabled:            esConfig.TLS.Enabled,
			InsecureSkipVerify: esConfig.TLS.InsecureSkipVerify,
			CertFile:           esConfig.TLS.CertFile,
			KeyFile:            esConfig.TLS.KeyFile,
			CAFile:             esConfig.TLS.CAFile,
		}
	}

	// Use standardized client with retry logic
	client, err := esclient.NewClient(ctx, esCfg, infLog)
	if err != nil {
		return ClientResult{}, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	return ClientResult{
		Client: client,
	}, nil
}
