package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/search/internal/api"
	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/search/internal/service"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		return 1
	}

	// Initialize logger using infrastructure logger
	log, err := infralogger.New(infralogger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()

	// Add service name to all log entries
	log = log.With(infralogger.String("service", "search-service"))

	log.Info("Starting search service",
		infralogger.String("name", cfg.Service.Name),
		infralogger.String("version", cfg.Service.Version),
		infralogger.Int("port", cfg.Service.Port),
		infralogger.Bool("debug", cfg.Service.Debug),
	)

	// Initialize Elasticsearch client
	log.Info("Connecting to Elasticsearch", infralogger.String("url", cfg.Elasticsearch.URL))
	esClient, esErr := elasticsearch.NewClient(&cfg.Elasticsearch)
	if esErr != nil {
		log.Error("Failed to create Elasticsearch client", infralogger.Error(esErr))
		return 1
	}
	log.Info("Successfully connected to Elasticsearch")

	// Initialize search service
	searchService := service.NewSearchService(esClient, cfg, log)
	log.Info("Search service initialized")

	// Initialize API handler
	handler := api.NewHandler(searchService, log)

	// Create and start HTTP server
	server := api.NewServer(handler, cfg, log)

	log.Info("Search service starting",
		infralogger.Int("port", cfg.Service.Port),
		infralogger.String("elasticsearch_pattern", cfg.Elasticsearch.ClassifiedContentPattern),
	)

	// Run server with graceful shutdown
	if runErr := server.Run(); runErr != nil {
		log.Error("Server error", infralogger.Error(runErr))
		return 1
	}

	log.Info("Search service exited cleanly")
	return 0
}
