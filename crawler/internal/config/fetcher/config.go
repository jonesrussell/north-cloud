// Package fetcher provides configuration types for the frontier fetcher worker pool.
package fetcher

import "time"

// Default configuration values.
const (
	DefaultWorkerCount        = 16
	DefaultUserAgent          = "NorthCloud-Fetcher/1.0"
	DefaultRequestTimeout     = 30 * time.Second
	DefaultClaimRetryDelay    = 5 * time.Second
	DefaultMaxRetries         = 3
	DefaultMaxRedirects       = 5
	DefaultStaleTimeout       = 10 * time.Minute
	DefaultStaleCheckInterval = 2 * time.Minute
)

// Config holds fetcher worker configuration.
type Config struct {
	Enabled            bool          `env:"FETCHER_ENABLED"              yaml:"enabled"`
	DatabaseURL        string        `env:"FETCHER_DATABASE_URL"         yaml:"database_url"`
	ElasticsearchURL   string        `env:"FETCHER_ELASTICSEARCH_URL"    yaml:"elasticsearch_url"`
	SourceManagerURL   string        `env:"FETCHER_SOURCE_MANAGER_URL"   yaml:"source_manager_url"`
	WorkerCount        int           `env:"FETCHER_WORKER_COUNT"         yaml:"worker_count"`
	UserAgent          string        `env:"FETCHER_USER_AGENT"           yaml:"user_agent"`
	RequestTimeout     time.Duration `env:"FETCHER_REQUEST_TIMEOUT"      yaml:"request_timeout"`
	ClaimRetryDelay    time.Duration `env:"FETCHER_CLAIM_RETRY_DELAY"    yaml:"claim_retry_delay"`
	MaxRetries         int           `env:"FETCHER_MAX_RETRIES"          yaml:"max_retries"`
	FollowRedirects    *bool         `env:"FETCHER_FOLLOW_REDIRECTS"     yaml:"follow_redirects"`
	MaxRedirects       int           `env:"FETCHER_MAX_REDIRECTS"        yaml:"max_redirects"`
	StaleTimeout       time.Duration `env:"FETCHER_STALE_TIMEOUT"        yaml:"stale_timeout"`
	StaleCheckInterval time.Duration `env:"FETCHER_STALE_CHECK_INTERVAL" yaml:"stale_check_interval"`
	LogLevel           string        `env:"FETCHER_LOG_LEVEL"            yaml:"log_level"`
}

// WithDefaults returns a copy of the config with default values applied for zero-value fields.
func (c Config) WithDefaults() Config {
	if c.WorkerCount <= 0 {
		c.WorkerCount = DefaultWorkerCount
	}
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent
	}
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = DefaultRequestTimeout
	}
	if c.ClaimRetryDelay <= 0 {
		c.ClaimRetryDelay = DefaultClaimRetryDelay
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = DefaultMaxRetries
	}
	if c.MaxRedirects <= 0 {
		c.MaxRedirects = DefaultMaxRedirects
	}
	if c.StaleTimeout <= 0 {
		c.StaleTimeout = DefaultStaleTimeout
	}
	if c.StaleCheckInterval <= 0 {
		c.StaleCheckInterval = DefaultStaleCheckInterval
	}
	// Default FollowRedirects to true when unset (nil) so redirects are followed by default.
	if c.FollowRedirects == nil {
		t := true
		c.FollowRedirects = &t
	}
	return c
}
