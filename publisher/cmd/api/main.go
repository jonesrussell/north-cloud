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

	"github.com/jmoiron/sqlx"
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
	defaultServerPort   = ":8070"
)

func main() {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Load and validate configuration
	cfg := loadAndValidateConfig()

	// Setup database and repository
	db, repo := setupDatabase(cfg)
	defer database.Close(db)

	// Setup Redis client
	redisClient := setupRedis(cfg)

	// Setup and start HTTP server
	server := setupHTTPServer(cfg, repo, redisClient)
	portDisplay := getPortDisplay(cfg.Server.Address)

	// Start server in goroutine
	startServer(server, portDisplay)

	// Wait for interrupt signal and shutdown gracefully
	waitForShutdown(server)
}

// loadAndValidateConfig loads configuration from file or creates default
func loadAndValidateConfig() *config.Config {
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional - create default config if file doesn't exist
		log.Printf("Warning: Failed to load config file (%s), using defaults: %v", configPath, err)
		cfg = &config.Config{}
		// Apply defaults manually
		if cfg.Server.Address == "" {
			cfg.Server.Address = defaultServerPort
		}
		if validateErr := cfg.Validate(); validateErr != nil {
			log.Fatalf("Invalid default configuration: %v", validateErr)
		}
	}
	return cfg
}

// setupDatabase creates database connection and repository
func setupDatabase(cfg *config.Config) (*sqlx.DB, *database.Repository) {
	dbConfig := database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}

	log.Println("Connecting to database...")
	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Database connection established")

	repo := database.NewRepository(db)
	return db, repo
}

// setupRedis creates Redis client if configured
func setupRedis(cfg *config.Config) *redis.Client {
	redisAddr := cfg.Redis.URL
	redisPassword := cfg.Redis.Password
	if redisAddr == "" {
		return nil
	}

	redisClient, err := redisclient.NewClient(redisAddr, redisPassword)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis (health checks will show disconnected): %v", err)
		return nil
	}
	return redisClient
}

// setupHTTPServer creates and configures the HTTP server
func setupHTTPServer(cfg *config.Config, repo *database.Repository, redisClient *redis.Client) *http.Server {
	router := api.NewRouter(repo, redisClient, cfg)
	ginEngine := router.SetupRoutes()

	return &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      ginEngine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
	}
}

// getPortDisplay extracts port from address for display purposes
func getPortDisplay(address string) string {
	if address == "" {
		address = defaultServerPort
	}
	if address != "" && address[0] == ':' {
		return address[1:]
	}
	return address
}

// startServer starts the HTTP server in a goroutine
func startServer(server *http.Server, portDisplay string) {
	go func() {
		log.Printf("Starting publisher API server on port %s...", portDisplay)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	log.Printf("Publisher API server started successfully on http://localhost:%s", portDisplay)
	log.Println("Press Ctrl+C to stop")
}

// waitForShutdown waits for interrupt signal and performs graceful shutdown
func waitForShutdown(server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
