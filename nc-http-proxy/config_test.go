package main

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Helper()
	// Clear any env vars
	os.Unsetenv("PROXY_PORT")
	os.Unsetenv("PROXY_MODE")

	cfg := LoadConfig()

	if cfg.Port != 8055 {
		t.Errorf("expected port 8055, got %d", cfg.Port)
	}
	if cfg.Mode != ModeReplay {
		t.Errorf("expected mode replay, got %s", cfg.Mode)
	}
	if cfg.FixturesDir != "/app/fixtures" {
		t.Errorf("expected fixtures dir /app/fixtures, got %s", cfg.FixturesDir)
	}
	if cfg.CacheDir != "/app/cache" {
		t.Errorf("expected cache dir /app/cache, got %s", cfg.CacheDir)
	}
	if cfg.CertsDir != "/app/certs" {
		t.Errorf("expected certs dir /app/certs, got %s", cfg.CertsDir)
	}
	if cfg.LiveTimeout.Seconds() != 30 {
		t.Errorf("expected timeout 30s, got %v", cfg.LiveTimeout)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Helper()
	t.Setenv("PROXY_PORT", "9000")
	t.Setenv("PROXY_MODE", "record")
	t.Setenv("PROXY_FIXTURES_DIR", "/custom/fixtures")
	t.Setenv("PROXY_CACHE_DIR", "/custom/cache")
	t.Setenv("PROXY_LIVE_TIMEOUT", "60s")

	cfg := LoadConfig()

	if cfg.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Port)
	}
	if cfg.Mode != ModeRecord {
		t.Errorf("expected mode record, got %s", cfg.Mode)
	}
	if cfg.FixturesDir != "/custom/fixtures" {
		t.Errorf("expected fixtures dir /custom/fixtures, got %s", cfg.FixturesDir)
	}
	if cfg.CacheDir != "/custom/cache" {
		t.Errorf("expected cache dir /custom/cache, got %s", cfg.CacheDir)
	}
}

func TestModeIsValid(t *testing.T) {
	t.Helper()
	validModes := []Mode{ModeReplay, ModeRecord, ModeLive}
	for _, m := range validModes {
		if !m.IsValid() {
			t.Errorf("expected %s to be valid", m)
		}
	}

	invalidMode := Mode("invalid")
	if invalidMode.IsValid() {
		t.Error("expected 'invalid' mode to be invalid")
	}
}
