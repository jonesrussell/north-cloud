//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/runner"
)

const (
	nfr009RecoveryCycles = 2
)

func TestNFR009_CrashIsolationAndRecovery(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()
	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	srv := NewMutableServer(BuildRSS([]Item{
		{
			Title:       "Blast radius probe",
			Link:        "https://nfr.example.ca/blast-radius/1",
			Description: "Fentanyl detected. Lab source: FTIR.",
		},
	}))
	defer srv.Close()

	src := domain.AlertSource{
		ID:                  "safersites-nfr009",
		Name:                "SaferSites NFR009",
		FeedURL:             srv.Server.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       72 * time.Hour,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	crashingRunner := runner.New(runner.Dependencies{
		Fetch:    h.Fetcher,
		Store:    h.Store,
		Indexer:  h.Indexer,
		Pub:      h.Publisher,
		Resolver: h.Resolver,
		SevInfer: func(domain.Hazard) domain.Severity {
			panic("synthetic crash during severity inference")
		},
		Metrics:       h.Metrics,
		Sources:       []domain.AlertSource{src},
		DefaultExpiry: 72 * time.Hour,
		Now:           time.Now,
	})

	require.Panics(t, func() {
		_ = crashingRunner.Run(ctx)
	}, "runner must panic for synthetic crash path")

	recoveryRunner := h.NewRunner([]domain.AlertSource{src})
	for range nfr009RecoveryCycles {
		require.NoError(t, recoveryRunner.Run(ctx), "recovery cycle")
	}

	alerts, err := h.QueryActiveAlerts(ctx)
	require.NoError(t, err)
	require.Len(t, alerts, 1, "recovered poll cycles should index alert without manual intervention")
}
