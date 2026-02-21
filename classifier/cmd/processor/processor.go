package processor

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
	"github.com/jonesrussell/north-cloud/classifier/internal/storage"
	infraconfig "github.com/north-cloud/infrastructure/config"
	esclient "github.com/north-cloud/infrastructure/elasticsearch"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/pipeline"
)

const (
	// Processor configuration constants
	defaultMinQualityScore     = 30
	defaultPollInterval        = 30 * time.Second
	defaultQualityWeight       = 0.25
	defaultMinWordCount        = 100
	defaultOptimalWordCount800 = 800
	// Source reputation constants
	defaultReputationScore70     = 70
	defaultReputationDecayRate95 = 0.95
	defaultSpamThreshold         = 30
	minArticlesForTrust          = 10
)

// ProcessorConfig holds processor-specific configuration derived from main config
type ProcessorConfig struct {
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
	PipelineURL       string
}

// LoadConfig loads configuration from config file with env var overrides
func LoadConfig() (*ProcessorConfig, *config.Config) {
	// Load main config
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Use fmt for early config warnings (before logger is initialized)
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config file (%s), using defaults: %v\n", configPath, err)
		// Create default config if file doesn't exist
		cfg = &config.Config{}
		if cfg.Service.PollInterval == 0 {
			cfg.Service.PollInterval = defaultPollInterval
		}
		if cfg.Service.BatchSize == 0 {
			cfg.Service.BatchSize = 100
		}
		if cfg.Service.Concurrency == 0 {
			cfg.Service.Concurrency = 5
		}
	}

	// Convert main config to processor config
	postgresPort := "5432"
	if cfg.Database.Port != 0 {
		postgresPort = strconv.Itoa(cfg.Database.Port)
	}

	return &ProcessorConfig{
		ElasticsearchURL:  cfg.Elasticsearch.URL,
		PostgresHost:      cfg.Database.Host,
		PostgresPort:      postgresPort,
		PostgresUser:      cfg.Database.User,
		PostgresPassword:  cfg.Database.Password,
		PostgresDB:        cfg.Database.Database,
		PostgresSSLMode:   cfg.Database.SSLMode,
		PollingInterval:   cfg.Service.PollInterval,
		BatchSize:         cfg.Service.BatchSize,
		ConcurrentWorkers: cfg.Service.Concurrency,
		PipelineURL:       cfg.Service.PipelineURL,
	}, cfg
}

// setupElasticsearch creates and tests Elasticsearch connection with retry logic
func setupElasticsearch(cfg *ProcessorConfig, log infralogger.Logger) (*storage.ElasticsearchStorage, error) {
	ctx := context.Background()

	// Create a logger for ES connection
	esLog, err := infralogger.New(infralogger.Config{
		Level:       "info",
		Format:      "json",
		Development: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch logger: %w", err)
	}

	// Use standardized client with retry logic
	esCfg := esclient.Config{
		URL: cfg.ElasticsearchURL,
	}

	esClient, err := esclient.NewClient(ctx, esCfg, esLog)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	esStorage := storage.NewElasticsearchStorage(esClient)
	if err = esStorage.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to verify Elasticsearch connection: %w", err)
	}
	log.Info("Elasticsearch connection established",
		infralogger.String("url", cfg.ElasticsearchURL),
	)
	return esStorage, nil
}

// setupDatabase creates PostgreSQL connection and repositories
func setupDatabase(cfg *ProcessorConfig, log infralogger.Logger) (
	*sqlx.DB,
	*database.RulesRepository,
	*database.SourceReputationRepository,
	*database.ClassificationHistoryRepository,
	error,
) {
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
		return nil, nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	log.Info("Database connection established",
		infralogger.String("host", cfg.PostgresHost),
		infralogger.String("port", cfg.PostgresPort),
		infralogger.String("database", cfg.PostgresDB),
	)

	rulesRepo := database.NewRulesRepository(db)
	sourceRepRepo := database.NewSourceReputationRepository(db)
	classificationHistoryRepo := database.NewClassificationHistoryRepository(db)

	return db, rulesRepo, sourceRepRepo, classificationHistoryRepo, nil
}

// loadRules loads classification rules from database
func loadRules(ctx context.Context, rulesRepo *database.RulesRepository, log infralogger.Logger) ([]domain.ClassificationRule, error) {
	enabledOnly := true
	rules, err := rulesRepo.List(ctx, domain.RuleTypeTopic, &enabledOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to load rules from database: %w", err)
	}
	log.Info("Classification rules loaded",
		infralogger.Int("rule_count", len(rules)),
	)

	ruleValues := make([]domain.ClassificationRule, len(rules))
	for i, rule := range rules {
		ruleValues[i] = *rule
	}
	return ruleValues, nil
}

// createClassifierConfig creates classifier configuration with optional crime classifier
func createClassifierConfig(cfg *config.Config, log infralogger.Logger) classifier.Config {
	return classifier.Config{
		Version:         "1.0.0",
		MinQualityScore: defaultMinQualityScore,
		UpdateSourceRep: true,
		QualityConfig: classifier.QualityConfig{
			WordCountWeight:   defaultQualityWeight,
			MetadataWeight:    defaultQualityWeight,
			RichnessWeight:    defaultQualityWeight,
			ReadabilityWeight: defaultQualityWeight,
			MinWordCount:      defaultMinWordCount,
			OptimalWordCount:  defaultOptimalWordCount800,
		},
		SourceReputationConfig: classifier.SourceReputationConfig{
			DefaultScore:               defaultReputationScore70,
			UpdateOnEachClassification: true,
			SpamThreshold:              defaultSpamThreshold,
			MinArticlesForTrust:        minArticlesForTrust,
			ReputationDecayRate:        defaultReputationDecayRate95,
		},
		CrimeClassifier: createCrimeClassifier(cfg, log),
		RoutingTable:    cfg.Classification.Routing,
	}
}

