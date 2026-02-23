package fetcher_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
)

func TestConfig_WithDefaults_AppliesStaleDefaults(t *testing.T) {
	t.Helper()

	cfg := fetcher.Config{}.WithDefaults()

	expectedTimeout := 10 * time.Minute
	expectedInterval := 2 * time.Minute

	if cfg.StaleTimeout != expectedTimeout {
		t.Errorf("expected StaleTimeout=%v, got %v", expectedTimeout, cfg.StaleTimeout)
	}
	if cfg.StaleCheckInterval != expectedInterval {
		t.Errorf("expected StaleCheckInterval=%v, got %v", expectedInterval, cfg.StaleCheckInterval)
	}
}

func TestConfig_WithDefaults_PreservesExplicitStaleValues(t *testing.T) {
	t.Helper()

	customTimeout := 5 * time.Minute
	customInterval := 30 * time.Second

	cfg := fetcher.Config{
		StaleTimeout:       customTimeout,
		StaleCheckInterval: customInterval,
	}.WithDefaults()

	if cfg.StaleTimeout != customTimeout {
		t.Errorf("expected StaleTimeout=%v, got %v", customTimeout, cfg.StaleTimeout)
	}
	if cfg.StaleCheckInterval != customInterval {
		t.Errorf("expected StaleCheckInterval=%v, got %v", customInterval, cfg.StaleCheckInterval)
	}
}
