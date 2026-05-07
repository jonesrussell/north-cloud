//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

const (
	nfr006Cycles      = 100
	nfr006Items       = 5
	nfr006EventWait   = 300 * time.Millisecond
)

func TestNFR006_IdempotentReplayProducesNoSpuriousEvents(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()
	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	items := make([]Item, 0, nfr006Items)
	for i := range nfr006Items {
		items = append(items, Item{
			Title:       fmt.Sprintf("Idempotency probe %d", i),
			Link:        fmt.Sprintf("https://nfr.example.ca/idempotency/%d", i),
			Description: "Fentanyl detected. Lab source: FTIR.",
		})
	}

	srv := NewMutableServer(BuildRSS(items))
	defer srv.Close()

	src := domain.AlertSource{
		ID:                  "safersites-nfr006",
		Name:                "SaferSites NFR006",
		FeedURL:             srv.Server.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       72 * time.Hour,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	r := h.NewRunner([]domain.AlertSource{src})

	require.NoError(t, r.Run(ctx), "first poll cycle")
	firstCycleEvents := collectLifecycleEvents(h, nfr006Items, nfr006EventWait)
	require.Len(t, firstCycleEvents, nfr006Items, "first cycle must create exactly %d events", nfr006Items)

	for i := 1; i < nfr006Cycles; i++ {
		require.NoError(t, r.Run(ctx), "cycle %d", i+1)
		spurious := collectLifecycleEvents(h, 1, nfr006EventWait)
		require.Lenf(t, spurious, 0, "spurious lifecycle events detected on cycle %d", i+1)
	}
}

func collectLifecycleEvents(h *Harness, max int, timeout time.Duration) []domain.LifecycleEvent {
	events := make([]domain.LifecycleEvent, 0, max)
	for range max {
		ev, ok := h.sub.Receive(timeout)
		if !ok {
			break
		}
		events = append(events, ev)
	}
	return events
}
