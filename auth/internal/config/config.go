package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultServerPort      = 8040
	defaultServerTimeout   = 30
	defaultDatabasePort    = 5432
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 5
	defaultJWTExpiry      = 24 // hours
)

type Config struct {
	Debug    bool           `yaml:"debug"`
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	JWT      JWTConfig      `yaml:"jwt"`
}

type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	DBName          string        `yaml:"dbname"`
	SSLMode         string        `yaml:"sslmode"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type JWTConfig struct {
	Secret     string        `yaml:"secret"`
	Expiry     time.Duration `yaml:"expiry"`
	RefreshExpiry time.Duration `yaml:"refresh_expiry"`
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
	if c.JWT.Secret == "" {
		return errors.New("jwt.secret is required")
	}
	return nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Set defaults
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
	if cfg.JWT.Expiry == 0 {
		cfg.JWT.Expiry = defaultJWTExpiry * time.Hour
	}
	if cfg.JWT.RefreshExpiry == 0 {
		cfg.JWT.RefreshExpiry = 7 * 24 * time.Hour // 7 days
	}

	// Override with environment variables
	overrideFromEnv(&cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func overrideFromEnv(cfg *Config) {
	if dbHost := os.Getenv("POSTGRES_AUTH_HOST"); dbHost != "" {
		cfg.Database.Host = dbHost
	}
	if dbPort := os.Getenv("POSTGRES_AUTH_PORT"); dbPort != "" {
		if port, err := strconv.Atoi(dbPort); err == nil {
			cfg.Database.Port = port
		}
	}
	if dbUser := os.Getenv("POSTGRES_AUTH_USER"); dbUser != "" {
		cfg.Database.User = dbUser
	}
	if dbPassword := os.Getenv("POSTGRES_AUTH_PASSWORD"); dbPassword != "" {
		cfg.Database.Password = dbPassword
	}
	if dbName := os.Getenv("POSTGRES_AUTH_DB"); dbName != "" {
		cfg.Database.DBName = dbName
	}
	if dbSSLMode := os.Getenv("POSTGRES_AUTH_SSLMODE"); dbSSLMode != "" {
		cfg.Database.SSLMode = dbSSLMode
	}
	if serverHost := os.Getenv("AUTH_SERVICE_HOST"); serverHost != "" {
		cfg.Server.Host = serverHost
	}
	if serverPort := os.Getenv("AUTH_SERVICE_PORT"); serverPort != "" {
		if port, err := strconv.Atoi(serverPort); err == nil {
			cfg.Server.Port = port
		}
	}
	if appDebug := os.Getenv("APP_DEBUG"); appDebug != "" {
		cfg.Debug = parseBool(appDebug)
	}
	if jwtSecret := os.Getenv("AUTH_JWT_SECRET"); jwtSecret != "" {
		cfg.JWT.Secret = jwtSecret
	}
	if jwtExpiry := os.Getenv("AUTH_JWT_EXPIRY"); jwtExpiry != "" {
		if duration, err := time.ParseDuration(jwtExpiry); err == nil {
			cfg.JWT.Expiry = duration
		}
	}
	if refreshExpiry := os.Getenv("AUTH_JWT_REFRESH_EXPIRY"); refreshExpiry != "" {
		if duration, err := time.ParseDuration(refreshExpiry); err == nil {
			cfg.JWT.RefreshExpiry = duration
		}
	}
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}

