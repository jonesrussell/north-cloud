package bootstrap

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/index-manager/internal/config"
	"github.com/jonesrussell/north-cloud/index-manager/internal/database"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch/mappings"
	infralogger "github.com/north-cloud/infrastructure/logger"
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

// CheckMappingVersionDrift logs warnings for indexes whose mapping version
// is behind the current version constants.
func CheckMappingVersionDrift(db *database.Connection, log infralogger.Logger) {
	ctx := context.Background()
	allMetadata, err := db.ListAllActiveMetadata(ctx)
	if err != nil {
		log.Warn("Failed to check mapping version drift", infralogger.Error(err))
		return
	}

	for _, meta := range allMetadata {
		currentVersion := mappings.GetMappingVersion(meta.IndexType)
		if meta.MappingVersion != currentVersion {
			log.Warn("Index mapping version drift detected",
				infralogger.String("index_name", meta.IndexName),
				infralogger.String("current_version", meta.MappingVersion),
				infralogger.String("latest_version", currentVersion),
				infralogger.String("index_type", meta.IndexType),
			)
		}
	}
}
