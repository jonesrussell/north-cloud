//nolint:testpackage // tests package-level constructor
package api

import (
	"testing"

	"github.com/jonesrussell/north-cloud/search/internal/config"
)

func TestNewServer_NilDeps(t *testing.T) {
	t.Helper()

	cfg := &config.Config{}
	setTestConfigDefaults(cfg)

	log := newTestLogger()
	handler := &Handler{logger: log}

	server := NewServer(handler, cfg, log, nil)
	if server == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_WithESPing(t *testing.T) {
	t.Helper()

	cfg := &config.Config{}
	setTestConfigDefaults(cfg)

	log := newTestLogger()
	handler := &Handler{logger: log}

	deps := &ServerDeps{
		ESPing: func() error { return nil },
	}

	server := NewServer(handler, cfg, log, deps)
	if server == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewHandler(t *testing.T) {
	t.Helper()

	log := newTestLogger()
	h := NewHandler(nil, log)

	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.logger == nil {
		t.Error("expected logger to be set")
	}
}

// setTestConfigDefaults sets minimal defaults so NewServer won't fail on validation.
func setTestConfigDefaults(cfg *config.Config) {
	t := &testing.T{}
	t.Helper()

	cfg.Service.Name = "search-test"
	cfg.Service.Version = "0.0.1"
	cfg.Service.Port = 8092
	cfg.Service.MaxPageSize = 100
	cfg.Service.DefaultPageSize = 20
	cfg.Service.MaxQueryLength = 500
	cfg.CORS.Enabled = false
	cfg.CORS.AllowedOrigins = []string{"*"}
	cfg.CORS.AllowedMethods = []string{"GET", "POST"}
	cfg.CORS.AllowedHeaders = []string{"Content-Type"}
	cfg.Elasticsearch.URL = "http://localhost:9200"
	cfg.Elasticsearch.ClassifiedContentPattern = "*_classified_content"
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"
}
