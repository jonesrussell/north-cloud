package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/api"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
)

func runAPIServer() {
	log.Println("Starting Publisher API Server...")

	// Load database configuration from environment
	dbConfig := database.Config{
		Host:     getEnv("POSTGRES_PUBLISHER_HOST", "localhost"),
		Port:     getEnv("POSTGRES_PUBLISHER_PORT", "5432"),
		User:     getEnv("POSTGRES_PUBLISHER_USER", "postgres"),
		Password: getEnv("POSTGRES_PUBLISHER_PASSWORD", ""),
		Database: getEnv("POSTGRES_PUBLISHER_DB", "publisher"),
		SSLMode:  getEnv("POSTGRES_PUBLISHER_SSLMODE", "disable"),
	}

	// Initialize database connection
	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Database connection established")

	// Initialize repository
	repo := database.NewRepository(db)

	// Setup router
	router := api.NewRouter(repo)
	ginEngine := router.SetupRoutes()

	// Get port from environment
	port := getEnv("PUBLISHER_PORT", "8070")

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      ginEngine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("API server listening on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
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
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
