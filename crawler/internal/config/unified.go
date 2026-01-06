// Package config provides configuration management for the crawler service.
// This file provides the new unified configuration using infrastructure/config.
package config

import (
	"fmt"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// UnifiedConfig represents the crawler configuration using the unified loader.
type UnifiedConfig struct {
	Server        ServerConfig        `yaml:"server"`
	Crawler       CrawlerConfig       `yaml:"crawler"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Database      DatabaseConfig      `yaml:"database"`
	MinIO         MinIOConfig         `yaml:"minio"`
	Logging       LoggingConfig       `yaml:"logging"`
}

// ServerConfig holds server-specific configuration.
type ServerConfig struct {
	Address         string        `yaml:"address" env:"SERVER_ADDRESS"`
	Port            int           `yaml:"port" env:"CRAWLER_PORT"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	SecurityEnabled bool          `yaml:"security_enabled"`
	APIKey          string        `yaml:"api_key"`
	Debug           bool          `yaml:"debug" env:"APP_DEBUG"`
}

// CrawlerConfig holds crawler-specific configuration.
type CrawlerConfig struct {
	MaxDepth         int           `yaml:"max_depth"`
	MaxConcurrency   int           `yaml:"max_concurrency"`
	RequestDelay     time.Duration `yaml:"request_delay"`
	RequestTimeout   time.Duration `yaml:"request_timeout"`
	UserAgent        string        `yaml:"user_agent"`
	RespectRobotsTxt bool          `yaml:"respect_robots_txt"`
	RateLimit        float64       `yaml:"rate_limit"`
}

// ElasticsearchConfig holds Elasticsearch configuration.
type ElasticsearchConfig struct {
	URL                string        `yaml:"url" env:"ELASTICSEARCH_URL"`
	Username           string        `yaml:"username"`
	Password           string        `yaml:"password"`
	MaxRetries         int           `yaml:"max_retries"`
	Timeout            time.Duration `yaml:"timeout"`
	RawContentSuffix   string        `yaml:"raw_content_suffix"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Host            string        `yaml:"host" env:"POSTGRES_CRAWLER_HOST"`
	Port            int           `yaml:"port" env:"POSTGRES_CRAWLER_PORT"`
	User            string        `yaml:"user" env:"POSTGRES_CRAWLER_USER"`
	Password        string        `yaml:"password" env:"POSTGRES_CRAWLER_PASSWORD"`
	Database        string        `yaml:"database" env:"POSTGRES_CRAWLER_DB"`
	SSLMode         string        `yaml:"sslmode"`
	MaxConnections  int           `yaml:"max_connections"`
	MaxIdleConns    int           `yaml:"max_idle_connections"`
	ConnMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// MinIOConfig holds MinIO configuration for HTML archiving.
type MinIOConfig struct {
	Endpoint        string `yaml:"endpoint" env:"MINIO_ENDPOINT"`
	AccessKeyID     string `yaml:"access_key_id" env:"MINIO_ACCESS_KEY"`
	SecretAccessKey string `yaml:"secret_access_key" env:"MINIO_SECRET_KEY"`
	BucketName      string `yaml:"bucket_name"`
	UseSSL          bool   `yaml:"use_ssl"`
	Enabled         bool   `yaml:"enabled"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"`
}

// LoadUnified loads configuration using the infrastructure config loader.
func LoadUnified(path string) (*UnifiedConfig, error) {
	cfg, err := infraconfig.LoadWithDefaults[UnifiedConfig](path, setUnifiedDefaults)
	if err != nil {
		return nil, err
	}

	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, fmt.Errorf("invalid config: %w", validateErr)
	}

	return cfg, nil
}

// setUnifiedDefaults applies default values to the config.
func setUnifiedDefaults(cfg *UnifiedConfig) {
	// Server defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8060
	}
	if cfg.Server.Address == "" {
		cfg.Server.Address = fmt.Sprintf(":%d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 60 * time.Second
	}

	// Crawler defaults
	if cfg.Crawler.MaxDepth == 0 {
		cfg.Crawler.MaxDepth = 3
	}
	if cfg.Crawler.MaxConcurrency == 0 {
		cfg.Crawler.MaxConcurrency = 10
	}
	if cfg.Crawler.RequestDelay == 0 {
		cfg.Crawler.RequestDelay = 100 * time.Millisecond
	}
	if cfg.Crawler.RequestTimeout == 0 {
		cfg.Crawler.RequestTimeout = 30 * time.Second
	}
	if cfg.Crawler.UserAgent == "" {
		cfg.Crawler.UserAgent = "NorthCloud-Crawler/1.0"
	}
	if cfg.Crawler.RateLimit == 0 {
		cfg.Crawler.RateLimit = 1.0
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
	if cfg.Elasticsearch.RawContentSuffix == "" {
		cfg.Elasticsearch.RawContentSuffix = "_raw_content"
	}

	// Database defaults
	if cfg.Database.Host == "" {
		cfg.Database.Host = "localhost"
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.User == "" {
		cfg.Database.User = "postgres"
	}
	if cfg.Database.Database == "" {
		cfg.Database.Database = "crawler"
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Database.MaxConnections == 0 {
		cfg.Database.MaxConnections = 25
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 5
	}
	if cfg.Database.ConnMaxLifetime == 0 {
		cfg.Database.ConnMaxLifetime = 5 * time.Minute
	}

	// MinIO defaults
	if cfg.MinIO.Endpoint == "" {
		cfg.MinIO.Endpoint = "localhost:9000"
	}
	if cfg.MinIO.BucketName == "" {
		cfg.MinIO.BucketName = "crawler-archives"
	}

	// Logging defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
}

// Validate validates the unified configuration.
func (c *UnifiedConfig) Validate() error {
	if c.Elasticsearch.URL == "" {
		return &infraconfig.ValidationError{Field: "elasticsearch.url", Message: "is required"}
	}
	if c.Database.Host == "" {
		return &infraconfig.ValidationError{Field: "database.host", Message: "is required"}
	}
	if err := infraconfig.ValidatePort("server.port", c.Server.Port); err != nil {
		return err
	}
	return nil
}

// GetAddress returns the server address. If Address is set, returns it,
// otherwise constructs from port.
func (c *UnifiedConfig) GetAddress() string {
	if c.Server.Address != "" {
		return c.Server.Address
	}
	return fmt.Sprintf(":%d", c.Server.Port)
}
