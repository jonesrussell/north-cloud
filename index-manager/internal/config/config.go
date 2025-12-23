package config

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

// Config holds the application configuration
type Config struct {
	Service       ServiceConfig       `yaml:"service"`
	Database      DatabaseConfig      `yaml:"database"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	IndexTypes    IndexTypesConfig    `yaml:"index_types"`
	Logging       LoggingConfig       `yaml:"logging"`
}

// ServiceConfig holds service configuration
type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `yaml:"port"`
	Debug   bool   `yaml:"debug"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host                  string        `yaml:"host"`
	Port                  int           `yaml:"port"`
	User                  string        `yaml:"user"`
	Password              string        `yaml:"password"`
	Database              string        `yaml:"database"`
	SSLMode               string        `yaml:"sslmode"`
	MaxConnections        int           `yaml:"max_connections"`
	MaxIdleConns          int           `yaml:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// ElasticsearchConfig holds Elasticsearch configuration
type ElasticsearchConfig struct {
	URL        string        `yaml:"url"`
	Username   string        `yaml:"username"`
	Password   string        `yaml:"password"`
	MaxRetries int           `yaml:"max_retries"`
	Timeout    time.Duration `yaml:"timeout"`
}

// IndexTypesConfig holds index type configurations
type IndexTypesConfig struct {
	RawContent        IndexTypeConfig `yaml:"raw_content"`
	ClassifiedContent IndexTypeConfig `yaml:"classified_content"`
	Article           IndexTypeConfig `yaml:"article"`
	Page              IndexTypeConfig `yaml:"page"`
}

// IndexTypeConfig holds configuration for a specific index type
type IndexTypeConfig struct {
	Suffix     string `yaml:"suffix"`
	AutoCreate bool   `yaml:"auto_create"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(&config)

	return &config, nil
}

// applyEnvOverrides applies environment variable overrides
func applyEnvOverrides(cfg *Config) {
	if port := os.Getenv("INDEX_MANAGER_PORT"); port != "" {
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err == nil {
			cfg.Service.Port = p
		}
	}

	if host := os.Getenv("POSTGRES_INDEX_MANAGER_HOST"); host != "" {
		cfg.Database.Host = host
	}

	if url := os.Getenv("ELASTICSEARCH_URL"); url != "" {
		cfg.Elasticsearch.URL = url
	}
}
