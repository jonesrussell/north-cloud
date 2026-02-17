package fetcher

import "time"

// Default configuration values.
const (
	defaultWorkerCount     = 4
	defaultUserAgent       = "NorthCloud-Fetcher/1.0"
	defaultRequestTimeout  = 30 * time.Second
	defaultClaimRetryDelay = 5 * time.Second
	defaultMaxRetries      = 3
)

// Config holds fetcher worker configuration.
type Config struct {
	DatabaseURL      string        `env:"FETCHER_DATABASE_URL"       yaml:"database_url"`
	ElasticsearchURL string        `env:"FETCHER_ELASTICSEARCH_URL"  yaml:"elasticsearch_url"`
	SourceManagerURL string        `env:"FETCHER_SOURCE_MANAGER_URL" yaml:"source_manager_url"`
	WorkerCount      int           `env:"FETCHER_WORKER_COUNT"       yaml:"worker_count"`
	UserAgent        string        `env:"FETCHER_USER_AGENT"         yaml:"user_agent"`
	RequestTimeout   time.Duration `env:"FETCHER_REQUEST_TIMEOUT"    yaml:"request_timeout"`
	ClaimRetryDelay  time.Duration `env:"FETCHER_CLAIM_RETRY_DELAY"  yaml:"claim_retry_delay"`
	MaxRetries       int           `env:"FETCHER_MAX_RETRIES"        yaml:"max_retries"`
	LogLevel         string        `env:"FETCHER_LOG_LEVEL"          yaml:"log_level"`
}

// WithDefaults returns a copy of the config with default values applied for zero-value fields.
func (c Config) WithDefaults() Config {
	if c.WorkerCount <= 0 {
		c.WorkerCount = defaultWorkerCount
	}
	if c.UserAgent == "" {
		c.UserAgent = defaultUserAgent
	}
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = defaultRequestTimeout
	}
	if c.ClaimRetryDelay <= 0 {
		c.ClaimRetryDelay = defaultClaimRetryDelay
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = defaultMaxRetries
	}
	return c
}
