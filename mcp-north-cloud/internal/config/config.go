package config

import (
	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Config holds the MCP server configuration.
type Config struct {
	Services ServicesConfig `yaml:"services"`
	Logging  LoggingConfig  `yaml:"logging"`
	Auth     AuthConfig     `yaml:"auth"`
}

// AuthConfig holds authentication configuration for service-to-service calls.
type AuthConfig struct {
	JWTSecret string `env:"AUTH_JWT_SECRET" yaml:"jwt_secret"`
}

// ServicesConfig holds URLs for all North Cloud services.
type ServicesConfig struct {
	IndexManagerURL  string `env:"INDEX_MANAGER_URL"  yaml:"index_manager_url"`
	CrawlerURL       string `env:"CRAWLER_URL"        yaml:"crawler_url"`
	SourceManagerURL string `env:"SOURCE_MANAGER_URL" yaml:"source_manager_url"`
	PublisherURL     string `env:"PUBLISHER_URL"      yaml:"publisher_url"`
	SearchURL        string `env:"SEARCH_URL"         yaml:"search_url"`
	ClassifierURL    string `env:"CLASSIFIER_URL"     yaml:"classifier_url"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
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

// NewDefault creates a new config with all default values and environment overrides.
// This is used when no config file exists.
func NewDefault() *Config {
	cfg := &Config{}
	setDefaults(cfg)
	// Apply environment variable overrides (loads .env files internally)
	_ = infraconfig.ApplyEnvOverrides(cfg)
	return cfg
}
