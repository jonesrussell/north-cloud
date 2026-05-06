package runner

import (
	"context"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// --- mockFetcher ---

type mockFetcher struct {
	out *rss.FetchOutput
	err error
}

func (m *mockFetcher) Fetch(_ context.Context, _ rss.FetchInput) (*rss.FetchOutput, error) {
	return m.out, m.err
}

// --- mockStore ---

type storeCall struct {
	method string
	args   []any
}

type mockStore struct {
	calls []storeCall

	checkpoints          map[string]*catalogue.PollCheckpoint
	catalogEntries       map[string]*catalogue.CatalogEntry
	consecutiveFailures  map[string]int
	rescindAbsentReturn  []string
	rescindAbsentErr     error
	loadCheckpointErr    error
	saveCheckpointErr    error
	markSeenErr          error
	markRescindedErr     error
	lookupAlertErr       error
	incrementFailuresErr error
}

func newMockStore() *mockStore {
	return &mockStore{
		checkpoints:         make(map[string]*catalogue.PollCheckpoint),
		catalogEntries:      make(map[string]*catalogue.CatalogEntry),
		consecutiveFailures: make(map[string]int),
	}
}

func (m *mockStore) record(method string, args ...any) {
	m.calls = append(m.calls, storeCall{method: method, args: args})
}

func (m *mockStore) LoadCheckpoint(_ context.Context, sourceID, feedURL string) (*catalogue.PollCheckpoint, error) {
	m.record("LoadCheckpoint", sourceID, feedURL)
	if m.loadCheckpointErr != nil {
		return nil, m.loadCheckpointErr
	}
	key := sourceID + "|" + feedURL
	return m.checkpoints[key], nil
}

func (m *mockStore) SaveCheckpoint(_ context.Context, c catalogue.PollCheckpoint) error {
	m.record("SaveCheckpoint", c)
	if m.saveCheckpointErr != nil {
		return m.saveCheckpointErr
	}
	key := c.SourceID + "|" + c.FeedURL
	cp := c
	m.checkpoints[key] = &cp
	return nil
}

func (m *mockStore) IncrementConsecutiveFailures(_ context.Context, sourceID, feedURL string) error {
	m.record("IncrementConsecutiveFailures", sourceID, feedURL)
	if m.incrementFailuresErr != nil {
		return m.incrementFailuresErr
	}
	key := sourceID + "|" + feedURL
	cp := m.checkpoints[key]
	if cp == nil {
		cp = &catalogue.PollCheckpoint{SourceID: sourceID, FeedURL: feedURL}
	}
	// Increment from the checkpoint's current value (not a separate counter)
	// so that pre-seeded checkpoints are respected.
	cp.ConsecutiveFailures++
	m.consecutiveFailures[key] = cp.ConsecutiveFailures
	m.checkpoints[key] = cp
	return nil
}

func (m *mockStore) ResetConsecutiveFailures(_ context.Context, sourceID, feedURL string) error {
	m.record("ResetConsecutiveFailures", sourceID, feedURL)
	key := sourceID + "|" + feedURL
	m.consecutiveFailures[key] = 0
	return nil
}

func (m *mockStore) LookupAlert(_ context.Context, sourceID, alertID string) (*catalogue.CatalogEntry, error) {
	m.record("LookupAlert", sourceID, alertID)
	if m.lookupAlertErr != nil {
		return nil, m.lookupAlertErr
	}
	key := sourceID + "|" + alertID
	e := m.catalogEntries[key]
	if e == nil {
		return nil, catalogue.ErrNotFound
	}
	return e, nil
}

func (m *mockStore) MarkSeen(_ context.Context, e catalogue.CatalogEntry) error {
	m.record("MarkSeen", e)
	if m.markSeenErr != nil {
		return m.markSeenErr
	}
	key := e.SourceID + "|" + e.AlertID
	entry := e
	m.catalogEntries[key] = &entry
	return nil
}

func (m *mockStore) RescindAbsent(_ context.Context, sourceID string, pollStartedAt time.Time) ([]string, error) {
	m.record("RescindAbsent", sourceID, pollStartedAt)
	return m.rescindAbsentReturn, m.rescindAbsentErr
}

func (m *mockStore) MarkRescinded(_ context.Context, sourceID, alertID string) error {
	m.record("MarkRescinded", sourceID, alertID)
	return m.markRescindedErr
}

// countCalls returns the number of times a method was called.
func (m *mockStore) countCalls(method string) int {
	n := 0
	for _, c := range m.calls {
		if c.method == method {
			n++
		}
	}
	return n
}

// --- mockIndexer ---

type indexerCall struct {
	method  string
	alertID string
}

type mockIndexer struct {
	calls      []indexerCall
	indexErr   error
	rescindErr error
}

func (m *mockIndexer) Index(_ context.Context, alert domain.Alert) error {
	m.calls = append(m.calls, indexerCall{method: "Index", alertID: alert.ID})
	return m.indexErr
}

func (m *mockIndexer) MarkRescinded(_ context.Context, alertID string, _ time.Time, _ string) error {
	m.calls = append(m.calls, indexerCall{method: "MarkRescinded", alertID: alertID})
	return m.rescindErr
}

func (m *mockIndexer) countCalls(method string) int {
	n := 0
	for _, c := range m.calls {
		if c.method == method {
			n++
		}
	}
	return n
}

// --- mockPublisher ---

type mockPublisher struct {
	events []domain.LifecycleEvent
	err    error
}

func (m *mockPublisher) Publish(_ context.Context, event domain.LifecycleEvent) error {
	m.events = append(m.events, event)
	return m.err
}

func (m *mockPublisher) countByType(et domain.EventType) int {
	n := 0
	for i := range m.events {
		if m.events[i].EventType == et {
			n++
		}
	}
	return n
}

// --- mockResolver ---

type mockResolver struct {
	scope []string
}

func (m *mockResolver) Resolve(_ domain.AlertSource, _ string) []string {
	if m.scope != nil {
		return m.scope
	}
	return []string{"test-region"}
}

// --- helpers ---

// minimalFeed returns a minimal valid RSS feed body for one item.
func minimalFeed(link, pubDate, title, description string) []byte {
	return []byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Test</title><item>` +
		`<title>` + title + `</title>` +
		`<link>` + link + `</link>` +
		`<pubDate>` + pubDate + `</pubDate>` +
		`<description>` + description + `</description>` +
		`</item></channel></rss>`)
}

// testSource returns an enabled AlertSource for use in tests.
func testSource() domain.AlertSource {
	return domain.AlertSource{
		ID:           "src-01",
		Name:         "Test Source",
		FeedURL:      "https://example.com/feed.rss",
		Enabled:      true,
		DefaultScope: []string{"region-a"},
		PollInterval: 30 * time.Minute,
	}
}
