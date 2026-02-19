package config

import (
	"fmt"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Default configuration values.
const (
	defaultServiceName  = "click-tracker"
	defaultServicePort  = 8093
	defaultVersion      = "0.1.0"
	defaultSigLength    = 12
	defaultBufferSize   = 1000
	defaultFlushThresh  = 500
	defaultLoggingLevel = "info"
	defaultLoggingFmt   = "json"
	defaultDBHost       = "localhost"
	defaultDBPort       = 5432
	defaultDBName       = "click_tracker"
	defaultDBUser       = "postgres"
	defaultDBSSLMode    = "disable"

	defaultMaxClicksPerMinute = 10
	defaultWindowSeconds      = 60

	defaultMaxTimestampAgeH = 24
	defaultFlushIntervalS   = 1
)

// Config holds the application configuration.
type Config struct {
	Service   ServiceConfig   `yaml:"service"`
	Database  DatabaseConfig  `yaml:"database"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// ServiceConfig holds service-level configuration.
type ServiceConfig struct {
	Name            string        `yaml:"name"`
	Version         string        `yaml:"version"`
	Port            int           `env:"CLICK_TRACKER_PORT"   yaml:"port"`
	Debug           bool          `env:"APP_DEBUG"            yaml:"debug"`
	HMACSecret      string        `env:"CLICK_TRACKER_SECRET" yaml:"hmac_secret"`
	SignatureLength int           `yaml:"signature_length"`
	MaxTimestampAge time.Duration `yaml:"max_timestamp_age"`
	BufferSize      int           `yaml:"buffer_size"`
	FlushInterval   time.Duration `yaml:"flush_interval"`
	FlushThreshold  int           `yaml:"flush_threshold"`
}

// DatabaseConfig holds PostgreSQL database configuration.
type DatabaseConfig struct {
	Host     string `env:"POSTGRES_CLICK_TRACKER_HOST"     yaml:"host"`
	Port     int    `env:"POSTGRES_CLICK_TRACKER_PORT"     yaml:"port"`
	User     string `env:"POSTGRES_CLICK_TRACKER_USER"     yaml:"user"`
	Password string `env:"POSTGRES_CLICK_TRACKER_PASSWORD" yaml:"password"`
	Database string `env:"POSTGRES_CLICK_TRACKER_DB"       yaml:"database"`
	SSLMode  string `env:"POSTGRES_CLICK_TRACKER_SSLMODE"  yaml:"sslmode"`
}

// DSN returns the PostgreSQL connection string.
func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Database, d.SSLMode,
	)
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	MaxClicksPerMinute int `yaml:"max_clicks_per_minute"`
	WindowSeconds      int `yaml:"window_seconds"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
}

// Load loads configuration from the specified path.
func Load(path string) (*Config, error) {
	return infraconfig.LoadWithDefaults[Config](path, setDefaults)
}

// setDefaults applies default values to the config.
func setDefaults(cfg *Config) {
	setServiceDefaults(&cfg.Service)
	setDatabaseDefaults(&cfg.Database)
	setRateLimitDefaults(&cfg.RateLimit)
	setLoggingDefaults(&cfg.Logging)
}

// setServiceDefaults applies default values to ServiceConfig.
func setServiceDefaults(svc *ServiceConfig) {
	if svc.Name == "" {
		svc.Name = defaultServiceName
	}
	if svc.Version == "" {
		svc.Version = defaultVersion
	}
	if svc.Port == 0 {
		svc.Port = defaultServicePort
	}
	if svc.SignatureLength == 0 {
		svc.SignatureLength = defaultSigLength
	}
	if svc.MaxTimestampAge == 0 {
		svc.MaxTimestampAge = defaultMaxTimestampAgeH * time.Hour
	}
	if svc.BufferSize == 0 {
		svc.BufferSize = defaultBufferSize
	}
	if svc.FlushInterval == 0 {
		svc.FlushInterval = defaultFlushIntervalS * time.Second
	}
	if svc.FlushThreshold == 0 {
		svc.FlushThreshold = defaultFlushThresh
	}
}

// setDatabaseDefaults applies default values to DatabaseConfig.
func setDatabaseDefaults(db *DatabaseConfig) {
	if db.Host == "" {
		db.Host = defaultDBHost
	}
	if db.Port == 0 {
		db.Port = defaultDBPort
	}
	if db.User == "" {
		db.User = defaultDBUser
	}
	if db.Database == "" {
		db.Database = defaultDBName
	}
	if db.SSLMode == "" {
		db.SSLMode = defaultDBSSLMode
	}
}

// setRateLimitDefaults applies default values to RateLimitConfig.
func setRateLimitDefaults(rl *RateLimitConfig) {
	if rl.MaxClicksPerMinute == 0 {
		rl.MaxClicksPerMinute = defaultMaxClicksPerMinute
	}
	if rl.WindowSeconds == 0 {
		rl.WindowSeconds = defaultWindowSeconds
	}
}

// setLoggingDefaults applies default values to LoggingConfig.
func setLoggingDefaults(log *LoggingConfig) {
	if log.Level == "" {
		log.Level = defaultLoggingLevel
	}
	if log.Format == "" {
		log.Format = defaultLoggingFmt
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if err := infraconfig.ValidatePort("service.port", c.Service.Port); err != nil {
		return err
	}
	if c.Service.HMACSecret == "" {
		return &infraconfig.ValidationError{
			Field:   "service.hmac_secret",
			Message: "is required",
		}
	}
	return nil
}
