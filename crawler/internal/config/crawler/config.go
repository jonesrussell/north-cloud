// Package crawler provides configuration management for the web crawler component.
// It handles loading, validation, and access to crawler-specific settings
// such as crawl depth, concurrency, rate limiting, and TLS configuration.
package crawler

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Default configuration values
const (
	DefaultRateLimit   = 1 * time.Second
	DefaultParallelism = 5
	DefaultUserAgent   = "crawler/1.0"
	DefaultTimeout     = 30 * time.Second
	DefaultMaxBodySize = 10 * 1024 * 1024 // 10MB
	DefaultRandomDelay = 500 * time.Millisecond
	DefaultMaxRetries  = 3
	DefaultRetryDelay  = 1 * time.Second
	// DefaultMaxRedirects is the default maximum number of redirects to follow
	DefaultMaxRedirects = 5
	// DefaultCleanupInterval is the default interval for cleanup operations
	DefaultCleanupInterval = 24 * time.Hour
	// DefaultHTTPRetryMax is the default number of HTTP retries in OnError
	DefaultHTTPRetryMax = 2
	// DefaultHTTPRetryDelay is the default delay between HTTP retries
	DefaultHTTPRetryDelay = 2 * time.Second
	// DefaultRedisStorageExpires is the default TTL for Colly visited URL keys in Redis
	DefaultRedisStorageExpires = 168 * time.Hour // 7 days
)

// Config represents the crawler configuration.
type Config struct {
	// MaxConcurrency is the maximum number of concurrent requests
	MaxConcurrency int `env:"CRAWLER_MAX_CONCURRENCY" yaml:"max_concurrency"`
	// RequestTimeout is the timeout for each request
	RequestTimeout time.Duration `env:"CRAWLER_REQUEST_TIMEOUT" yaml:"request_timeout"`
	// UserAgent is the user agent to use for requests
	UserAgent string `env:"CRAWLER_USER_AGENT" yaml:"user_agent"`
	// RespectRobotsTxt indicates whether to respect robots.txt
	RespectRobotsTxt bool `env:"CRAWLER_RESPECT_ROBOTS_TXT" yaml:"respect_robots_txt"`
	// AllowedDomains is the list of domains to crawl
	AllowedDomains []string `env:"CRAWLER_ALLOWED_DOMAINS" yaml:"allowed_domains"`
	// DisallowedDomains is the list of domains to exclude
	DisallowedDomains []string `env:"CRAWLER_DISALLOWED_DOMAINS" yaml:"disallowed_domains"`
	// Delay is the delay between requests
	Delay time.Duration `env:"CRAWLER_DELAY" yaml:"delay"`
	// RandomDelay is the random delay to add to the base delay
	RandomDelay time.Duration `env:"CRAWLER_RANDOM_DELAY" yaml:"random_delay"`
	// SourcesAPIURL is the URL of the source-manager API service
	SourcesAPIURL string `env:"CRAWLER_SOURCES_API_URL" yaml:"sources_api_url"`
	// Debug enables debug logging
	Debug bool `env:"APP_DEBUG" yaml:"debug"`
	// TLS contains TLS configuration
	TLS TLSConfig `yaml:"tls"`
	// MaxRetries is the maximum number of retries for failed requests
	MaxRetries int `env:"CRAWLER_MAX_RETRIES" yaml:"max_retries"`
	// RetryDelay is the delay between retries
	RetryDelay time.Duration `env:"CRAWLER_RETRY_DELAY" yaml:"retry_delay"`
	// FollowRedirects indicates whether to follow redirects
	FollowRedirects bool `env:"CRAWLER_FOLLOW_REDIRECTS" yaml:"follow_redirects"`
	// MaxRedirects is the maximum number of redirects to follow
	MaxRedirects int `env:"CRAWLER_MAX_REDIRECTS" yaml:"max_redirects"`
	// ValidateURLs indicates whether to validate URLs before visiting
	ValidateURLs bool `env:"CRAWLER_VALIDATE_URLS" yaml:"validate_urls"`
	// CleanupInterval is the interval for cleaning up resources
	CleanupInterval time.Duration `env:"CRAWLER_CLEANUP_INTERVAL" yaml:"cleanup_interval"`
	// SaveDiscoveredLinks indicates whether to save discovered links to database for later processing
	SaveDiscoveredLinks bool `env:"CRAWLER_SAVE_DISCOVERED_LINKS" yaml:"save_discovered_links"`
	// UseRandomUserAgent enables RandomUserAgent extension (overrides UserAgent when true)
	UseRandomUserAgent bool `env:"CRAWLER_USE_RANDOM_USER_AGENT" yaml:"use_random_user_agent"`
	// UseReferer enables Referer extension
	UseReferer bool `env:"CRAWLER_USE_REFERER" yaml:"use_referer"`
	// MaxURLLength filters out URLs longer than this (0 = no filter)
	MaxURLLength int `env:"CRAWLER_MAX_URL_LENGTH" yaml:"max_url_length"`
	// MaxRequests caps total requests per crawl (0 = no limit)
	MaxRequests uint32 `env:"CRAWLER_MAX_REQUESTS" yaml:"max_requests"`
	// DetectCharset enables character encoding detection for non-UTF-8 responses
	DetectCharset bool `env:"CRAWLER_DETECT_CHARSET" yaml:"detect_charset"`
	// TraceHTTP enables HTTP trace collection on Response.Trace
	TraceHTTP bool `env:"CRAWLER_TRACE_HTTP" yaml:"trace_http"`
	// MaxBodySize is the maximum response body size in bytes (0 = use Colly default)
	MaxBodySize int `env:"CRAWLER_MAX_BODY_SIZE" yaml:"max_body_size"`
	// HTTPRetryMax is the maximum HTTP retries in OnError for transient failures
	HTTPRetryMax int `env:"CRAWLER_HTTP_RETRY_MAX" yaml:"http_retry_max"`
	// HTTPRetryDelay is the delay between HTTP retries in OnError
	HTTPRetryDelay time.Duration `env:"CRAWLER_HTTP_RETRY_DELAY" yaml:"http_retry_delay"`
	// RedisStorageEnabled enables Redis-backed Colly storage for visited URLs, cookies, and request queue
	RedisStorageEnabled bool `env:"CRAWLER_REDIS_STORAGE_ENABLED" yaml:"redis_storage_enabled"`
	// RedisStorageExpires is the TTL for visited URL keys in Redis (0 = no expiry)
	RedisStorageExpires time.Duration `env:"CRAWLER_REDIS_STORAGE_EXPIRES" yaml:"redis_storage_expires"`
	// ProxiesEnabled enables round-robin proxy rotation for requests
	ProxiesEnabled bool `env:"CRAWLER_PROXIES_ENABLED" yaml:"proxies_enabled"`
	// ProxyURLs is the list of proxy URLs (HTTP or SOCKS5) for round-robin rotation
	ProxyURLs []string `env:"CRAWLER_PROXY_URLS" yaml:"proxy_urls"`
}

