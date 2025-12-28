package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	infracontext "github.com/north-cloud/infrastructure/context"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultReadTimeoutSeconds is the default read timeout in seconds
	DefaultReadTimeoutSeconds = 10
	// DefaultWriteTimeoutSeconds is the default write timeout in seconds
	DefaultWriteTimeoutSeconds = 30
	// DefaultShutdownTimeoutSeconds is the default shutdown timeout in seconds
	DefaultShutdownTimeoutSeconds = 30
)

type Config struct {
	Debug         bool                `yaml:"debug"` // Application debug mode (controls log level and format)
	Server        ServerConfig        `yaml:"server"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Redis         RedisConfig         `yaml:"redis"`
	Service       ServiceConfig       `yaml:"service"`
	Cities        []CityConfig        `yaml:"cities"`
	Sources       SourcesConfig       `yaml:"sources"` // Optional: Sources service configuration
}

type ElasticsearchConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type RedisConfig struct {
	URL      string `yaml:"url"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type ServiceConfig struct {
	CheckInterval        time.Duration `yaml:"check_interval"`
	UseClassifiedContent bool          `yaml:"use_classified_content"` // Use classified_content indexes instead of articles
	MinQualityScore      int           `yaml:"min_quality_score"`      // Minimum quality score for classified content (0-100)
	IndexSuffix          string        `yaml:"index_suffix"`           // Index suffix (_articles or _classified_content)
}

type CityConfig struct {
	Name  string `yaml:"name"`
	Index string `yaml:"index"`
}

type SourcesConfig struct {
	URL     string        `yaml:"url"`     // Sources service API URL (e.g., "http://localhost:8080")
	Timeout time.Duration `yaml:"timeout"` // Request timeout (default: 5s)
	Enabled bool          `yaml:"enabled"` // Enable fetching cities from sources service
}

type ServerConfig struct {
	Address      string        `yaml:"address"`       // e.g., ":8070"
	ReadTimeout  time.Duration `yaml:"read_timeout"`  // Default: 10s
	WriteTimeout time.Duration `yaml:"write_timeout"` // Default: 30s
}

// Validate checks if the server configuration is valid and sets defaults.
func (c *ServerConfig) Validate() error {
	if c.Address == "" {
		c.Address = ":8070" // Default port
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = DefaultReadTimeoutSeconds * time.Second
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = DefaultWriteTimeoutSeconds * time.Second
	}
	return nil
}

// Validate checks if the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	if c.Elasticsearch.URL == "" {
		return errors.New("elasticsearch.url is required")
	}
	if c.Redis.URL == "" {
		return errors.New("redis.url is required")
	}
	if c.Service.CheckInterval <= 0 {
		return fmt.Errorf("service.check_interval must be positive, got %v", c.Service.CheckInterval)
	}
	if c.Sources.Enabled && c.Sources.URL == "" {
		return errors.New("sources.url is required when sources.enabled is true")
	}
	for i, city := range c.Cities {
		if city.Name == "" {
			return fmt.Errorf("cities[%d].name is required", i)
		}
	}
	return nil
}

// setDefaults sets default values for configuration fields
func setDefaults(cfg *Config) {
	if cfg.Server.Address == "" {
		cfg.Server.Address = ":8070"
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = DefaultReadTimeoutSeconds * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = DefaultWriteTimeoutSeconds * time.Second
	}
	if cfg.Service.CheckInterval == 0 {
		cfg.Service.CheckInterval = 5 * time.Minute
	}
	if cfg.Sources.Timeout == 0 {
		cfg.Sources.Timeout = 5 * time.Second
	}
	// Classified content defaults
	if cfg.Service.MinQualityScore == 0 {
		cfg.Service.MinQualityScore = 50 // Default minimum quality score
	}
	if cfg.Service.IndexSuffix == "" {
		if cfg.Service.UseClassifiedContent {
			cfg.Service.IndexSuffix = "_classified_content"
		} else {
			cfg.Service.IndexSuffix = "_articles"
		}
	}
}

// overrideWithEnvVars overrides configuration with environment variables
func overrideWithEnvVars(cfg *Config) {
	if esURL := os.Getenv("ES_URL"); esURL != "" {
		cfg.Elasticsearch.URL = esURL
	}
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		cfg.Redis.URL = redisURL
	}
	if sourcesURL := os.Getenv("SOURCES_URL"); sourcesURL != "" {
		cfg.Sources.URL = sourcesURL
	}
	if sourcesEnabled := os.Getenv("SOURCES_ENABLED"); sourcesEnabled != "" {
		cfg.Sources.Enabled = parseBool(sourcesEnabled)
	}
	// Parse APP_DEBUG environment variable
	if appDebug := os.Getenv("APP_DEBUG"); appDebug != "" {
		cfg.Debug = parseBool(appDebug)
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Set defaults
	setDefaults(&cfg)

	// Override with environment variables if present
	overrideWithEnvVars(&cfg)

	// Set server defaults
	if err := cfg.Server.Validate(); err != nil {
		return nil, fmt.Errorf("server config validation: %w", err)
	}

	// Override server config with environment variable if present
	if publisherPort := os.Getenv("PUBLISHER_PORT"); publisherPort != "" {
		cfg.Server.Address = ":" + publisherPort
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// LoadWithSources loads configuration and optionally fetches cities from sources service.
// If sources service is enabled and cities are fetched successfully, they override the config file cities.
func LoadWithSources(path string, sourcesClient interface {
	GetCities(context.Context) ([]CityConfig, error)
}) (*Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	// If sources service is enabled, try to fetch cities
	if cfg.Sources.Enabled && sourcesClient != nil {
		ctx, cancel := infracontext.WithTimeout(cfg.Sources.Timeout)
		defer cancel()

		cities, err := sourcesClient.GetCities(ctx)
		if err != nil {
			// Log warning but don't fail - fallback to config file cities
			// In production, you might want to use a logger here
			_ = err
			return cfg, nil
		}

		if len(cities) > 0 {
			cfg.Cities = cities
		}
	}

	return cfg, nil
}

// parseBool parses a string value as a boolean.
// Returns true for "true", "1", "yes" (case-insensitive), false otherwise.
// This function handles common boolean string representations.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}
