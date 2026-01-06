package config

import (
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Config holds the application configuration.
type Config struct {
	Service       ServiceConfig       `yaml:"service"`
	Database      DatabaseConfig      `yaml:"database"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	IndexTypes    IndexTypesConfig    `yaml:"index_types"`
	Logging       LoggingConfig       `yaml:"logging"`
}

// ServiceConfig holds service configuration.
type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `yaml:"port" env:"INDEX_MANAGER_PORT"`
	Debug   bool   `yaml:"debug" env:"APP_DEBUG"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Host                  string        `yaml:"host" env:"POSTGRES_INDEX_MANAGER_HOST"`
	Port                  int           `yaml:"port" env:"POSTGRES_INDEX_MANAGER_PORT"`
	User                  string        `yaml:"user" env:"POSTGRES_INDEX_MANAGER_USER"`
	Password              string        `yaml:"password" env:"POSTGRES_INDEX_MANAGER_PASSWORD"`
	Database              string        `yaml:"database" env:"POSTGRES_INDEX_MANAGER_DB"`
	SSLMode               string        `yaml:"sslmode"`
	MaxConnections        int           `yaml:"max_connections"`
	MaxIdleConns          int           `yaml:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// ElasticsearchConfig holds Elasticsearch configuration.
type ElasticsearchConfig struct {
	URL        string        `yaml:"url" env:"ELASTICSEARCH_URL"`
	Username   string        `yaml:"username"`
	Password   string        `yaml:"password"`
	MaxRetries int           `yaml:"max_retries"`
	Timeout    time.Duration `yaml:"timeout"`
}

// IndexTypesConfig holds index type configurations.
type IndexTypesConfig struct {
	RawContent        IndexTypeConfig `yaml:"raw_content"`
	ClassifiedContent IndexTypeConfig `yaml:"classified_content"`
	Article           IndexTypeConfig `yaml:"article"`
	Page              IndexTypeConfig `yaml:"page"`
}

// IndexTypeConfig holds configuration for a specific index type.
type IndexTypeConfig struct {
	Suffix     string `yaml:"suffix"`
	AutoCreate bool   `yaml:"auto_create"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"`
	Output string `yaml:"output"`
}

// Load loads configuration from a YAML file.
func Load(path string) (*Config, error) {
	return infraconfig.LoadWithDefaults[Config](path, setDefaults)
}

// setDefaults applies default values to the config.
func setDefaults(cfg *Config) {
	// Service defaults
	if cfg.Service.Name == "" {
		cfg.Service.Name = "index-manager"
	}
	if cfg.Service.Version == "" {
		cfg.Service.Version = "1.0.0"
	}
	if cfg.Service.Port == 0 {
		cfg.Service.Port = 8090
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
		cfg.Database.Database = "index_manager"
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
	if cfg.Database.ConnectionMaxLifetime == 0 {
		cfg.Database.ConnectionMaxLifetime = 5 * time.Minute
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

	// Logging defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if err := infraconfig.ValidatePort("service.port", c.Service.Port); err != nil {
		return err
	}
	if c.Database.Host == "" {
		return &infraconfig.ValidationError{Field: "database.host", Message: "is required"}
	}
	if c.Elasticsearch.URL == "" {
		return &infraconfig.ValidationError{Field: "elasticsearch.url", Message: "is required"}
	}
	return nil
}
