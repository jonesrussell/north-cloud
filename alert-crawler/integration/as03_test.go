//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

const (
	as03EventTimeout  = 3 * time.Second
	as03DefaultExpiry = 72 * time.Hour
	// as03MaxDrainEvents is the upper bound for event draining in round 2.
	as03MaxDrainEvents = 4
)

// TestAS03_RescindedAlert verifies that when round 1 produces 2 active alerts
// and round 2 drops one of them from the feed, the missing alert receives a
// "rescinded" lifecycle event.
func TestAS03_RescindedAlert(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()

	const (
		link1 = "https://safersites.example.ca/alerts/fentanyl-003a"
		link2 = "https://safersites.example.ca/alerts/fentanyl-003b"
	)

	round1Body := BuildRSS([]Item{
		{
			Title:       "Alert A",
			Link:        link1,
			Description: "Fentanyl detected. Lab source: FTIR.",
		},
		{
			Title:       "Alert B",
			Link:        link2,
			Description: "Methamphetamine detected. Lab source: FTIR.",
			PubDate:     FixturePubDate(time.Hour),
		},
	})

	// Round 2 omits link2 — it should be rescinded.
	round2Body := BuildRSS([]Item{
		{
			Title:       "Alert A",
			Link:        link1,
			Description: "Fentanyl detected. Lab source: FTIR.",
		},
	})

	ms := NewMutableServer(round1Body)
	defer ms.Close()

	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	src := domain.AlertSource{
		ID:                  "safersites-as03",
		Name:                "SaferSites AS03",
		FeedURL:             ms.Server.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       as03DefaultExpiry,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	r := h.NewRunner([]domain.AlertSource{src})

	// Round 1: 2 alerts created.
	require.NoError(t, r.Run(ctx), "round 1 poll")
	// Drain both created events.
	_, ok1a := h.WaitForEvent(t, as03EventTimeout)
	require.True(t, ok1a, "expected first created event")
	_, ok1b := h.WaitForEvent(t, as03EventTimeout)
	require.True(t, ok1b, "expected second created event")

	activeAfterRound1, esErr := h.QueryActiveAlerts(ctx)
	require.NoError(t, esErr)
	require.Len(t, activeAfterRound1, 2, "two active alerts after round 1")

	// Swap feed to omit link2.
	ms.SetBody(round2Body)

	// Round 2: link2 should be rescinded.
	require.NoError(t, r.Run(ctx), "round 2 poll")

	// Collect events: may include an unchanged + rescinded; we look for rescinded.
	rescindedCount := 0

	for range as03MaxDrainEvents {
		ev, ok := h.WaitForEvent(t, as03EventTimeout)
		if !ok {
			break
		}

		if ev.EventType == domain.EventRescinded {
			rescindedCount++
		}
	}

	assert.Equal(t, 1, rescindedCount, "exactly one rescinded event expected")

	activeAfterRound2, esErr2 := h.QueryActiveAlerts(ctx)
	require.NoError(t, esErr2)
	assert.Len(t, activeAfterRound2, 1, "one active alert remaining after rescission")
}
