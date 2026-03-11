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
	t.Setenv("AI_OBSERVER_ENABLED", "true")
	os.Unsetenv("ANTHROPIC_API_KEY")

	_, err := bootstrap.LoadConfig()
	if err == nil {
		t.Error("expected error when ANTHROPIC_API_KEY is missing and observer is enabled")
	}
}

func TestLoadConfig_DisabledNoAPIKey(t *testing.T) {
	t.Helper()
	t.Setenv("AI_OBSERVER_ENABLED", "false")
	os.Unsetenv("ANTHROPIC_API_KEY")

	_, err := bootstrap.LoadConfig()
	if err != nil {
		t.Errorf("expected no error when observer is disabled without API key, got: %v", err)
	}
}

func TestLoadConfig_InsightDefaults(t *testing.T) {
	t.Setenv("AI_OBSERVER_ENABLED", "false")

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const expectedCooldownHours = 24
	if cfg.Observer.InsightCooldownHours != expectedCooldownHours {
		t.Errorf("expected cooldown %d hours, got %d",
			expectedCooldownHours, cfg.Observer.InsightCooldownHours)
	}

	const expectedRetentionDays = 30
	if cfg.Observer.InsightRetentionDays != expectedRetentionDays {
		t.Errorf("expected retention %d days, got %d",
			expectedRetentionDays, cfg.Observer.InsightRetentionDays)
	}
}

func TestLoadConfig_DriftDefaults(t *testing.T) {
	t.Helper()
	t.Setenv("AI_OBSERVER_ENABLED", "false")

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Observer.Categories.DriftEnabled {
		t.Error("expected drift disabled by default")
	}

	const expectedKLThreshold = 0.30
	if cfg.Observer.Categories.DriftKLThreshold != expectedKLThreshold {
		t.Errorf("expected KL threshold %f, got %f",
			expectedKLThreshold, cfg.Observer.Categories.DriftKLThreshold)
	}

	const expectedPSIThreshold = 0.25
	if cfg.Observer.Categories.DriftPSIThreshold != expectedPSIThreshold {
		t.Errorf("expected PSI threshold %f, got %f",
			expectedPSIThreshold, cfg.Observer.Categories.DriftPSIThreshold)
	}

	const expectedMatrixThreshold = 0.20
	if cfg.Observer.Categories.DriftMatrixThreshold != expectedMatrixThreshold {
		t.Errorf("expected matrix threshold %f, got %f",
			expectedMatrixThreshold, cfg.Observer.Categories.DriftMatrixThreshold)
	}
}
