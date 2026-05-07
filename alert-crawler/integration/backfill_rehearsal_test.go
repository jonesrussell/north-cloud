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
	backfillRehearsalFeedItems = 30
	backfillLimitExpected      = 20
	backfillEventTimeout       = 300 * time.Millisecond
)

func TestBackfillRehearsal_FirstDeployAndIdempotentRerun(t *testing.T) {
	WithIntegration(t)

	ctx := context.Background()
	h := NewHarness(t)
	t.Cleanup(h.Cleanup)

	items := make([]Item, 0, backfillRehearsalFeedItems)
	for i := range backfillRehearsalFeedItems {
		items = append(items, Item{
			Title:       fmt.Sprintf("Backfill rehearsal %02d", i),
			Link:        fmt.Sprintf("https://nfr.example.ca/backfill/%02d", i),
			Description: "Fentanyl detected. Lab source: FTIR.",
			PubDate:     FixturePubDate(time.Duration(i) * time.Minute),
		})
	}

	srv := NewMutableServer(BuildRSS(items))
	defer srv.Close()

	src := domain.AlertSource{
		ID:                  "safersites-backfill-rehearsal",
		Name:                "SaferSites Backfill Rehearsal",
		FeedURL:             srv.Server.URL,
		Enabled:             true,
		PollInterval:        30 * time.Minute,
		DefaultScope:        []string{"treaty:1"},
		DefaultCategory:     domain.CategoryHarmReduction,
		DefaultExpiry:       72 * time.Hour,
		AcquisitionStrategy: domain.AcquisitionRSS,
	}

	r := h.NewRunner([]domain.AlertSource{src})

	require.NoError(t, r.Backfill(ctx), "first backfill run")
	createdFirstRun := collectLifecycleEvents(h, backfillLimitExpected+5, backfillEventTimeout)
	require.Len(t, createdFirstRun, backfillLimitExpected, "first backfill must emit exactly 20 created events")

	activeAfterFirstRun, err := h.QueryActiveAlerts(ctx)
	require.NoError(t, err)
	require.Len(t, activeAfterFirstRun, backfillLimitExpected, "first backfill should index top 20 items")

	require.NoError(t, r.Backfill(ctx), "second backfill run must be idempotent")
	createdSecondRun := collectLifecycleEvents(h, 1, backfillEventTimeout)
	require.Len(t, createdSecondRun, 0, "second backfill should emit zero new events")

	require.NoError(t, r.Run(ctx), "normal poll after backfill")
	createdAfterNormalRun := collectLifecycleEvents(h, backfillRehearsalFeedItems, backfillEventTimeout)
	require.NotEmpty(t, createdAfterNormalRun, "normal poll should emit created events for previously skipped items")
}