// Validate validates the crawler configuration.
func (c *Config) Validate() error {
	if c.MaxConcurrency < 1 {
		return errors.New("max_concurrency must be positive")
	}
	if c.RequestTimeout < 0 {
		return errors.New("request_timeout must be non-negative")
	}
	if c.Delay < 0 {
		return errors.New("delay must be non-negative")
	}
	if c.RandomDelay < 0 {
		return errors.New("random_delay must be non-negative")
	}
	if c.CleanupInterval <= 0 {
		return errors.New("cleanup_interval must be positive")
	}
	if c.MaxURLLength < 0 {
		return errors.New("max_url_length must be non-negative")
	}
	if c.HTTPRetryMax < 0 {
		return errors.New("http_retry_max must be non-negative")
	}
	if c.MaxBodySize < 0 {
		return errors.New("max_body_size must be non-negative")
	}
	return c.TLS.Validate()
}

// New creates a new crawler configuration with the given options.
func New(opts ...Option) *Config {
	cfg := &Config{
		MaxConcurrency:    DefaultParallelism,
		RequestTimeout:    DefaultTimeout,
		Delay:             DefaultRateLimit,
		RandomDelay:       DefaultRandomDelay,
		UserAgent:         DefaultUserAgent,
		RespectRobotsTxt:  true,
		AllowedDomains:    []string{"*"},
		DisallowedDomains: []string{},
		SourcesAPIURL:     "http://localhost:8050/api/v1/sources",
		Debug:             false,
		TLS: TLSConfig{
			InsecureSkipVerify:       false, // Default to secure TLS verification
			MinVersion:               tls.VersionTLS12,
			MaxVersion:               0, // Use highest supported version
			PreferServerCipherSuites: true,
		},
		MaxRetries:          DefaultMaxRetries,
		RetryDelay:          DefaultRetryDelay,
		FollowRedirects:     true,
		MaxRedirects:        DefaultMaxRedirects,
		ValidateURLs:        true,
		CleanupInterval:     DefaultCleanupInterval,
		SaveDiscoveredLinks: false,
		UseRandomUserAgent:  false,
		UseReferer:          true,
		MaxURLLength:        0,
		MaxRequests:         0,
		DetectCharset:       false,
		TraceHTTP:           false,
		MaxBodySize:         DefaultMaxBodySize,
		HTTPRetryMax:        DefaultHTTPRetryMax,
		HTTPRetryDelay:      DefaultHTTPRetryDelay,
		RedisStorageEnabled: false,
		RedisStorageExpires: DefaultRedisStorageExpires,
		ProxiesEnabled:      false,
		ProxyURLs:           nil,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// Option is a function that configures a crawler configuration.
type Option func(*Config)

// WithMaxConcurrency sets the maximum concurrency.
func WithMaxConcurrency(concurrency int) Option {
	return func(c *Config) {
		c.MaxConcurrency = concurrency
	}
}

// WithRequestTimeout sets the request timeout.
func WithRequestTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.RequestTimeout = timeout
	}
}

