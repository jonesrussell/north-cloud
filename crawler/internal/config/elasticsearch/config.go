// Package elasticsearch provides Elasticsearch configuration management.
package elasticsearch

import (
	"fmt"
	"strings"
	"time"
)

// ValidationLevel represents the level of configuration validation required
type ValidationLevel int

const (
	// BasicValidation only validates essential connection settings
	BasicValidation ValidationLevel = iota
	// FullValidation performs complete configuration validation
	FullValidation
)

// Default configuration values
const (
	DefaultAddresses     = "http://127.0.0.1:9200"
	DefaultIndexName     = "crawler"
	DefaultRetryEnabled  = true
	DefaultInitialWait   = 1 * time.Second
	DefaultMaxWait       = 5 * time.Second
	DefaultMaxRetries    = 3
	DefaultBulkSize      = 1000
	DefaultFlushInterval = 30 * time.Second
	MinPasswordLength    = 8
	DefaultDiscoverNodes = false // Default to false to prevent node discovery
	DefaultUsername      = "elastic"
	DefaultPassword      = "changeme"
	DefaultMaxSize       = 1024 * 1024 * 1024 // 1 GB
	DefaultMaxItems      = 10000

	// Retry configuration constants
	HAInitialWait = 2 * time.Second
	HAMaxWait     = 10 * time.Second
	HAMaxRetries  = 5
)

// Error codes for configuration validation
const (
	ErrCodeEmptyAddresses  = "EMPTY_ADDRESSES"
	ErrCodeEmptyIndexName  = "EMPTY_INDEX_NAME"
	ErrCodeMissingAPIKey   = "MISSING_API_KEY"
	ErrCodeInvalidFormat   = "INVALID_FORMAT"
	ErrCodeWeakPassword    = "WEAK_PASSWORD"
	ErrCodeInvalidRetry    = "INVALID_RETRY"
	ErrCodeInvalidBulkSize = "INVALID_BULK_SIZE"
	ErrCodeInvalidFlush    = "INVALID_FLUSH"
	ErrCodeInvalidTLS      = "INVALID_TLS"
)

