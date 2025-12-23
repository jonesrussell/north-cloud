package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/api"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
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

// mockSourceReputationDB is a temporary in-memory implementation
// TODO: Replace with actual PostgreSQL implementation
type mockSourceReputationDB struct {
	sources map[string]*domain.SourceReputation
}

func newMockSourceReputationDB() *mockSourceReputationDB {
	return &mockSourceReputationDB{
		sources: make(map[string]*domain.SourceReputation),
	}
}

func (m *mockSourceReputationDB) GetSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error) {
	source, ok := m.sources[sourceName]
	if !ok {
		return nil, fmt.Errorf("source not found")
	}
	return source, nil
}

func (m *mockSourceReputationDB) CreateSource(ctx context.Context, source *domain.SourceReputation) error {
	m.sources[source.SourceName] = source
	return nil
}

func (m *mockSourceReputationDB) UpdateSource(ctx context.Context, source *domain.SourceReputation) error {
	m.sources[source.SourceName] = source
	return nil
}

func (m *mockSourceReputationDB) GetOrCreateSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error) {
	source, ok := m.sources[sourceName]
	if !ok {
		source = &domain.SourceReputation{
			SourceName:          sourceName,
			Category:            domain.SourceCategoryUnknown,
			ReputationScore:     50,
			TotalArticles:       0,
			AverageQualityScore: 0,
			SpamCount:           0,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}
		m.sources[sourceName] = source
	}
	return source, nil
}

func main() {
	// Get configuration from environment
	debug := os.Getenv("APP_DEBUG") == "true"
	port := getEnvInt("CLASSIFIER_PORT", 8070)

	logger := NewLogger(debug)
	logger.Info("Starting classifier HTTP server", "port", port, "debug", debug)

	// Create classification rules
	// TODO: Load from database
	rules := []domain.ClassificationRule{
		{
			ID:            1,
			RuleName:      "crime_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "crime",
			Keywords:      []string{"police", "arrest", "charged", "suspect", "incident", "crime", "criminal", "investigation", "officer"},
			MinConfidence: 0.3,
			Enabled:       true,
			Priority:      1,
		},
		{
			ID:            2,
			RuleName:      "sports_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "sports",
			Keywords:      []string{"team", "championship", "game", "match", "sports", "player", "won", "score", "tournament"},
			MinConfidence: 0.3,
			Enabled:       true,
			Priority:      1,
		},
		{
			ID:            3,
			RuleName:      "politics_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "politics",
			Keywords:      []string{"government", "election", "politician", "parliament", "minister", "vote", "policy", "legislation"},
			MinConfidence: 0.3,
			Enabled:       true,
			Priority:      1,
		},
		{
			ID:            4,
			RuleName:      "local_news_detection",
			RuleType:      domain.RuleTypeTopic,
			TopicName:     "local_news",
			Keywords:      []string{"community", "local", "neighbourhood", "mayor", "council", "resident", "downtown"},
			MinConfidence: 0.3,
			Enabled:       true,
			Priority:      1,
		},
	}

	// Create source reputation DB
	// TODO: Replace with actual PostgreSQL implementation
	sourceRepDB := newMockSourceReputationDB()

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
	classifierInstance := classifier.NewClassifier(logger, rules, sourceRepDB, config)
	logger.Info("Classifier initialized", "version", config.Version, "rules_count", len(rules))

	// Create batch processor
	concurrency := getEnvInt("CLASSIFIER_CONCURRENCY", 10)
	batchProcessor := processor.NewBatchProcessor(classifierInstance, concurrency, logger)
	logger.Info("Batch processor initialized", "concurrency", concurrency)

	// Get individual classifiers for API handlers
	sourceRepScorer := classifier.NewSourceReputationScorer(logger, sourceRepDB)
	topicClassifier := classifier.NewTopicClassifier(logger, rules)

	// Create API handler
	handler := api.NewHandler(classifierInstance, batchProcessor, sourceRepScorer, topicClassifier, logger)

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

// getEnvInt retrieves an integer from environment variable with a default fallback
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
