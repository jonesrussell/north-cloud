package config

import (
	"fmt"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Config holds all configuration for the search service.
type Config struct {
	Service       ServiceConfig       `yaml:"service"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Facets        FacetsConfig        `yaml:"facets"`
	Logging       LoggingConfig       `yaml:"logging"`
	CORS          CORSConfig          `yaml:"cors"`
}

// ServiceConfig holds service-level configuration.
type ServiceConfig struct {
	Name            string        `yaml:"name"`
	Version         string        `yaml:"version"`
	Port            int           `yaml:"port" env:"SEARCH_PORT"`
	Debug           bool          `yaml:"debug" env:"SEARCH_DEBUG"`
	MaxPageSize     int           `yaml:"max_page_size" env:"SEARCH_MAX_PAGE_SIZE"`
	DefaultPageSize int           `yaml:"default_page_size" env:"SEARCH_DEFAULT_PAGE_SIZE"`
	MaxQueryLength  int           `yaml:"max_query_length"`
	SearchTimeout   time.Duration `yaml:"search_timeout"`
}

// ElasticsearchConfig holds Elasticsearch connection and search configuration.
type ElasticsearchConfig struct {
	URL                      string        `yaml:"url" env:"ELASTICSEARCH_URL"`
	Username                 string        `yaml:"username" env:"ELASTICSEARCH_USERNAME"`
	Password                 string        `yaml:"password" env:"ELASTICSEARCH_PASSWORD"`
	MaxRetries               int           `yaml:"max_retries"`
	Timeout                  time.Duration `yaml:"timeout"`
	ClassifiedContentPattern string        `yaml:"classified_content_pattern"`
	DefaultBoost             BoostConfig   `yaml:"default_boost"`
	HighlightEnabled         bool          `yaml:"highlight_enabled"`
	HighlightFragmentSize    int           `yaml:"highlight_fragment_size"`
	HighlightMaxFragments    int           `yaml:"highlight_max_fragments"`
}

// BoostConfig holds field boosting values.
type BoostConfig struct {
	Title           float64 `yaml:"title"`
	OGTitle         float64 `yaml:"og_title"`
	RawText         float64 `yaml:"raw_text"`
	OGDescription   float64 `yaml:"og_description"`
	MetaDescription float64 `yaml:"meta_description"`
}

// FacetsConfig holds faceted search configuration.
type FacetsConfig struct {
	Enabled         bool `yaml:"enabled"`
	MaxTopics       int  `yaml:"max_topics"`
	MaxSources      int  `yaml:"max_sources"`
	MaxContentTypes int  `yaml:"max_content_types"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"`
	Output string `yaml:"output"`
}

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins" env:"CORS_ORIGINS"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// Load loads configuration from file and environment variables.
func Load(path string) (*Config, error) {
	cfg, err := infraconfig.LoadWithDefaults[Config](path, setDefaults)
	if err != nil {
		return nil, err
	}

	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, fmt.Errorf("invalid configuration: %w", validateErr)
	}

	return cfg, nil
}

// setDefaults applies default values to the config.
func setDefaults(cfg *Config) {
	// Service defaults
	if cfg.Service.Name == "" {
		cfg.Service.Name = "search"
	}
	if cfg.Service.Version == "" {
		cfg.Service.Version = "1.0.0"
	}
	if cfg.Service.Port == 0 {
		cfg.Service.Port = 8092
	}
	if cfg.Service.MaxPageSize == 0 {
		cfg.Service.MaxPageSize = 100
	}
	if cfg.Service.DefaultPageSize == 0 {
		cfg.Service.DefaultPageSize = 20
	}
	if cfg.Service.MaxQueryLength == 0 {
		cfg.Service.MaxQueryLength = 500
	}
	if cfg.Service.SearchTimeout == 0 {
		cfg.Service.SearchTimeout = 5 * time.Second
	}

	// Elasticsearch defaults
	if cfg.Elasticsearch.URL == "" {
		cfg.Elasticsearch.URL = "http://localhost:9200"
	}
	if cfg.Elasticsearch.MaxRetries == 0 {
		cfg.Elasticsearch.MaxRetries = 3
	}
	if cfg.Elasticsearch.Timeout == 0 {
		cfg.Elasticsearch.Timeout = 30 * time.Second
	}
	if cfg.Elasticsearch.ClassifiedContentPattern == "" {
		cfg.Elasticsearch.ClassifiedContentPattern = "*_classified_content"
	}
	if cfg.Elasticsearch.DefaultBoost.Title == 0 {
		cfg.Elasticsearch.DefaultBoost.Title = 3.0
	}
	if cfg.Elasticsearch.DefaultBoost.OGTitle == 0 {
		cfg.Elasticsearch.DefaultBoost.OGTitle = 2.0
	}
	if cfg.Elasticsearch.DefaultBoost.RawText == 0 {
		cfg.Elasticsearch.DefaultBoost.RawText = 1.0
	}
	if cfg.Elasticsearch.HighlightFragmentSize == 0 {
		cfg.Elasticsearch.HighlightFragmentSize = 150
	}
	if cfg.Elasticsearch.HighlightMaxFragments == 0 {
		cfg.Elasticsearch.HighlightMaxFragments = 3
	}

	// Facets defaults
	if cfg.Facets.MaxTopics == 0 {
		cfg.Facets.MaxTopics = 20
	}
	if cfg.Facets.MaxSources == 0 {
		cfg.Facets.MaxSources = 20
	}
	if cfg.Facets.MaxContentTypes == 0 {
		cfg.Facets.MaxContentTypes = 10
	}

	// Logging defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}

	// CORS defaults
	if len(cfg.CORS.AllowedOrigins) == 0 {
		cfg.CORS.AllowedOrigins = []string{"*"}
	}
	if len(cfg.CORS.AllowedMethods) == 0 {
		cfg.CORS.AllowedMethods = []string{"GET", "POST", "OPTIONS"}
	}
	if len(cfg.CORS.AllowedHeaders) == 0 {
		cfg.CORS.AllowedHeaders = []string{"Content-Type", "Authorization"}
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Service.Port < 1 || c.Service.Port > 65535 {
		return &infraconfig.ValidationError{Field: "service.port", Message: fmt.Sprintf("invalid port: %d", c.Service.Port)}
	}
	if c.Service.MaxPageSize < 1 {
		return &infraconfig.ValidationError{Field: "service.max_page_size", Message: "must be greater than 0"}
	}
	if c.Service.DefaultPageSize < 1 || c.Service.DefaultPageSize > c.Service.MaxPageSize {
		return &infraconfig.ValidationError{
			Field:   "service.default_page_size",
			Message: fmt.Sprintf("must be between 1 and %d", c.Service.MaxPageSize),
		}
	}
	if c.Elasticsearch.URL == "" {
		return &infraconfig.ValidationError{Field: "elasticsearch.url", Message: "is required"}
	}
	if c.Elasticsearch.ClassifiedContentPattern == "" {
		return &infraconfig.ValidationError{Field: "elasticsearch.classified_content_pattern", Message: "is required"}
	}
	if err := infraconfig.ValidateLogLevel(c.Logging.Level); err != nil {
		return err
	}
	if err := infraconfig.ValidateLogFormat(c.Logging.Format); err != nil {
		return err
	}
	return nil
}