// ConfigError represents a configuration validation error
type ConfigError struct {
	Code    string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Config represents Elasticsearch configuration settings.
type Config struct {
	// Addresses is a list of Elasticsearch node addresses
	// Supports both ELASTICSEARCH_HOSTS and ELASTICSEARCH_ADDRESSES (comma-separated)
	Addresses []string `env:"ELASTICSEARCH_ADDRESSES" yaml:"addresses"`
	// APIKey is the base64 encoded API key for authentication
	APIKey string `env:"ELASTICSEARCH_API_KEY" yaml:"api_key"`
	// Username is the username for authentication
	Username string `env:"ELASTICSEARCH_USERNAME" yaml:"username"`
	// Password is the password for authentication (minimum 8 characters)
	Password string `env:"ELASTICSEARCH_PASSWORD" yaml:"password"`
	// IndexName is the name of the index
	// Supports both ELASTICSEARCH_INDEX_PREFIX and ELASTICSEARCH_INDEX_NAME
	IndexName string `env:"ELASTICSEARCH_INDEX_NAME" yaml:"index_name"`
	// Cloud contains cloud-specific configuration
	Cloud struct {
		ID     string `yaml:"id"`
		APIKey string `yaml:"api_key"`
	} `yaml:"cloud"`
	// TLS contains TLS configuration
	TLS *TLSConfig `yaml:"tls"`
	// Retry contains retry configuration
	Retry struct {
		Enabled     bool          `env:"ELASTICSEARCH_RETRY_ENABLED"      yaml:"enabled"`
		InitialWait time.Duration `env:"ELASTICSEARCH_RETRY_INITIAL_WAIT" yaml:"initial_wait"`
		MaxWait     time.Duration `env:"ELASTICSEARCH_RETRY_MAX_WAIT"     yaml:"max_wait"`
		MaxRetries  int           `env:"ELASTICSEARCH_MAX_RETRIES"        yaml:"max_retries"`
	} `yaml:"retry"`
	// BulkSize is the number of documents to bulk index
	BulkSize int `env:"ELASTICSEARCH_BULK_SIZE" yaml:"bulk_size"`
	// FlushInterval is the interval at which to flush the bulk indexer
	FlushInterval time.Duration `env:"ELASTICSEARCH_FLUSH_INTERVAL" yaml:"flush_interval"`
	// DiscoverNodes enables/disables node discovery
	DiscoverNodes bool `env:"ELASTICSEARCH_DISCOVER_NODES" yaml:"discover_nodes"`
	// MaxSize is the maximum size of the storage in bytes
	MaxSize int64 `yaml:"max_size"`
	// MaxItems is the maximum number of items to store
	MaxItems int `yaml:"max_items"`
	// Compression is whether to compress stored data
	Compression bool `yaml:"compression"`
}

// TLSConfig represents TLS configuration settings.
type TLSConfig struct {
	CertFile           string `env:"ELASTICSEARCH_TLS_CERT_FILE"            yaml:"cert_file"`
	KeyFile            string `env:"ELASTICSEARCH_TLS_KEY_FILE"             yaml:"key_file"`
	CAFile             string `env:"ELASTICSEARCH_TLS_CA_FILE"              yaml:"ca_file"`
	InsecureSkipVerify bool   `env:"ELASTICSEARCH_TLS_INSECURE_SKIP_VERIFY" yaml:"insecure_skip_verify"`
	Enabled            bool   `env:"ELASTICSEARCH_TLS_ENABLED"              yaml:"enabled"`
}

// validateTLS validates the TLS configuration
func (c *Config) validateTLS() error {
	if c.TLS != nil {
		if (c.TLS.CertFile != "" && c.TLS.KeyFile == "") || (c.TLS.CertFile == "" && c.TLS.KeyFile != "") {
			return &ConfigError{
				Code:    ErrCodeInvalidTLS,
				Message: "both cert file and key file must be provided for TLS",
			}
		}
	}
	return nil
}

// validateRequiredFields validates required configuration fields
func (c *Config) validateRequiredFields() error {
	if len(c.Addresses) == 0 {
		return &ConfigError{
			Code:    ErrCodeEmptyAddresses,
			Message: "at least one address is required",
		}
	}

	if c.IndexName == "" {
		return &ConfigError{
			Code:    ErrCodeEmptyIndexName,
			Message: "index name is required",
		}
	}

	// Allow either API key or username/password authentication
	// Skip auth requirement for localhost/development connections
	// (http://localhost, http://127.0.0.1, http://elasticsearch)
	if c.APIKey == "" && (c.Username == "" || c.Password == "") {
		// Check if any address is a localhost/development address without auth
		hasLocalDevAddress := false
		for _, addr := range c.Addresses {
			if addr == "http://localhost:9200" || addr == "http://127.0.0.1:9200" ||
				addr == "http://elasticsearch:9200" || addr == "http://elasticsearch" {
				hasLocalDevAddress = true
				break
			}
		}

		// Require auth only if not using localhost/development addresses
		if !hasLocalDevAddress {
			return &ConfigError{
				Code:    ErrCodeMissingAPIKey,
				Message: "either API key or username/password is required",
			}
		}
	}

	return nil
}

// validatePassword validates the password configuration
func (c *Config) validatePassword() error {
	if c.Password != "" && len(c.Password) < MinPasswordLength {
		return &ConfigError{
			Code:    ErrCodeWeakPassword,
			Message: fmt.Sprintf("password must be at least %d characters", MinPasswordLength),
		}
	}
	return nil
}

// validateRetry validates the retry configuration
func (c *Config) validateRetry() error {
	if c.Retry.Enabled {
		if c.Retry.InitialWait < 0 || c.Retry.MaxWait < 0 || c.Retry.MaxRetries < 0 {
			return &ConfigError{
				Code:    ErrCodeInvalidRetry,
				Message: "retry configuration must be non-negative",
			}
		}
	}
	return nil
}

// validateBulkConfig validates bulk indexing configuration
func (c *Config) validateBulkConfig() error {
	if c.FlushInterval <= 0 {
		return &ConfigError{
			Code:    ErrCodeInvalidFlush,
			Message: "flush interval must be positive",
		}
	}

	if c.BulkSize <= 0 {
		return &ConfigError{
			Code:    ErrCodeInvalidBulkSize,
			Message: "bulk size must be positive",
		}
	}

	return nil
}

// validateAPIKeyFormat validates the API key format
func (c *Config) validateAPIKeyFormat() error {
	if c.APIKey != "" {
		parts := strings.Split(c.APIKey, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return &ConfigError{
				Code:    ErrCodeInvalidFormat,
				Message: "API key must be in the format 'id:key'",
			}
		}
	}
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c == nil {
		return &ConfigError{
			Code:    ErrCodeEmptyAddresses,
			Message: "configuration is required",
		}
	}

	// Validate each component
	if err := c.validateTLS(); err != nil {
		return err
	}

	if err := c.validateRequiredFields(); err != nil {
		return err
	}

	if err := c.validatePassword(); err != nil {
		return err
	}

	if err := c.validateRetry(); err != nil {
		return err
	}

	if err := c.validateBulkConfig(); err != nil {
		return err
	}

	if err := c.validateAPIKeyFormat(); err != nil {
		return err
	}

	return nil
}

// NewConfig creates a new Config instance with default values.
func NewConfig() *Config {
	return &Config{
		Addresses: []string{DefaultAddresses},
		IndexName: DefaultIndexName,
		Retry: struct {
			Enabled     bool          `env:"ELASTICSEARCH_RETRY_ENABLED"      yaml:"enabled"`
			InitialWait time.Duration `env:"ELASTICSEARCH_RETRY_INITIAL_WAIT" yaml:"initial_wait"`
			MaxWait     time.Duration `env:"ELASTICSEARCH_RETRY_MAX_WAIT"     yaml:"max_wait"`
			MaxRetries  int           `env:"ELASTICSEARCH_MAX_RETRIES"        yaml:"max_retries"`
		}{
			Enabled:     DefaultRetryEnabled,
			InitialWait: DefaultInitialWait,
			MaxWait:     DefaultMaxWait,
			MaxRetries:  DefaultMaxRetries,
		},
		BulkSize:      DefaultBulkSize,
		FlushInterval: DefaultFlushInterval,
		TLS: &TLSConfig{
			Enabled:            true,
			InsecureSkipVerify: false,
		},
		MaxSize:       DefaultMaxSize,
		MaxItems:      DefaultMaxItems,
		Compression:   true,
		DiscoverNodes: DefaultDiscoverNodes,
	}
}

// ParseAddressesFromString parses comma-separated addresses from a string.
func ParseAddressesFromString(addrStr string) []string {
	addresses := strings.Split(addrStr, ",")
	// Trim whitespace from each address
	for i := range addresses {
		addresses[i] = strings.TrimSpace(addresses[i])
	}
	// Remove any empty strings after trimming
	filtered := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		if addr != "" {
			filtered = append(filtered, addr)
		}
	}
	return filtered
}
