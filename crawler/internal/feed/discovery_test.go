package feed_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/feed"
)

// --- Test fixtures ---

const htmlWithRSSLink = `<!DOCTYPE html>
<html>
<head>
  <link rel="alternate" type="application/rss+xml" title="RSS" href="/feed.xml">
</head>
<body></body>
</html>`

const htmlWithAtomLink = `<!DOCTYPE html>
<html>
<head>
  <link rel="alternate" type="application/atom+xml" title="Atom" href="/atom.xml">
</head>
<body></body>
</html>`

const htmlWithRelativeFeedLink = `<!DOCTYPE html>
<html>
<head>
  <link rel="alternate" type="application/rss+xml" href="blog/feed">
</head>
<body></body>
</html>`

const htmlWithNoFeedLinks = `<!DOCTYPE html>
<html>
<head>
  <link rel="stylesheet" href="/style.css">
</head>
<body><p>No feeds here</p></body>
</html>`

const validRSSBody = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <item>
      <title>Article</title>
      <link>https://example.com/article</link>
    </item>
  </channel>
</rss>`

// --- Mock implementations ---

// urlMockFetcher returns different responses based on the requested URL.
type urlMockFetcher struct {
	responses map[string]*feed.FetchResponse
	errors    map[string]error
}

func (m *urlMockFetcher) Fetch(
	_ context.Context,
	url string,
	_, _ *string,
) (*feed.FetchResponse, error) {
	if err, ok := m.errors[url]; ok {
		return nil, err
	}

	if resp, ok := m.responses[url]; ok {
		return resp, nil
	}

	return &feed.FetchResponse{StatusCode: http.StatusNotFound}, nil
}

// mockSourceUpdater records calls to UpdateFeedURL.
type mockSourceUpdater struct {
	calls []sourceUpdateCall
	err   error
}

type sourceUpdateCall struct {
	sourceID string
	feedURL  string
}

func (m *mockSourceUpdater) UpdateFeedURL(_ context.Context, sourceID, feedURL string) error {
	m.calls = append(m.calls, sourceUpdateCall{sourceID: sourceID, feedURL: feedURL})
	return m.err
}

// --- Helper ---

// retryAfterForTests is a long duration so recently-attempted checks pass.
const retryAfterForTests = 24 * time.Hour

func newTestDiscoverer(
	t *testing.T,
	fetcher feed.HTTPFetcher,
	updater feed.SourceFeedUpdater,
) *feed.Discoverer {
	t.Helper()

	return feed.NewDiscoverer(fetcher, updater, &mockLogger{}, retryAfterForTests)
}

// --- Tests ---

func TestDiscoverFeed_HTMLRSSLink(t *testing.T) {
	t.Parallel()

	fetcher := &urlMockFetcher{
		responses: map[string]*feed.FetchResponse{
			"https://example.com": {
				StatusCode: http.StatusOK,
				Body:       htmlWithRSSLink,
			},
			"https://example.com/feed.xml": {
				StatusCode: http.StatusOK,
				Body:       validRSSBody,
			},
		},
	}
	updater := &mockSourceUpdater{}
	d := newTestDiscoverer(t, fetcher, updater)

	result := d.DiscoverFeed(context.Background(), "src-1", "https://example.com")

	assertEqual(t, "https://example.com/feed.xml", result)
}

func TestDiscoverFeed_HTMLAtomLink(t *testing.T) {
	t.Parallel()

	fetcher := &urlMockFetcher{
		responses: map[string]*feed.FetchResponse{
			"https://example.com": {
				StatusCode: http.StatusOK,
				Body:       htmlWithAtomLink,
			},
			"https://example.com/atom.xml": {
				StatusCode: http.StatusOK,
				Body:       validRSSBody, // gofeed parses both RSS and Atom
			},
		},
	}
	updater := &mockSourceUpdater{}
	d := newTestDiscoverer(t, fetcher, updater)

	result := d.DiscoverFeed(context.Background(), "src-1", "https://example.com")

	assertEqual(t, "https://example.com/atom.xml", result)
}

func TestDiscoverFeed_RelativeFeedURL(t *testing.T) {
	t.Parallel()

	fetcher := &urlMockFetcher{
		responses: map[string]*feed.FetchResponse{
			"https://example.com": {
				StatusCode: http.StatusOK,
				Body:       htmlWithRelativeFeedLink,
			},
			"https://example.com/blog/feed": {
				StatusCode: http.StatusOK,
				Body:       validRSSBody,
			},
		},
	}
	updater := &mockSourceUpdater{}
	d := newTestDiscoverer(t, fetcher, updater)

	result := d.DiscoverFeed(context.Background(), "src-1", "https://example.com")

	assertEqual(t, "https://example.com/blog/feed", result)
}

func TestDiscoverFeed_CommonPathFallback(t *testing.T) {
	t.Parallel()

	fetcher := &urlMockFetcher{
		responses: map[string]*feed.FetchResponse{
			"https://example.com": {
				StatusCode: http.StatusOK,
				Body:       htmlWithNoFeedLinks,
			},
			// /feed returns 404, but /rss returns valid RSS
			"https://example.com/rss": {
				StatusCode: http.StatusOK,
				Body:       validRSSBody,
			},
		},
	}
	updater := &mockSourceUpdater{}
	d := newTestDiscoverer(t, fetcher, updater)

	result := d.DiscoverFeed(context.Background(), "src-1", "https://example.com")

	assertEqual(t, "https://example.com/rss", result)
}

func TestDiscoverFeed_NoFeedsFound(t *testing.T) {
	t.Parallel()

	fetcher := &urlMockFetcher{
		responses: map[string]*feed.FetchResponse{
			"https://example.com": {
				StatusCode: http.StatusOK,
				Body:       htmlWithNoFeedLinks,
			},
		},
	}
	updater := &mockSourceUpdater{}
	d := newTestDiscoverer(t, fetcher, updater)

	result := d.DiscoverFeed(context.Background(), "src-1", "https://example.com")

	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestDiscoverFeed_RecentlyAttemptedSkipped(t *testing.T) {
	t.Parallel()

	fetcher := &urlMockFetcher{
		responses: map[string]*feed.FetchResponse{
			"https://example.com": {
				StatusCode: http.StatusOK,
				Body:       htmlWithRSSLink,
			},
			"https://example.com/feed.xml": {
				StatusCode: http.StatusOK,
				Body:       validRSSBody,
			},
		},
	}
	updater := &mockSourceUpdater{}
	d := newTestDiscoverer(t, fetcher, updater)

	// First attempt discovers the feed.
	first := d.DiscoverFeed(context.Background(), "src-1", "https://example.com")
	assertEqual(t, "https://example.com/feed.xml", first)

	// Second attempt is skipped because retryAfter hasn't elapsed.
	second := d.DiscoverFeed(context.Background(), "src-1", "https://example.com")
	if second != "" {
		t.Errorf("expected empty result for recently attempted source, got %q", second)
	}
}

func TestDiscoverFeed_InvalidCandidateSkipped(t *testing.T) {
	t.Parallel()

	// HTML points to /feed.xml but it returns HTML, not a valid feed.
	// Common path /rss returns valid RSS.
	htmlWithBadFeedLink := `<!DOCTYPE html>
