package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/search/internal/api"
	"github.com/jonesrussell/north-cloud/search/internal/config"
	"github.com/jonesrussell/north-cloud/search/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/search/internal/service"
	"github.com/north-cloud/infrastructure/clickurl"
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
	if pyroProfiler, pyroErr := profiling.StartPyroscope("search"); pyroErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Pyroscope failed to start: %v\n", pyroErr)
	} else if pyroProfiler != nil {
		defer pyroProfiler.Stop() //nolint:errcheck // best-effort cleanup
	}

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		return 1
	}

	// Initialize logger
	log, err := createLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting search service",
		infralogger.String("name", cfg.Service.Name),
		infralogger.String("version", cfg.Service.Version),
		infralogger.Int("port", cfg.Service.Port),
		infralogger.Bool("debug", cfg.Service.Debug),
	)

	// Setup Elasticsearch
	esClient, err := setupElasticsearch(cfg, log)
	if err != nil {
		log.Error("Failed to create Elasticsearch client", infralogger.Error(err))
		return 1
	}

	// Setup search service and run server
	return runServer(cfg, esClient, log)
}

// loadConfig loads configuration from config file.
func loadConfig() (*config.Config, error) {
	configPath := infraconfig.GetConfigPath("config.yml")
	return config.Load(configPath)
}

// createLogger creates a logger instance from configuration.
func createLogger(cfg *config.Config) (infralogger.Logger, error) {
	log, err := infralogger.New(infralogger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		return nil, err
	}
	return log.With(infralogger.String("service", "search-service")), nil
}

// setupElasticsearch creates and connects the Elasticsearch client.
func setupElasticsearch(cfg *config.Config, log infralogger.Logger) (*elasticsearch.Client, error) {
	log.Info("Connecting to Elasticsearch", infralogger.String("url", cfg.Elasticsearch.URL))
	esClient, err := elasticsearch.NewClient(&cfg.Elasticsearch)
	if err != nil {
		return nil, err
	}
	log.Info("Successfully connected to Elasticsearch")
	return esClient, nil
}

// runServer creates the search service, handler, and HTTP server, then runs with graceful shutdown.
func runServer(cfg *config.Config, esClient *elasticsearch.Client, log infralogger.Logger) int {
	// Create click URL signer if enabled
	var clickSigner *clickurl.Signer
	if cfg.ClickTracker.Enabled && cfg.ClickTracker.Secret != "" {
		clickSigner = clickurl.NewSigner(cfg.ClickTracker.Secret)
		log.Info("Click tracking enabled",
			infralogger.String("base_url", cfg.ClickTracker.BaseURL),
		)
	}

	searchService := service.NewSearchService(esClient, cfg, log, clickSigner)
	log.Info("Search service initialized")

	handler := api.NewHandler(searchService, log)
	server := api.NewServer(handler, cfg, log)

	log.Info("Search service starting",
		infralogger.Int("port", cfg.Service.Port),
		infralogger.String("elasticsearch_pattern", cfg.Elasticsearch.ClassifiedContentPattern),
	)

	if runErr := server.Run(); runErr != nil {
		log.Error("Server error", infralogger.Error(runErr))
		return 1
	}

	log.Info("Search service exited cleanly")
	return 0
}
