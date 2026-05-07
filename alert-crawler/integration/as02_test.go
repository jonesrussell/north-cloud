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
	as02EventTimeout  = 3 * time.Second
	as02DefaultExpiry = 72 * time.Hour
)

// TestAS02_CorrectedAlert verifies that when the same alert item appears in
// two consecutive polls with changed content, the first poll produces a
// "created" event and the second produces an "updated" event.
// The alert's revision_history is expected to grow between rounds.
func TestAS02_CorrectedAlert(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()

	const alertLink = "https://safersites.example.ca/alerts/fentanyl-002"

	round1Body := BuildRSS([]Item{
		{
			Title:       "Fentanyl advisory - round 1",
			Link:        alertLink,
			Description: "Fentanyl detected. Lab source: FTIR.",
		},
	})

	round2Body := BuildRSS([]Item{
		{
			Title:       "Fentanyl advisory - round 2 corrected",
			Link:        alertLink,
			Description: "Fentanyl AND carfentanil detected. Lab source: FTIR.",
		},
	})

	ms := NewMutableServer(round1Body)
	defer ms.Close()

	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	src := domain.AlertSource{
		ID:                  "safersites-as02",
		Name:                "SaferSites AS02",
		FeedURL:             ms.Server.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       as02DefaultExpiry,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	r := h.NewRunner([]domain.AlertSource{src})

	// Round 1: expect "created".
	require.NoError(t, r.Run(ctx), "round 1 poll")
	ev1, ok1 := h.WaitForEvent(t, as02EventTimeout)
	require.True(t, ok1, "expected created event")
	assert.Equal(t, domain.EventCreated, ev1.EventType)

	// Swap to updated feed body.
	ms.SetBody(round2Body)

	// Round 2: expect "updated".
	require.NoError(t, r.Run(ctx), "round 2 poll")
	ev2, ok2 := h.WaitForEvent(t, as02EventTimeout)
	require.True(t, ok2, "expected updated event")
	assert.Equal(t, domain.EventUpdated, ev2.EventType)
	assert.Equal(t, ev1.AlertID, ev2.AlertID, "same alert ID across rounds")

	// Revision history should have grown.
	alerts, esErr := h.QueryActiveAlerts(ctx)
	require.NoError(t, esErr)
	require.Len(t, alerts, 1)
	assert.GreaterOrEqual(t, len(alerts[0].RevisionHistory), 1,
		"revision_history should contain at least one entry after update")
}
