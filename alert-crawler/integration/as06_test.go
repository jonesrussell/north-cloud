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
	as06EventTimeout  = 3 * time.Second
	as06DefaultExpiry = 72 * time.Hour
)

// TestAS06_ScopeVocabulary verifies that when a source has default scope
// [treaty:1, canada:manitoba] and the alert title contains the word "Winnipeg",
// the resolved scope includes "canada:manitoba:winnipeg".
func TestAS06_ScopeVocabulary(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()

	body := BuildRSS([]Item{
		{
			Title:       "Fentanyl advisory Winnipeg",
			Link:        "https://safersites.example.ca/alerts/fentanyl-006",
			Description: "Fentanyl detected. Lab source: FTIR.",
		},
	})

	ms := NewMutableServer(body)
	defer ms.Close()

	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	// Source defaults include both treaty:1 and canada:manitoba.
	// The resolver (scope.Resolver) detects "Winnipeg" in the title and
	// adds canada:manitoba:winnipeg to the resolved scope.
	src := domain.AlertSource{
		ID:                  "safersites-as06",
		Name:                "SaferSites AS06",
		FeedURL:             ms.Server.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1", "canada:manitoba"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       as06DefaultExpiry,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	r := h.NewRunner([]domain.AlertSource{src})
	require.NoError(t, r.Run(ctx))

	alerts, esErr := h.QueryActiveAlerts(ctx)
	require.NoError(t, esErr)
	require.Len(t, alerts, 1)

	scope := alerts[0].Scope
	assert.Contains(t, scope, "treaty:1")
	assert.Contains(t, scope, "canada:manitoba")
	// winnipeg sub-region resolved from title hint.
	assert.Contains(t, scope, "canada:manitoba:winnipeg",
		"scope should include winnipeg sub-region resolved from title")

	ev, ok := h.WaitForEvent(t, as06EventTimeout)
	require.True(t, ok, "expected created event")
	assert.Equal(t, domain.EventCreated, ev.EventType)
	assert.Contains(t, ev.Scope, "canada:manitoba:winnipeg")
}
