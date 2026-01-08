package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/api"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
	"github.com/jonesrussell/north-cloud/classifier/internal/storage"
	esclient "github.com/north-cloud/infrastructure/elasticsearch"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
	"github.com/north-cloud/infrastructure/retry"
)

const (
	// Server configuration constants
	defaultClassifierPort = 8070
	defaultHTTPTimeout    = 30 * time.Second
	defaultConcurrency    = 10
	// Classifier configuration constants
	defaultMinQualityScore50    = 50
	defaultQualityWeight025     = 0.25
	defaultMinWordCount100      = 100
	defaultOptimalWordCount1000 = 1000
	// Source reputation constants
	defaultReputationScore50     = 50
	defaultSpamThreshold         = 30
	minArticlesForTrust          = 10
	defaultReputationDecayRate01 = 0.1
)

// Logger is a simple logger implementation
type Logger struct {
	debug bool
}

func NewLogger(debug bool) *Logger {
	return &Logger{debug: debug}
}

func (l *Logger) Debug(msg string, keysAndValues ...any) {
	if l.debug {
		log.Printf("[DEBUG] %s %v\n", msg, keysAndValues)
	}
}

func (l *Logger) Info(msg string, keysAndValues ...any) {
	log.Printf("[INFO] %s %v\n", msg, keysAndValues)
}

func (l *Logger) Warn(msg string, keysAndValues ...any) {
	log.Printf("[WARN] %s %v\n", msg, keysAndValues)
}

func (l *Logger) Error(msg string, keysAndValues ...any) {
	log.Printf("[ERROR] %s %v\n", msg, keysAndValues)
}

// serverComponents holds all components needed for the HTTP server
type serverComponents struct {
	db                        *sqlx.DB
	rulesRepo                 *database.RulesRepository
	sourceRepRepo             *database.SourceReputationRepository
	classificationHistoryRepo *database.ClassificationHistoryRepository
	esStorage                 *storage.ElasticsearchStorage
	classifierInstance        *classifier.Classifier
	batchProcessor            *processor.BatchProcessor
	sourceRepScorer           *classifier.SourceReputationScorer
	topicClassifier           *classifier.TopicClassifier
	handler                   *api.Handler
}

// setupDatabaseAndRepos creates database connection and repositories
func setupDatabaseAndRepos(logger *Logger) (*serverComponents, error) {
	dbConfig := database.Config{
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     getEnv("POSTGRES_PORT", "5432"),
		User:     getEnv("POSTGRES_USER", "postgres"),
		Password: getEnv("POSTGRES_PASSWORD", ""),
		DBName:   getEnv("POSTGRES_DB", "classifier"),
		SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
	}

	logger.Info("Connecting to PostgreSQL database",
		"host", dbConfig.Host,
		"port", dbConfig.Port,
		"database", dbConfig.DBName,
	)

	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	logger.Info("Database connected successfully")

	rulesRepo := database.NewRulesRepository(db)
	sourceRepRepo := database.NewSourceReputationRepository(db)
	classificationHistoryRepo := database.NewClassificationHistoryRepository(db)

	logger.Info("Repositories initialized")

	// Setup Elasticsearch storage for re-classification endpoint
	// This is optional - if ES is unavailable, re-classification endpoint won't work
	// but the service can still start and serve other endpoints
	esURL := getEnv("ELASTICSEARCH_URL", "http://localhost:9200")
	ctx := context.Background()

	// Try to create ES client with retry logic, but don't fail startup if it fails
	// Use shorter retry config for optional connection (ES is not required for HTTP server)
	const (
		optionalESMaxAttempts  = 3
		optionalESInitialDelay = 1 * time.Second
		optionalESMaxDelay     = 5 * time.Second
		optionalESMultiplier   = 2.0
	)
	esclientCfg := esclient.Config{
		URL: esURL,
		RetryConfig: &retry.Config{
			MaxAttempts:  optionalESMaxAttempts,
			InitialDelay: optionalESInitialDelay,
			MaxDelay:     optionalESMaxDelay,
			Multiplier:   optionalESMultiplier,
		},
	}

	// Create a simple logger for ES connection using infrastructure logger
	esLog, err := infralogger.New(infralogger.Config{
		Level:  "info",
		Format: "console",
	})
	if err != nil {
		esLog = nil
	}

	esClient, err := esclient.NewClient(ctx, esclientCfg, esLog)
	var esStorage *storage.ElasticsearchStorage
	if err != nil {
		logger.Warn("Failed to connect to Elasticsearch", "error", err)
		logger.Info("Re-classification endpoint will not be available")
		// Continue without ES - this is optional functionality
		esStorage = nil
	} else {
		esStorage = storage.NewElasticsearchStorage(esClient)
		if err = esStorage.TestConnection(ctx); err != nil {
			logger.Warn("Failed to verify Elasticsearch connection", "error", err)
			logger.Info("Re-classification endpoint may not work correctly")
			esStorage = nil
		} else {
			logger.Info("Elasticsearch connected successfully")
		}
	}

	return &serverComponents{
		db:                        db,
		rulesRepo:                 rulesRepo,
		sourceRepRepo:             sourceRepRepo,
		classificationHistoryRepo: classificationHistoryRepo,
		esStorage:                 esStorage,
	}, nil
}

