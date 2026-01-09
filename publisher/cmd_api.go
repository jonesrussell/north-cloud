package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jonesrussell/north-cloud/publisher/internal/api"
	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	redisclient "github.com/jonesrussell/north-cloud/publisher/internal/redis"
	infraconfig "github.com/north-cloud/infrastructure/config"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

func runAPIServer() {
	os.Exit(runAPIServerInternal())
}

func runAPIServerInternal() int {
	log.Println("Starting Publisher API Server...")

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
		if validateErr := cfg.Validate(); validateErr != nil {
			log.Printf("Invalid default configuration: %v", validateErr)
			return 1
		}
	}

	// Initialize logger
	infraLog, logErr := logger.New(logger.Config{
		Level:       "info",
		Format:      "json",
		Development: cfg.Debug,
	})
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", logErr)
		return 1
	}
	defer func() { _ = infraLog.Sync() }()

	// Convert config database to database.Config
	dbConfig := database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}

	// Initialize database connection
	db, dbErr := database.NewPostgresConnection(dbConfig)
	if dbErr != nil {
		infraLog.Error("Failed to connect to database", logger.Error(dbErr))
		return 1
	}
	defer db.Close()

	log.Println("Database connection established")

	// Initialize repository
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

	// Setup router and create server using infrastructure gin
	router := api.NewRouter(repo, redisClient, cfg)
	server := router.NewServer(infraLog)

	// Run server with graceful shutdown
	if runErr := server.Run(); runErr != nil {
		infraLog.Error("Server error", logger.Error(runErr))
		return 1
	}

	log.Println("Server stopped")
	return 0
}
