package metrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/publisher/internal/metrics"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTracker(t *testing.T, cities []string) (*metrics.Tracker, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	tracker := metrics.NewTracker(client, cities, infralogger.NewNop())
	return tracker, mr
}

func TestTracker_IncrementPosted(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, []string{"thunder_bay"})
	ctx := context.Background()

	require.NoError(t, tracker.IncrementPosted(ctx, "thunder_bay"))
	require.NoError(t, tracker.IncrementPosted(ctx, "thunder_bay"))

	val, err := mr.Get("metrics:posted:thunder_bay")
	require.NoError(t, err)
	assert.Equal(t, "2", val)

	// Verify TTL was set
	assert.Positive(t, mr.TTL("metrics:posted:thunder_bay"))
}

func TestTracker_IncrementSkipped(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, []string{"ottawa"})
	ctx := context.Background()

	require.NoError(t, tracker.IncrementSkipped(ctx, "ottawa"))

	val, err := mr.Get("metrics:skipped:ottawa")
	require.NoError(t, err)
	assert.Equal(t, "1", val)
}

func TestTracker_IncrementErrors(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, []string{"ottawa"})
	ctx := context.Background()

	require.NoError(t, tracker.IncrementErrors(ctx, "ottawa"))
	require.NoError(t, tracker.IncrementErrors(ctx, "ottawa"))
	require.NoError(t, tracker.IncrementErrors(ctx, "ottawa"))

	val, err := mr.Get("metrics:errors:ottawa")
	require.NoError(t, err)
	assert.Equal(t, "3", val)
}

func TestTracker_IncrementBackfill(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, nil)
	ctx := context.Background()

	require.NoError(t, tracker.IncrementBackfillTotal(ctx))
	require.NoError(t, tracker.IncrementBackfillSuccess(ctx))
	require.NoError(t, tracker.IncrementBackfillFailed(ctx))

	total, err := mr.Get(metrics.KeyBackfillTotal)
	require.NoError(t, err)
	assert.Equal(t, "1", total)

	success, err := mr.Get(metrics.KeyBackfillSuccess)
	require.NoError(t, err)
	assert.Equal(t, "1", success)

	failed, err := mr.Get(metrics.KeyBackfillFailed)
	require.NoError(t, err)
	assert.Equal(t, "1", failed)
}

func TestTracker_AddRecentItem_RecentItem(t *testing.T) {
	t.Helper()

	tracker, _ := setupTracker(t, nil)
	ctx := context.Background()

	item := metrics.RecentItem{
		ID:       "item-1",
		Title:    "Test Article",
		URL:      "https://example.com/article",
		City:     "Thunder Bay",
		PostedAt: time.Now().UTC().Truncate(time.Second),
	}

	require.NoError(t, tracker.AddRecentItem(ctx, item))

	items, err := tracker.GetRecentItems(ctx, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "item-1", items[0].ID)
	assert.Equal(t, "Test Article", items[0].Title)
}

func TestTracker_AddRecentItem_Map(t *testing.T) {
	t.Helper()

	tracker, _ := setupTracker(t, nil)
	ctx := context.Background()

	item := map[string]any{
		"id":        "map-item-1",
		"title":     "Map Article",
		"url":       "https://example.com/map",
		"city":      "Ottawa",
		"posted_at": time.Now().UTC().Format(time.RFC3339),
	}

	require.NoError(t, tracker.AddRecentItem(ctx, item))

	items, err := tracker.GetRecentItems(ctx, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "map-item-1", items[0].ID)
}

