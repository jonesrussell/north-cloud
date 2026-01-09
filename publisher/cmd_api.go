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
	// Initialize logger early (before config loading to use structured logging)
	// Use default config for logger initialization
	infraLog, logErr := logger.New(logger.Config{
		Level:       "info",
		Format:      "json",
		Development: false, // Will be updated after config load
	})
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", logErr)
		return 1
	}
	defer func() { _ = infraLog.Sync() }()

	// Add service field to all log entries
	infraLog = infraLog.With(logger.String("service", "publisher-api"))

	infraLog.Info("Starting Publisher API Server")

	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional - create default config if file doesn't exist
		infraLog.Warn("Failed to load config file, using defaults",
			logger.String("config_path", configPath),
			logger.Error(err),
		)
		cfg = &config.Config{}
		// Apply defaults manually
		if cfg.Server.Address == "" {
			cfg.Server.Address = ":8070"
		}
		if validateErr := cfg.Validate(); validateErr != nil {
			infraLog.Fatal("Invalid default configuration", logger.Error(validateErr))
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

	// Initialize database connection
	infraLog.Info("Connecting to database")
	db, dbErr := database.NewPostgresConnection(dbConfig)
	if dbErr != nil {
		infraLog.Fatal("Failed to connect to database", logger.Error(dbErr))
	}
	defer db.Close()

	infraLog.Info("Database connection established")

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

	infraLog.Info("Server stopped")
	return 0
}
