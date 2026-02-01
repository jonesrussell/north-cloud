package bootstrap

import (
	"context"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/storage"
	esclient "github.com/north-cloud/infrastructure/elasticsearch"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/retry"
)

// SetupElasticsearch creates optional Elasticsearch storage for re-classification.
// Returns nil if ES is unavailable (service can still run).
func SetupElasticsearch(cfg *config.Config, logger infralogger.Logger) *storage.ElasticsearchStorage {
	esURL := cfg.Elasticsearch.URL
	if esURL == "" {
		esURL = "http://localhost:9200"
	}

	const (
		optionalESMaxAttempts  = 3
		optionalESInitialDelay = 1 * time.Second
		optionalESMaxDelay     = 5 * time.Second
		optionalESMultiplier   = 2.0
	)
	esclientCfg := esclient.Config{
		URL: esURL,
		RetryConfig: &retry.Config{
			MaxAttempts:  optionalESMaxAttempts,
			InitialDelay: optionalESInitialDelay,
			MaxDelay:     optionalESMaxDelay,
			Multiplier:   optionalESMultiplier,
		},
	}

	esLog, err := infralogger.New(infralogger.Config{Level: "info", Format: "json"})
	if err != nil {
		esLog = nil
	}

	esClient, err := esclient.NewClient(context.Background(), esclientCfg, esLog)
	if err != nil {
		logger.Warn("Failed to connect to Elasticsearch", infralogger.Error(err))
		logger.Info("Re-classification endpoint will not be available")
		return nil
	}

	esStorage := storage.NewElasticsearchStorage(esClient)
	if err = esStorage.TestConnection(context.Background()); err != nil {
		logger.Warn("Failed to verify Elasticsearch connection", infralogger.Error(err))
		logger.Info("Re-classification endpoint may not work correctly")
		return nil
	}

	logger.Info("Elasticsearch connected successfully")
	return esStorage
}
