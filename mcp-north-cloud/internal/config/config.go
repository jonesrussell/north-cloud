package config

import (
	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Config holds the MCP server configuration.
type Config struct {
	Services ServicesConfig `yaml:"services"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServicesConfig holds URLs for all North Cloud services.
type ServicesConfig struct {
	IndexManagerURL  string `yaml:"index_manager_url" env:"INDEX_MANAGER_URL"`
	CrawlerURL       string `yaml:"crawler_url" env:"CRAWLER_URL"`
	SourceManagerURL string `yaml:"source_manager_url" env:"SOURCE_MANAGER_URL"`
	PublisherURL     string `yaml:"publisher_url" env:"PUBLISHER_URL"`
	SearchURL        string `yaml:"search_url" env:"SEARCH_URL"`
	ClassifierURL    string `yaml:"classifier_url" env:"CLASSIFIER_URL"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"`
}

// Load loads configuration from the specified path.
func Load(path string) (*Config, error) {
	return infraconfig.LoadWithDefaults[Config](path, setDefaults)
}

// setDefaults applies default values to the config.
func setDefaults(cfg *Config) {
	// Service defaults
	if cfg.Services.IndexManagerURL == "" {
		cfg.Services.IndexManagerURL = "http://localhost:8090"
	}
	if cfg.Services.CrawlerURL == "" {
		cfg.Services.CrawlerURL = "http://localhost:8060"
	}
	if cfg.Services.SourceManagerURL == "" {
		cfg.Services.SourceManagerURL = "http://localhost:8050"
	}
	if cfg.Services.PublisherURL == "" {
		cfg.Services.PublisherURL = "http://localhost:8080"
	}
	if cfg.Services.SearchURL == "" {
		cfg.Services.SearchURL = "http://localhost:8090"
	}
	if cfg.Services.ClassifierURL == "" {
		cfg.Services.ClassifierURL = "http://localhost:8070"
	}

	// Logging defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
}

// LoadOrDefault loads config from file, or returns defaults if file doesn't exist.
// This is useful for MCP servers where config file is optional.
func LoadOrDefault(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		// Return config with defaults
		cfg = &Config{}
		setDefaults(cfg)
		// Apply env overrides directly since Load didn't run
		infraconfig.MustLoad[Config](path) // This will use defaults
	}
	return cfg
}

// NewDefault creates a new config with all default values.
func NewDefault() *Config {
	cfg := &Config{}
	setDefaults(cfg)
	return cfg
}
