package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/api"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/drillmlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
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
	mlClientTimeout              = 5 * time.Second
	mlClientRetryCount           = 2
	mlClientRetryDelay           = 100 * time.Millisecond
	mlClientBreakerTrips         = 5
	mlClientBreakerCooldown      = 30 * time.Second
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
	if cfg.Classification.SidecarRegistryFromYAML {
		logger.Warn("classification.sidecar_registry is set in config but is not yet consumed; " +
			"use the named fields (crime.enabled, mining.enabled, etc.) to control sidecar behaviour")
	}

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
	topicClassifier := classifier.NewTopicClassifier(logger, ruleValues, cfg.Classification.Topic.MaxTopics)

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

// newMLClient creates a unified ML client with standard options for the given module.
func newMLClient(moduleName, mlURL string) *mlclient.Client {
	return mlclient.NewClient(moduleName, mlURL,
		mlclient.WithTimeout(mlClientTimeout),
		mlclient.WithRetry(mlClientRetryCount, mlClientRetryDelay),
		mlclient.WithCircuitBreaker(mlClientBreakerTrips, mlClientBreakerCooldown),
	)
}

// createClassifierConfig creates the classifier configuration with all sub-components.
func createClassifierConfig(cfg *config.Config, logger infralogger.Logger) classifier.Config {
	crimeCC := createOptionalClassifier(
		cfg.Classification.Crime.Enabled, cfg.Classification.Crime.MLServiceURL, logger,
		"Crime classifier", "crime",
		func(c *mlclient.Client, l infralogger.Logger, e bool) *classifier.CrimeClassifier {
			return classifier.NewCrimeClassifier(c, l, e)
		})
	miningCC := createOptionalClassifier(
		cfg.Classification.Mining.Enabled, cfg.Classification.Mining.MLServiceURL, logger,
		"Mining classifier", "mining",
		func(c *mlclient.Client, l infralogger.Logger, e bool) *classifier.MiningClassifier {
			return classifier.NewMiningClassifier(c, l, e)
		})
	coforgeCC := createOptionalClassifier(
		cfg.Classification.Coforge.Enabled, cfg.Classification.Coforge.MLServiceURL, logger,
		"Coforge classifier", "coforge",
		func(c *mlclient.Client, l infralogger.Logger, e bool) *classifier.CoforgeClassifier {
			return classifier.NewCoforgeClassifier(c, l, e)
		})
	entertainmentCC := createOptionalClassifier(
		cfg.Classification.Entertainment.Enabled, cfg.Classification.Entertainment.MLServiceURL, logger,
		"Entertainment classifier", "entertainment",
		func(c *mlclient.Client, l infralogger.Logger, e bool) *classifier.EntertainmentClassifier {
			return classifier.NewEntertainmentClassifier(c, l, e)
		})
	indigenousCC := createOptionalClassifier(
		cfg.Classification.Indigenous.Enabled, cfg.Classification.Indigenous.MLServiceURL, logger,
		"Indigenous classifier", "indigenous",
		func(c *mlclient.Client, l infralogger.Logger, e bool) *classifier.IndigenousClassifier {
			return classifier.NewIndigenousClassifier(c, l, e)
		})

	// Wire drill extraction into mining classifier when both mining and drill are enabled
	if miningCC != nil && cfg.Classification.DrillExtraction.Enabled {
		drillCfg := cfg.Classification.DrillExtraction
		var drillClient classifier.DrillExtractor
		if drillCfg.LLMFallback && drillCfg.AnthropicKey != "" {
			drillClient = drillmlclient.New(
				drillCfg.AnthropicBaseURL,
				drillCfg.AnthropicKey,
				drillCfg.AnthropicModel,
				drillCfg.MaxBodyChars,
			)
			logger.Info("Drill extraction enabled with LLM fallback",
				infralogger.String("model", drillCfg.AnthropicModel))
		} else {
			logger.Info("Drill extraction enabled (regex-only, no LLM fallback)")
		}
		miningCC.WithDrillExtraction(drillClient, drillCfg)
	}

	recipeExtractor, jobExtractor, rfpExtractor, needSignalExtractor, sectorAlignment := createExtractors(cfg, logger)

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
		IndigenousClassifier:    indigenousCC,
		RecipeExtractor:         recipeExtractor,
		JobExtractor:            jobExtractor,
		RFPExtractor:            rfpExtractor,
		NeedSignalExtractor:     needSignalExtractor,
		SectorAlignment:         sectorAlignment,
		RoutingTable:            cfg.Classification.Routing,
		MaxTopics:               cfg.Classification.Topic.MaxTopics,
	}
}

// createExtractors creates the optional structured extractors (recipe, job, RFP, need signal).
func createExtractors(
	cfg *config.Config, logger infralogger.Logger,
) (
	*classifier.RecipeExtractor,
	*classifier.JobExtractor,
	*classifier.RFPExtractor,
	*classifier.NeedSignalExtractor,
	*classifier.SectorAlignmentExtractor,
) {
	var recipeExtractor *classifier.RecipeExtractor
	if cfg.Classification.Recipe.Enabled {
		recipeExtractor = classifier.NewRecipeExtractor(logger)
		logger.Info("Recipe extractor enabled")
	}

	var jobExtractor *classifier.JobExtractor
	if cfg.Classification.Job.Enabled {
		jobExtractor = classifier.NewJobExtractor(logger)
		logger.Info("Job extractor enabled")
	}

	var rfpExtractor *classifier.RFPExtractor
	if cfg.Classification.RFP.Enabled {
		rfpExtractor = classifier.NewRFPExtractor(logger)
		logger.Info("RFP extractor enabled")
	}

	var needSignalExtractor *classifier.NeedSignalExtractor
	if cfg.Classification.NeedSignal.Enabled {
		needSignalExtractor = classifier.NewNeedSignalExtractor(logger)
		logger.Info("Need signal extractor enabled")
	}

	var sectorAlignment *classifier.SectorAlignmentExtractor
	if cfg.Classification.SectorAlignment.Enabled {
		provider := classifier.NewHTTPICPSeedProvider(
			cfg.Classification.SectorAlignment.SourceManagerURL,
			cfg.Classification.SectorAlignment.RefreshInterval,
			nil,
		)
		sectorAlignment = classifier.NewSectorAlignmentExtractor(provider)
		logger.Info("Sector alignment extractor enabled",
			infralogger.String("source_manager_url", cfg.Classification.SectorAlignment.SourceManagerURL))
	}

	return recipeExtractor, jobExtractor, rfpExtractor, needSignalExtractor, sectorAlignment
}

// createOptionalClassifier creates an optional ML classifier when enabled; returns nil otherwise.
// moduleName is used for the unified ML client's module identifier. Label is used for logging.
func createOptionalClassifier[T any](
	enabled bool, mlURL string, logger infralogger.Logger, label string, moduleName string,
	newClassifier func(*mlclient.Client, infralogger.Logger, bool) T,
) T {
	if !enabled {
		var zero T
		return zero
	}
	var client *mlclient.Client
	if mlURL != "" {
		client = newMLClient(moduleName, mlURL)
	}
	if mlURL == "" {
		logger.Warn(label+" enabled but ML service URL is empty; running in rules-only mode",
			infralogger.String("ml_service_url", ""),
		)
	} else {
		logger.Info(label+" enabled", infralogger.String("ml_service_url", mlURL))
	}
	return newClassifier(client, logger, true)
}
