package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/api"
	"github.com/jonesrussell/north-cloud/index-manager/internal/config"
	"github.com/jonesrussell/north-cloud/index-manager/internal/database"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/index-manager/internal/logging"
	"github.com/jonesrussell/north-cloud/index-manager/internal/service"
	infraconfig "github.com/north-cloud/infrastructure/config"
	"github.com/north-cloud/infrastructure/logger"
)

const shutdownTimeout = 10 * time.Second

func main() {
	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Create logger adapter for services
	logAdapter := logging.NewAdapter(log)

	log.Info("Starting Index Manager Service",
		logger.String("name", cfg.Service.Name),
		logger.String("version", cfg.Service.Version),
		logger.Int("port", cfg.Service.Port),
	)

	// Initialize Elasticsearch client
	esConfig := &elasticsearch.Config{
		URL:        cfg.Elasticsearch.URL,
		Username:   cfg.Elasticsearch.Username,
		Password:   cfg.Elasticsearch.Password,
		MaxRetries: cfg.Elasticsearch.MaxRetries,
		Timeout:    cfg.Elasticsearch.Timeout,
	}

	esClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		log.Error("Failed to create Elasticsearch client", logger.Error(err))
		os.Exit(1)
	}
	log.Info("Elasticsearch client initialized")

	// Initialize database connection
	dbConfig := &database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		SSLMode:         cfg.Database.SSLMode,
		MaxConnections:  cfg.Database.MaxConnections,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnectionMaxLifetime,
	}

	db, err := database.NewConnection(dbConfig)
	if err != nil {
		log.Error("Failed to connect to database", logger.Error(err))
		os.Exit(1)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("Failed to close database connection", logger.Error(closeErr))
		}
	}()
	log.Info("Database connection established")

	// Initialize services
	indexService := service.NewIndexService(esClient, db, logAdapter)
	documentService := service.NewDocumentService(esClient, logAdapter)

	// Initialize API handler
	handler := api.NewHandler(indexService, documentService, logAdapter)

	// Initialize HTTP server
	const httpTimeoutSeconds = 15
	serverConfig := api.ServerConfig{
		Port:         cfg.Service.Port,
		ReadTimeout:  httpTimeoutSeconds * time.Second,
		WriteTimeout: httpTimeoutSeconds * time.Second,
		Debug:        cfg.Service.Debug,
	}

	server := api.NewServer(handler, serverConfig, logAdapter)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if startErr := server.Start(); startErr != nil {
			serverErr <- startErr
		}
	}()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for signal or error
	select {
	case err := <-serverErr:
		log.Error("Server error", logger.Error(err))
	case sig := <-sigChan:
		log.Info("Shutdown signal received", logger.String("signal", sig.String()))
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if shutdownErr := server.Shutdown(ctx); shutdownErr != nil {
		log.Error("Server shutdown error", logger.Error(shutdownErr))
	}

	log.Info("Index Manager Service stopped")
}
