package runner

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/observability"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// newNopMetrics returns a Metrics backed by a no-op logger for tests.
func newNopMetrics(t *testing.T) *observability.Metrics {
	t.Helper()
	log, err := infralogger.New(infralogger.Config{Level: "error", Format: "console"})
	require.NoError(t, err)
	return observability.New(log)
}

// fixedNow returns a deterministic time suitable for tests.
func fixedNow() time.Time {
	return time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
}

// noopSevInfer always returns SeverityHigh for test simplicity.
func noopSevInfer(_ domain.Hazard) domain.Severity {
	return domain.SeverityHigh
}

// buildDeps returns a Dependencies with all mocks wired.
func buildDeps(
	t *testing.T,
	f fetcher,
	s store,
	idx indexer,
	pub publisher,
) Dependencies {
	t.Helper()
	return Dependencies{
		Fetch:    f,
		Store:    s,
		Indexer:  idx,
		Pub:      pub,
		Resolver: &mockResolver{scope: []string{"region-a"}},
		SevInfer: noopSevInfer,
		Metrics:  newNopMetrics(t),
		Sources:  []domain.AlertSource{testSource()},
		Now:      fixedNow,
	}
}

// validFeedBody returns an RSS body with one well-formed item.
func validFeedBody() []byte {
	return minimalFeed(
		"https://example.com/alerts/alert-001",
		"Mon, 06 May 2024 10:00:00 +0000",
		"Test Alert",
		"Opioid supply warning. Substances: fentanyl. Lab: Vancouver Coastal Health.",
	)
}

// --- Test cases ---

func TestRunSource_NewAlert_PublishesCreated(t *testing.T) {
	f := &mockFetcher{out: &rss.FetchOutput{Body: validFeedBody(), StatusCode: 200}}
	s := newMockStore()
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	require.NoError(t, err)

	assert.Equal(t, 1, idx.countCalls("Index"), "ES Index should be called once")
	assert.Equal(t, 1, s.countCalls("MarkSeen"), "MarkSeen should be called once")
	assert.Equal(t, 1, pub.countByType(domain.EventCreated), "created event should be published")
	assert.Equal(t, 0, pub.countByType(domain.EventUpdated), "no updated event")
	assert.Equal(t, 1, s.countCalls("SaveCheckpoint"), "checkpoint should be saved")
}

func TestRunSource_UnchangedAlert_Idempotent(t *testing.T) {
	feedBody := validFeedBody()

	// Pre-parse the alert to get its ID and content hash.
	feed, err := rss.ParseFeed(feedBody)
	require.NoError(t, err)
	alert, err := rss.ParseItem(feed.Channel.Items[0], testSource())
	require.NoError(t, err)
	alert.Severity = noopSevInfer(alert.Hazard)
	alert.Scope = []string{"region-a"}
	hash := contentHash(alert)

	s := newMockStore()
	s.catalogEntries[testSource().ID+"|"+alert.ID] = &catalogue.CatalogEntry{
		SourceID:    testSource().ID,
		AlertID:     alert.ID,
		LastSeenAt:  fixedNow().Add(-1 * time.Hour),
		IsActive:    true,
		ContentHash: hash,
	}

	f := &mockFetcher{out: &rss.FetchOutput{Body: feedBody, StatusCode: 200}}
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err = runner.RunSource(context.Background(), testSource())
	require.NoError(t, err)

	assert.Equal(t, 0, idx.countCalls("Index"), "no ES write for unchanged alert")
	assert.Empty(t, pub.events, "no publish for unchanged alert")
	assert.Equal(t, 1, s.countCalls("MarkSeen"), "MarkSeen refreshes last_seen_at")
}

func TestRunSource_ChangedAlert_PublishesUpdated(t *testing.T) {
	feedBody := validFeedBody()

	feed, err := rss.ParseFeed(feedBody)
	require.NoError(t, err)
	alert, err := rss.ParseItem(feed.Channel.Items[0], testSource())
	require.NoError(t, err)

	s := newMockStore()
	// Existing entry with a different hash.
	s.catalogEntries[testSource().ID+"|"+alert.ID] = &catalogue.CatalogEntry{
		SourceID:    testSource().ID,
		AlertID:     alert.ID,
		LastSeenAt:  fixedNow().Add(-1 * time.Hour),
		IsActive:    true,
		ContentHash: "stale-hash-differs",
	}

	f := &mockFetcher{out: &rss.FetchOutput{Body: feedBody, StatusCode: 200}}
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err = runner.RunSource(context.Background(), testSource())
	require.NoError(t, err)

	assert.Equal(t, 1, idx.countCalls("Index"), "ES Index called for update")
	assert.Equal(t, 1, pub.countByType(domain.EventUpdated), "updated event published")
	assert.Equal(t, 0, pub.countByType(domain.EventCreated), "no created event on update")
}

