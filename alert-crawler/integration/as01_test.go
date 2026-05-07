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
	as01PollTimeout   = 5 * time.Second
	as01EventTimeout  = 3 * time.Second
	as01DefaultExpiry = 72 * time.Hour
)

// TestAS01_HappyPath verifies that a single valid RSS item is indexed as an
// active alert in ES and produces a "created" lifecycle event on Redis.
func TestAS01_HappyPath(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()

	body := BuildRSS([]Item{
		{
			Title:       "Fentanyl advisory - Treaty 1 region",
			Link:        "https://safersites.example.ca/alerts/fentanyl-001",
			Description: "Fentanyl detected. Lab source: FTIR analysis.",
		},
	})

	ms := NewMutableServer(body)
	defer ms.Close()

	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	src := domain.AlertSource{
		ID:                  "safersites-as01",
		Name:                "SaferSites AS01",
		FeedURL:             ms.Server.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       as01DefaultExpiry,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	r := h.NewRunner([]domain.AlertSource{src})
	require.NoError(t, r.Run(ctx), "first poll cycle should succeed")

	alerts, esErr := h.QueryActiveAlerts(ctx)
	require.NoError(t, esErr)
	require.Len(t, alerts, 1, "exactly one active alert expected")

	alert := alerts[0]
	assert.Equal(t, domain.LifecycleActive, alert.LifecycleState)
	assert.Contains(t, alert.Scope, "treaty:1")

	ev, ok := h.WaitForEvent(t, as01EventTimeout)
	require.True(t, ok, "expected a lifecycle event within timeout")
	assert.Equal(t, domain.EventCreated, ev.EventType)
	assert.Equal(t, alert.ID, ev.AlertID)
}

// Ensure as01PollTimeout is referenced to satisfy the unused-const lint check.
var _ = as01PollTimeout
