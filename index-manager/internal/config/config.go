package config

import (
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Default configuration values.
const (
	defaultServiceName     = "index-manager"
	defaultServiceVersion  = "1.0.0"
	defaultServicePort     = 8090
	defaultDBHost          = "localhost"
	defaultDBPort          = 5432
	defaultDBUser          = "postgres"
	defaultDBName          = "index_manager"
	defaultDBSSLMode       = "disable"
	defaultDBMaxConns      = 25
	defaultDBMaxIdleConns  = 5
	defaultDBConnLifetimeM = 5
	defaultESURL           = "http://localhost:9200"
	defaultESMaxRetries    = 3
	defaultESTimeoutSec    = 30
	defaultLogLevel        = "info"
	defaultLogFormat       = "json"
	defaultShards          = 1
	defaultReplicas        = 1
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
	Port    int    `env:"INDEX_MANAGER_PORT" yaml:"port"`
	Debug   bool   `env:"APP_DEBUG"          yaml:"debug"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Host                  string        `env:"POSTGRES_INDEX_MANAGER_HOST"     yaml:"host"`
	Port                  int           `env:"POSTGRES_INDEX_MANAGER_PORT"     yaml:"port"`
	User                  string        `env:"POSTGRES_INDEX_MANAGER_USER"     yaml:"user"`
	Password              string        `env:"POSTGRES_INDEX_MANAGER_PASSWORD" yaml:"password"`
	Database              string        `env:"POSTGRES_INDEX_MANAGER_DB"       yaml:"database"`
	SSLMode               string        `yaml:"sslmode"`
	MaxConnections        int           `yaml:"max_connections"`
	MaxIdleConns          int           `yaml:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// ElasticsearchConfig holds Elasticsearch configuration.
type ElasticsearchConfig struct {
	URL        string        `env:"ELASTICSEARCH_URL" yaml:"url"`
	Username   string        `yaml:"username"`
	Password   string        `yaml:"password"`
	MaxRetries int           `yaml:"max_retries"`
	Timeout    time.Duration `yaml:"timeout"`
}

// IndexTypesConfig holds index type configurations.
type IndexTypesConfig struct {
	RawContent        IndexTypeConfig `yaml:"raw_content"`
	ClassifiedContent IndexTypeConfig `yaml:"classified_content"`
}

// IndexTypeConfig holds configuration for a specific index type.
type IndexTypeConfig struct {
	Suffix     string `yaml:"suffix"`
	AutoCreate bool   `yaml:"auto_create"`
	Shards     int    `yaml:"shards"`
	Replicas   int    `yaml:"replicas"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
	Output string `yaml:"output"`
}

// Load loads configuration from a YAML file.
func Load(path string) (*Config, error) {
	return infraconfig.LoadWithDefaults[Config](path, setDefaults)
}

// setDefaults applies default values to the config.
func setDefaults(cfg *Config) {
	setServiceDefaults(&cfg.Service)
	setDatabaseDefaults(&cfg.Database)
	setElasticsearchDefaults(&cfg.Elasticsearch)
	setIndexTypeDefaults(&cfg.IndexTypes)
	setLoggingDefaults(&cfg.Logging)
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
	if d.ConnectionMaxLifetime == 0 {
		d.ConnectionMaxLifetime = defaultDBConnLifetimeM * time.Minute
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
}

func setIndexTypeDefaults(cfg *IndexTypesConfig) {
	if cfg.RawContent.Shards == 0 {
		cfg.RawContent.Shards = defaultShards
	}
	// raw_content replicas default 0 (transient, rebuildable from source)
	// No special handling needed since Go zero-value is 0

	if cfg.ClassifiedContent.Shards == 0 {
		cfg.ClassifiedContent.Shards = defaultShards
	}
	if cfg.ClassifiedContent.Replicas == 0 {
		cfg.ClassifiedContent.Replicas = defaultReplicas
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
