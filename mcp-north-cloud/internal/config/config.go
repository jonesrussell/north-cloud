package config

import (
	"fmt"
	"os"

	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
)

// defaultURLPublisherClassifier is the default base URL for publisher and classifier (same port in dev).
const defaultURLPublisherClassifier = "http://localhost:8070"

// Config holds the MCP server configuration.
type Config struct {
	Env      string         `env:"MCP_ENV"   yaml:"env"`
	Services ServicesConfig `yaml:"services"`
	Client   ClientConfig   `yaml:"client"`
	Logging  LoggingConfig  `yaml:"logging"`
	Auth     AuthConfig     `yaml:"auth"`
}

// ClientConfig holds client-level settings (e.g. HTTP timeouts).
type ClientConfig struct {
	HTTPTimeoutSeconds int `env:"MCP_HTTP_TIMEOUT_SECONDS" yaml:"http_timeout_seconds"`
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
	GrafanaURL       string `env:"GRAFANA_URL"        yaml:"grafana_url"`
	GrafanaUsername  string `env:"GRAFANA_USERNAME"   yaml:"grafana_username"`
	GrafanaPassword  string `env:"GRAFANA_PASSWORD"   yaml:"grafana_password"`
	AuthURL          string `env:"AUTH_URL"           yaml:"auth_url"`
	PipelineURL      string `env:"PIPELINE_URL"       yaml:"pipeline_url"`
	ClickTrackerURL  string `env:"CLICK_TRACKER_URL"  yaml:"click_tracker_url"`
	RFPIngestorURL   string `env:"RFP_INGESTOR_URL"   yaml:"rfp_ingestor_url"`
	// OllamaURL is the base URL for the Ollama API (used by fetch_url extract_schema).
	// If empty, extract_schema returns an error.
	OllamaURL string `env:"OLLAMA_URL" yaml:"ollama_url"`
	// OllamaModel is the model to use for schema-guided extraction (default: qwen3:4b).
	OllamaModel string `env:"OLLAMA_MODEL" yaml:"ollama_model"`
	// RendererURL is the URL of the Playwright renderer sidecar for JS-heavy pages.
	// If empty, js_render=true in fetch_url returns an error.
	RendererURL string `env:"RENDERER_URL" yaml:"renderer_url"`
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
	// Environment default
	if cfg.Env == "" {
		cfg.Env = "local"
	}

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
		cfg.Services.PublisherURL = defaultURLPublisherClassifier
	}
	if cfg.Services.SearchURL == "" {
		cfg.Services.SearchURL = "http://localhost:8090"
	}
	if cfg.Services.ClassifierURL == "" {
		cfg.Services.ClassifierURL = defaultURLPublisherClassifier
	}
	if cfg.Services.OllamaModel == "" {
		cfg.Services.OllamaModel = "qwen3:4b"
	}
	if cfg.Services.GrafanaURL == "" {
		cfg.Services.GrafanaURL = "http://localhost:3000"
	}
	if cfg.Services.AuthURL == "" {
		cfg.Services.AuthURL = "http://localhost:8040"
	}
	if cfg.Services.PipelineURL == "" {
		cfg.Services.PipelineURL = "http://localhost:8075"
	}
	if cfg.Services.ClickTrackerURL == "" {
		cfg.Services.ClickTrackerURL = "http://localhost:8093"
	}
	if cfg.Services.RFPIngestorURL == "" {
		cfg.Services.RFPIngestorURL = "http://localhost:8095"
	}

	// Client defaults
	if cfg.Client.HTTPTimeoutSeconds == 0 {
		cfg.Client.HTTPTimeoutSeconds = 30
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
		cfg = &Config{}
		setDefaults(cfg)
		_ = infraconfig.ApplyEnvOverrides(cfg)
	}
	return cfg
}

// NewDefault creates a new config with all default values and environment overrides.
// This is used when no config file exists.
func NewDefault() *Config {
	cfg := &Config{}
	setDefaults(cfg)
	// Apply environment variable overrides (loads .env files internally)
	if err := infraconfig.ApplyEnvOverrides(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to apply env overrides: %v\n", err)
	}
	return cfg
}
