package catalogue_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
)

// openMemStore opens an in-memory SQLite store and returns it with a cleanup func.
func openMemStore(t *testing.T) *catalogue.Store {
	t.Helper()

	s, err := catalogue.Open(context.Background(), ":memory:")
	require.NoError(t, err, "Open should succeed with in-memory SQLite")

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func TestOpen_RunsMigrations(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)

	// If migrations ran, SaveCheckpoint (which requires the table) should succeed.
	cp := catalogue.PollCheckpoint{
		SourceID:     "src1",
		FeedURL:      "https://example.com/feed",
		LastPolledAt: time.Now().UTC(),
	}

	err := s.SaveCheckpoint(context.Background(), cp)
	assert.NoError(t, err, "tables must exist after Open runs migrations")
}

func TestSaveAndLoadCheckpoint(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)
	ctx := context.Background()

	want := catalogue.PollCheckpoint{
		SourceID:            "src-save",
		FeedURL:             "https://feed.example.com/rss",
		LastPolledAt:        time.Now().UTC().Truncate(time.Second),
		LastEtag:            `"abc123"`,
		LastModified:        "Wed, 06 May 2026 00:00:00 GMT",
		LastStatus:          200,
		ConsecutiveFailures: 0,
	}

	require.NoError(t, s.SaveCheckpoint(ctx, want))

	got, err := s.LoadCheckpoint(ctx, want.SourceID, want.FeedURL)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, want.SourceID, got.SourceID)
	assert.Equal(t, want.FeedURL, got.FeedURL)
	assert.Equal(t, want.LastEtag, got.LastEtag)
	assert.Equal(t, want.LastModified, got.LastModified)
	assert.Equal(t, want.LastStatus, got.LastStatus)
	assert.Equal(t, want.ConsecutiveFailures, got.ConsecutiveFailures)
	assert.WithinDuration(t, want.LastPolledAt, got.LastPolledAt, time.Second)
}

func TestLoadCheckpoint_Missing(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)

	got, err := s.LoadCheckpoint(context.Background(), "no-source", "https://missing.example.com/feed")
	require.NoError(t, err)
	assert.Nil(t, got, "missing checkpoint should return nil, nil")
}

func TestIncrementAndResetFailures(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)
	ctx := context.Background()

	const sourceID = "src-fail"
	const feedURL = "https://fail.example.com/rss"

	// No prior row — increment should insert with count = 1.
	require.NoError(t, s.IncrementConsecutiveFailures(ctx, sourceID, feedURL))

	got, err := s.LoadCheckpoint(ctx, sourceID, feedURL)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.ConsecutiveFailures)

	// Second increment — counter must reach 2.
	require.NoError(t, s.IncrementConsecutiveFailures(ctx, sourceID, feedURL))

	got, err = s.LoadCheckpoint(ctx, sourceID, feedURL)
	require.NoError(t, err)
	assert.Equal(t, 2, got.ConsecutiveFailures)

	// Reset — counter must return to 0.
	require.NoError(t, s.ResetConsecutiveFailures(ctx, sourceID, feedURL))

	got, err = s.LoadCheckpoint(ctx, sourceID, feedURL)
	require.NoError(t, err)
	assert.Equal(t, 0, got.ConsecutiveFailures)
}

func TestMarkSeen_Upserts(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)
	ctx := context.Background()

	entry := catalogue.CatalogEntry{
		SourceID:    "src-seen",
		AlertID:     "alert-001",
		LastSeenAt:  time.Now().UTC().Truncate(time.Second),
		IsActive:    true,
		ContentHash: "hash-v1",
	}

	// First insert.
	require.NoError(t, s.MarkSeen(ctx, entry))

	got, err := s.LookupAlert(ctx, entry.SourceID, entry.AlertID)
	require.NoError(t, err)
	assert.True(t, got.IsActive)
	assert.Equal(t, "hash-v1", got.ContentHash)

	// Simulate rescission then re-seeing — is_active must flip back to true.
	require.NoError(t, s.MarkRescinded(ctx, entry.SourceID, entry.AlertID))

	rescinded, err := s.LookupAlert(ctx, entry.SourceID, entry.AlertID)
	require.NoError(t, err)
	assert.False(t, rescinded.IsActive, "should be inactive after rescission")

	entry.ContentHash = "hash-v2"
	require.NoError(t, s.MarkSeen(ctx, entry))

	reactivated, err := s.LookupAlert(ctx, entry.SourceID, entry.AlertID)
	require.NoError(t, err)
	assert.True(t, reactivated.IsActive, "re-seeing a rescinded alert must flip is_active back to true")
	assert.Equal(t, "hash-v2", reactivated.ContentHash)
}

