package bootstrap_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
)

func TestCreateLogger_DefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := &mockConfig{
		loggingConfig: &config.LoggingConfig{
			Level: "info",
			Env:   "production",
		},
	}

	log, err := bootstrap.CreateLogger(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestCreateLogger_DevEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &mockConfig{
		loggingConfig: &config.LoggingConfig{
			Level: "debug",
			Env:   "development",
		},
	}

	log, err := bootstrap.CreateLogger(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestCreateLogger_DebugOverride(t *testing.T) {
	t.Parallel()

	cfg := &mockConfig{
		loggingConfig: &config.LoggingConfig{
			Level: "warn",
			Debug: true,
			Env:   "production",
		},
	}

	log, err := bootstrap.CreateLogger(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestCreateLogger_EmptyLevel(t *testing.T) {
	t.Parallel()

	cfg := &mockConfig{
		loggingConfig: &config.LoggingConfig{
			Level: "",
			Env:   "",
		},
	}

	log, err := bootstrap.CreateLogger(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestCreateLogger_ExplicitEncoding(t *testing.T) {
	t.Parallel()

	cfg := &mockConfig{
		loggingConfig: &config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Env:    "development",
		},
	}

	log, err := bootstrap.CreateLogger(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if log == nil {
		t.Fatal("expected non-nil logger")
	}
}
