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
	logsconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/logs"
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
	// GetLogsConfig returns the job logs configuration.
	GetLogsConfig() *logsconfig.Config
	// GetAuthConfig returns the authentication configuration.
	GetAuthConfig() *AuthConfig
	// GetLoggingConfig returns the logging configuration.
	GetLoggingConfig() *LoggingConfig
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
	// Logs holds job logs streaming and archival configuration
	Logs *logsconfig.Config `yaml:"logs"`
	// Auth holds authentication configuration
	Auth *AuthConfig `yaml:"auth"`
	// Logging holds logging configuration
	Logging *LoggingConfig `yaml:"logging"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	JWTSecret string `env:"AUTH_JWT_SECRET" yaml:"jwt_secret"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level       string   `env:"LOG_LEVEL"        yaml:"level"`
	Format      string   `env:"LOG_FORMAT"       yaml:"format"`
	OutputPaths []string `env:"LOG_OUTPUT_PATHS" yaml:"output_paths"`
	Debug       bool     `env:"APP_DEBUG"        yaml:"debug"`
	Env         string   `env:"APP_ENV"          yaml:"env"`
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
	cfg, err := infraconfig.LoadWithDefaults(path, setDefaults)
	if err != nil {
		return nil, err
	}

	// Apply backward compatibility fixes after env overrides
	applyBackwardCompatibility(cfg)

	// Ensure cleanup interval is valid after env overrides
	ensureValidCleanupInterval(cfg)

	return cfg, nil
}

// applyBackwardCompatibility applies backward compatibility fixes for environment variables.
// This runs after env overrides to handle legacy env var names.
// Note: This function uses os.Getenv directly for backward compatibility with legacy env var names.
// This is acceptable since it's a migration path and the env vars are already loaded by infrastructure/config.
func applyBackwardCompatibility(cfg *Config) {
	if cfg.Elasticsearch == nil {
		return
	}

	// Support ELASTICSEARCH_HOSTS as fallback for ELASTICSEARCH_ADDRESSES
	// Check if legacy env var exists and Addresses is empty (infrastructure/config already loaded ELASTICSEARCH_ADDRESSES)
	if len(cfg.Elasticsearch.Addresses) == 0 {
		//nolint:forbidigo // Backward compatibility for legacy ELASTICSEARCH_HOSTS env var
		if hostsEnv := os.Getenv("ELASTICSEARCH_HOSTS"); hostsEnv != "" {
			cfg.Elasticsearch.Addresses = elasticsearch.ParseAddressesFromString(hostsEnv)
		}
	}

	// Support ELASTICSEARCH_INDEX_PREFIX as fallback for ELASTICSEARCH_INDEX_NAME
	// Check if legacy env var exists and IndexName is empty (infrastructure/config already loaded ELASTICSEARCH_INDEX_NAME)
	if cfg.Elasticsearch.IndexName == "" {
		//nolint:forbidigo // Backward compatibility for legacy ELASTICSEARCH_INDEX_PREFIX env var
		if indexPrefix := os.Getenv("ELASTICSEARCH_INDEX_PREFIX"); indexPrefix != "" {
			cfg.Elasticsearch.IndexName = indexPrefix
		}
	}
}

// ensureValidCleanupInterval ensures that CleanupInterval is positive after environment variable loading.
// This prevents panics when NewTicker is called with a non-positive interval.
func ensureValidCleanupInterval(cfg *Config) {
	if cfg.Crawler == nil {
		return
	}

	// If cleanup interval is zero or negative (e.g., from CRAWLER_CLEANUP_INTERVAL=0 or empty),
	// use the default to prevent NewTicker panic
	if cfg.Crawler.CleanupInterval <= 0 {
		cfg.Crawler.CleanupInterval = crawler.DefaultCleanupInterval
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
	if cfg.Logs == nil {
		cfg.Logs = logsconfig.NewConfig()
	}
	if cfg.Auth == nil {
		cfg.Auth = &AuthConfig{}
	}
	if cfg.Logging == nil {
		cfg.Logging = &LoggingConfig{
			Level:       "info",
			Format:      "json",
			OutputPaths: []string{"stdout"},
			Debug:       false,
			Env:         "production",
		}
	}
	// Set default output paths if empty
	if len(cfg.Logging.OutputPaths) == 0 {
		cfg.Logging.OutputPaths = []string{"stdout"}
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

// GetLogsConfig returns the job logs configuration.
func (c *Config) GetLogsConfig() *logsconfig.Config {
	if c.Logs == nil {
		// Return default config if not initialized
		return logsconfig.NewConfig()
	}
	return c.Logs
}

// GetAuthConfig returns the authentication configuration.
func (c *Config) GetAuthConfig() *AuthConfig {
	if c.Auth == nil {
		// Return default config if not initialized
		return &AuthConfig{}
	}
	return c.Auth
}

// GetLoggingConfig returns the logging configuration.
func (c *Config) GetLoggingConfig() *LoggingConfig {
	if c.Logging == nil {
		// Return default config if not initialized
		return &LoggingConfig{
			Level:       "info",
			Format:      "json",
			OutputPaths: []string{"stdout"},
			Debug:       false,
			Env:         "production",
		}
	}
	return c.Logging
}

// setupDevelopmentLogging configures logging settings based on environment variables.
// It separates concerns: debug level (controlled by APP_DEBUG) vs development formatting (controlled by APP_ENV).
// Note: This is a placeholder for any future logging-related config adjustments.
// Logging configuration is handled by the logger package itself.
func setupDevelopmentLogging(cfg *Config) {
	_ = cfg
	// Note: Logging configuration is handled by the logger package itself.
	// This function is a placeholder for any future logging-related config adjustments.
}
