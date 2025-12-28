package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	"github.com/jonesrussell/north-cloud/publisher/internal/router"
)

func runRouter() {
	log.Println("Starting Publisher Router Service...")

	// Load configuration
	cfg := LoadRouterConfig()

	// Initialize database connection
	db, dbErr := database.NewPostgresConnection(cfg.Database)
	if dbErr != nil {
		log.Fatalf("Failed to connect to database: %v", dbErr)
	}
	defer db.Close()
	log.Println("Database connection established")

	// Initialize repository
	repo := database.NewRepository(db)

	// Initialize Elasticsearch client
	esClient := initElasticsearchClient(cfg.ESURL)

	// Initialize Redis client
	redisClient := initRedisClient(cfg.RedisAddr, cfg.RedisPassword)
	defer redisClient.Close()

	// Initialize router service
	routerConfig := router.Config{
		CheckInterval: cfg.CheckInterval,
		BatchSize:     cfg.BatchSize,
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

	log.Printf("Router service started (check interval: %s, batch size: %d)", cfg.CheckInterval, cfg.BatchSize)

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
	const shutdownTimeout = 30 * time.Second
	const gracefulShutdownWait = 5 * time.Second
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
