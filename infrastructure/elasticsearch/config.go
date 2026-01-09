package elasticsearch

import (
	"time"

	"github.com/north-cloud/infrastructure/retry"
)

// Config holds Elasticsearch client configuration
type Config struct {
	// URL is the Elasticsearch server URL (e.g., http://elasticsearch:9200)
	URL string

	// Username is the optional basic auth username
	Username string

	// Password is the optional basic auth password
	Password string

	// APIKey is the optional API key for authentication
	APIKey string

	// CloudID is the optional Elastic Cloud ID
	CloudID string

	// CloudAPIKey is the optional Elastic Cloud API key
	CloudAPIKey string

	// TLS configuration for secure connections
	TLS *TLSConfig

	// MaxRetries is the maximum number of retries for client operations (default: 3)
	MaxRetries int

	// PingTimeout is the timeout for ping verification (default: 5s)
	PingTimeout time.Duration

	// RetryConfig is the configuration for connection retry logic
	// If nil, default retry config will be used (5 attempts, 2s initial, 10s max)
	RetryConfig *retry.Config
}

// TLSConfig holds TLS configuration for Elasticsearch connections
type TLSConfig struct {
	// Enabled enables TLS
	Enabled bool

	// InsecureSkipVerify skips certificate verification (for development/testing)
	InsecureSkipVerify bool

	// CertFile is the path to the client certificate file
	CertFile string

	// KeyFile is the path to the client private key file
	KeyFile string

	// CAFile is the path to the CA certificate file
	CAFile string
}

// Default retry configuration constants for Elasticsearch connection
// These are used across all services for consistent retry behavior
const (
	// DefaultMaxRetries is the default number of retries for ES client operations
	DefaultMaxRetries = 3

	// DefaultPingTimeout is the default timeout for ping verification
	DefaultPingTimeout = 5 * time.Second

	// DefaultRetryMaxAttempts is the default max connection attempts during startup
	// Set high enough to handle slow ES startup in containers
	DefaultRetryMaxAttempts = 10

	// DefaultRetryInitialDelay is the initial delay between retry attempts
	DefaultRetryInitialDelay = 3 * time.Second

	// DefaultRetryMaxDelay is the maximum delay between retry attempts
	DefaultRetryMaxDelay = 15 * time.Second

	// DefaultRetryMultiplier is the exponential backoff multiplier
	DefaultRetryMultiplier = 2.0
)

// SetDefaults applies default values to the config if not set
func (c *Config) SetDefaults() {
	if c.URL == "" {
		c.URL = "http://localhost:9200"
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = DefaultMaxRetries
	}
	if c.PingTimeout == 0 {
		c.PingTimeout = DefaultPingTimeout
	}
	if c.RetryConfig == nil {
		c.RetryConfig = &retry.Config{
			MaxAttempts:  DefaultRetryMaxAttempts,
			InitialDelay: DefaultRetryInitialDelay,
			MaxDelay:     DefaultRetryMaxDelay,
			Multiplier:   DefaultRetryMultiplier,
		}
	}
}
