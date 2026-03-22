package main

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8"
	esclient "github.com/jonesrussell/north-cloud/infrastructure/elasticsearch"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
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
	redisClient := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	// Test Redis connection
	pingCtx := context.Background()
	if pingErr := redisClient.Ping(pingCtx).Err(); pingErr != nil {
		appLogger.Fatal("Failed to connect to Redis", logger.Error(pingErr))
	}
	appLogger.Info("Redis connection established")

	return redisClient
}
