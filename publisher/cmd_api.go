package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/publisher/internal/api"
	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	redisclient "github.com/jonesrussell/north-cloud/publisher/internal/redis"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

func runAPIServer() {
	os.Exit(runAPIServerInternal())
}

// runAPIServerWithStop starts the API server and returns a stop function
// This allows the server to run concurrently with other services
//
//nolint:funlen // Function length is acceptable for server initialization
func runAPIServerWithStop() (func(), error) {
	// Initialize logger early (before config loading to use structured logging)
	infraLog, logErr := infralogger.New(infralogger.Config{
		Level:       "info",
		Format:      "json",
		Development: false, // Will be updated after config load
	})
	if logErr != nil {
		return nil, fmt.Errorf("failed to create logger: %w", logErr)
	}

	// Add service field to all log entries
	infraLog = infraLog.With(infralogger.String("service", "publisher-api"))

	infraLog.Info("Starting Publisher API Server")

	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional - try loading from environment variables only
		infraLog.Warn("Failed to load config file, trying environment variables",
			infralogger.String("config_path", configPath),
			infralogger.Error(err),
		)
		cfg = &config.Config{}
		// Apply environment variables to the config
		if envErr := infraconfig.ApplyEnvOverrides(cfg); envErr != nil {
			infraLog.Error("Failed to apply environment overrides", infralogger.Error(envErr))
		}
		// Apply defaults for any missing values
		config.SetDefaults(cfg)
		if validateErr := cfg.Validate(); validateErr != nil {
			infraLog.Error("Invalid configuration from environment", infralogger.Error(validateErr))
			_ = infraLog.Sync()
			return nil, fmt.Errorf("invalid configuration from environment: %w", validateErr)
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
		infraLog.Error("Failed to connect to database", infralogger.Error(dbErr))
		_ = infraLog.Sync()
		return nil, fmt.Errorf("failed to connect to database: %w", dbErr)
	}

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
			infraLog.Warn("Failed to connect to Redis (health checks will show disconnected)",
				infralogger.Error(redisErr),
			)
			// Continue without Redis - health check will show disconnected status
		}
	}

	// Initialize Elasticsearch client (optional - for indexes endpoint)
	var esClient = initElasticsearchClientOptional(cfg.Elasticsearch.URL, infraLog)

	// Setup router and create server using infrastructure gin
	router := api.NewRouter(repo, redisClient, esClient, cfg)
	server := router.NewServer(infraLog)

	// Start server in goroutine
	errChan := server.StartAsync()

	// Monitor server errors in background
	go func() {
		for err := range errChan {
			if err != nil {
				infraLog.Error("Server error", infralogger.Error(err))
			}
		}
	}()

	// Return stop function
	stop := func() {
		infraLog.Info("Stopping API server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), server.Config().ShutdownTimeout)
		defer cancel()

		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			infraLog.Error("Error shutting down API server", infralogger.Error(shutdownErr))
		}

		db.Close()
		_ = infraLog.Sync()
		infraLog.Info("API server stopped")
	}

	return stop, nil
}

func runAPIServerInternal() int {
	// Initialize logger early (before config loading to use structured logging)
	// Use default config for logger initialization
	infraLog, logErr := infralogger.New(infralogger.Config{
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
	infraLog = infraLog.With(infralogger.String("service", "publisher-api"))

	infraLog.Info("Starting Publisher API Server")

	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional - try loading from environment variables only
		infraLog.Warn("Failed to load config file, trying environment variables",
			infralogger.String("config_path", configPath),
			infralogger.Error(err),
		)
		cfg = &config.Config{}
		// Apply environment variables to the config
		if envErr := infraconfig.ApplyEnvOverrides(cfg); envErr != nil {
			infraLog.Error("Failed to apply environment overrides", infralogger.Error(envErr))
		}
		// Apply defaults for any missing values
		config.SetDefaults(cfg)
		if validateErr := cfg.Validate(); validateErr != nil {
			infraLog.Fatal("Invalid configuration from environment", infralogger.Error(validateErr))
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
		infraLog.Fatal("Failed to connect to database", infralogger.Error(dbErr))
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
			infraLog.Warn("Failed to connect to Redis (health checks will show disconnected)",
				infralogger.Error(redisErr),
			)
			// Continue without Redis - health check will show disconnected status
		}
	}

	// Initialize Elasticsearch client (optional - for indexes endpoint)
	var esClient = initElasticsearchClientOptional(cfg.Elasticsearch.URL, infraLog)

	// Setup router and create server using infrastructure gin
	router := api.NewRouter(repo, redisClient, esClient, cfg)
	server := router.NewServer(infraLog)

	// Run server with graceful shutdown
	if runErr := server.Run(); runErr != nil {
		infraLog.Error("Server error", infralogger.Error(runErr))
		return 1
	}

	infraLog.Info("Server stopped")
	return 0
}
