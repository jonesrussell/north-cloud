package processor

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
	"github.com/jonesrussell/north-cloud/classifier/internal/storage"
)

// Config holds processor configuration
type Config struct {
	ElasticsearchURL  string
	PostgresHost      string
	PostgresPort      string
	PostgresUser      string
	PostgresPassword  string
	PostgresDB        string
	PostgresSSLMode   string
	PollingInterval   time.Duration
	BatchSize         int
	ConcurrentWorkers int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		ElasticsearchURL:  getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),
		PostgresHost:      getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:      getEnv("POSTGRES_PORT", "5432"),
		PostgresUser:      getEnv("POSTGRES_USER", "postgres"),
		PostgresPassword:  getEnv("POSTGRES_PASSWORD", ""),
		PostgresDB:        getEnv("POSTGRES_DB", "classifier"),
		PostgresSSLMode:   getEnv("POSTGRES_SSLMODE", "disable"),
		PollingInterval:   parseDuration(getEnv("POLLING_INTERVAL", "30s")),
		BatchSize:         parseInt(getEnv("BATCH_SIZE", "100")),
		ConcurrentWorkers: parseInt(getEnv("CONCURRENT_WORKERS", "5")),
	}
}

// Start starts the processor
func Start() error {
	cfg := LoadConfig()

	log.Println("Starting Classifier Processor")
	log.Printf("Elasticsearch URL: %s", cfg.ElasticsearchURL)
	log.Printf("Polling Interval: %s", cfg.PollingInterval)
	log.Printf("Batch Size: %d", cfg.BatchSize)
	log.Printf("Concurrent Workers: %d", cfg.ConcurrentWorkers)

	// Create logger
	logger := storage.NewSimpleLogger("[Processor] ")

	// Create Elasticsearch client
	esCfg := es.Config{
		Addresses: []string{cfg.ElasticsearchURL},
	}
	esClient, err := es.NewClient(esCfg)
	if err != nil {
		return fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	// Test Elasticsearch connection
	esStorage := storage.NewElasticsearchStorage(esClient)
	ctx := context.Background()
	if err := esStorage.TestConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to Elasticsearch: %w", err)
	}
	log.Println("Connected to Elasticsearch")

	// Create PostgreSQL connection
	dbConfig := database.Config{
		Host:     cfg.PostgresHost,
		Port:     cfg.PostgresPort,
		User:     cfg.PostgresUser,
		Password: cfg.PostgresPassword,
		DBName:   cfg.PostgresDB,
		SSLMode:  cfg.PostgresSSLMode,
	}
	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	// Create repositories
	rulesRepo := database.NewRulesRepository(db)
	sourceRepRepo := database.NewSourceReputationRepository(db)
	classificationHistoryRepo := database.NewClassificationHistoryRepository(db)

	// Create database adapter for poller
	dbAdapter := storage.NewDatabaseAdapter(classificationHistoryRepo)

	// Load classification rules from database
	enabledOnly := true
	rules, err := rulesRepo.List(ctx, domain.RuleTypeTopic, &enabledOnly)
	if err != nil {
		return fmt.Errorf("failed to load rules from database: %w", err)
	}
	log.Printf("Loaded %d classification rules from database", len(rules))

	// Convert rules to value slice for classifier
	ruleValues := make([]domain.ClassificationRule, len(rules))
	for i, rule := range rules {
		ruleValues[i] = *rule
	}

	// Create classifier with proper config
	classifierConfig := classifier.Config{
		Version:         "1.0.0",
		MinQualityScore: 30,
		UpdateSourceRep: true,
		QualityConfig: classifier.QualityConfig{
			WordCountWeight:   0.25,
			MetadataWeight:    0.25,
			RichnessWeight:    0.25,
			ReadabilityWeight: 0.25,
			MinWordCount:      100,
			OptimalWordCount:  800,
		},
		SourceReputationConfig: classifier.SourceReputationConfig{
			DefaultScore:               70,
			UpdateOnEachClassification: true,
			SpamThreshold:              30,
			MinArticlesForTrust:        10,
			ReputationDecayRate:        0.95,
		},
	}
	clf := classifier.NewClassifier(logger, ruleValues, sourceRepRepo, classifierConfig)
	log.Println("Classifier initialized")

	// Create batch processor
	batchProcessor := processor.NewBatchProcessor(clf, cfg.ConcurrentWorkers, logger)

	// Create poller
	pollerConfig := processor.PollerConfig{
		BatchSize:    cfg.BatchSize,
		PollInterval: cfg.PollingInterval,
	}
	poller := processor.NewPoller(
		esStorage,
		dbAdapter,
		batchProcessor,
		logger,
		pollerConfig,
	)

	// Start poller
	if err := poller.Start(ctx); err != nil {
		return fmt.Errorf("failed to start poller: %w", err)
	}

	log.Println("Processor started, polling for raw_content...")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down...", sig)
	poller.Stop()

	log.Println("Processor stopped successfully")
	return nil
}

