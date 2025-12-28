package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/redis/go-redis/v9"
)

const (
	defaultBatchSize     = 100
	shutdownTimeout      = 30 * time.Second
	gracefulShutdownWait = 5 * time.Second
)

func main() {
	log.Println("Starting Publisher Router Service...")

	// Load configuration from environment
	dbConfig := database.Config{
		Host:     getEnv("POSTGRES_PUBLISHER_HOST", "localhost"),
		Port:     getEnv("POSTGRES_PUBLISHER_PORT", "5432"),
		User:     getEnv("POSTGRES_PUBLISHER_USER", "postgres"),
		Password: getEnv("POSTGRES_PUBLISHER_PASSWORD", ""),
		DBName:   getEnv("POSTGRES_PUBLISHER_DB", "publisher"),
		SSLMode:  getEnv("POSTGRES_PUBLISHER_SSLMODE", "disable"),
	}

	esURL := getEnv("ELASTICSEARCH_URL", "http://localhost:9200")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")

	checkIntervalStr := getEnv("PUBLISHER_ROUTER_CHECK_INTERVAL", "5m")
	checkInterval, parseErr := time.ParseDuration(checkIntervalStr)
	if parseErr != nil {
		log.Fatalf("Invalid check interval: %v", parseErr)
	}

	batchSize := getEnvInt("PUBLISHER_ROUTER_BATCH_SIZE", defaultBatchSize)

	// Initialize database connection
	db, dbErr := database.NewPostgresConnection(dbConfig)
	if dbErr != nil {
		log.Fatalf("Failed to connect to database: %v", dbErr)
	}
	defer db.Close()

	log.Println("Database connection established")

	// Initialize repository
	repo := database.NewRepository(db)

	// Initialize Elasticsearch client
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

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})

	// Test Redis connection
	pingCtx := context.Background()
	if pingErr := redisClient.Ping(pingCtx).Err(); pingErr != nil {
		log.Fatalf("Failed to connect to Redis: %v", pingErr)
	}
	defer redisClient.Close()
	log.Println("Redis connection established")

	// Initialize router service
	routerConfig := router.Config{
		CheckInterval: checkInterval,
		BatchSize:     batchSize,
	}
	routerService := router.NewService(repo, esClient, redisClient, routerConfig)

	// Setup graceful shutdown
	serviceCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start router service in goroutine
	errChan := make(chan error, 1)
	go func() {
		startErr := routerService.Start(serviceCtx)
		if startErr != nil && !errors.Is(startErr, context.Canceled) {
			errChan <- startErr
		}
	}()

	log.Printf("Router service started (check interval: %s, batch size: %d)", checkInterval, batchSize)

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Println("Received shutdown signal")
		cancel()
	case serviceErr := <-errChan:
		log.Printf("Router service error: %v", serviceErr)
		cancel()
	}

	// Wait for graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Give the service time to finish current operations
	done := make(chan struct{})
	go func() {
		time.Sleep(gracefulShutdownWait)
		close(done)
	}()

	select {
	case <-done:
		log.Println("Router service stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout exceeded, forcing exit")
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intValue, parseErr := strconv.Atoi(value)
		if parseErr == nil {
			return intValue
		}
	}
	return defaultValue
}
