//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

const (
	// consecutiveFailureThreshold is the inclusive count at which a WARN should
	// be emitted per NFR-005. Must match runner.consecutiveFailureWarnThreshold.
	consecutiveFailureThreshold = 6

	// as05RecoveryEventTimeout is how long we wait for the recovery event.
	as05RecoveryEventTimeout = 3 * time.Second

	as05DefaultExpiry = 72 * time.Hour
)

// TestAS05_SourceUnreachable exercises the consecutive-failure metric path
// (NFR-005). Six cycles of HTTP 503 drive consecutive_failures to 6, after
// which a 7th successful cycle resets the counter and produces a created event.
func TestAS05_SourceUnreachable(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()

	var requestCount atomic.Int64

	successBody := BuildRSS([]Item{
		{
			Title:       "Recovery advisory",
			Link:        "https://safersites.example.ca/alerts/fentanyl-005",
			Description: "Fentanyl detected. Lab source: FTIR.",
		},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := requestCount.Add(1)

		if n <= consecutiveFailureThreshold {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(successBody))
	}))
	defer srv.Close()

	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	src := domain.AlertSource{
		ID:                  "safersites-as05",
		Name:                "SaferSites AS05",
		FeedURL:             srv.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       as05DefaultExpiry,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	r := h.NewRunner([]domain.AlertSource{src})

	// Run consecutiveFailureThreshold failing cycles; each should return an error.
	for i := range consecutiveFailureThreshold {
		runErr := r.Run(ctx)
		require.Error(t, runErr,
			"cycle %d: expected error on 503 response", i+1)
	}

	// 7th cycle: 200 OK — should succeed and publish a created event.
	require.NoError(t, r.Run(ctx), "recovery cycle should succeed")

	ev, ok := h.WaitForEvent(t, as05RecoveryEventTimeout)
	require.True(t, ok, "expected a created event on recovery")
	assert.Equal(t, domain.EventCreated, ev.EventType)
}
