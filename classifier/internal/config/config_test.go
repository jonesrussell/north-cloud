package config

import "testing"

func TestDefaults_DrillExtraction(t *testing.T) {
	cfg := &Config{}
	SetDefaults(cfg)

	if cfg.Classification.DrillExtraction.MaxBodyChars != 4000 {
		t.Errorf("MaxBodyChars = %d, want 4000", cfg.Classification.DrillExtraction.MaxBodyChars)
	}
	if cfg.Classification.DrillExtraction.AnthropicModel != "claude-haiku-4-5" {
		t.Errorf("AnthropicModel = %q, want claude-haiku-4-5", cfg.Classification.DrillExtraction.AnthropicModel)
	}
	if cfg.Classification.DrillExtraction.AnthropicBaseURL != "https://api.anthropic.com" {
		t.Errorf("AnthropicBaseURL = %q, want https://api.anthropic.com", cfg.Classification.DrillExtraction.AnthropicBaseURL)
	}
	// Enabled and LLMFallback default to false (safe defaults)
	if cfg.Classification.DrillExtraction.Enabled {
		t.Error("expected Enabled=false by default")
	}
	if cfg.Classification.DrillExtraction.LLMFallback {
		t.Error("expected LLMFallback=false by default")
	}
}
