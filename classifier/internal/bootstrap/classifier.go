package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/api"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/coforgemlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/miningmlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultClassifierPort        = 8070
	defaultHTTPTimeout           = 30 * time.Second
	defaultConcurrency           = 10
	defaultMinQualityScore50     = 50
	defaultQualityWeight025      = 0.25
	defaultMinWordCount100       = 100
	defaultOptimalWordCount1000  = 1000
	defaultReputationScore50     = 50
	defaultSpamThreshold         = 30
	minArticlesForTrust          = 10
	defaultReputationDecayRate01 = 0.1
)

// HTTPComponents holds all components needed for the HTTP server.
type HTTPComponents struct {
	DB       *sqlx.DB
	Handler  *api.Handler
	Server   *infragin.Server
	InfraLog infralogger.Logger
}

// NewHTTPComponents creates all components for the HTTP server.
func NewHTTPComponents(cfg *config.Config, logger infralogger.Logger) (*HTTPComponents, error) {
	// Setup database
	dbComps, err := SetupDatabase(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("setup database: %w", err)
	}

	// Setup optional Elasticsearch
	esStorage := SetupElasticsearch(cfg, logger)

	// Load rules and create classifier
	enabledOnly := true
	rules, err := dbComps.RulesRepo.List(context.Background(), domain.RuleTypeTopic, &enabledOnly)
	if err != nil {
		_ = dbComps.DB.Close()
		return nil, fmt.Errorf("load rules: %w", err)
	}
	logger.Info("Rules loaded from database", infralogger.Int("count", len(rules)))

	ruleValues := make([]domain.ClassificationRule, len(rules))
	for i, rule := range rules {
		ruleValues[i] = *rule
	}

	// Create classifier config with optional Crime
	classifierConfig := createClassifierConfig(cfg, logger)

	classifierInstance := classifier.NewClassifier(logger, ruleValues, dbComps.SourceRepRepo, classifierConfig)
	logger.Info("Classifier initialized",
		infralogger.String("version", classifierConfig.Version),
		infralogger.Int("rules_count", len(rules)),
	)

	concurrency := cfg.Service.Concurrency
	if concurrency == 0 {
		concurrency = defaultConcurrency
	}
	batchProcessor := processor.NewBatchProcessor(classifierInstance, concurrency, logger)
	logger.Info("Batch processor initialized", infralogger.Int("concurrency", concurrency))

	sourceRepScorer := classifier.NewSourceReputationScorer(logger, dbComps.SourceRepRepo)
	topicClassifier := classifier.NewTopicClassifier(logger, ruleValues)

	handler := api.NewHandler(
		classifierInstance,
		batchProcessor,
		sourceRepScorer,
		topicClassifier,
		dbComps.RulesRepo,
		dbComps.SourceRepRepo,
		dbComps.ClassificationHistoryRepo,
		esStorage,
		cfg,
		logger,
	)

	port := cfg.Service.Port
	if port == 0 {
		port = defaultClassifierPort
	}

	infraLog, logErr := infralogger.New(infralogger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if logErr != nil {
		_ = dbComps.DB.Close()
		return nil, fmt.Errorf("create infrastructure logger: %w", logErr)
	}

	serverConfig := api.ServerConfig{
		Port:         port,
		ReadTimeout:  defaultHTTPTimeout,
		WriteTimeout: defaultHTTPTimeout,
		Debug:        cfg.Service.Debug,
	}
	server := api.NewServer(handler, serverConfig, cfg, infraLog)

	return &HTTPComponents{
		DB:       dbComps.DB,
		Handler:  handler,
		Server:   server,
		InfraLog: infraLog,
	}, nil
}

// HTTPShutdownTimeout returns the timeout for HTTP server graceful shutdown.
func HTTPShutdownTimeout() time.Duration {
	return defaultHTTPTimeout
}

// createClassifierConfig creates the classifier configuration with all sub-components.
func createClassifierConfig(cfg *config.Config, logger infralogger.Logger) classifier.Config {
	return classifier.Config{
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
		CrimeClassifier:   createCrimeClassifier(cfg, logger),
		MiningClassifier:  createMiningClassifier(cfg, logger),
		CoforgeClassifier: createCoforgeClassifier(cfg, logger),
	}
}

// createCrimeClassifier creates a Crime classifier if enabled in config.
func createCrimeClassifier(cfg *config.Config, logger infralogger.Logger) *classifier.CrimeClassifier {
	if !cfg.Classification.Crime.Enabled {
		return nil
	}

	var mlClient classifier.MLClassifier
	if cfg.Classification.Crime.MLServiceURL != "" {
		mlClient = mlclient.NewClient(cfg.Classification.Crime.MLServiceURL)
	}

	logger.Info("Crime classifier enabled",
		infralogger.String("ml_service_url", cfg.Classification.Crime.MLServiceURL))

	return classifier.NewCrimeClassifier(mlClient, logger, true)
}

// createMiningClassifier creates a Mining classifier if enabled in config.
func createMiningClassifier(cfg *config.Config, logger infralogger.Logger) *classifier.MiningClassifier {
	if !cfg.Classification.Mining.Enabled {
		return nil
	}

	var mlClient classifier.MiningMLClassifier
	if cfg.Classification.Mining.MLServiceURL != "" {
		mlClient = miningmlclient.NewClient(cfg.Classification.Mining.MLServiceURL)
	}

	logger.Info("Mining classifier enabled",
		infralogger.String("ml_service_url", cfg.Classification.Mining.MLServiceURL))

	return classifier.NewMiningClassifier(mlClient, logger, true)
}

// createCoforgeClassifier creates a Coforge classifier if enabled in config.
func createCoforgeClassifier(cfg *config.Config, logger infralogger.Logger) *classifier.CoforgeClassifier {
	if !cfg.Classification.Coforge.Enabled {
		return nil
	}

	var mlClient classifier.CoforgeMLClassifier
	if cfg.Classification.Coforge.MLServiceURL != "" {
		mlClient = coforgemlclient.NewClient(cfg.Classification.Coforge.MLServiceURL)
	}

	logger.Info("Coforge classifier enabled",
		infralogger.String("ml_service_url", cfg.Classification.Coforge.MLServiceURL))

	return classifier.NewCoforgeClassifier(mlClient, logger, true)
}
