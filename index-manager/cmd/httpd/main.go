package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jonesrussell/north-cloud/index-manager/internal/api"
	"github.com/jonesrussell/north-cloud/index-manager/internal/config"
	"github.com/jonesrussell/north-cloud/index-manager/internal/database"
	"github.com/jonesrussell/north-cloud/index-manager/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/index-manager/internal/logging"
	"github.com/jonesrussell/north-cloud/index-manager/internal/service"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
)

const (
	httpTimeoutSeconds = 15
)

func main() {
	os.Exit(run())
}

func run() int {
	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		return 1
	}

	// Validate configuration
	if validationErr := cfg.Validate(); validationErr != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", validationErr)
		return 1
	}

	// Initialize logger
	log, err := logger.New(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()

	// Create logger adapter for services
	logAdapter := logging.NewAdapter(log)

	log.Info("Starting Index Manager Service",
		logger.String("name", cfg.Service.Name),
		logger.String("version", cfg.Service.Version),
		logger.Int("port", cfg.Service.Port),
	)

	// Initialize dependencies
	esClient, db, cleanup, initErr := initDependencies(cfg, log)
	if initErr != nil {
		log.Error("Failed to initialize dependencies", logger.Error(initErr))
		return 1
	}
	defer cleanup()

	// Initialize and run server
	server := initServer(cfg, esClient, db, logAdapter, log)
	return runServer(server, log)
}

func initDependencies(cfg *config.Config, log logger.Logger) (
	*elasticsearch.Client, *database.Connection, func(), error,
) {
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
		return nil, nil, nil, fmt.Errorf("elasticsearch client: %w", err)
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
		return nil, nil, nil, fmt.Errorf("database connection: %w", err)
	}
	log.Info("Database connection established")

	cleanup := func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("Failed to close database connection", logger.Error(closeErr))
		}
	}

	return esClient, db, cleanup, nil
}

func initServer(
	cfg *config.Config,
	esClient *elasticsearch.Client,
	db *database.Connection,
	logAdapter *logging.Adapter,
	infraLog logger.Logger,
) *infragin.Server {
	// Initialize services
	indexService := service.NewIndexService(esClient, db, logAdapter)
	documentService := service.NewDocumentService(esClient, logAdapter)

	// Initialize API handler
	handler := api.NewHandler(indexService, documentService, logAdapter)

	// Initialize HTTP server
	serverConfig := api.ServerConfig{
		Port:         cfg.Service.Port,
		ReadTimeout:  httpTimeoutSeconds * time.Second,
		WriteTimeout: httpTimeoutSeconds * time.Second,
		Debug:        cfg.Service.Debug,
		ServiceName:  cfg.Service.Name,
	}

	return api.NewServer(handler, serverConfig, logAdapter, infraLog)
}

func runServer(server *infragin.Server, log logger.Logger) int {
	// Run server with graceful shutdown
	if runErr := server.Run(); runErr != nil {
		log.Error("Server error", logger.Error(runErr))
		return 1
	}

	log.Info("Index Manager Service stopped")
	return 0
}
