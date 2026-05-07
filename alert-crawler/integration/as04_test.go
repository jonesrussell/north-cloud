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
	as04DefaultExpiry = 72 * time.Hour
	// as04PollCycles is the number of poll cycles run before asserting ES state.
	as04PollCycles = 3
)

// TestAS04_SubscriberRecovery verifies that alerts indexed during poll cycles
// that happen before a subscriber connects are still queryable from ES.
// This tests the "late subscriber reads ES directly" pattern from NFR-004.
func TestAS04_SubscriberRecovery(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()

	body := BuildRSS([]Item{
		{
			Title:       "Alert Alpha",
			Link:        "https://safersites.example.ca/alerts/fentanyl-004a",
			Description: "Fentanyl detected. Lab source: FTIR.",
		},
		{
			Title:       "Alert Beta",
			Link:        "https://safersites.example.ca/alerts/fentanyl-004b",
			Description: "Methamphetamine detected. Lab source: FTIR.",
			PubDate:     FixturePubDate(time.Hour),
		},
		{
			Title:       "Alert Gamma",
			Link:        "https://safersites.example.ca/alerts/fentanyl-004c",
			Description: "Benzodiazepine detected. Lab source: FTIR.",
			PubDate:     FixturePubDate(2 * time.Hour),
		},
	})

	ms := NewMutableServer(body)
	defer ms.Close()

	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	src := domain.AlertSource{
		ID:                  "safersites-as04",
		Name:                "SaferSites AS04",
		FeedURL:             ms.Server.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       as04DefaultExpiry,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	r := h.NewRunner([]domain.AlertSource{src})

	// Run as04PollCycles poll cycles. The first creates all 3; subsequent cycles
	// see unchanged content and mark them seen without re-publishing.
	for range as04PollCycles {
		require.NoError(t, r.Run(ctx), "poll cycle")
	}

	// A late subscriber (simulated by h.QueryActiveAlerts) must see all 3 alerts.
	alerts, esErr := h.QueryActiveAlerts(ctx)
	require.NoError(t, esErr)
	assert.Len(t, alerts, 3,
		"all 3 alerts must be queryable from ES after %d cycles", as04PollCycles)

	for _, a := range alerts {
		assert.Equal(t, domain.LifecycleActive, a.LifecycleState)
	}
}
