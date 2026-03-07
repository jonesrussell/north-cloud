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
	ClassifierEnabled   bool
	ClassifierMaxEvents int
	ClassifierModel     string
}

// AnthropicConfig holds Anthropic API config.
type AnthropicConfig struct {
	APIKey       string
	DefaultModel string
}

const (
	defaultIntervalSeconds      = 1800
	defaultMaxTokensPerInterval = 25000
	defaultClassifierMaxEvents  = 200
	defaultClassifierModel      = "claude-haiku-4-5-20251001"
	serviceName                 = "ai-observer"
	serviceVersion              = "0.1.0"
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
				ClassifierEnabled:   os.Getenv("AI_OBSERVER_CATEGORY_CLASSIFIER_ENABLED") != "false",
				ClassifierMaxEvents: defaultClassifierMaxEvents,
				ClassifierModel:     defaultClassifierModel,
			},
		},
		Anthropic: AnthropicConfig{
			APIKey:       apiKey,
			DefaultModel: defaultClassifierModel,
		},
	}, nil
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
