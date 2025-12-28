package main

import (
	"context"
	"log"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
)

// initElasticsearchClient initializes and tests the Elasticsearch client
func initElasticsearchClient(esURL string) *elasticsearch.Client {
	esCfg := elasticsearch.Config{
		Addresses: []string{esURL},
	}
	esClient, esErr := elasticsearch.NewClient(esCfg)
	if esErr != nil {
		log.Fatalf("Failed to create Elasticsearch client: %v", esErr)
	}

	// Test Elasticsearch connection
	info, infoErr := esClient.Info()
	if infoErr != nil {
		log.Fatalf("Failed to connect to Elasticsearch: %v", infoErr)
	}
	defer info.Body.Close()
	log.Println("Elasticsearch connection established")

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
