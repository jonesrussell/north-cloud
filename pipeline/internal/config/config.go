package config

import (
	"fmt"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Default service configuration values.
const (
	defaultServiceName    = "pipeline"
	defaultServiceVersion = "1.0.0"
	defaultServicePort    = 8075
	defaultLogLevel       = "info"
	defaultLogFormat      = "json"
)

// Default database configuration values.
const (
	defaultDBHost          = "localhost"
	defaultDBPort          = 5432
	defaultDBUser          = "postgres"
	defaultDBName          = "pipeline"
	defaultDBSSLMode       = "disable"
	defaultDBMaxConns      = 25
	defaultDBMaxIdleConns  = 5
	defaultDBConnLifetimeH = 1
)

// Config holds the application configuration.
type Config struct {
	Service  ServiceConfig  `yaml:"service"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServiceConfig holds service identity and runtime settings.
type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `env:"PIPELINE_PORT" yaml:"port"`
	Debug   bool   `env:"APP_DEBUG"     yaml:"debug"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host                  string        `env:"POSTGRES_PIPELINE_HOST"     yaml:"host"`
	Port                  int           `env:"POSTGRES_PIPELINE_PORT"     yaml:"port"`
	User                  string        `env:"POSTGRES_PIPELINE_USER"     yaml:"user"`
	Password              string        `env:"POSTGRES_PIPELINE_PASSWORD" yaml:"password"`
	Database              string        `env:"POSTGRES_PIPELINE_DB"       yaml:"database"`
	SSLMode               string        `yaml:"sslmode"`
	MaxConnections        int           `yaml:"max_connections"`
	MaxIdleConns          int           `yaml:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret string `env:"AUTH_JWT_SECRET" yaml:"jwt_secret"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
}

// Load loads configuration from a YAML file, applies defaults, then env overrides.
func Load(path string) (*Config, error) {
	cfg, loadErr := infraconfig.LoadWithDefaults(path, setDefaults)
	if loadErr != nil {
		return nil, fmt.Errorf("load config: %w", loadErr)
	}

	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, fmt.Errorf("validate config: %w", validateErr)
	}

	return cfg, nil
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if err := infraconfig.ValidatePort("service.port", c.Service.Port); err != nil {
		return err
	}

	if c.Database.Host == "" {
		return &infraconfig.ValidationError{Field: "database.host", Message: "is required"}
	}

	if c.Database.Database == "" {
		return &infraconfig.ValidationError{Field: "database.database", Message: "is required"}
	}

	return nil
}

// setDefaults applies default values to all configuration sections.
func setDefaults(cfg *Config) {
	setServiceDefaults(&cfg.Service)
	setDatabaseDefaults(&cfg.Database)
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
		d.ConnectionMaxLifetime = defaultDBConnLifetimeH * time.Hour
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
