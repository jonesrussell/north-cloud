package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/anishinaabemlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/api"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/coforgemlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/entertainmentmlclient"
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
	crimeCC := createOptionalClassifier(
		cfg.Classification.Crime.Enabled, cfg.Classification.Crime.MLServiceURL, logger,
		"Crime classifier", mlclient.NewClient,
		func(c *mlclient.Client, l infralogger.Logger, e bool) *classifier.CrimeClassifier {
			return classifier.NewCrimeClassifier(c, l, e)
		})
	miningCC := createOptionalClassifier(
		cfg.Classification.Mining.Enabled, cfg.Classification.Mining.MLServiceURL, logger,
		"Mining classifier", miningmlclient.NewClient,
		func(c *miningmlclient.Client, l infralogger.Logger, e bool) *classifier.MiningClassifier {
			return classifier.NewMiningClassifier(c, l, e)
		})
	coforgeCC := createOptionalClassifier(
		cfg.Classification.Coforge.Enabled, cfg.Classification.Coforge.MLServiceURL, logger,
		"Coforge classifier", coforgemlclient.NewClient,
		func(c *coforgemlclient.Client, l infralogger.Logger, e bool) *classifier.CoforgeClassifier {
			return classifier.NewCoforgeClassifier(c, l, e)
		})
	entertainmentCC := createOptionalClassifier(
		cfg.Classification.Entertainment.Enabled, cfg.Classification.Entertainment.MLServiceURL, logger,
		"Entertainment classifier", entertainmentmlclient.NewClient,
		func(c *entertainmentmlclient.Client, l infralogger.Logger, e bool) *classifier.EntertainmentClassifier {
			return classifier.NewEntertainmentClassifier(c, l, e)
		})
	anishinaabeCC := createOptionalClassifier(
		cfg.Classification.Anishinaabe.Enabled, cfg.Classification.Anishinaabe.MLServiceURL, logger,
		"Anishinaabe classifier", anishinaabemlclient.NewClient,
		func(c *anishinaabemlclient.Client, l infralogger.Logger, e bool) *classifier.AnishinaabeClassifier {
			return classifier.NewAnishinaabeClassifier(c, l, e)
		})

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
		CrimeClassifier:         crimeCC,
		MiningClassifier:        miningCC,
		CoforgeClassifier:       coforgeCC,
		EntertainmentClassifier: entertainmentCC,
		AnishinaabeClassifier:   anishinaabeCC,
	}
}

// createOptionalClassifier creates an optional ML classifier when enabled; returns nil otherwise.
// newClient is only called when mlURL is non-empty. Label is used for logging.
func createOptionalClassifier[C any, T any](
	enabled bool, mlURL string, logger infralogger.Logger, label string,
	newClient func(string) C, newClassifier func(C, infralogger.Logger, bool) T,
) T {
	if !enabled {
		var zero T
		return zero
	}
	var client C
	if mlURL != "" {
		client = newClient(mlURL)
	}
	logger.Info(label+" enabled", infralogger.String("ml_service_url", mlURL))
	return newClassifier(client, logger, true)
}
