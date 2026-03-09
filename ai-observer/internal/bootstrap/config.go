package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the ai-observer service.
type Config struct {
	Service   ServiceConfig
	ES        ESConfig
	Observer  ObserverConfig
	Anthropic AnthropicConfig
}

// ServiceConfig holds service identity fields.
type ServiceConfig struct {
	Name    string
	Version string
}

// ESConfig holds Elasticsearch connection config.
type ESConfig struct {
	URL      string
	Username string
	Password string
}

// ObserverConfig holds polling and budget config.
type ObserverConfig struct {
	Enabled              bool
	DryRun               bool
	IntervalSeconds      int
	MaxTokensPerInterval int
	Categories           CategoriesConfig
}

// CategoriesConfig holds per-category feature flags.
type CategoriesConfig struct {
	ClassifierEnabled       bool
	ClassifierMaxEvents     int
	ClassifierModel         string
	DriftEnabled            bool
	DriftIntervalSeconds    int
	DriftKLThreshold        float64
	DriftPSIThreshold       float64
	DriftMatrixThreshold    float64
	DriftBaselineWindowDays int
	DriftBaselineRetention  int
}

// AnthropicConfig holds Anthropic API config.
type AnthropicConfig struct {
	APIKey       string
	DefaultModel string
}

const (
	defaultIntervalSeconds         = 1800
	defaultMaxTokensPerInterval    = 25000
	defaultClassifierMaxEvents     = 200
	defaultClassifierModel         = "claude-haiku-4-5-20251001"
	defaultDriftIntervalSeconds    = 21600
	defaultDriftKLThreshold        = 0.15
	defaultDriftPSIThreshold       = 0.25
	defaultDriftMatrixThreshold    = 0.20
	defaultDriftBaselineWindowDays = 7
	defaultDriftBaselineRetention  = 30
	float64BitSize                 = 64
	serviceName                    = "ai-observer"
	serviceVersion                 = "0.1.0"
)

// LoadConfig loads configuration from environment variables.
// ANTHROPIC_API_KEY is only required when AI_OBSERVER_ENABLED is not "false".
func LoadConfig() (Config, error) {
	enabled := os.Getenv("AI_OBSERVER_ENABLED") != "false"

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if enabled && apiKey == "" {
		return Config{}, errors.New("ANTHROPIC_API_KEY is required when AI_OBSERVER_ENABLED is true")
	}

	esURL := os.Getenv("ES_URL")
	if esURL == "" {
		esURL = "http://localhost:9200"
	}

	intervalSeconds, err := envInt("AI_OBSERVER_INTERVAL_SECONDS", defaultIntervalSeconds)
	if err != nil {
		return Config{}, err
	}

	maxTokens, err := envInt("AI_OBSERVER_MAX_TOKENS_PER_INTERVAL", defaultMaxTokensPerInterval)
	if err != nil {
		return Config{}, err
	}

	driftCfg, err := loadDriftConfig()
	if err != nil {
		return Config{}, err
	}

	return Config{
		Service: ServiceConfig{
			Name:    serviceName,
			Version: serviceVersion,
		},
		ES: ESConfig{
			URL:      esURL,
			Username: os.Getenv("ES_USERNAME"),
			Password: os.Getenv("ES_PASSWORD"),
		},
		Observer: ObserverConfig{
			Enabled:              enabled,
			DryRun:               os.Getenv("AI_OBSERVER_DRY_RUN") == "true",
			IntervalSeconds:      intervalSeconds,
			MaxTokensPerInterval: maxTokens,
			Categories: CategoriesConfig{
				ClassifierEnabled:       os.Getenv("AI_OBSERVER_CATEGORY_CLASSIFIER_ENABLED") != "false",
				ClassifierMaxEvents:     defaultClassifierMaxEvents,
				ClassifierModel:         defaultClassifierModel,
				DriftEnabled:            driftCfg.DriftEnabled,
				DriftIntervalSeconds:    driftCfg.DriftIntervalSeconds,
				DriftKLThreshold:        driftCfg.DriftKLThreshold,
				DriftPSIThreshold:       driftCfg.DriftPSIThreshold,
				DriftMatrixThreshold:    driftCfg.DriftMatrixThreshold,
				DriftBaselineWindowDays: driftCfg.DriftBaselineWindowDays,
				DriftBaselineRetention:  driftCfg.DriftBaselineRetention,
			},
		},
		Anthropic: AnthropicConfig{
			APIKey:       apiKey,
			DefaultModel: defaultClassifierModel,
		},
	}, nil
}

func loadDriftConfig() (CategoriesConfig, error) {
	driftInterval, err := envInt("AI_OBSERVER_DRIFT_INTERVAL_SECONDS", defaultDriftIntervalSeconds)
	if err != nil {
		return CategoriesConfig{}, err
	}

	klThreshold, err := envFloat("AI_OBSERVER_DRIFT_KL_THRESHOLD", defaultDriftKLThreshold)
	if err != nil {
		return CategoriesConfig{}, err
	}

	psiThreshold, err := envFloat("AI_OBSERVER_DRIFT_PSI_THRESHOLD", defaultDriftPSIThreshold)
	if err != nil {
		return CategoriesConfig{}, err
	}

	matrixThreshold, err := envFloat("AI_OBSERVER_DRIFT_MATRIX_THRESHOLD", defaultDriftMatrixThreshold)
	if err != nil {
		return CategoriesConfig{}, err
	}

	baselineDays, err := envInt("AI_OBSERVER_DRIFT_BASELINE_WINDOW_DAYS", defaultDriftBaselineWindowDays)
	if err != nil {
		return CategoriesConfig{}, err
	}

	baselineRetention, err := envInt("AI_OBSERVER_DRIFT_BASELINE_RETENTION", defaultDriftBaselineRetention)
	if err != nil {
		return CategoriesConfig{}, err
	}

	return CategoriesConfig{
		DriftEnabled:            os.Getenv("AI_OBSERVER_DRIFT_ENABLED") == "true",
		DriftIntervalSeconds:    driftInterval,
		DriftKLThreshold:        klThreshold,
		DriftPSIThreshold:       psiThreshold,
		DriftMatrixThreshold:    matrixThreshold,
		DriftBaselineWindowDays: baselineDays,
		DriftBaselineRetention:  baselineRetention,
	}, nil
}

func envFloat(key string, def float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseFloat(v, float64BitSize)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return n, nil
}

func envInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return n, nil
}
