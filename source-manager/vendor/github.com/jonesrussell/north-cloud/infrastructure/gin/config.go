// Package gin provides shared HTTP server utilities for Gin-based services.
// It ensures consistent CORS, logging, health endpoints, and server lifecycle
// management across all North Cloud microservices.
package gin

import (
	"time"
)

// Default timeout values for HTTP server configuration.
const (
	DefaultReadTimeout     = 30 * time.Second
	DefaultWriteTimeout    = 60 * time.Second
	DefaultIdleTimeout     = 120 * time.Second
	DefaultShutdownTimeout = 30 * time.Second
	DefaultCORSMaxAge      = 12 * time.Hour
)

// Config holds the HTTP server configuration.
type Config struct {
	// Port is the port number to listen on.
	Port int

	// Debug enables debug mode (verbose logging, Gin debug mode).
	Debug bool

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the next request.
	IdleTimeout time.Duration

	// ShutdownTimeout is the maximum duration to wait for active connections to close.
	ShutdownTimeout time.Duration

	// CORS holds the CORS configuration.
	CORS CORSConfig

	// ServiceName is the name of the service (used in health responses).
	ServiceName string

	// ServiceVersion is the version of the service (used in health responses).
	ServiceVersion string
}

// CORSConfig holds the CORS middleware configuration.
type CORSConfig struct {
	// Enabled determines whether CORS middleware is applied.
	Enabled bool

	// AllowedOrigins is a list of origins a cross-domain request can be executed from.
	// If the special "*" value is present, all origins will be allowed.
	AllowedOrigins []string

	// AllowedMethods is a list of methods the client is allowed to use.
	AllowedMethods []string

	// AllowedHeaders is a list of non-simple headers the client is allowed to use.
	AllowedHeaders []string

	// AllowCredentials indicates whether the request can include user credentials.
	AllowCredentials bool

	// MaxAge indicates how long the results of a preflight request can be cached.
	MaxAge time.Duration
}

// SetDefaults applies default values to the config where values are not set.
func (c *Config) SetDefaults() {
	if c.ReadTimeout == 0 {
		c.ReadTimeout = DefaultReadTimeout
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = DefaultWriteTimeout
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = DefaultIdleTimeout
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}
	if c.ServiceVersion == "" {
		c.ServiceVersion = "1.0.0"
	}

	c.CORS.SetDefaults()
}

// DefaultAllowedMethods returns the default list of allowed HTTP methods.
func DefaultAllowedMethods() []string {
	return []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
}

// DefaultAllowedHeaders returns the default list of allowed HTTP headers.
func DefaultAllowedHeaders() []string {
	return []string{
		"Origin",
		"Content-Type",
		"Content-Length",
		"Accept-Encoding",
		"X-CSRF-Token",
		"Authorization",
		"accept",
		"origin",
		"Cache-Control",
		"X-Requested-With",
		"X-API-Key",
	}
}

// SetDefaults applies default values to the CORS config where values are not set.
func (c *CORSConfig) SetDefaults() {
	// Enable CORS by default
	if !c.Enabled && len(c.AllowedOrigins) == 0 {
		c.Enabled = true
	}

	if len(c.AllowedOrigins) == 0 {
		c.AllowedOrigins = []string{"*"}
	}
	if len(c.AllowedMethods) == 0 {
		c.AllowedMethods = DefaultAllowedMethods()
	}
	if len(c.AllowedHeaders) == 0 {
		c.AllowedHeaders = DefaultAllowedHeaders()
	}
	if c.MaxAge == 0 {
		c.MaxAge = DefaultCORSMaxAge
	}
}

// NewConfig creates a new Config with sensible defaults.
func NewConfig(serviceName string, port int) *Config {
	cfg := &Config{
		Port:        port,
		ServiceName: serviceName,
		CORS: CORSConfig{
			Enabled: true,
		},
	}
	cfg.SetDefaults()
	return cfg
}

// WithDebug sets the debug mode.
func (c *Config) WithDebug(debug bool) *Config {
	c.Debug = debug
	return c
}

// WithVersion sets the service version.
func (c *Config) WithVersion(version string) *Config {
	c.ServiceVersion = version
	return c
}

// WithCORSOrigins sets the allowed CORS origins.
func (c *Config) WithCORSOrigins(origins []string) *Config {
	c.CORS.AllowedOrigins = origins
	return c
}

// WithTimeouts sets all timeout values.
func (c *Config) WithTimeouts(read, write, idle time.Duration) *Config {
	c.ReadTimeout = read
	c.WriteTimeout = write
	c.IdleTimeout = idle
	return c
}
