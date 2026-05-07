package runner

import (
	"context"
	"fmt"
	"strings"
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

// multiFeed returns a valid RSS 2.0 body with n items, each with a unique
// GUID, link, and title so ParseItem produces distinct Alert IDs.
func multiFeed(n int) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>`)
	for i := range n {
		guid := fmt.Sprintf("https://example.com/alerts/%04d", i+1)
		fmt.Fprintf(&b,
			`<item><title>Alert %d</title><link>%s</link><guid>%s</guid>`+
				`<pubDate>Tue, 06 May 2026 10:00:00 +0000</pubDate>`+
				`<description>Opioid supply warning.</description></item>`,
			i+1, guid, guid,
		)
	}
	b.WriteString(`</channel></rss>`)
	return []byte(b.String())
}

// enrichedHash applies the same enrichment as processBackfillItem and returns
// the contentHash that the backfill pipeline will produce for a given item.
// This lets tests pre-seed catalogue entries with hashes that match.
func enrichedHash(alert domain.Alert, src domain.AlertSource) string {
	alert.Severity = domain.SeverityMedium // mirrors SevInfer in newBackfillDeps
	alert.Scope = []string{"test-region"}  // mirrors mockResolver.Resolve default
	if src.DefaultExpiry > 0 {
		e := alert.IssuedAt.Add(src.DefaultExpiry)
		alert.ExpiresAt = &e
	}
	return contentHash(alert)
}

// newBackfillDeps builds a Dependencies struct wired for backfill unit tests.
func newBackfillDeps(
	t *testing.T,
	fetch fetcher,
	store store,
	indexer indexer,
	pub publisher,
) Dependencies {
	t.Helper()
	log, err := infralogger.New(infralogger.Config{Level: "error", Format: "json"})
	require.NoError(t, err)
	metrics := observability.New(log)
	return Dependencies{
		Fetch:    fetch,
		Store:    store,
		Indexer:  indexer,
		Pub:      pub,
		Resolver: &mockResolver{},
		SevInfer: func(_ domain.Hazard) domain.Severity { return domain.SeverityMedium },
		Metrics:  metrics,
		Sources:  []domain.AlertSource{testSource()},
		Now:      func() time.Time { return time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC) },
	}
}

// TestBackfill_EmptyCatalogue_Writes20 verifies that when the catalogue is
// empty and the feed has exactly 20 items, Backfill writes all 20 to ES and
// publishes 20 EventCreated events.
func TestBackfill_EmptyCatalogue_Writes20(t *testing.T) {
	t.Parallel()

	store := newMockStore()
	indexer := &mockIndexer{}
	pub := &mockPublisher{}
	fetcher := &mockFetcher{out: &rss.FetchOutput{
		Body:       multiFeed(backfillLimit),
		StatusCode: 200,
	}}

	deps := newBackfillDeps(t, fetcher, store, indexer, pub)
	r := New(deps)

	require.NoError(t, r.Backfill(context.Background()))

	assert.Equal(t, backfillLimit, indexer.countCalls("Index"), "expected 20 ES writes")
	assert.Equal(t, backfillLimit, pub.countByType(domain.EventCreated), "expected 20 EventCreated")
	assert.Equal(t, 1, store.countCalls("SaveCheckpoint"), "expected checkpoint saved once")
}

// TestBackfill_FullCatalogue_NoOp verifies that when the catalogue already
// contains all feed items with matching content hashes, Backfill is a no-op:
// no ES writes and no events.
func TestBackfill_FullCatalogue_NoOp(t *testing.T) {
	t.Parallel()

	store := newMockStore()
	indexer := &mockIndexer{}
	pub := &mockPublisher{}

	// Pre-populate the catalogue with all 20 items.
	feedBody := multiFeed(backfillLimit)
	feed, parseErr := rss.ParseFeed(feedBody)
	require.NoError(t, parseErr)

	src := testSource()
	for i := range feed.Channel.Items {
		alert, err := rss.ParseItem(feed.Channel.Items[i], src)
		require.NoError(t, err)
		hash := enrichedHash(alert, src)
		key := src.ID + "|" + alert.ID
		entry := catalogue.CatalogEntry{
			SourceID:    src.ID,
			AlertID:     alert.ID,
			LastSeenAt:  time.Now(),
			IsActive:    true,
			ContentHash: hash,
		}
		store.catalogEntries[key] = &entry
	}

	fetcher := &mockFetcher{out: &rss.FetchOutput{
		Body:       feedBody,
		StatusCode: 200,
	}}

	deps := newBackfillDeps(t, fetcher, store, indexer, pub)
	r := New(deps)

	require.NoError(t, r.Backfill(context.Background()))

	assert.Equal(t, 0, indexer.countCalls("Index"), "no ES writes expected: all items already catalogued")
	assert.Empty(t, pub.events, "no events expected: all items already catalogued")
}

// TestBackfill_PartialCatalogue_OnlyWritesNew verifies that when the catalogue
// contains 10 of 20 feed items, Backfill writes only the 10 new items.
func TestBackfill_PartialCatalogue_OnlyWritesNew(t *testing.T) {
	t.Parallel()

	const totalItems = backfillLimit
	const preSeeded = 10

	store := newMockStore()
	indexer := &mockIndexer{}
	pub := &mockPublisher{}

	feedBody := multiFeed(totalItems)
	feed, parseErr := rss.ParseFeed(feedBody)
	require.NoError(t, parseErr)

	src := testSource()
	// Pre-seed only the first preSeeded items.
	for i := range preSeeded {
		alert, err := rss.ParseItem(feed.Channel.Items[i], src)
		require.NoError(t, err)
		hash := enrichedHash(alert, src)
		key := src.ID + "|" + alert.ID
		entry := catalogue.CatalogEntry{
			SourceID:    src.ID,
			AlertID:     alert.ID,
			LastSeenAt:  time.Now(),
			IsActive:    true,
			ContentHash: hash,
		}
		store.catalogEntries[key] = &entry
	}

	fetcher := &mockFetcher{out: &rss.FetchOutput{
		Body:       feedBody,
		StatusCode: 200,
	}}

	deps := newBackfillDeps(t, fetcher, store, indexer, pub)
	r := New(deps)

	require.NoError(t, r.Backfill(context.Background()))

	const expectedNew = totalItems - preSeeded
	assert.Equal(t, expectedNew, indexer.countCalls("Index"), "expected only new items written to ES")
	assert.Equal(t, expectedNew, pub.countByType(domain.EventCreated), "expected only new EventCreated events")
}

// TestBackfill_HonorsLimit verifies that when the feed has more than
// backfillLimit items, only the first backfillLimit are processed.
func TestBackfill_HonorsLimit(t *testing.T) {
	t.Parallel()

	const feedSize = backfillLimit + 10 // 30 items in feed

	store := newMockStore()
	indexer := &mockIndexer{}
	pub := &mockPublisher{}
	fetcher := &mockFetcher{out: &rss.FetchOutput{
		Body:       multiFeed(feedSize),
		StatusCode: 200,
	}}

	deps := newBackfillDeps(t, fetcher, store, indexer, pub)
	r := New(deps)

	require.NoError(t, r.Backfill(context.Background()))

	assert.Equal(t, backfillLimit, indexer.countCalls("Index"),
		"expected exactly backfillLimit=%d ES writes, not feedSize=%d", backfillLimit, feedSize)
	assert.Equal(t, backfillLimit, pub.countByType(domain.EventCreated),
		"expected exactly backfillLimit=%d EventCreated events", backfillLimit)
}
