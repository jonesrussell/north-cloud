package main

import (
	"os"
	"strconv"
	"time"
)

// Mode represents the proxy operating mode.
type Mode string

const (
	ModeReplay Mode = "replay"
	ModeRecord Mode = "record"
	ModeLive   Mode = "live"
)

// IsValid returns true if the mode is a recognized value.
func (m Mode) IsValid() bool {
	switch m {
	case ModeReplay, ModeRecord, ModeLive:
		return true
	default:
		return false
	}
}

// Config holds the proxy configuration.
type Config struct {
	Port        int
	Mode        Mode
	FixturesDir string
	CacheDir    string
	CertFile    string
	KeyFile     string
	LiveTimeout time.Duration
}

// Default configuration values.
const (
	defaultPort        = 8055
	defaultMode        = ModeReplay
	defaultFixturesDir = "/app/fixtures"
	defaultCacheDir    = "/app/cache"
	defaultCertFile    = "/app/certs/proxy.crt"
	defaultKeyFile     = "/app/certs/proxy.key"
	defaultLiveTimeout = 30 * time.Second
)

// LoadConfig loads configuration from environment variables.
func LoadConfig() *Config {
	cfg := &Config{
		Port:        defaultPort,
		Mode:        defaultMode,
		FixturesDir: defaultFixturesDir,
		CacheDir:    defaultCacheDir,
		CertFile:    defaultCertFile,
		KeyFile:     defaultKeyFile,
		LiveTimeout: defaultLiveTimeout,
	}

	if port := os.Getenv("PROXY_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}

	if mode := os.Getenv("PROXY_MODE"); mode != "" {
		m := Mode(mode)
		if m.IsValid() {
			cfg.Mode = m
		}
	}

	if fixtures := os.Getenv("PROXY_FIXTURES_DIR"); fixtures != "" {
		cfg.FixturesDir = fixtures
	}

	if cache := os.Getenv("PROXY_CACHE_DIR"); cache != "" {
		cfg.CacheDir = cache
	}

	if cert := os.Getenv("PROXY_CERT_FILE"); cert != "" {
		cfg.CertFile = cert
	}

	if key := os.Getenv("PROXY_KEY_FILE"); key != "" {
		cfg.KeyFile = key
	}

	if timeout := os.Getenv("PROXY_LIVE_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.LiveTimeout = d
		}
	}

	return cfg
}
