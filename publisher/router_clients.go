package main

import (
	"context"
	"log"

	"github.com/elastic/go-elasticsearch/v8"
	esclient "github.com/north-cloud/infrastructure/elasticsearch"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

// initElasticsearchClient initializes and tests the Elasticsearch client with retry logic
func initElasticsearchClient(esURL string) *elasticsearch.Client {
	ctx := context.Background()

	// Create a simple logger for connection initialization
	// Using console format and info level for startup messages
	loggerInstance, err := logger.New(logger.Config{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		// Fallback to standard log if logger creation fails
		log.Printf("Failed to create logger, using standard log: %v", err)
		loggerInstance = nil
	}

	cfg := esclient.Config{
		URL: esURL,
	}

	esClient, err := esclient.NewClient(ctx, cfg, loggerInstance)
	if err != nil {
		log.Fatalf("Failed to create Elasticsearch client: %v", err)
	}

	return esClient
}

// initRedisClient initializes and tests the Redis client
func initRedisClient(addr, password string) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	// Test Redis connection
	pingCtx := context.Background()
	if pingErr := redisClient.Ping(pingCtx).Err(); pingErr != nil {
		log.Fatalf("Failed to connect to Redis: %v", pingErr)
	}
	log.Println("Redis connection established")

	return redisClient
}
