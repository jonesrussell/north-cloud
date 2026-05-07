// Package main_test contains an end-to-end smoke test for the alert-crawler
// wiring. It exercises the full dependency graph (runner.New + all collaborators)
// against httptest fixtures, without invoking main() directly.
//
// Coverage of main() itself is intentionally omitted: flag parsing and os.Exit
// paths are not testable without subprocess tricks that add more complexity than
// value. The smoke test below validates that the dependency graph wires correctly
// and that one complete poll cycle produces an ES write and a Redis publish.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/observability"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/runner"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/scope"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/severity"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// minimalRSSFeed returns a well-formed RSS 2.0 body with one item.
const minimalRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Alert Feed</title>
    <link>https://example.com/alerts</link>
    <description>Test</description>
    <item>
      <title>Smoke Test Alert</title>
      <link>https://example.com/alerts/smoke-001</link>
      <guid>https://example.com/alerts/smoke-001</guid>
      <pubDate>Tue, 06 May 2026 10:00:00 +0000</pubDate>
      <description>Opioid supply warning. Fentanyl detected in supply.</description>
    </item>
  </channel>
</rss>`

// mockRedisPublisher records Publish calls without a real Redis connection.
type mockRedisPublisher struct {
	mu     sync.Mutex
	events []domain.LifecycleEvent
}

func (m *mockRedisPublisher) Publish(_ context.Context, event domain.LifecycleEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockRedisPublisher) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

// mockESHandler records PUT _doc requests and returns 201 Created.
type mockESHandler struct {
	mu       sync.Mutex
	indexed  []json.RawMessage
	mappings int
}

func (h *mockESHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch {
	case r.Method == http.MethodHead:
		// Simulate index not found so EnsureIndex calls PUT mapping.
		w.WriteHeader(http.StatusNotFound)

	case r.Method == http.MethodPut && r.URL.Path != "" && !isDocPath(r.URL.Path):
		// PUT /<index> — mapping creation.
		h.mappings++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"acknowledged":true}`))

	case r.Method == http.MethodPut && isDocPath(r.URL.Path):
		// PUT /<index>/_doc/<id>
		var body json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		h.indexed = append(h.indexed, body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"result":"created"}`))

	default:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}
}

func (h *mockESHandler) indexedCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.indexed)
}

// isDocPath returns true when the URL path contains "_doc".
func isDocPath(path string) bool {
	for i := range path {
		if i+4 < len(path) && path[i:i+4] == "_doc" {
			return true
		}
	}
	return false
}

// TestSmoke_RunnerEndToEnd verifies that one poll cycle produces one ES write
// and one Redis publish when given a valid RSS fixture feed.
func TestSmoke_RunnerEndToEnd(t *testing.T) {
	t.Parallel()

	// Start mock RSS server.
	rssSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalRSSFeed))
	}))
	defer rssSrv.Close()

	// Start mock ES server.
	esHandler := &mockESHandler{}
	esSrv := httptest.NewServer(esHandler)
	defer esSrv.Close()

	// Build the full dependency graph (mirrors main.go wiring).
	log, err := infralogger.New(infralogger.Config{Level: "error", Format: "json"})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	store, err := catalogue.Open(context.Background(), tmpDir+"/alerts.db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	indexer := elasticsearch.New(elasticsearch.Config{
		BaseURL: esSrv.URL,
		Index:   "alert_classified_content",
	})
	require.NoError(t, indexer.EnsureIndex(context.Background()))

	pub := &mockRedisPublisher{}
	metrics := observability.New(log)
	resolver := scope.New()
	sevTable := severity.NewTable(nil)
	sevInfer := func(h domain.Hazard) domain.Severity {
		return severity.Infer(h, sevTable)
	}

	src := domain.AlertSource{
		ID:      "smoke-test-source",
		FeedURL: rssSrv.URL,
		Enabled: true,
	}

	fetcher := rss.New(rss.WithHTTPClient(rssSrv.Client()))

	r := runner.New(runner.Dependencies{
		Fetch:         fetcher,
		Store:         store,
		Indexer:       indexer,
		Pub:           pub,
		Resolver:      resolver,
		SevInfer:      sevInfer,
		Metrics:       metrics,
		Sources:       []domain.AlertSource{src},
		DefaultExpiry: defaultExpiry,
		Now:           func() time.Time { return time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC) },
	})

	runErr := r.Run(context.Background())
	require.NoError(t, runErr)

	// One alert document must have reached ES.
	assert.Equal(t, 1, esHandler.indexedCount(), "expected one alert indexed in ES")

	// One lifecycle event must have been published to Redis.
	assert.Equal(t, 1, pub.count(), "expected one lifecycle event published to Redis")
}

// rssNItems returns a valid RSS 2.0 feed body with n unique items.
func rssNItems(n int) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Backfill Test</title>`)
	for i := range n {
		guid := fmt.Sprintf("https://example.com/alerts/bf-%04d", i+1)
		fmt.Fprintf(&b,
			`<item><title>Backfill Alert %d</title><link>%s</link><guid>%s</guid>`+
				`<pubDate>Tue, 06 May 2026 10:00:00 +0000</pubDate>`+
				`<description>Opioid supply warning. Fentanyl detected.</description></item>`,
			i+1, guid, guid,
		)
	}
	b.WriteString(`</channel></rss>`)
	return []byte(b.String())
}

