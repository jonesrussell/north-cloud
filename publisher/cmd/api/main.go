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
	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	redisclient "github.com/jonesrussell/north-cloud/publisher/internal/redis"
	infraconfig "github.com/north-cloud/infrastructure/config"
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

	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional - create default config if file doesn't exist
		log.Printf("Warning: Failed to load config file (%s), using defaults: %v", configPath, err)
		cfg = &config.Config{}
		// Apply defaults manually
		if cfg.Server.Address == "" {
			cfg.Server.Address = ":8070"
		}
		if err := cfg.Validate(); err != nil {
			log.Fatalf("Invalid default configuration: %v", err)
		}
	}

	// Convert config database to database.Config
	dbConfig := database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}

	// Connect to database
	log.Println("Connecting to database...")
	db, dbErr := database.NewPostgresConnection(dbConfig)
	if dbErr != nil {
		log.Fatalf("Failed to connect to database: %v", dbErr)
	}
	defer database.Close(db)
	log.Println("Database connection established")

	// Create repository
	repo := database.NewRepository(db)

	// Initialize Redis client (optional - for health checks)
	var redisClient *redis.Client
	redisAddr := cfg.Redis.URL
	redisPassword := cfg.Redis.Password
	if redisAddr != "" {
		var redisErr error
		redisClient, redisErr = redisclient.NewClient(redisAddr, redisPassword)
		if redisErr != nil {
			log.Printf("Warning: Failed to connect to Redis (health checks will show disconnected): %v", redisErr)
			// Continue without Redis - health check will show disconnected status
		}
	}

	// Create router with config
	router := api.NewRouter(repo, redisClient, cfg)
	ginEngine := router.SetupRoutes()

	// Extract port from server address for display
	portDisplay := cfg.Server.Address
	if portDisplay == "" {
		portDisplay = ":8070"
	}
	if len(portDisplay) > 0 && portDisplay[0] == ':' {
		portDisplay = portDisplay[1:]
	}

	// Configure HTTP server
	server := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      ginEngine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting publisher API server on port %s...", portDisplay)
		serveErr := server.ListenAndServe()
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", serveErr)
		}
	}()

	log.Printf("Publisher API server started successfully on http://localhost:%s", portDisplay)
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