// loadRulesAndCreateClassifier loads rules and creates classifier components
func loadRulesAndCreateClassifier(ctx context.Context, comps *serverComponents, logger *Logger) error {
	enabledOnly := true
	rules, err := comps.rulesRepo.List(ctx, domain.RuleTypeTopic, &enabledOnly)
	if err != nil {
		return fmt.Errorf("failed to load rules from database: %w", err)
	}
	logger.Info("Rules loaded from database", "count", len(rules))

	ruleValues := make([]domain.ClassificationRule, len(rules))
	for i, rule := range rules {
		ruleValues[i] = *rule
	}

	config := classifier.Config{
		Version:         "1.0.0",
		MinQualityScore: defaultMinQualityScore50,
		UpdateSourceRep: true,
		QualityConfig: classifier.QualityConfig{
			WordCountWeight:   defaultQualityWeight025,
			MetadataWeight:    defaultQualityWeight025,
			RichnessWeight:    defaultQualityWeight025,
			ReadabilityWeight: defaultQualityWeight025,
			MinWordCount:      defaultMinWordCount100,
			OptimalWordCount:  defaultOptimalWordCount1000,
		},
		SourceReputationConfig: classifier.SourceReputationConfig{
			DefaultScore:               defaultReputationScore50,
			UpdateOnEachClassification: true,
			SpamThreshold:              defaultSpamThreshold,
			MinArticlesForTrust:        minArticlesForTrust,
			ReputationDecayRate:        defaultReputationDecayRate01,
		},
	}

	comps.classifierInstance = classifier.NewClassifier(logger, ruleValues, comps.sourceRepRepo, config)
	logger.Info("Classifier initialized", "version", config.Version, "rules_count", len(rules))

	concurrency := getEnvInt("CLASSIFIER_CONCURRENCY", defaultConcurrency)
	comps.batchProcessor = processor.NewBatchProcessor(comps.classifierInstance, concurrency, logger)
	logger.Info("Batch processor initialized", "concurrency", concurrency)

	comps.sourceRepScorer = classifier.NewSourceReputationScorer(logger, comps.sourceRepRepo)
	comps.topicClassifier = classifier.NewTopicClassifier(logger, ruleValues)

	comps.handler = api.NewHandler(
		comps.classifierInstance,
		comps.batchProcessor,
		comps.sourceRepScorer,
		comps.topicClassifier,
		comps.rulesRepo,
		comps.sourceRepRepo,
		comps.classificationHistoryRepo,
		comps.esStorage,
		logger,
	)

	return nil
}

// StartHTTPServer starts the HTTP server for the classifier service
func StartHTTPServer() {
	// Start profiling server (if enabled)
	profiling.StartPprofServer()

	debug := os.Getenv("APP_DEBUG") == "true"
	port := getEnvInt("CLASSIFIER_PORT", defaultClassifierPort)

	logger := NewLogger(debug)
	logger.Info("Starting classifier HTTP server", "port", port, "debug", debug)

	comps, err := setupDatabaseAndRepos(logger)
	if err != nil {
		logger.Error("Failed to setup database", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err = loadRulesAndCreateClassifier(ctx, comps, logger); err != nil {
		logger.Error("Failed to load rules and create classifier", "error", err)
		_ = comps.db.Close()
		os.Exit(1)
	}

	serverConfig := api.ServerConfig{
		Port:         port,
		ReadTimeout:  defaultHTTPTimeout,
		WriteTimeout: defaultHTTPTimeout,
		Debug:        debug,
	}
	server := api.NewServer(comps.handler, serverConfig, logger)

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Start()
	}()

	// Wait for interrupt signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case serverErr := <-serverErrors:
		logger.Error("Server error", "error", serverErr)
		_ = comps.db.Close() // Explicit cleanup before exit
		os.Exit(1)
	case sig := <-shutdown:
		logger.Info("Shutdown signal received", "signal", sig)

		// Graceful shutdown with 30 second timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)

		if err = server.Shutdown(shutdownCtx); err != nil {
			cancel() // Explicit cancel before exit
			logger.Error("Graceful shutdown failed", "error", err)
			_ = comps.db.Close() // Explicit cleanup before exit
			os.Exit(1)
		}
		cancel() // Cancel context after successful shutdown

		logger.Info("Server stopped gracefully")
		_ = comps.db.Close() // Cleanup on normal shutdown
	}
}

// StartHTTPServerWithStop starts the HTTP server and returns a stop function
// This allows the server to run concurrently with other services
func StartHTTPServerWithStop() (func(), error) {
	debug := os.Getenv("APP_DEBUG") == "true"
	port := getEnvInt("CLASSIFIER_PORT", defaultClassifierPort)

	logger := NewLogger(debug)
	logger.Info("Starting classifier HTTP server", "port", port, "debug", debug)

	comps, err := setupDatabaseAndRepos(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to setup database: %w", err)
	}

	ctx := context.Background()
	if err = loadRulesAndCreateClassifier(ctx, comps, logger); err != nil {
		_ = comps.db.Close()
		return nil, fmt.Errorf("failed to load rules and create classifier: %w", err)
	}

	serverConfig := api.ServerConfig{
		Port:         port,
		ReadTimeout:  defaultHTTPTimeout,
		WriteTimeout: defaultHTTPTimeout,
		Debug:        debug,
	}
	server := api.NewServer(comps.handler, serverConfig, logger)

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		if err = server.Start(); err != nil {
			serverErrors <- err
		}
	}()

	// Monitor server errors in background
	go func() {
		if serverErr := <-serverErrors; serverErr != nil {
			logger.Error("Server error", "error", serverErr)
		}
	}()

	// Return stop function
	stopFunc := func() {
		logger.Info("Stopping HTTP server...")

		// Graceful shutdown with 30 second timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)

		if err = server.Shutdown(shutdownCtx); err != nil {
			cancel() // Explicit cancel before cleanup
			logger.Error("Graceful shutdown failed", "error", err)
		} else {
			cancel() // Cancel context after successful shutdown
			logger.Info("HTTP server stopped gracefully")
		}

		_ = comps.db.Close()
	}

	return stopFunc, nil
}

// getEnv retrieves a string from environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves an integer from environment variable with a default fallback
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
