package config

import (
	"os"
	"time"
)

// Config holds the application configuration
type Config struct {
	Username  string
	Password  string
	JWTSecret string
	Port      string
	Debug     bool
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		Username:  getEnv("AUTH_USERNAME", "admin"),
		Password:  getEnv("AUTH_PASSWORD", "admin"),
		JWTSecret: getEnv("AUTH_JWT_SECRET", "change-me-in-production"),
		Port:      getEnv("AUTH_PORT", "8040"),
		Debug:     getEnv("APP_DEBUG", "false") == "true",
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Username == "" {
		return &ConfigError{Field: "AUTH_USERNAME", Message: "username is required"}
	}
	if c.Password == "" {
		return &ConfigError{Field: "AUTH_PASSWORD", Message: "password is required"}
	}
	// Only validate JWT secret in production mode (when APP_DEBUG is false)
	// In development, allow default for easier setup
	if !c.Debug && (c.JWTSecret == "" || c.JWTSecret == "change-me-in-production" || c.JWTSecret == "change-me-in-production-generate-strong-secret") {
		return &ConfigError{Field: "AUTH_JWT_SECRET", Message: "JWT secret must be set and not use default value in production"}
	}
	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// JWTConfig holds JWT-specific configuration
type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

// GetJWTConfig returns JWT configuration
func (c *Config) GetJWTConfig() *JWTConfig {
	return &JWTConfig{
		Secret:     c.JWTSecret,
		Expiration: 24 * time.Hour, // 24 hours
	}
}

