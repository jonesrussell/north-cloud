package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/api"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	redisclient "github.com/jonesrussell/north-cloud/publisher/internal/redis"
	"github.com/north-cloud/infrastructure/profiling"
	"github.com/redis/go-redis/v9"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 30 * time.Second
	defaultIdleTimeout  = 60 * time.Second
	shutdownTimeout     = 30 * time.Second
)

func main() {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Load database configuration from environment
	dbConfig := database.Config{
		Host:     getEnv("POSTGRES_PUBLISHER_HOST", "localhost"),
		Port:     getEnv("POSTGRES_PUBLISHER_PORT", "5432"),
		User:     getEnv("POSTGRES_PUBLISHER_USER", "postgres"),
		Password: getEnv("POSTGRES_PUBLISHER_PASSWORD", "postgres"),
		DBName:   getEnv("POSTGRES_PUBLISHER_DB", "publisher"),
		SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
	}

	// Connect to database
	log.Println("Connecting to database...")
	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close(db)
	log.Println("Database connection established")

	// Create repository
	repo := database.NewRepository(db)

	// Initialize Redis client (optional - for health checks)
	var redisClient *redis.Client
	redisAddr := getEnv("REDIS_ADDR", "")
	if redisAddr == "" {
		// Fallback to REDIS_HOST:REDIS_PORT if REDIS_ADDR not set
		redisHost := getEnv("REDIS_HOST", "localhost")
		redisPort := getEnv("REDIS_PORT", "6379")
		redisAddr = redisHost + ":" + redisPort
	}
	redisPassword := getEnv("REDIS_PASSWORD", "")
	if redisAddr != "" {
		var redisErr error
		redisClient, redisErr = redisclient.NewClient(redisAddr, redisPassword)
		if redisErr != nil {
			log.Printf("Warning: Failed to connect to Redis (health checks will show disconnected): %v", redisErr)
			// Continue without Redis - health check will show disconnected status
		}
	}

	// Create router
	router := api.NewRouter(repo, redisClient)
	ginEngine := router.SetupRoutes()

	// Configure HTTP server
	port := getEnv("PUBLISHER_PORT", "8070")
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      ginEngine,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting publisher API server on port %s...", port)
		serveErr := server.ListenAndServe()
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", serveErr)
		}
	}()

	log.Printf("Publisher API server started successfully on http://localhost:%s", port)
	log.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
		log.Printf("Server forced to shutdown: %v", shutdownErr)
	}

	log.Println("Server exited")
}

// getEnv retrieves environment variable or returns default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
