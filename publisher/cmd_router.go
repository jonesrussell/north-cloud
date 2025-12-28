package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/jonesrussell/north-cloud/publisher/internal/router"
	"github.com/redis/go-redis/v9"
)

func runRouter() {
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
	checkInterval, err := time.ParseDuration(checkIntervalStr)
	if err != nil {
		log.Fatalf("Invalid check interval: %v", err)
	}

	batchSize := getEnvInt("PUBLISHER_ROUTER_BATCH_SIZE", 100)

	// Initialize database connection
	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Database connection established")

	// Initialize repository
	repo := database.NewRepository(db)

	// Initialize Elasticsearch client
	esCfg := elasticsearch.Config{
		Addresses: []string{esURL},
	}
	esClient, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		log.Fatalf("Failed to create Elasticsearch client: %v", err)
	}

	// Test Elasticsearch connection
	info, err := esClient.Info()
	if err != nil {
		log.Fatalf("Failed to connect to Elasticsearch: %v", err)
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
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start router service in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := routerService.Start(ctx); err != nil && err != context.Canceled {
			errChan <- err
		}
	}()

	log.Printf("Router service started (check interval: %s, batch size: %d)", checkInterval, batchSize)

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Println("Received shutdown signal")
		cancel()
	case err := <-errChan:
		log.Printf("Router service error: %v", err)
		cancel()
	}

	// Wait for graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Give the service time to finish current operations
	done := make(chan struct{})
	go func() {
		time.Sleep(5 * time.Second)
		close(done)
	}()

	select {
	case <-done:
		log.Println("Router service stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout exceeded, forcing exit")
	}
}
