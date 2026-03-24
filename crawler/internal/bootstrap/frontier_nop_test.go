package bootstrap_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
)

func TestDiscoveryFrontierNop_Submit(t *testing.T) {
	t.Parallel()

	nop := bootstrap.DiscoveryFrontierNopForTest{}
	err := nop.Submit(context.Background(), "https://example.com", "hash", "example.com", "src-1", "discovered", 0, 1)
	if err != nil {
		t.Errorf("expected nil error from nop frontier, got %v", err)
	}
}