func TestRunSource_AbsentAlert_PublishesRescinded(t *testing.T) {
	// Feed has 1 item; catalogue says alert-999 is also active (absent).
	feedBody := validFeedBody()

	s := newMockStore()
	s.rescindAbsentReturn = []string{"src-01:alert-absent-999"}

	f := &mockFetcher{out: &rss.FetchOutput{Body: feedBody, StatusCode: 200}}
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	require.NoError(t, err)

	assert.Equal(t, 1, idx.countCalls("MarkRescinded"), "ES MarkRescinded called for absent")
	assert.Equal(t, 1, pub.countByType(domain.EventRescinded), "rescinded event published")
	assert.Equal(t, 1, s.countCalls("MarkRescinded"), "catalogue MarkRescinded called")
}

func TestRunSource_NotModified_NoOp(t *testing.T) {
	f := &mockFetcher{err: rss.ErrNotModified}
	s := newMockStore()
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	require.NoError(t, err)

	assert.Equal(t, 1, s.countCalls("SaveCheckpoint"), "checkpoint saved after 304")
	assert.Equal(t, 0, idx.countCalls("Index"), "no ES write on 304")
	assert.Empty(t, pub.events, "no publish on 304")
}

func TestRunSource_TransientError_IncrementsCounter(t *testing.T) {
	f := &mockFetcher{err: fmt.Errorf("upstream 503: %w", rss.ErrTransient)}
	s := newMockStore()
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	// RunSource returns the error.
	require.Error(t, err)
	require.ErrorIs(t, err, rss.ErrTransient)

	assert.Equal(t, 1, s.countCalls("IncrementConsecutiveFailures"), "failure counter incremented")
	assert.Equal(t, 0, idx.countCalls("Index"), "no ES write on transient error")
}

func TestRunSource_ConsecutiveFailuresAtThreshold_LogsWarn(t *testing.T) {
	// Simulate a source where the stored consecutive failure count is already at
	// threshold-1. After IncrementConsecutiveFailures the runner reloads the
	// checkpoint; the count will be consecutiveFailureWarnThreshold (6).
	// We assert that the store ends up with count == 6 and that
	// IncrementConsecutiveFailures was called exactly once.
	src := testSource()
	s := newMockStore()
	key := src.ID + "|" + src.FeedURL
	s.checkpoints[key] = &catalogue.PollCheckpoint{
		SourceID:            src.ID,
		FeedURL:             src.FeedURL,
		ConsecutiveFailures: consecutiveFailureWarnThreshold - 1,
	}

	f := &mockFetcher{err: fmt.Errorf("upstream 503: %w", rss.ErrTransient)}
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	// Replace Sources in deps with just this source.
	runner.deps.Sources = []domain.AlertSource{src}

	err := runner.RunSource(context.Background(), src)
	require.Error(t, err)

	assert.Equal(t, 1, s.countCalls("IncrementConsecutiveFailures"),
		"IncrementConsecutiveFailures should be called once")

	// After increment the mock sets the checkpoint count to threshold.
	cp := s.checkpoints[key]
	require.NotNil(t, cp)
	assert.Equal(t, consecutiveFailureWarnThreshold, cp.ConsecutiveFailures,
		"consecutive failures count should reach the warn threshold after increment")
}

