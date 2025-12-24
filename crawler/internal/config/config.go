// Package config provides configuration management for the GoCrawl application.
// It handles loading, validation, and access to configuration values from both
// YAML files and environment variables using Viper.
package config

import (
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	dbconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/elasticsearch"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/minio"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/server"
	"github.com/spf13/viper"
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
	// GetConfigFile returns the path to the configuration file.
	GetConfigFile() string
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

// LoadConfig loads the configuration from Viper
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Server:        server.NewConfig(),
		Elasticsearch: elasticsearch.LoadFromViper(viper.GetViper()),
		Crawler:       crawler.LoadFromViper(viper.GetViper()),
		Database:      dbconfig.LoadFromViper(viper.GetViper()),
		MinIO:         minio.LoadFromViper(viper.GetViper()),
	}

	// Set server config from Viper with defaults
	cfg.Server.Address = viper.GetString("server.address")
	if cfg.Server.Address == "" {
		cfg.Server.Address = defaultServerAddress
	}

	cfg.Server.ReadTimeout = viper.GetDuration("server.readTimeout")
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = defaultServerReadTimeout
	}

	cfg.Server.WriteTimeout = viper.GetDuration("server.writeTimeout")
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = defaultServerWriteTimeout
	}

	cfg.Server.IdleTimeout = viper.GetDuration("server.idleTimeout")
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = defaultServerIdleTimeout
	}

	cfg.Server.SecurityEnabled = viper.GetBool("server.security.enabled")
	cfg.Server.APIKey = viper.GetString("server.security.apiKey")

	// Validate the configuration
	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, fmt.Errorf("invalid config: %w", validateErr)
	}

	// Validate MinIO configuration
	if cfg.MinIO != nil {
		if err := cfg.MinIO.Validate(); err != nil {
			return nil, fmt.Errorf("minio: %w", err)
		}
	}

	return cfg, nil
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
		return dbconfig.LoadFromViper(viper.GetViper())
	}
	return c.Database
}

// GetMinIOConfig returns the MinIO configuration.
func (c *Config) GetMinIOConfig() *minio.Config {
	if c.MinIO == nil {
		// Return default config if not initialized
		return minio.LoadFromViper(viper.GetViper())
	}
	return c.MinIO
}

// GetConfigFile returns the path to the configuration file.
func (c *Config) GetConfigFile() string {
	return viper.ConfigFileUsed()
}