func TestRescindAbsent(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)
	ctx := context.Background()

	pollStart := time.Now().UTC().Truncate(time.Second)

	const sourceID = "src-rescind"

	// Insert three alerts; two with last_seen_at >= pollStart, one before it.
	seenBefore := catalogue.CatalogEntry{
		SourceID:   sourceID,
		AlertID:    "alert-stale",
		LastSeenAt: pollStart.Add(-10 * time.Minute),
		IsActive:   true,
	}
	seenDuring1 := catalogue.CatalogEntry{
		SourceID:   sourceID,
		AlertID:    "alert-seen-1",
		LastSeenAt: pollStart.Add(time.Second),
		IsActive:   true,
	}
	seenDuring2 := catalogue.CatalogEntry{
		SourceID:   sourceID,
		AlertID:    "alert-seen-2",
		LastSeenAt: pollStart.Add(2 * time.Second),
		IsActive:   true,
	}

	require.NoError(t, s.MarkSeen(ctx, seenBefore))
	require.NoError(t, s.MarkSeen(ctx, seenDuring1))
	require.NoError(t, s.MarkSeen(ctx, seenDuring2))

	ids, err := s.RescindAbsent(ctx, sourceID, pollStart)
	require.NoError(t, err)

	assert.Len(t, ids, 1)
	assert.Equal(t, "alert-stale", ids[0])
}

func TestLookupAlert_NotFound(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)

	_, err := s.LookupAlert(context.Background(), "no-src", "no-alert")
	assert.ErrorIs(t, err, catalogue.ErrNotFound)
}

func TestOpen_BadPath(t *testing.T) {
	t.Parallel()

	// A path that cannot be created (directory, not a file).
	_, err := catalogue.Open(context.Background(), "/dev/null/impossible/path/alerts.db")
	assert.Error(t, err, "Open with invalid path must return an error")
}

func TestStore_Close(t *testing.T) {
	t.Parallel()

	s, err := catalogue.Open(context.Background(), ":memory:")
	require.NoError(t, err)

	// Close must succeed on a healthy store.
	assert.NoError(t, s.Close())
}

func TestRescindAbsent_NoActive(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)
	ctx := context.Background()

	// No alerts inserted — result must be empty, no error.
	ids, err := s.RescindAbsent(ctx, "src-empty", time.Now().UTC())
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestSaveCheckpoint_Upsert(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)
	ctx := context.Background()

	cp := catalogue.PollCheckpoint{
		SourceID:     "src-upsert",
		FeedURL:      "https://upsert.example.com/rss",
		LastPolledAt: time.Now().UTC().Truncate(time.Second),
		LastStatus:   200,
	}

	require.NoError(t, s.SaveCheckpoint(ctx, cp))

	// Update the same checkpoint — status changes.
	cp.LastStatus = 304
	require.NoError(t, s.SaveCheckpoint(ctx, cp))

	got, err := s.LoadCheckpoint(ctx, cp.SourceID, cp.FeedURL)
	require.NoError(t, err)
	assert.Equal(t, 304, got.LastStatus)
}

func TestOpen_FileBasedDB(t *testing.T) {
	t.Parallel()

	// Use a real temp file to exercise the ping + WAL journal path in Open.
	dir := t.TempDir()
	path := dir + "/test.db"

	s, err := catalogue.Open(context.Background(), path)
	require.NoError(t, err, "Open must succeed with a writable temp-file path")

	// Tables created by migration must be usable.
	cp := catalogue.PollCheckpoint{
		SourceID:     "src-file",
		FeedURL:      "https://file.example.com/rss",
		LastPolledAt: time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, s.SaveCheckpoint(context.Background(), cp))
	require.NoError(t, s.Close())
}
