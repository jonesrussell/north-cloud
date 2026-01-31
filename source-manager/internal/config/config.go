package config

import (
	"errors"
	"fmt"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

const (
	defaultServerPort      = 8050
	defaultServerTimeout   = 30
	defaultDatabasePort    = 5432
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 5
	defaultRedisAddress    = "localhost:6379"
	defaultRedisDB         = 0
)

type Config struct {
	Debug    bool           `env:"APP_DEBUG" yaml:"debug"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Redis    RedisConfig    `yaml:"redis"`
}

// RedisConfig holds Redis connection configuration for event publishing.
type RedisConfig struct {
	Address  string `env:"REDIS_ADDRESS"        yaml:"address"`
	Password string `env:"REDIS_PASSWORD"       yaml:"password"`
	DB       int    `env:"REDIS_DB"             yaml:"db"`
	Enabled  bool   `env:"REDIS_EVENTS_ENABLED" yaml:"enabled"` // Feature flag for event publishing
}

type ServerConfig struct {
	Host         string        `env:"SERVER_HOST"            yaml:"host"`
	Port         int           `env:"SERVER_PORT"            yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	APIURL       string        `env:"SOURCE_MANAGER_API_URL" yaml:"api_url"`
	CORSOrigins  []string      `env:"CORS_ORIGINS"           yaml:"cors_origins"`
}

type DatabaseConfig struct {
	Host            string        `env:"DB_HOST"            yaml:"host"`
	Port            int           `env:"DB_PORT"            yaml:"port"`
	User            string        `env:"DB_USER"            yaml:"user"`
	Password        string        `env:"DB_PASSWORD"        yaml:"password"`
	DBName          string        `env:"DB_NAME"            yaml:"dbname"`
	SSLMode         string        `env:"DB_SSLMODE"         yaml:"sslmode"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type AuthConfig struct {
	JWTSecret string `env:"AUTH_JWT_SECRET" yaml:"jwt_secret"`
}

func (c *Config) Validate() error {
	if c.Server.Host == "" {
		return errors.New("server.host is required")
	}
	if c.Server.Port <= 0 {
		return errors.New("server.port is required and must be positive")
	}
	if c.Database.Host == "" {
		return errors.New("database.host is required")
	}
	if c.Database.Port <= 0 {
		return errors.New("database.port is required and must be positive")
	}
	if c.Database.User == "" {
		return errors.New("database.user is required")
	}
	if c.Database.DBName == "" {
		return errors.New("database.dbname is required")
	}
	return nil
}

func Load(path string) (*Config, error) {
	cfg, err := infraconfig.LoadWithDefaults(path, setDefaults)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = defaultServerPort
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = defaultServerTimeout * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = defaultServerTimeout * time.Second
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = defaultDatabasePort
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = defaultMaxOpenConns
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = defaultMaxIdleConns
	}
	if cfg.Database.ConnMaxLifetime == 0 {
		cfg.Database.ConnMaxLifetime = defaultConnMaxLifetime * time.Minute
	}
	// Set default CORS origins if not provided
	if len(cfg.Server.CORSOrigins) == 0 {
		cfg.Server.CORSOrigins = []string{
			"http://localhost:3000", // Source manager frontend
			"http://localhost:3001", // Crawler frontend
			"http://localhost:3002", // Unified dashboard frontend
		}
	}
	// Set default Redis configuration
	if cfg.Redis.Address == "" {
		cfg.Redis.Address = defaultRedisAddress
	}
	if cfg.Redis.DB == 0 {
		cfg.Redis.DB = defaultRedisDB
	}
	// Note: cfg.Redis.Enabled defaults to false (feature flag)
}
