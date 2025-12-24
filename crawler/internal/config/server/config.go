// Package server provides server configuration types and functions.
package server

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Config represents server-specific configuration settings.
type Config struct {
	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration `yaml:"read_timeout"`
	// WriteTimeout is the maximum duration before timing out writes of the response
	WriteTimeout time.Duration `yaml:"write_timeout"`
	// IdleTimeout is the maximum amount of time to wait for the next request
	IdleTimeout time.Duration `yaml:"idle_timeout"`
	// SecurityEnabled determines if security features are enabled
	SecurityEnabled bool `yaml:"security_enabled"`
	// APIKey is the API key used for authentication
	APIKey string `yaml:"api_key"`
	// Address is the address to listen on (e.g., ":8080")
	Address string `yaml:"address"`
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.SecurityEnabled {
		if c.APIKey == "" {
			return errors.New("server security is enabled but no API key is provided")
		}

		// Validate API key format (expected format: "id:key")
		parts := strings.Split(c.APIKey, ":")
		if len(parts) != 2 {
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
