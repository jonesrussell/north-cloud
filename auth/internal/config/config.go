package config

import (
	"strconv"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Default configuration values.
const (
	defaultServiceName    = "auth"
	defaultServicePort    = 8040
	defaultUsername       = "admin"
	defaultPassword       = "admin"
	defaultJWTSecret      = "change-me-in-production"
	defaultJWTExpirationH = 24
	defaultLoggingLevel   = "info"
	defaultLoggingFormat  = "json"
)

// Config holds the application configuration.
type Config struct {
	Service ServiceConfig `yaml:"service"`
	Auth    AuthConfig    `yaml:"auth"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServiceConfig holds service-level configuration.
type ServiceConfig struct {
	Name  string `yaml:"name"`
	Port  int    `env:"AUTH_PORT" yaml:"port"`
	Debug bool   `env:"APP_DEBUG" yaml:"debug"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Username      string        `env:"AUTH_USERNAME"   yaml:"username"`
	Password      string        `env:"AUTH_PASSWORD"   yaml:"password"`
	JWTSecret     string        `env:"AUTH_JWT_SECRET" yaml:"jwt_secret"`
	JWTExpiration time.Duration `yaml:"jwt_expiration"`
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
	if cfg.Service.Name == "" {
		cfg.Service.Name = defaultServiceName
	}
	if cfg.Service.Port == 0 {
		cfg.Service.Port = defaultServicePort
	}
	if cfg.Auth.Username == "" {
		cfg.Auth.Username = defaultUsername
	}
	if cfg.Auth.Password == "" {
		cfg.Auth.Password = defaultPassword
	}
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = defaultJWTSecret
	}
	if cfg.Auth.JWTExpiration == 0 {
		cfg.Auth.JWTExpiration = defaultJWTExpirationH * time.Hour
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = defaultLoggingLevel
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = defaultLoggingFormat
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Auth.Username == "" {
		return &infraconfig.ValidationError{Field: "auth.username", Message: "is required"}
	}
	if c.Auth.Password == "" {
		return &infraconfig.ValidationError{Field: "auth.password", Message: "is required"}
	}
	// Only validate JWT secret in production mode
	if !c.Service.Debug && (c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "change-me-in-production") {
		return &infraconfig.ValidationError{
			Field:   "auth.jwt_secret",
			Message: "must be set and not use default value in production",
		}
	}
	if err := infraconfig.ValidatePort("service.port", c.Service.Port); err != nil {
		return err
	}
	return nil
}

// JWTConfig returns JWT-specific configuration.
type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

// GetJWTConfig returns JWT configuration.
func (c *Config) GetJWTConfig() *JWTConfig {
	return &JWTConfig{
		Secret:     c.Auth.JWTSecret,
		Expiration: c.Auth.JWTExpiration,
	}
}

// Address returns the server address.
func (c *Config) Address() string {
	return ":" + formatPort(c.Service.Port)
}

func formatPort(port int) string {
	return strconv.Itoa(port)
}
