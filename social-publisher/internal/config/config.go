package config

import (
	"fmt"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Config holds all configuration for the social-publisher service.
type Config struct {
	Debug    bool           `env:"SOCIAL_PUBLISHER_DEBUG"  yaml:"debug"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Service  ServiceConfig  `yaml:"service"`
	Auth     AuthConfig     `yaml:"auth"`
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Address      string `env:"SOCIAL_PUBLISHER_ADDRESS"       yaml:"address"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string `env:"POSTGRES_SOCIAL_PUBLISHER_HOST"     yaml:"host"`
	Port     int    `env:"POSTGRES_SOCIAL_PUBLISHER_PORT"     yaml:"port"`
	User     string `env:"POSTGRES_SOCIAL_PUBLISHER_USER"     yaml:"user"`
	Password string `env:"POSTGRES_SOCIAL_PUBLISHER_PASSWORD" yaml:"password"`
	DBName   string `env:"POSTGRES_SOCIAL_PUBLISHER_DB"       yaml:"db"`
	SSLMode  string `env:"POSTGRES_SOCIAL_PUBLISHER_SSL_MODE" yaml:"ssl_mode"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL      string `env:"REDIS_ADDR"     yaml:"url"`
	Password string `env:"REDIS_PASSWORD" yaml:"password"`
}

// ServiceConfig holds operational parameters.
type ServiceConfig struct {
	RetryInterval    string `env:"SOCIAL_PUBLISHER_RETRY_INTERVAL"    yaml:"retry_interval"`
	ScheduleInterval string `env:"SOCIAL_PUBLISHER_SCHEDULE_INTERVAL" yaml:"schedule_interval"`
	MaxRetries       int    `env:"SOCIAL_PUBLISHER_MAX_RETRIES"       yaml:"max_retries"`
	BatchSize        int    `env:"SOCIAL_PUBLISHER_BATCH_SIZE"        yaml:"batch_size"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret string `env:"SOCIAL_PUBLISHER_JWT_SECRET" yaml:"jwt_secret"`
}

// Load reads configuration from a YAML file with environment variable overrides.
func Load(path string) (*Config, error) {
	cfg, err := infraconfig.LoadWithDefaults(path, SetDefaults)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

// SetDefaults applies default values to the config.
func SetDefaults(cfg *Config) {
	if cfg.Server.Address == "" {
		cfg.Server.Address = ":8077"
	}
	if cfg.Server.ReadTimeout == "" {
		cfg.Server.ReadTimeout = "10s"
	}
	if cfg.Server.WriteTimeout == "" {
		cfg.Server.WriteTimeout = "30s"
	}
	if cfg.Service.RetryInterval == "" {
		cfg.Service.RetryInterval = "30s"
	}
	if cfg.Service.ScheduleInterval == "" {
		cfg.Service.ScheduleInterval = "60s"
	}
	if cfg.Service.MaxRetries == 0 {
		cfg.Service.MaxRetries = 3
	}
	if cfg.Service.BatchSize == 0 {
		cfg.Service.BatchSize = 50
	}
}

// Validate checks that required configuration fields are present.
func (c *Config) Validate() error {
	if c.Database.Host == "" || c.Database.DBName == "" {
		return fmt.Errorf("database configuration is required")
	}
	if c.Redis.URL == "" {
		return fmt.Errorf("redis URL is required")
	}
	return nil
}