// TestSmoke_BackfillEndToEnd verifies that:
//  1. A first Backfill call with 20-item feed writes 20 docs to ES and emits
//     20 EventCreated events.
//  2. A second Backfill call (idempotency check) against the same feed produces
//     no additional ES writes and no additional events.
func TestSmoke_BackfillEndToEnd(t *testing.T) {
	t.Parallel()

	const itemCount = 20

	feedBody := rssNItems(itemCount)

	// Start mock RSS server — always returns the same feed.
	rssSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(feedBody)
	}))
	defer rssSrv.Close()

	// Start mock ES server.
	esHandler := &mockESHandler{}
	esSrv := httptest.NewServer(esHandler)
	defer esSrv.Close()

	// Build the full dependency graph.
	log, err := infralogger.New(infralogger.Config{Level: "error", Format: "json"})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	store, storeErr := catalogue.Open(context.Background(), tmpDir+"/alerts.db")
	require.NoError(t, storeErr)
	t.Cleanup(func() { _ = store.Close() })

	indexer := elasticsearch.New(elasticsearch.Config{
		BaseURL: esSrv.URL,
		Index:   "alert_classified_content",
	})
	require.NoError(t, indexer.EnsureIndex(context.Background()))

	pub := &mockRedisPublisher{}
	metrics := observability.New(log)
	resolver := scope.New()
	sevTable := severity.NewTable(nil)
	sevInfer := func(h domain.Hazard) domain.Severity { return severity.Infer(h, sevTable) }

	src := domain.AlertSource{
		ID:      "backfill-smoke-source",
		FeedURL: rssSrv.URL,
		Enabled: true,
	}

	fetcher := rss.New(rss.WithHTTPClient(rssSrv.Client()))

	r := runner.New(runner.Dependencies{
		Fetch:         fetcher,
		Store:         store,
		Indexer:       indexer,
		Pub:           pub,
		Resolver:      resolver,
		SevInfer:      sevInfer,
		Metrics:       metrics,
		Sources:       []domain.AlertSource{src},
		DefaultExpiry: defaultExpiry,
		Now:           func() time.Time { return time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC) },
	})

	// First backfill: all items are new.
	require.NoError(t, r.Backfill(context.Background()))
	assert.Equal(t, itemCount, esHandler.indexedCount(), "first backfill: expected %d ES writes", itemCount)
	assert.Equal(t, itemCount, pub.count(), "first backfill: expected %d EventCreated events", itemCount)

	// Second backfill: all items already in catalogue — must be a no-op.
	countBefore := esHandler.indexedCount()
	eventsBefore := pub.count()
	require.NoError(t, r.Backfill(context.Background()))
	assert.Equal(t, countBefore, esHandler.indexedCount(), "second backfill must not write additional ES docs")
	assert.Equal(t, eventsBefore, pub.count(), "second backfill must not emit additional events")
}
