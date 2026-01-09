package config

import (
	"fmt"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Default configuration values.
const (
	defaultServiceName       = "search"
	defaultServiceVersion    = "1.0.0"
	defaultServicePort       = 8092
	defaultMaxPageSize       = 100
	defaultPageSize          = 20
	defaultMaxQueryLength    = 500
	defaultSearchTimeoutSec  = 5
	defaultESURL             = "http://localhost:9200"
	defaultESMaxRetries      = 3
	defaultESTimeoutSec      = 30
	defaultContentPattern    = "*_classified_content"
	defaultBoostTitle        = 3.0
	defaultBoostOGTitle      = 2.0
	defaultBoostRawText      = 1.0
	defaultHighlightFragment = 150
	defaultHighlightMax      = 3
	defaultMaxTopics         = 20
	defaultMaxSources        = 20
	defaultMaxContentTypes   = 10
	defaultLogLevel          = "info"
	defaultLogFormat         = "json"
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
	Port            int           `env:"SEARCH_PORT"              yaml:"port"`
	Debug           bool          `env:"SEARCH_DEBUG"             yaml:"debug"`
	MaxPageSize     int           `env:"SEARCH_MAX_PAGE_SIZE"     yaml:"max_page_size"`
	DefaultPageSize int           `env:"SEARCH_DEFAULT_PAGE_SIZE" yaml:"default_page_size"`
	MaxQueryLength  int           `yaml:"max_query_length"`
	SearchTimeout   time.Duration `yaml:"search_timeout"`
}

// ElasticsearchConfig holds Elasticsearch connection and search configuration.
type ElasticsearchConfig struct {
	URL                      string        `env:"ELASTICSEARCH_URL"           yaml:"url"`
	Username                 string        `env:"ELASTICSEARCH_USERNAME"      yaml:"username"`
	Password                 string        `env:"ELASTICSEARCH_PASSWORD"      yaml:"password"`
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
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
	Output string `yaml:"output"`
}

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `env:"CORS_ORIGINS"       yaml:"allowed_origins"`
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
	setServiceDefaults(&cfg.Service)
	setElasticsearchDefaults(&cfg.Elasticsearch)
	setFacetsDefaults(&cfg.Facets)
	setLoggingDefaults(&cfg.Logging)
	setCORSDefaults(&cfg.CORS)
}

func setServiceDefaults(s *ServiceConfig) {
	if s.Name == "" {
		s.Name = defaultServiceName
	}
	if s.Version == "" {
		s.Version = defaultServiceVersion
	}
	if s.Port == 0 {
		s.Port = defaultServicePort
	}
	if s.MaxPageSize == 0 {
		s.MaxPageSize = defaultMaxPageSize
	}
	if s.DefaultPageSize == 0 {
		s.DefaultPageSize = defaultPageSize
	}
	if s.MaxQueryLength == 0 {
		s.MaxQueryLength = defaultMaxQueryLength
	}
	if s.SearchTimeout == 0 {
		s.SearchTimeout = defaultSearchTimeoutSec * time.Second
	}
}

func setElasticsearchDefaults(e *ElasticsearchConfig) {
	if e.URL == "" {
		e.URL = defaultESURL
	}
	if e.MaxRetries == 0 {
		e.MaxRetries = defaultESMaxRetries
	}
	if e.Timeout == 0 {
		e.Timeout = defaultESTimeoutSec * time.Second
	}
	if e.ClassifiedContentPattern == "" {
		e.ClassifiedContentPattern = defaultContentPattern
	}
	if e.DefaultBoost.Title == 0 {
		e.DefaultBoost.Title = defaultBoostTitle
	}
	if e.DefaultBoost.OGTitle == 0 {
		e.DefaultBoost.OGTitle = defaultBoostOGTitle
	}
	if e.DefaultBoost.RawText == 0 {
		e.DefaultBoost.RawText = defaultBoostRawText
	}
	if e.HighlightFragmentSize == 0 {
		e.HighlightFragmentSize = defaultHighlightFragment
	}
	if e.HighlightMaxFragments == 0 {
		e.HighlightMaxFragments = defaultHighlightMax
	}
}

func setFacetsDefaults(f *FacetsConfig) {
	if f.MaxTopics == 0 {
		f.MaxTopics = defaultMaxTopics
	}
	if f.MaxSources == 0 {
		f.MaxSources = defaultMaxSources
	}
	if f.MaxContentTypes == 0 {
		f.MaxContentTypes = defaultMaxContentTypes
	}
}

func setLoggingDefaults(l *LoggingConfig) {
	if l.Level == "" {
		l.Level = defaultLogLevel
	}
	if l.Format == "" {
		l.Format = defaultLogFormat
	}
}

func setCORSDefaults(c *CORSConfig) {
	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = []string{"*"}
	}
	if len(c.AllowedMethods) == 0 {
		c.AllowedMethods = []string{"GET", "POST", "OPTIONS"}
	}
	if len(c.AllowedHeaders) == 0 {
		c.AllowedHeaders = []string{"Content-Type", "Authorization"}
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
