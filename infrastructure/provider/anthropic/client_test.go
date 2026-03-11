package anthropic_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/infrastructure/provider/anthropic"
)

func TestNewClient(t *testing.T) {
	t.Helper()

	client := anthropic.New("test-key", "claude-haiku-4-5-20251001")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}
