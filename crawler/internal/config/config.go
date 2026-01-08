// Package config provides configuration management for the GoCrawl application.
// It handles loading, validation, and access to configuration values from both
// YAML files and environment variables using infrastructure/config.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	dbconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/elasticsearch"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/minio"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/server"
	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Interface defines the interface for configuration management.
type Interface interface {
	// GetServerConfig returns the server configuration.
	GetServerConfig() *server.Config
	// GetCrawlerConfig returns the crawler configuration.
	GetCrawlerConfig() *crawler.Config
	// GetElasticsearchConfig returns the Elasticsearch configuration.
	GetElasticsearchConfig() *elasticsearch.Config
	// GetDatabaseConfig returns the database configuration.
	GetDatabaseConfig() *dbconfig.Config
	// GetMinIOConfig returns the MinIO configuration.
	GetMinIOConfig() *minio.Config
	// Validate validates the configuration based on the current command.
	Validate() error
}

// Server defaults
const (
	defaultServerAddress      = ":8080"
	defaultServerReadTimeout  = 30 * time.Second
	defaultServerWriteTimeout = 30 * time.Second
	defaultServerIdleTimeout  = 60 * time.Second
)

// Ensure Config implements Interface
var _ Interface = (*Config)(nil)

// Config represents the application configuration.
type Config struct {
	// Server holds server-specific configuration
	Server *server.Config `yaml:"server"`
	// Crawler holds crawler-specific configuration
	Crawler *crawler.Config `yaml:"crawler"`
	// Elasticsearch holds Elasticsearch configuration
	Elasticsearch *elasticsearch.Config `yaml:"elasticsearch"`
	// Database holds database configuration
	Database *dbconfig.Config `yaml:"database"`
	// MinIO holds MinIO configuration for HTML archiving
	MinIO *minio.Config `yaml:"minio"`
}

// validateHTTPDConfig validates the configuration for the httpd command
func (c *Config) validateHTTPDConfig() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := c.Elasticsearch.Validate(); err != nil {
		return fmt.Errorf("elasticsearch: %w", err)
	}
	return nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	return c.validateHTTPDConfig()
}

// Load loads configuration from the specified path.
func Load(path string) (*Config, error) {
	cfg, err := infraconfig.LoadWithDefaults[Config](path, setDefaults)
	if err != nil {
		return nil, err
	}

	// Apply backward compatibility fixes after env overrides
	applyBackwardCompatibility(cfg)

	return cfg, nil
}

// applyBackwardCompatibility applies backward compatibility fixes for environment variables.
// This runs after env overrides to handle legacy env var names.
func applyBackwardCompatibility(cfg *Config) {
	if cfg.Elasticsearch == nil {
		return
	}

	// Support ELASTICSEARCH_HOSTS as fallback for ELASTICSEARCH_ADDRESSES
	if len(cfg.Elasticsearch.Addresses) == 0 {
		if hostsEnv := os.Getenv("ELASTICSEARCH_HOSTS"); hostsEnv != "" {
			cfg.Elasticsearch.Addresses = elasticsearch.ParseAddressesFromString(hostsEnv)
		}
	}

	// Support ELASTICSEARCH_INDEX_PREFIX as fallback for ELASTICSEARCH_INDEX_NAME
	if cfg.Elasticsearch.IndexName == "" {
		if indexPrefix := os.Getenv("ELASTICSEARCH_INDEX_PREFIX"); indexPrefix != "" {
			cfg.Elasticsearch.IndexName = indexPrefix
		}
	}
}

// setDefaults applies default values to the config.
func setDefaults(cfg *Config) {
	// Initialize nil pointers
	if cfg.Server == nil {
		cfg.Server = server.NewConfig()
	}
	if cfg.Crawler == nil {
		cfg.Crawler = crawler.New()
	}
	if cfg.Elasticsearch == nil {
		cfg.Elasticsearch = elasticsearch.NewConfig()
	}
	if cfg.Database == nil {
		cfg.Database = dbconfig.NewConfig()
	}
	if cfg.MinIO == nil {
		cfg.MinIO = minio.NewConfig()
	}

	// Set server defaults
	if cfg.Server.Address == "" {
		cfg.Server.Address = defaultServerAddress
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = defaultServerReadTimeout
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = defaultServerWriteTimeout
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = defaultServerIdleTimeout
	}

	// Apply development logging settings based on environment
	setupDevelopmentLogging(cfg)
}

// GetServerConfig returns the server configuration.
func (c *Config) GetServerConfig() *server.Config {
	return c.Server
}

// GetCrawlerConfig returns the crawler configuration.
func (c *Config) GetCrawlerConfig() *crawler.Config {
	return c.Crawler
}

// GetElasticsearchConfig returns the Elasticsearch configuration.
func (c *Config) GetElasticsearchConfig() *elasticsearch.Config {
	return c.Elasticsearch
}

// GetDatabaseConfig returns the database configuration.
func (c *Config) GetDatabaseConfig() *dbconfig.Config {
	if c.Database == nil {
		// Return default config if not initialized
		return dbconfig.NewConfig()
	}
	return c.Database
}

// GetMinIOConfig returns the MinIO configuration.
func (c *Config) GetMinIOConfig() *minio.Config {
	if c.MinIO == nil {
		// Return default config if not initialized
		return minio.NewConfig()
	}
	return c.MinIO
}

// setupDevelopmentLogging configures logging settings based on environment variables.
// It separates concerns: debug level (controlled by APP_DEBUG) vs development formatting (controlled by APP_ENV).
func setupDevelopmentLogging(cfg *Config) {
	// Note: Logging configuration is handled by the logger package itself,
	// but we can set debug flag if APP_DEBUG is set
	// This is a placeholder for any future logging-related config adjustments
	_ = cfg
	_ = os.Getenv("APP_DEBUG")
	_ = os.Getenv("APP_ENV")
}