// WithUserAgent sets the user agent.
func WithUserAgent(agent string) Option {
	return func(c *Config) {
		c.UserAgent = agent
	}
}

// WithRespectRobotsTxt sets whether to respect robots.txt.
func WithRespectRobotsTxt(respect bool) Option {
	return func(c *Config) {
		c.RespectRobotsTxt = respect
	}
}

// WithAllowedDomains sets the allowed domains.
func WithAllowedDomains(domains []string) Option {
	return func(c *Config) {
		c.AllowedDomains = domains
	}
}

// WithDisallowedDomains sets the disallowed domains.
func WithDisallowedDomains(domains []string) Option {
	return func(c *Config) {
		c.DisallowedDomains = domains
	}
}

// WithDelay sets the delay between requests.
func WithDelay(delay time.Duration) Option {
	return func(c *Config) {
		c.Delay = delay
	}
}

// WithRandomDelay sets the random delay.
func WithRandomDelay(delay time.Duration) Option {
	return func(c *Config) {
		c.RandomDelay = delay
	}
}

// ParseRateLimit parses a rate limit string into a time.Duration.
// Accepts Go duration strings ("10s", "1m") or bare numbers as seconds ("10", "10.0").
func ParseRateLimit(limit string) (time.Duration, error) {
	limit = strings.TrimSpace(limit)
	if limit == "" {
		return 0, errors.New("rate limit cannot be empty")
	}

	duration, err := time.ParseDuration(limit)
	if err != nil {
		// Bare integer as seconds (e.g. "10")
		if n, parseErr := strconv.Atoi(limit); parseErr == nil && n > 0 {
			return time.Duration(n) * time.Second, nil
		}
		// Bare float as seconds (e.g. "10.0")
		if f, parseErr := strconv.ParseFloat(limit, 64); parseErr == nil && f > 0 {
			return time.Duration(f * float64(time.Second)), nil
		}
		return 0, fmt.Errorf("error parsing duration: %w", err)
	}

	if duration <= 0 {
		return 0, errors.New("rate limit must be positive")
	}

	return duration, nil
}

// TLSConfig holds TLS-related configuration settings.
type TLSConfig struct {
	// InsecureSkipVerify controls whether a client verifies the server's certificate chain and host name.
	// If InsecureSkipVerify is true, TLS accepts any certificate presented by the server
	// and any host name in that certificate.
	// In this mode, TLS is susceptible to man-in-the-middle attacks. This should be used
	// only for testing or with trusted sources.
	// Default: false
	InsecureSkipVerify bool `env:"CRAWLER_TLS_INSECURE_SKIP_VERIFY" yaml:"insecure_skip_verify"`

	// MinVersion is the minimum TLS version that is acceptable.
	// Default: TLS 1.2
	MinVersion uint16 `env:"CRAWLER_TLS_MIN_VERSION" yaml:"min_version"`

	// MaxVersion is the maximum TLS version that is acceptable.
	// If zero, the maximum version supported by this package is used, which is currently TLS 1.3.
	// Default: 0 (use highest supported version)
	MaxVersion uint16 `env:"CRAWLER_TLS_MAX_VERSION" yaml:"max_version"`

	// PreferServerCipherSuites controls whether the server's preference for cipher suites is honored.
	// If true, the server's preference is used. If false, the client's preference is used.
	// Default: true
	PreferServerCipherSuites bool `env:"CRAWLER_TLS_PREFER_SERVER_CIPHER_SUITES" yaml:"prefer_server_cipher_suites"`
}

// NewTLSConfig creates a new TLS configuration with secure defaults.
func NewTLSConfig() *TLSConfig {
	return &TLSConfig{
		InsecureSkipVerify:       false,
		MinVersion:               tls.VersionTLS12,
		MaxVersion:               0, // Use highest supported version
		PreferServerCipherSuites: true,
	}
}

// Validate validates the TLS configuration.
func (c *TLSConfig) Validate() error {
	if c.InsecureSkipVerify {
		return errors.New("insecure_skip_verify is enabled. This is not recommended for " +
			"production use as it makes HTTPS connections vulnerable to man-in-the-middle attacks")
	}
	return nil
}
