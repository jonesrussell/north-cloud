package anthropic_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
	anthclient "github.com/jonesrussell/north-cloud/ai-observer/internal/provider/anthropic"
)

func TestAnthropicClient_ImplementsInterface(t *testing.T) {
	t.Helper()
	var _ provider.LLMProvider = &anthclient.Client{}
}

func TestAnthropicClient_Name(t *testing.T) {
	t.Helper()
	c := anthclient.New("test-key", "claude-haiku-4-5-20251001")
	if c.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got %q", c.Name())
	}
}
