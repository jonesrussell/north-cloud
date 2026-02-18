package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetDefaults_PublisherURL_Is8070(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Services.PublisherURL != defaultURLPublisherClassifier {
		t.Errorf("expected PublisherURL %s, got %q", defaultURLPublisherClassifier, cfg.Services.PublisherURL)
	}
}

func TestSetDefaults_HTTPTimeoutSeconds_Is30WhenUnset(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Client.HTTPTimeoutSeconds != 30 {
		t.Errorf("expected HTTPTimeoutSeconds 30 when unset, got %d", cfg.Client.HTTPTimeoutSeconds)
	}
}
