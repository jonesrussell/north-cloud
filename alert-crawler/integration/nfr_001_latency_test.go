//go:build integration

package integration

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

const (
	nfr001Iterations      = 100
	nfr001PollTimeout     = 15 * time.Second
	nfr001P95Threshold    = 60 * time.Minute
	nfr001P99Threshold    = 120 * time.Minute
	nfr001BasePubDateRFC  = "Mon, 02 Jan 2006 15:04:05 MST"
)

func TestNFR001_LatencyPercentiles(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()
	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	srv := NewMutableServer(BuildRSS(nil))
	defer srv.Close()

	src := domain.AlertSource{
		ID:                  "safersites-nfr001",
		Name:                "SaferSites NFR001",
		FeedURL:             srv.Server.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       72 * time.Hour,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	r := h.NewRunner([]domain.AlertSource{src})
	latencies := make([]time.Duration, 0, nfr001Iterations)

	for i := range nfr001Iterations {
		issuedAt := time.Now().UTC()
		item := Item{
			Title:       fmt.Sprintf("Latency probe %03d", i),
			Link:        fmt.Sprintf("https://nfr.example.ca/latency/%03d", i),
			Description: "Fentanyl detected. Lab source: FTIR.",
			PubDate:     issuedAt.Format(nfr001BasePubDateRFC),
		}

		srv.SetBody(BuildRSS([]Item{item}))
		start := time.Now()
		require.NoError(t, r.Run(ctx), "poll cycle %d", i)
		cycleLatency := time.Since(start)
		require.Less(t, cycleLatency, nfr001PollTimeout, "poll cycle %d took too long", i)
		latencies = append(latencies, cycleLatency)
	}

	p95 := percentileDuration(latencies, 0.95)
	p99 := percentileDuration(latencies, 0.99)

	require.LessOrEqualf(
		t,
		p95,
		nfr001P95Threshold,
		"NFR-001 breach: p95 latency %s exceeds threshold %s",
		p95,
		nfr001P95Threshold,
	)
	require.LessOrEqualf(
		t,
		p99,
		nfr001P99Threshold,
		"NFR-001 breach: p99 latency %s exceeds threshold %s",
		p99,
		nfr001P99Threshold,
	)
}

func percentileDuration(values []time.Duration, pct float64) time.Duration {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]time.Duration(nil), values...)
	slices.Sort(sorted)

	idx := int(float64(len(sorted)-1) * pct)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
