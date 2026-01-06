// Package config provides configuration management for the crawler service.
// This file provides the new unified configuration using infrastructure/config.
package config

import (
	"fmt"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Default configuration values.
const (
	defaultServerPort         = 8060
	defaultReadTimeoutSec     = 30
	defaultWriteTimeoutSec    = 30
	defaultIdleTimeoutSec     = 60
	defaultMaxDepth           = 3
	defaultMaxConcurrency     = 10
	defaultRequestDelayMs     = 100
	defaultRequestTimeoutSec  = 30
	defaultUserAgent          = "NorthCloud-Crawler/1.0"
	defaultRateLimit          = 1.0
	defaultESURL              = "http://localhost:9200"
	defaultESMaxRetries       = 3
	defaultESTimeoutSec       = 30
	defaultESRawContentSuffix = "_raw_content"
	defaultDBHost             = "localhost"
	defaultDBPort             = 5432
	defaultDBUser             = "postgres"
	defaultDBName             = "crawler"
	defaultDBSSLMode          = "disable"
	defaultDBMaxConns         = 25
	defaultDBMaxIdleConns     = 5
	defaultDBConnMaxLifetimeM = 5
	defaultMinIOEndpoint      = "localhost:9000"
	defaultMinIOBucket        = "crawler-archives"
	defaultLogLevel           = "info"
	defaultLogFormat          = "json"
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
	Address         string        `env:"SERVER_ADDRESS"    yaml:"address"`
	Port            int           `env:"CRAWLER_PORT"      yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	SecurityEnabled bool          `yaml:"security_enabled"`
	APIKey          string        `yaml:"api_key"`
	Debug           bool          `env:"APP_DEBUG"         yaml:"debug"`
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
	URL              string        `env:"ELASTICSEARCH_URL"   yaml:"url"`
	Username         string        `yaml:"username"`
	Password         string        `yaml:"password"`
	MaxRetries       int           `yaml:"max_retries"`
	Timeout          time.Duration `yaml:"timeout"`
	RawContentSuffix string        `yaml:"raw_content_suffix"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Host            string        `env:"POSTGRES_CRAWLER_HOST"     yaml:"host"`
	Port            int           `env:"POSTGRES_CRAWLER_PORT"     yaml:"port"`
	User            string        `env:"POSTGRES_CRAWLER_USER"     yaml:"user"`
	Password        string        `env:"POSTGRES_CRAWLER_PASSWORD" yaml:"password"`
	Database        string        `env:"POSTGRES_CRAWLER_DB"       yaml:"database"`
	SSLMode         string        `yaml:"sslmode"`
	MaxConnections  int           `yaml:"max_connections"`
	MaxIdleConns    int           `yaml:"max_idle_connections"`
	ConnMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// MinIOConfig holds MinIO configuration for HTML archiving.
type MinIOConfig struct {
	Endpoint        string `env:"MINIO_ENDPOINT"   yaml:"endpoint"`
	AccessKeyID     string `env:"MINIO_ACCESS_KEY" yaml:"access_key_id"`
	SecretAccessKey string `env:"MINIO_SECRET_KEY" yaml:"secret_access_key"`
	BucketName      string `yaml:"bucket_name"`
	UseSSL          bool   `yaml:"use_ssl"`
	Enabled         bool   `yaml:"enabled"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
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
	setServerDefaults(&cfg.Server)
	setCrawlerDefaults(&cfg.Crawler)
	setElasticsearchDefaults(&cfg.Elasticsearch)
	setDatabaseDefaults(&cfg.Database)
	setMinIODefaults(&cfg.MinIO)
	setLoggingDefaults(&cfg.Logging)
}

func setServerDefaults(s *ServerConfig) {
	if s.Port == 0 {
		s.Port = defaultServerPort
	}
	if s.Address == "" {
		s.Address = fmt.Sprintf(":%d", s.Port)
	}
	if s.ReadTimeout == 0 {
		s.ReadTimeout = defaultReadTimeoutSec * time.Second
	}
	if s.WriteTimeout == 0 {
		s.WriteTimeout = defaultWriteTimeoutSec * time.Second
	}
	if s.IdleTimeout == 0 {
		s.IdleTimeout = defaultIdleTimeoutSec * time.Second
	}
}

func setCrawlerDefaults(c *CrawlerConfig) {
	if c.MaxDepth == 0 {
		c.MaxDepth = defaultMaxDepth
	}
	if c.MaxConcurrency == 0 {
		c.MaxConcurrency = defaultMaxConcurrency
	}
	if c.RequestDelay == 0 {
		c.RequestDelay = defaultRequestDelayMs * time.Millisecond
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = defaultRequestTimeoutSec * time.Second
	}
	if c.UserAgent == "" {
		c.UserAgent = defaultUserAgent
	}
	if c.RateLimit == 0 {
		c.RateLimit = defaultRateLimit
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
	if e.RawContentSuffix == "" {
		e.RawContentSuffix = defaultESRawContentSuffix
	}
}

func setDatabaseDefaults(d *DatabaseConfig) {
	if d.Host == "" {
		d.Host = defaultDBHost
	}
	if d.Port == 0 {
		d.Port = defaultDBPort
	}
	if d.User == "" {
		d.User = defaultDBUser
	}
	if d.Database == "" {
		d.Database = defaultDBName
	}
	if d.SSLMode == "" {
		d.SSLMode = defaultDBSSLMode
	}
	if d.MaxConnections == 0 {
		d.MaxConnections = defaultDBMaxConns
	}
	if d.MaxIdleConns == 0 {
		d.MaxIdleConns = defaultDBMaxIdleConns
	}
	if d.ConnMaxLifetime == 0 {
		d.ConnMaxLifetime = defaultDBConnMaxLifetimeM * time.Minute
	}
}

func setMinIODefaults(m *MinIOConfig) {
	if m.Endpoint == "" {
		m.Endpoint = defaultMinIOEndpoint
	}
	if m.BucketName == "" {
		m.BucketName = defaultMinIOBucket
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
