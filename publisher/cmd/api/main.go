package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/publisher/internal/api"
	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	redisclient "github.com/jonesrussell/north-cloud/publisher/internal/redis"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
	"github.com/redis/go-redis/v9"
)

const defaultServerPort = ":8070"

func main() {
	os.Exit(run())
}

func run() int {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Load and validate configuration
	cfg := loadAndValidateConfig()

	// Initialize logger
	infraLog, err := logger.New(logger.Config{
		Level:       "info",
		Format:      "json",
		Development: cfg.Debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = infraLog.Sync() }()

	// Setup database and repository
	db, repo := setupDatabase(cfg)
	defer database.Close(db)

	// Setup Redis client
	redisClient := setupRedis(cfg)

	// Setup and start HTTP server using infrastructure gin
	server := setupHTTPServer(cfg, repo, redisClient, infraLog)

	// Run server with graceful shutdown
	if runErr := server.Run(); runErr != nil {
		infraLog.Error("Server error", logger.Error(runErr))
		return 1
	}

	infraLog.Info("Publisher API server stopped")
	return 0
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

// setupHTTPServer creates and configures the HTTP server using infrastructure gin
func setupHTTPServer(
	cfg *config.Config,
	repo *database.Repository,
	redisClient *redis.Client,
	infraLog logger.Logger,
) *infragin.Server {
	router := api.NewRouter(repo, redisClient, cfg)
	return router.NewServer(infraLog)
}