<html>
<head>
  <link rel="alternate" type="application/rss+xml" href="/feed.xml">
</head>
<body></body>
</html>`

	fetcher := &urlMockFetcher{
		responses: map[string]*feed.FetchResponse{
			"https://example.com": {
				StatusCode: http.StatusOK,
				Body:       htmlWithBadFeedLink,
			},
			"https://example.com/feed.xml": {
				StatusCode: http.StatusOK,
				Body:       "<html><body>Not a feed</body></html>",
			},
			"https://example.com/rss": {
				StatusCode: http.StatusOK,
				Body:       validRSSBody,
			},
		},
	}
	updater := &mockSourceUpdater{}
	d := newTestDiscoverer(t, fetcher, updater)

	result := d.DiscoverFeed(context.Background(), "src-1", "https://example.com")

	// Should fall through to common path probing.
	assertEqual(t, "https://example.com/rss", result)
}

func TestDiscoverFeed_FetchBaseURLError(t *testing.T) {
	t.Parallel()

	fetcher := &urlMockFetcher{
		errors: map[string]error{
			"https://example.com": errors.New("connection refused"),
		},
	}
	updater := &mockSourceUpdater{}
	d := newTestDiscoverer(t, fetcher, updater)

	result := d.DiscoverFeed(context.Background(), "src-1", "https://example.com")

	// Should still try common paths even if base URL fetch fails.
	if result != "" {
		t.Errorf("expected empty result, got %q", result)
	}
}

func TestDiscoveryLoop_UpdatesSource(t *testing.T) {
	t.Parallel()

	fetcher := &urlMockFetcher{
		responses: map[string]*feed.FetchResponse{
			"https://example.com": {
				StatusCode: http.StatusOK,
				Body:       htmlWithRSSLink,
			},
			"https://example.com/feed.xml": {
				StatusCode: http.StatusOK,
				Body:       validRSSBody,
			},
		},
	}
	updater := &mockSourceUpdater{}
	d := newTestDiscoverer(t, fetcher, updater)

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	listUndiscovered := func(_ context.Context) ([]feed.UndiscoveredSource, error) {
		callCount++
		if callCount > 1 {
			// Cancel on second call (ticker) so the loop exits.
			cancel()
			return nil, nil
		}

		return []feed.UndiscoveredSource{
			{SourceID: "src-1", BaseURL: "https://example.com"},
		}, nil
	}

	shortInterval := 100 * time.Millisecond
	err := d.RunDiscoveryLoop(ctx, shortInterval, listUndiscovered)
	requireNoError(t, err)

	if len(updater.calls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(updater.calls))
	}

	assertEqual(t, "src-1", updater.calls[0].sourceID)
	assertEqual(t, "https://example.com/feed.xml", updater.calls[0].feedURL)
}

func TestDiscoveryLoop_UpdateErrorLogged(t *testing.T) {
	t.Parallel()

	fetcher := &urlMockFetcher{
		responses: map[string]*feed.FetchResponse{
			"https://example.com": {
				StatusCode: http.StatusOK,
				Body:       htmlWithRSSLink,
			},
			"https://example.com/feed.xml": {
				StatusCode: http.StatusOK,
				Body:       validRSSBody,
			},
		},
	}
	updater := &mockSourceUpdater{err: errors.New("api error")}
	d := newTestDiscoverer(t, fetcher, updater)

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	listUndiscovered := func(_ context.Context) ([]feed.UndiscoveredSource, error) {
		callCount++
		if callCount > 1 {
			cancel()
			return nil, nil
		}

		return []feed.UndiscoveredSource{
			{SourceID: "src-1", BaseURL: "https://example.com"},
		}, nil
	}

	shortInterval := 100 * time.Millisecond
	err := d.RunDiscoveryLoop(ctx, shortInterval, listUndiscovered)
	requireNoError(t, err)

	// Update was attempted (error logged but loop continues).
	if len(updater.calls) != 1 {
		t.Fatalf("expected 1 update attempt, got %d", len(updater.calls))
	}
}