// StartWithStop returns a stop function that can be called to stop the processor
// This allows the processor to run concurrently with other services
func StartWithStop() (func(), error) {
	cfg := LoadConfig()

	log.Println("Starting Classifier Processor")
	log.Printf("Elasticsearch URL: %s", cfg.ElasticsearchURL)
	log.Printf("Polling Interval: %s", cfg.PollingInterval)
	log.Printf("Batch Size: %d", cfg.BatchSize)
	log.Printf("Concurrent Workers: %d", cfg.ConcurrentWorkers)

	// Create logger
	logger := storage.NewSimpleLogger("[Processor] ")

	// Create Elasticsearch client
	esCfg := es.Config{
		Addresses: []string{cfg.ElasticsearchURL},
	}
	esClient, err := es.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	// Test Elasticsearch connection
	esStorage := storage.NewElasticsearchStorage(esClient)
	ctx := context.Background()
	if err := esStorage.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Elasticsearch: %w", err)
	}
	log.Println("Connected to Elasticsearch")

	// Create PostgreSQL connection
	dbConfig := database.Config{
		Host:     cfg.PostgresHost,
		Port:     cfg.PostgresPort,
		User:     cfg.PostgresUser,
		Password: cfg.PostgresPassword,
		DBName:   cfg.PostgresDB,
		SSLMode:  cfg.PostgresSSLMode,
	}
	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	log.Println("Connected to PostgreSQL")

	// Create repositories
	rulesRepo := database.NewRulesRepository(db)
	sourceRepRepo := database.NewSourceReputationRepository(db)
	classificationHistoryRepo := database.NewClassificationHistoryRepository(db)

	// Create database adapter for poller
	dbAdapter := storage.NewDatabaseAdapter(classificationHistoryRepo)

	// Load classification rules from database
	enabledOnly := true
	rules, err := rulesRepo.List(ctx, domain.RuleTypeTopic, &enabledOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to load rules from database: %w", err)
	}
	log.Printf("Loaded %d classification rules from database", len(rules))

	// Convert rules to value slice for classifier
	ruleValues := make([]domain.ClassificationRule, len(rules))
	for i, rule := range rules {
		ruleValues[i] = *rule
	}

	// Create classifier with proper config
	classifierConfig := classifier.Config{
		Version:         "1.0.0",
		MinQualityScore: 30,
		UpdateSourceRep: true,
		QualityConfig: classifier.QualityConfig{
			WordCountWeight:   0.25,
			MetadataWeight:    0.25,
			RichnessWeight:    0.25,
			ReadabilityWeight: 0.25,
			MinWordCount:      100,
			OptimalWordCount:  800,
		},
		SourceReputationConfig: classifier.SourceReputationConfig{
			DefaultScore:               70,
			UpdateOnEachClassification: true,
			SpamThreshold:              30,
			MinArticlesForTrust:        10,
			ReputationDecayRate:        0.95,
		},
	}
	clf := classifier.NewClassifier(logger, ruleValues, sourceRepRepo, classifierConfig)
	log.Println("Classifier initialized")

	// Create batch processor
	batchProcessor := processor.NewBatchProcessor(clf, cfg.ConcurrentWorkers, logger)

	// Create poller
	pollerConfig := processor.PollerConfig{
		BatchSize:    cfg.BatchSize,
		PollInterval: cfg.PollingInterval,
	}
	poller := processor.NewPoller(
		esStorage,
		dbAdapter,
		batchProcessor,
		logger,
		pollerConfig,
	)

	// Start poller
	if err := poller.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start poller: %w", err)
	}

	log.Println("Processor started, polling for raw_content...")

	// Return stop function
	stopFunc := func() {
		log.Println("Stopping processor...")
		poller.Stop()
		_ = db.Close()
		log.Println("Processor stopped successfully")
	}

	return stopFunc, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("Warning: Invalid duration %q, using 30s", s)
		return 30 * time.Second
	}
	return d
}

func parseInt(s string) int {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	if err != nil {
		log.Printf("Warning: Invalid integer %q, using 0", s)
		return 0
	}
	return i
}