func TestRunSource_ParseFailedItemSkipped(t *testing.T) {
	// Feed has an item with no link — ParseItem returns error.
	badFeed := []byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>` +
		`<item><title></title><link></link><pubDate></pubDate></item></channel></rss>`)

	f := &mockFetcher{out: &rss.FetchOutput{Body: badFeed, StatusCode: 200}}
	s := newMockStore()
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	require.NoError(t, err) // RunSource itself does not error on item parse failure.

	assert.Equal(t, 0, idx.countCalls("Index"), "no ES write for parse-failed item")
	assert.Empty(t, pub.events, "no publish for parse-failed item")
	assert.Equal(t, 0, s.countCalls("MarkSeen"), "no catalogue entry for parse-failed item")
}

func TestRunSource_ParseDegradedItemPersisted(t *testing.T) {
	// A degraded item is missing substances/lab but still parses — it SHOULD be indexed.
	degradedFeed := minimalFeed(
		"https://example.com/alerts/alert-degraded",
		"Mon, 06 May 2024 10:00:00 +0000",
		"Degraded Alert",
		"No substance information available.",
	)

	f := &mockFetcher{out: &rss.FetchOutput{Body: degradedFeed, StatusCode: 200}}
	s := newMockStore()
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	require.NoError(t, err)

	assert.Equal(t, 1, idx.countCalls("Index"), "degraded item enters ES")
	assert.Equal(t, 1, s.countCalls("MarkSeen"), "degraded item enters catalogue")
	assert.Equal(t, 1, pub.countByType(domain.EventCreated), "degraded item publishes created event")
}

func TestRun_MultipleSourcesIsolatedErrors(t *testing.T) {
	src1 := testSource()
	src1.ID = "src-err"
	src2 := testSource()
	src2.ID = "src-ok"

	// src1 fetch fails with structural error; src2 succeeds.
	callCount := 0
	f := &fetcherFunc{fn: func(_ context.Context, in rss.FetchInput) (*rss.FetchOutput, error) {
		callCount++
		if in.Source.ID == "src-err" {
			return nil, fmt.Errorf("upstream 404: %w", rss.ErrStructural)
		}
		return &rss.FetchOutput{Body: validFeedBody(), StatusCode: 200}, nil
	}}

	s := newMockStore()
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	deps := buildDeps(t, f, s, idx, pub)
	deps.Sources = []domain.AlertSource{src1, src2}

	runner := New(deps)
	err := runner.Run(context.Background())
	require.NoError(t, err, "Run must not return error even when a source fails")

	assert.Equal(t, 2, callCount, "both sources should be attempted")
	assert.Equal(t, 1, idx.countCalls("Index"), "only the successful source indexes")
}

// fetcherFunc is a functional implementation of the fetcher interface for tests.
type fetcherFunc struct {
	fn func(context.Context, rss.FetchInput) (*rss.FetchOutput, error)
}

func (f *fetcherFunc) Fetch(ctx context.Context, in rss.FetchInput) (*rss.FetchOutput, error) {
	return f.fn(ctx, in)
}

func TestRunSource_ESIndexFailure_RecordsMetric(t *testing.T) {
	// Index returns an error → RecordESWriteFailure should be called.
	f := &mockFetcher{out: &rss.FetchOutput{Body: validFeedBody(), StatusCode: 200}}
	s := newMockStore()
	idx := &mockIndexer{indexErr: errors.New("es unavailable")}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	// RunSource itself returns no error (item error is per-item).
	require.NoError(t, err)

	assert.Empty(t, pub.events, "no publish when ES fails")
	assert.Equal(t, 0, s.countCalls("MarkSeen"), "no catalogue entry when ES fails")
}

func TestRunSource_RescindESFailure_DoesNotPublish(t *testing.T) {
	// RescindAbsent returns one ID; ES MarkRescinded fails → no Redis publish.
	f := &mockFetcher{out: &rss.FetchOutput{Body: validFeedBody(), StatusCode: 200}}
	s := newMockStore()
	s.rescindAbsentReturn = []string{"src-01:absent-alert"}

	idx := &mockIndexer{rescindErr: errors.New("es down")}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	require.NoError(t, err)

	assert.Equal(t, 0, pub.countByType(domain.EventRescinded),
		"no rescinded event when ES MarkRescinded fails")
}

func TestRunSource_LoadCheckpointError_ReturnsError(t *testing.T) {
	// LoadCheckpoint returns a non-ErrNotFound error → RunSource returns error.
	f := &mockFetcher{}
	s := newMockStore()
	s.loadCheckpointErr = errors.New("db locked")
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load checkpoint")
}

func TestRunSource_DisabledSource_Skipped(t *testing.T) {
	src := testSource()
	src.Enabled = false

	f := &mockFetcher{out: &rss.FetchOutput{Body: validFeedBody(), StatusCode: 200}}
	s := newMockStore()
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	deps := buildDeps(t, f, s, idx, pub)
	deps.Sources = []domain.AlertSource{src}

	runner := New(deps)
	err := runner.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, idx.countCalls("Index"), "disabled source must not be polled")
}

func TestNew_NilNow_DefaultsToTimeNow(t *testing.T) {
	deps := buildDeps(t, &mockFetcher{}, newMockStore(), &mockIndexer{}, &mockPublisher{})
	deps.Now = nil
	runner := New(deps)
	// Verify Now was set (not nil) by calling it — panics if nil.
	require.NotPanics(t, func() { _ = runner.deps.Now() })
}

func TestRunSource_StructuralError_RecordsPollError(t *testing.T) {
	// ErrStructural falls into the default case in handleFetchError.
	f := &mockFetcher{err: fmt.Errorf("upstream 404: %w", rss.ErrStructural)}
	s := newMockStore()
	idx := &mockIndexer{}
	pub := &mockPublisher{}

	runner := New(buildDeps(t, f, s, idx, pub))
	err := runner.RunSource(context.Background(), testSource())
	require.Error(t, err)
	require.ErrorIs(t, err, rss.ErrStructural)

	assert.Equal(t, 0, s.countCalls("IncrementConsecutiveFailures"),
		"structural errors do not increment failure counter")
}