func TestTracker_GetRecentItems_EmptyList(t *testing.T) {
	t.Helper()

	tracker, _ := setupTracker(t, nil)
	ctx := context.Background()

	items, err := tracker.GetRecentItems(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestTracker_GetRecentItems_LimitDefaults(t *testing.T) {
	t.Helper()

	tracker, _ := setupTracker(t, nil)
	ctx := context.Background()

	// Add a few items
	for i := range 3 {
		item := metrics.RecentItem{
			ID:       "item-" + string(rune('a'+i)),
			Title:    "Article",
			PostedAt: time.Now(),
		}
		require.NoError(t, tracker.AddRecentItem(ctx, item))
	}

	// Limit 0 should default to 50
	items, err := tracker.GetRecentItems(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, items, 3)

	// Limit > MaxRecentItems should be capped
	items, err = tracker.GetRecentItems(ctx, 200)
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

func TestTracker_GetStats_WithData(t *testing.T) {
	t.Helper()

	cities := []string{"thunder_bay", "ottawa"}
	tracker, _ := setupTracker(t, cities)
	ctx := context.Background()

	// Increment some counters
	require.NoError(t, tracker.IncrementPosted(ctx, "thunder_bay"))
	require.NoError(t, tracker.IncrementPosted(ctx, "thunder_bay"))
	require.NoError(t, tracker.IncrementPosted(ctx, "ottawa"))
	require.NoError(t, tracker.IncrementSkipped(ctx, "thunder_bay"))
	require.NoError(t, tracker.IncrementErrors(ctx, "ottawa"))

	stats, err := tracker.GetStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, int64(3), stats.TotalPosted)
	assert.Equal(t, int64(1), stats.TotalSkipped)
	assert.Equal(t, int64(1), stats.TotalErrors)
	require.Len(t, stats.Cities, 2)

	// Find Thunder Bay stats
	var tbStats metrics.CityStats
	for _, cs := range stats.Cities {
		if cs.Name == "thunder_bay" {
			tbStats = cs
			break
		}
	}
	assert.Equal(t, int64(2), tbStats.Posted)
	assert.Equal(t, int64(1), tbStats.Skipped)
	assert.Equal(t, int64(0), tbStats.Errors)
}

func TestTracker_GetStats_Empty(t *testing.T) {
	t.Helper()

	cities := []string{"thunder_bay"}
	tracker, _ := setupTracker(t, cities)
	ctx := context.Background()

	stats, err := tracker.GetStats(ctx)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, int64(0), stats.TotalPosted)
	assert.Equal(t, int64(0), stats.TotalSkipped)
	assert.Equal(t, int64(0), stats.TotalErrors)
	assert.True(t, stats.LastSync.IsZero())
}

func TestTracker_UpdateLastSync(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, []string{"thunder_bay"})
	ctx := context.Background()

	require.NoError(t, tracker.UpdateLastSync(ctx))

	val, err := mr.Get(metrics.KeyLastSync)
	require.NoError(t, err)
	assert.NotEmpty(t, val)

	// Verify it's a valid RFC3339 timestamp
	_, parseErr := time.Parse(time.RFC3339, val)
	assert.NoError(t, parseErr)
}

func TestTracker_GetStats_WithLastSync(t *testing.T) {
	t.Helper()

	cities := []string{"thunder_bay"}
	tracker, _ := setupTracker(t, cities)
	ctx := context.Background()

	require.NoError(t, tracker.UpdateLastSync(ctx))

	stats, err := tracker.GetStats(ctx)
	require.NoError(t, err)
	assert.False(t, stats.LastSync.IsZero())
	assert.WithinDuration(t, time.Now(), stats.LastSync, 5*time.Second)
}

func TestTracker_IncrementPosted_RedisDown(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, []string{"thunder_bay"})
	mr.Close()

	err := tracker.IncrementPosted(context.Background(), "thunder_bay")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "increment posted counter")
}

func TestTracker_IncrementSkipped_RedisDown(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, []string{"ottawa"})
	mr.Close()

	err := tracker.IncrementSkipped(context.Background(), "ottawa")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "increment skipped counter")
}

func TestTracker_IncrementErrors_RedisDown(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, []string{"ottawa"})
	mr.Close()

	err := tracker.IncrementErrors(context.Background(), "ottawa")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "increment error counter")
}

func TestTracker_AddRecentItem_RedisDown(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, nil)
	mr.Close()

	item := metrics.RecentItem{ID: "x", Title: "y", PostedAt: time.Now()}
	err := tracker.AddRecentItem(context.Background(), item)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "add recent item")
}

func TestTracker_UpdateLastSync_RedisDown(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, nil)
	mr.Close()

	err := tracker.UpdateLastSync(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update last sync")
}

func TestTracker_IncrementBackfillTotal_RedisDown(t *testing.T) {
	t.Helper()

	tracker, mr := setupTracker(t, nil)
	mr.Close()

	err := tracker.IncrementBackfillTotal(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "increment backfill_total counter")
}
