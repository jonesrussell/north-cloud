package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the search service
type Config struct {
	Service       ServiceConfig       `yaml:"service"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Facets        FacetsConfig        `yaml:"facets"`
	Logging       LoggingConfig       `yaml:"logging"`
	CORS          CORSConfig          `yaml:"cors"`
}

// ServiceConfig holds service-level configuration
type ServiceConfig struct {
	Name             string        `yaml:"name"`
	Version          string        `yaml:"version"`
	Port             int           `yaml:"port"`
	Debug            bool          `yaml:"debug"`
	MaxPageSize      int           `yaml:"max_page_size"`
	DefaultPageSize  int           `yaml:"default_page_size"`
	MaxQueryLength   int           `yaml:"max_query_length"`
	SearchTimeout    time.Duration `yaml:"search_timeout"`
}

// ElasticsearchConfig holds Elasticsearch connection and search configuration
type ElasticsearchConfig struct {
	URL                       string                `yaml:"url"`
	Username                  string                `yaml:"username"`
	Password                  string                `yaml:"password"`
	MaxRetries                int                   `yaml:"max_retries"`
	Timeout                   time.Duration         `yaml:"timeout"`
	ClassifiedContentPattern  string                `yaml:"classified_content_pattern"`
	DefaultBoost              BoostConfig           `yaml:"default_boost"`
	HighlightEnabled          bool                  `yaml:"highlight_enabled"`
	HighlightFragmentSize     int                   `yaml:"highlight_fragment_size"`
	HighlightMaxFragments     int                   `yaml:"highlight_max_fragments"`
}

// BoostConfig holds field boosting values
type BoostConfig struct {
	Title          float64 `yaml:"title"`
	OGTitle        float64 `yaml:"og_title"`
	RawText        float64 `yaml:"raw_text"`
	OGDescription  float64 `yaml:"og_description"`
	MetaDescription float64 `yaml:"meta_description"`
}

// FacetsConfig holds faceted search configuration
type FacetsConfig struct {
	Enabled          bool `yaml:"enabled"`
	MaxTopics        int  `yaml:"max_topics"`
	MaxSources       int  `yaml:"max_sources"`
	MaxContentTypes  int  `yaml:"max_content_types"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	configPath := getEnv("CONFIG_PATH", "config.yml")

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If config file doesn't exist, use config.yml.example
		data, err = os.ReadFile("config.yml.example")
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Override with environment variables
	applyEnvironmentOverrides(&cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// applyEnvironmentOverrides applies environment variable overrides
func applyEnvironmentOverrides(cfg *Config) {
	// Service
	if port := getEnv("SEARCH_PORT", ""); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Service.Port = p
		}
	}
	if debug := getEnv("SEARCH_DEBUG", ""); debug != "" {
		cfg.Service.Debug = debug == "true"
	}
	if maxPageSize := getEnv("SEARCH_MAX_PAGE_SIZE", ""); maxPageSize != "" {
		if m, err := strconv.Atoi(maxPageSize); err == nil {
			cfg.Service.MaxPageSize = m
		}
	}
	if defaultPageSize := getEnv("SEARCH_DEFAULT_PAGE_SIZE", ""); defaultPageSize != "" {
		if d, err := strconv.Atoi(defaultPageSize); err == nil {
			cfg.Service.DefaultPageSize = d
		}
	}

	// Elasticsearch
	if url := getEnv("ELASTICSEARCH_URL", ""); url != "" {
		cfg.Elasticsearch.URL = url
	}
	if username := getEnv("ELASTICSEARCH_USERNAME", ""); username != "" {
		cfg.Elasticsearch.Username = username
	}
	if password := getEnv("ELASTICSEARCH_PASSWORD", ""); password != "" {
		cfg.Elasticsearch.Password = password
	}

	// Logging
	if level := getEnv("LOG_LEVEL", ""); level != "" {
		cfg.Logging.Level = level
	}
	if format := getEnv("LOG_FORMAT", ""); format != "" {
		cfg.Logging.Format = format
	}

	// CORS
	if origins := getEnv("CORS_ORIGINS", ""); origins != "" {
		cfg.CORS.AllowedOrigins = strings.Split(origins, ",")
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Service validation
	if c.Service.Port < 1 || c.Service.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Service.Port)
	}
	if c.Service.MaxPageSize < 1 {
		return fmt.Errorf("max_page_size must be greater than 0")
	}
	if c.Service.DefaultPageSize < 1 || c.Service.DefaultPageSize > c.Service.MaxPageSize {
		return fmt.Errorf("default_page_size must be between 1 and %d", c.Service.MaxPageSize)
	}

	// Elasticsearch validation
	if c.Elasticsearch.URL == "" {
		return fmt.Errorf("elasticsearch url is required")
	}
	if c.Elasticsearch.ClassifiedContentPattern == "" {
		return fmt.Errorf("classified_content_pattern is required")
	}

	// Logging validation
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}
	validFormats := map[string]bool{"json": true, "console": true}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	return nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
