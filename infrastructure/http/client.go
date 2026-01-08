// Package http provides shared HTTP client utilities for the North Cloud microservices.
package http

import (
	"crypto/tls"
	"net/http"
	"time"
)

const (
	// DefaultTimeout is the default timeout for HTTP requests
	DefaultTimeout = 30 * time.Second

	// DefaultMaxIdleConns is the default maximum number of idle connections
	DefaultMaxIdleConns = 100

	// DefaultMaxIdleConnsPerHost is the default maximum number of idle connections per host
	DefaultMaxIdleConnsPerHost = 10

	// DefaultIdleConnTimeout is the default idle connection timeout
	DefaultIdleConnTimeout = 90 * time.Second

	// DefaultResponseHeaderTimeout is the default response header timeout
	DefaultResponseHeaderTimeout = 30 * time.Second

	// DefaultExpectContinueTimeout is the default expect continue timeout
	DefaultExpectContinueTimeout = 1 * time.Second

	// DefaultTLSHandshakeTimeout is the default TLS handshake timeout
	DefaultTLSHandshakeTimeout = 10 * time.Second
)

// ClientConfig configures an HTTP client.
type ClientConfig struct {
	// Timeout specifies a time limit for requests made by this Client.
	// A Timeout of zero means no timeout.
	Timeout time.Duration

	// TLSConfig specifies the TLS configuration to use for tls.Client.
	// If nil, the default configuration is used.
	TLSConfig *tls.Config

	// MaxIdleConns controls the maximum number of idle (keep-alive) connections
	// across all hosts. Zero means no limit.
	MaxIdleConns int

	// MaxIdleConnsPerHost, if non-zero, controls the maximum idle
	// (keep-alive) connections to keep per-host. If zero, DefaultMaxIdleConnsPerHost is used.
	MaxIdleConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle (keep-alive)
	// connection will remain idle before closing itself.
	// Zero means no timeout.
	IdleConnTimeout time.Duration

	// ResponseHeaderTimeout, if non-zero, specifies the amount of
	// time to wait for a server's response headers after fully
	// writing the request (including its body, if any). This
	// time does not include the time to read the response body.
	ResponseHeaderTimeout time.Duration

	// ExpectContinueTimeout, if non-zero, specifies the amount of
	// time to wait for a server's first response headers after fully
	// writing the request headers if the request has an
	// "Expect: 100-continue" header. Zero means no timeout and
	// causes the body to be sent immediately.
	ExpectContinueTimeout time.Duration

	// TLSHandshakeTimeout specifies the maximum amount of time waiting to
	// wait for a TLS handshake. Zero means no timeout.
	TLSHandshakeTimeout time.Duration

	// DisableKeepAlives, if true, disables HTTP keep-alives and
	// will only use the connection to the server for a single
	// HTTP request.
	DisableKeepAlives bool
}

// NewClient creates a new HTTP client with standardized configuration.
// If cfg is nil, default values are used.
func NewClient(cfg *ClientConfig) *http.Client {
	if cfg == nil {
		cfg = &ClientConfig{}
	}

	// Set defaults
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	maxIdleConns := cfg.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = DefaultMaxIdleConns
	}

	maxIdleConnsPerHost := cfg.MaxIdleConnsPerHost
	if maxIdleConnsPerHost == 0 {
		maxIdleConnsPerHost = DefaultMaxIdleConnsPerHost
	}

	idleConnTimeout := cfg.IdleConnTimeout
	if idleConnTimeout == 0 {
		idleConnTimeout = DefaultIdleConnTimeout
	}

	responseHeaderTimeout := cfg.ResponseHeaderTimeout
	if responseHeaderTimeout == 0 {
		responseHeaderTimeout = DefaultResponseHeaderTimeout
	}

	expectContinueTimeout := cfg.ExpectContinueTimeout
	if expectContinueTimeout == 0 {
		expectContinueTimeout = DefaultExpectContinueTimeout
	}

	tlsHandshakeTimeout := cfg.TLSHandshakeTimeout
	if tlsHandshakeTimeout == 0 {
		tlsHandshakeTimeout = DefaultTLSHandshakeTimeout
	}

	// Create transport
	transport := &http.Transport{
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		DisableKeepAlives:     cfg.DisableKeepAlives,
	}

	// Configure TLS if provided
	if cfg.TLSConfig != nil {
		transport.TLSClientConfig = cfg.TLSConfig
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// NewClientWithTLS creates a new HTTP client with TLS configuration.
// If skipVerify is true, TLS certificate verification is disabled.
func NewClientWithTLS(timeout time.Duration, skipVerify bool) *http.Client {
	cfg := &ClientConfig{
		Timeout: timeout,
	}

	if skipVerify {
		cfg.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	return NewClient(cfg)
}

// NewDefaultClient creates a new HTTP client with all default settings.
func NewDefaultClient() *http.Client {
	return NewClient(nil)
}
