package main

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8"
	esclient "github.com/jonesrussell/north-cloud/infrastructure/elasticsearch"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
	infraredis "github.com/jonesrussell/north-cloud/infrastructure/redis"
	"github.com/redis/go-redis/v9"
)

// initElasticsearchClient initializes and tests the Elasticsearch client with retry logic
func initElasticsearchClient(esURL string, appLogger logger.Logger) *elasticsearch.Client {
	ctx := context.Background()

	cfg := esclient.Config{
		URL: esURL,
	}

	esClient, err := esclient.NewClient(ctx, cfg, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to create Elasticsearch client", logger.Error(err))
	}

	return esClient
}

// initRedisClient initializes and tests the Redis client
func initRedisClient(addr, password string, appLogger logger.Logger) *redis.Client {
	client, err := infraredis.NewClient(infraredis.Config{
		Address:  addr,
		Password: password,
	})
	if err != nil {
		appLogger.Fatal("Failed to connect to Redis", logger.Error(err))
	}
	appLogger.Info("Redis connection established")
	return client
}