// createCrimeClassifier creates a Crime classifier if enabled in config.
func createCrimeClassifier(cfg *config.Config, log infralogger.Logger) *classifier.CrimeClassifier {
	if !cfg.Classification.Crime.Enabled {
		return nil
	}

	var mlClient classifier.MLClassifier
	if cfg.Classification.Crime.MLServiceURL != "" {
		mlClient = mlclient.NewClient(cfg.Classification.Crime.MLServiceURL)
	}

	log.Info("Crime classifier enabled for processor",
		infralogger.String("ml_service_url", cfg.Classification.Crime.MLServiceURL))

	return classifier.NewCrimeClassifier(mlClient, log, true)
}

// Start starts the processor
func Start() error {
	// Initialize logger
	log, err := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	log = log.With(infralogger.String("service", "classifier-processor"))

	cfg, fullCfg := LoadConfig()

	log.Info("Processor starting",
		infralogger.String("elasticsearch_url", cfg.ElasticsearchURL),
		infralogger.Duration("polling_interval", cfg.PollingInterval),
		infralogger.Int("batch_size", cfg.BatchSize),
		infralogger.Int("concurrent_workers", cfg.ConcurrentWorkers),
	)

	procLogger, err := storage.NewComponentLogger("processor")
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	esStorage, err := setupElasticsearch(cfg, log)
	if err != nil {
		return err
	}

	db, rulesRepo, sourceRepRepo, classificationHistoryRepo, err := setupDatabase(cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error("Error closing database connection", infralogger.Error(closeErr))
		}
	}()

	dbAdapter := storage.NewDatabaseAdapterWithLogger(classificationHistoryRepo, procLogger)

	ctx := context.Background()
	ruleValues, err := loadRules(ctx, rulesRepo, log)
	if err != nil {
		return err
	}

	classifierConfig := createClassifierConfig(fullCfg, log)
	clf := classifier.NewClassifier(procLogger, ruleValues, sourceRepRepo, classifierConfig)
	log.Info("Classifier initialized")

	batchProcessor := processor.NewBatchProcessor(clf, cfg.ConcurrentWorkers, procLogger)

	pipelineClient := pipeline.NewClient(cfg.PipelineURL, "classifier")

	pollerConfig := processor.PollerConfig{
		BatchSize:    cfg.BatchSize,
		PollInterval: cfg.PollingInterval,
	}
	poller := processor.NewPoller(
		esStorage,
		dbAdapter,
		batchProcessor,
		procLogger,
		pollerConfig,
		pipelineClient,
	)

	if err = poller.Start(ctx); err != nil {
		return fmt.Errorf("failed to start poller: %w", err)
	}

	log.Info("Processor started, polling for raw_content")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	log.Info("Shutdown signal received", infralogger.String("signal", sig.String()))
	poller.Stop()

	log.Info("Processor stopped successfully")
	return nil
}

// StartWithStop returns a stop function that can be called to stop the processor
// This allows the processor to run concurrently with other services
func StartWithStop() (func(), error) {
	// Initialize logger
	log, err := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}
	log = log.With(infralogger.String("service", "classifier-processor"))

	cfg, fullCfg := LoadConfig()

	log.Info("Processor starting",
		infralogger.String("elasticsearch_url", cfg.ElasticsearchURL),
		infralogger.Duration("polling_interval", cfg.PollingInterval),
		infralogger.Int("batch_size", cfg.BatchSize),
		infralogger.Int("concurrent_workers", cfg.ConcurrentWorkers),
	)

	procLogger, err := storage.NewComponentLogger("processor")
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	esStorage, err := setupElasticsearch(cfg, log)
	if err != nil {
		return nil, err
	}

	db, rulesRepo, sourceRepRepo, classificationHistoryRepo, err := setupDatabase(cfg, log)
	if err != nil {
		return nil, err
	}

	dbAdapter := storage.NewDatabaseAdapterWithLogger(classificationHistoryRepo, procLogger)

	ctx := context.Background()
	ruleValues, err := loadRules(ctx, rulesRepo, log)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	classifierConfig := createClassifierConfig(fullCfg, log)
	clf := classifier.NewClassifier(procLogger, ruleValues, sourceRepRepo, classifierConfig)
	log.Info("Classifier initialized")

	batchProcessor := processor.NewBatchProcessor(clf, cfg.ConcurrentWorkers, procLogger)

	pipelineClient := pipeline.NewClient(cfg.PipelineURL, "classifier")

	pollerConfig := processor.PollerConfig{
		BatchSize:    cfg.BatchSize,
		PollInterval: cfg.PollingInterval,
	}
	poller := processor.NewPoller(
		esStorage,
		dbAdapter,
		batchProcessor,
		procLogger,
		pollerConfig,
		pipelineClient,
	)

	if err = poller.Start(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to start poller: %w", err)
	}

	log.Info("Processor started, polling for raw_content")

	// Return stop function
	stopFunc := func() {
		log.Info("Stopping processor")
		poller.Stop()
		_ = db.Close()
		log.Info("Processor stopped successfully")
	}

	return stopFunc, nil
}
