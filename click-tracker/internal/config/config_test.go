package config

import (
	"testing"
	"time"
)

func TestSetDefaults(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	setDefaults(cfg)

	assertStringEqual(t, "service.name", defaultServiceName, cfg.Service.Name)
	assertStringEqual(t, "service.version", defaultVersion, cfg.Service.Version)
	assertIntEqual(t, "service.port", defaultServicePort, cfg.Service.Port)
	assertIntEqual(t, "service.signature_length", defaultSigLength, cfg.Service.SignatureLength)
	assertIntEqual(t, "service.buffer_size", defaultBufferSize, cfg.Service.BufferSize)
	assertIntEqual(t, "service.flush_threshold", defaultFlushThresh, cfg.Service.FlushThreshold)

	expectedMaxAge := defaultMaxTimestampAgeH * time.Hour
	if cfg.Service.MaxTimestampAge != expectedMaxAge {
		t.Errorf("service.max_timestamp_age: got %v, want %v",
			cfg.Service.MaxTimestampAge, expectedMaxAge)
	}

	expectedFlushInterval := defaultFlushIntervalS * time.Second
	if cfg.Service.FlushInterval != expectedFlushInterval {
		t.Errorf("service.flush_interval: got %v, want %v",
			cfg.Service.FlushInterval, expectedFlushInterval)
	}

	assertStringEqual(t, "database.host", defaultDBHost, cfg.Database.Host)
	assertIntEqual(t, "database.port", defaultDBPort, cfg.Database.Port)
	assertStringEqual(t, "database.user", defaultDBUser, cfg.Database.User)
	assertStringEqual(t, "database.database", defaultDBName, cfg.Database.Database)
	assertStringEqual(t, "database.sslmode", defaultDBSSLMode, cfg.Database.SSLMode)

	assertIntEqual(t, "rate_limit.max_clicks_per_minute",
		defaultMaxClicksPerMinute, cfg.RateLimit.MaxClicksPerMinute)
	assertIntEqual(t, "rate_limit.window_seconds",
		defaultWindowSeconds, cfg.RateLimit.WindowSeconds)

	assertStringEqual(t, "logging.level", defaultLoggingLevel, cfg.Logging.Level)
	assertStringEqual(t, "logging.format", defaultLoggingFmt, cfg.Logging.Format)
}

func TestValidate_MissingSecret(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	setDefaults(cfg)
	cfg.Service.HMACSecret = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing HMAC secret, got nil")
	}

	expected := "service.hmac_secret: is required"
	if err.Error() != expected {
		t.Errorf("error message: got %q, want %q", err.Error(), expected)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	setDefaults(cfg)
	cfg.Service.HMACSecret = "test-secret-key"

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no validation error, got: %v", err)
	}
}

func TestDSN(t *testing.T) {
	t.Helper()

	db := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Database: "click_tracker",
		SSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=postgres password=secret dbname=click_tracker sslmode=disable"
	got := db.DSN()

	if got != expected {
		t.Errorf("DSN:\ngot:  %q\nwant: %q", got, expected)
	}
}

// assertStringEqual is a test helper that checks string equality.
func assertStringEqual(t *testing.T, field, want, got string) {
	t.Helper()

	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}

// assertIntEqual is a test helper that checks int equality.
func assertIntEqual(t *testing.T, field string, want, got int) {
	t.Helper()

	if got != want {
		t.Errorf("%s: got %d, want %d", field, got, want)
	}
}
