package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/api"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
)

// Logger is a simple logger implementation
type Logger struct {
	debug bool
}

func NewLogger(debug bool) *Logger {
	return &Logger{debug: debug}
}

func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	if l.debug {
		fmt.Printf("[DEBUG] %s %v\n", msg, keysAndValues)
	}
}

func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[INFO] %s %v\n", msg, keysAndValues)
}

func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[WARN] %s %v\n", msg, keysAndValues)
}

func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	fmt.Printf("[ERROR] %s %v\n", msg, keysAndValues)
}

// StartHTTPServer starts the HTTP server for the classifier service
func StartHTTPServer() {
	// Get configuration from environment
	debug := os.Getenv("APP_DEBUG") == "true"
	port := getEnvInt("CLASSIFIER_PORT", 8070)

	logger := NewLogger(debug)
	logger.Info("Starting classifier HTTP server", "port", port, "debug", debug)

	// Database configuration
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

	// Connect to database
	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Info("Database connected successfully")

	// Create repositories
	rulesRepo := database.NewRulesRepository(db)
	sourceRepRepo := database.NewSourceReputationRepository(db)
	classificationHistoryRepo := database.NewClassificationHistoryRepository(db)

	logger.Info("Repositories initialized")

	// Load classification rules from database
	ctx := context.Background()
	enabledOnly := true
	rules, err := rulesRepo.List(ctx, domain.RuleTypeTopic, &enabledOnly)
	if err != nil {
		logger.Error("Failed to load rules from database", "error", err)
		os.Exit(1)
	}
	logger.Info("Rules loaded from database", "count", len(rules))

	// Convert []*ClassificationRule to []ClassificationRule for classifier
	ruleValues := make([]domain.ClassificationRule, len(rules))
	for i, rule := range rules {
		ruleValues[i] = *rule
	}

	// Create classifier config
	config := classifier.Config{
		Version:         "1.0.0",
		MinQualityScore: 50,
		UpdateSourceRep: true,
		QualityConfig: classifier.QualityConfig{
			WordCountWeight:   0.25,
			MetadataWeight:    0.25,
			RichnessWeight:    0.25,
			ReadabilityWeight: 0.25,
			MinWordCount:      100,
			OptimalWordCount:  1000,
		},
		SourceReputationConfig: classifier.SourceReputationConfig{
			DefaultScore:               50,
			UpdateOnEachClassification: true,
			SpamThreshold:              30,
			MinArticlesForTrust:        10,
			ReputationDecayRate:        0.1,
		},
	}

	// Create classifier
	classifierInstance := classifier.NewClassifier(logger, ruleValues, sourceRepRepo, config)
	logger.Info("Classifier initialized", "version", config.Version, "rules_count", len(rules))

	// Create batch processor
	concurrency := getEnvInt("CLASSIFIER_CONCURRENCY", 10)
	batchProcessor := processor.NewBatchProcessor(classifierInstance, concurrency, logger)
	logger.Info("Batch processor initialized", "concurrency", concurrency)

	// Get individual classifiers for API handlers
	sourceRepScorer := classifier.NewSourceReputationScorer(logger, sourceRepRepo)
	topicClassifier := classifier.NewTopicClassifier(logger, ruleValues)

	// Create API handler with repositories
	handler := api.NewHandler(
		classifierInstance,
		batchProcessor,
		sourceRepScorer,
		topicClassifier,
		rulesRepo,
		sourceRepRepo,
		classificationHistoryRepo,
		logger,
	)

	// Create HTTP server
	serverConfig := api.ServerConfig{
		Port:         port,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		Debug:        debug,
	}
	server := api.NewServer(handler, serverConfig, logger)

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Start()
	}()

	// Wait for interrupt signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Error("Server error", "error", err)
		os.Exit(1)
	case sig := <-shutdown:
		logger.Info("Shutdown signal received", "signal", sig)

		// Graceful shutdown with 30 second timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Graceful shutdown failed", "error", err)
			os.Exit(1)
		}

		logger.Info("Server stopped gracefully")
	}
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
