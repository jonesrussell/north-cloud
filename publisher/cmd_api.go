package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/api"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 30 * time.Second
	defaultIdleTimeout  = 60 * time.Second
	shutdownTimeout     = 30 * time.Second
)

func runAPIServer() {
	log.Println("Starting Publisher API Server...")

	// Load database configuration from environment
	dbConfig := database.Config{
		Host:     getEnv("POSTGRES_PUBLISHER_HOST", "localhost"),
		Port:     getEnv("POSTGRES_PUBLISHER_PORT", "5432"),
		User:     getEnv("POSTGRES_PUBLISHER_USER", "postgres"),
		Password: getEnv("POSTGRES_PUBLISHER_PASSWORD", ""),
		DBName:   getEnv("POSTGRES_PUBLISHER_DB", "publisher"),
		SSLMode:  getEnv("POSTGRES_PUBLISHER_SSLMODE", "disable"),
	}

	// Initialize database connection
	db, dbErr := database.NewPostgresConnection(dbConfig)
	if dbErr != nil {
		log.Fatalf("Failed to connect to database: %v", dbErr)
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
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("API server listening on port %s", port)
		serveErr := server.ListenAndServe()
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", serveErr)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
		log.Printf("Server forced to shutdown: %v", shutdownErr)
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
		intValue, parseErr := strconv.Atoi(value)
		if parseErr == nil {
			return intValue
		}
	}
	return defaultValue
}
