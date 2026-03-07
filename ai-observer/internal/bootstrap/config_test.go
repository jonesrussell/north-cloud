package bootstrap_test

import (
	"os"
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/bootstrap"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Helper()
	t.Setenv("ES_URL", "http://localhost:9200")
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	// Clear optional vars to test defaults.
	os.Unsetenv("AI_OBSERVER_INTERVAL_SECONDS")
	os.Unsetenv("AI_OBSERVER_MAX_TOKENS_PER_INTERVAL")

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Observer.IntervalSeconds == 0 {
		t.Error("expected non-zero IntervalSeconds default")
	}

	if cfg.Observer.MaxTokensPerInterval == 0 {
		t.Error("expected non-zero MaxTokensPerInterval default")
	}
}

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	t.Helper()
	os.Unsetenv("ANTHROPIC_API_KEY")

	_, err := bootstrap.LoadConfig()
	if err == nil {
		t.Error("expected error when ANTHROPIC_API_KEY is missing")
	}
}
