// Package server provides server configuration types and functions.
package server

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// APIKeyParts is the number of parts in an API key (id:key)
const APIKeyParts = 2

// Config represents server-specific configuration settings.
type Config struct {
	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration `env:"CRAWLER_SERVER_READ_TIMEOUT" yaml:"read_timeout"`
	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration `env:"CRAWLER_SERVER_WRITE_TIMEOUT" yaml:"write_timeout"`
	// IdleTimeout is the maximum amount of time to wait for the next request
	IdleTimeout time.Duration `env:"CRAWLER_SERVER_IDLE_TIMEOUT" yaml:"idle_timeout"`
	// SecurityEnabled determines if security features are enabled
	SecurityEnabled bool `env:"CRAWLER_SERVER_SECURITY_ENABLED" yaml:"security_enabled"`
	// APIKey is the API key used for authentication
	APIKey string `env:"CRAWLER_SERVER_API_KEY" json:"-" yaml:"api_key"`
	// Address is the address to listen on (e.g., ":8080")
	Address string `env:"CRAWLER_SERVER_ADDRESS" yaml:"address"`
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.SecurityEnabled {
		if c.APIKey == "" {
			return errors.New("server security is enabled but no API key is provided")
		}

		// Validate API key format (expected format: "id:key")
		parts := strings.Split(c.APIKey, ":")
		if len(parts) != APIKeyParts {
			return fmt.Errorf("invalid API key format: expected 'id:key' but got %q", c.APIKey)
		}

		// Validate API key parts
		id, key := parts[0], parts[1]
		if id == "" {
			return errors.New("API key ID cannot be empty")
		}
		if key == "" {
			return errors.New("API key value cannot be empty")
		}
	}

	return nil
}

// NewConfig creates a new Config instance with default values.
func NewConfig() *Config {
	return &Config{}
}
