package feed_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/feed"
)

// expectedFeedPriority is the priority assigned to URLs discovered via feeds.
const expectedFeedPriority = domain.FrontierDefaultPriority + domain.FrontierFeedBonus

// rssFixtureForPoller contains two items used by poller tests.
const rssFixtureForPoller = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Poller Test</title>
    <item>
      <title>Article One</title>
      <link>https://example.com/one</link>
    </item>
    <item>
      <title>Article Two</title>
      <link>https://example.com/two</link>
    </item>
  </channel>
</rss>`

// pollerFixtureItemCount is the number of items in rssFixtureForPoller.
const pollerFixtureItemCount = 2

// --- Mock implementations ---

// mockFetcher implements feed.HTTPFetcher for testing.
type mockFetcher struct {
	response *feed.FetchResponse
	err      error
	// captured inputs
	calledURL          string
	calledETag         *string
	calledLastModified *string
}

func (m *mockFetcher) Fetch(
	_ context.Context,
	url string,
	etag, lastModified *string,
) (*feed.FetchResponse, error) {
	m.calledURL = url
	m.calledETag = etag
	m.calledLastModified = lastModified

	return m.response, m.err
}

// mockFeedStateStore implements feed.FeedStateStore for testing.
type mockFeedStateStore struct {
	state           *domain.FeedState
	getOrCreateErr  error
	updateSuccErr   error
	updateErrErr    error
	successCalled   bool
	errorCalled     bool
	lastSuccResult  feed.PollResult
	lastErrType     string
	lastErrMsg      string
	lastErrSourceID string
}

func (m *mockFeedStateStore) GetOrCreate(
	_ context.Context,
	sourceID, feedURL string,
) (*domain.FeedState, error) {
	if m.getOrCreateErr != nil {
		return nil, m.getOrCreateErr
	}

	if m.state != nil {
		return m.state, nil
	}

	return &domain.FeedState{
		SourceID: sourceID,
		FeedURL:  feedURL,
	}, nil
}

func (m *mockFeedStateStore) UpdateSuccess(
	_ context.Context,
	_ string,
	result feed.PollResult,
) error {
	m.successCalled = true
	m.lastSuccResult = result

	return m.updateSuccErr
}

func (m *mockFeedStateStore) UpdateError(
	_ context.Context,
	sourceID, errorType, errMsg string,
) error {
	m.errorCalled = true
	m.lastErrSourceID = sourceID
	m.lastErrType = errorType
	m.lastErrMsg = errMsg

	return m.updateErrErr
}

// mockFrontier implements feed.FrontierSubmitter for testing.
type mockFrontier struct {
	submitted []feed.SubmitParams
	err       error
}

func (m *mockFrontier) Submit(_ context.Context, params feed.SubmitParams) error {
	if m.err != nil {
		return m.err
	}

	m.submitted = append(m.submitted, params)

	return nil
}

// mockDisabler implements feed.SourceFeedDisabler for testing.
type mockDisabler struct {
	disableCalled bool
	enableCalled  bool
	lastSourceID  string
	lastReason    string
	disableErr    error
	enableErr     error
}

func (m *mockDisabler) DisableFeed(_ context.Context, sourceID, reason string) error {
	m.disableCalled = true
	m.lastSourceID = sourceID
	m.lastReason = reason
	return m.disableErr
}

func (m *mockDisabler) EnableFeed(_ context.Context, sourceID string) error {
	m.enableCalled = true
	m.lastSourceID = sourceID
	return m.enableErr
}

// mockLogger implements feed.Logger for testing.
type mockLogger struct {
	warnCalled  bool
	errorCalled bool
	lastMsg     string
}

func (m *mockLogger) Info(_ string, _ ...any) {}

func (m *mockLogger) Warn(msg string, _ ...any) {
	m.warnCalled = true
	m.lastMsg = msg
}

func (m *mockLogger) Error(msg string, _ ...any) {
	m.errorCalled = true
	m.lastMsg = msg
}

// --- Helper functions ---

// newTestPoller creates a Poller with the given mock dependencies (no disabler).
func newTestPoller(
	t *testing.T,
	fetcher feed.HTTPFetcher,
	stateStore feed.FeedStateStore,
	frontierSubmitter feed.FrontierSubmitter,
) *feed.Poller {
	t.Helper()

	return feed.NewPoller(fetcher, stateStore, frontierSubmitter, nil, &mockLogger{})
}

// newTestPollerWithDisabler creates a Poller with a disabler for auto-disable tests.
func newTestPollerWithDisabler(
	t *testing.T,
	fetcher feed.HTTPFetcher,
	stateStore feed.FeedStateStore,
	frontierSubmitter feed.FrontierSubmitter,
	disabler feed.SourceFeedDisabler,
) *feed.Poller {
	t.Helper()

	return feed.NewPoller(fetcher, stateStore, frontierSubmitter, disabler, &mockLogger{})
}

// newOKResponse creates a FetchResponse with HTTP 200 and the given body.
func newOKResponse(t *testing.T, body string) *feed.FetchResponse {
	t.Helper()

	return &feed.FetchResponse{
		StatusCode: http.StatusOK,
		Body:       body,
	}
}

// --- Tests ---

func TestPollFeed_Success(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{response: newOKResponse(t, rssFixtureForPoller)}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	requireNoError(t, err)

	requireLen(t, frontierMock.submitted, pollerFixtureItemCount)

	// Verify submitted URLs are normalized (https scheme).
	assertContainsURL(t, frontierMock.submitted, "https://example.com/one")
	assertContainsURL(t, frontierMock.submitted, "https://example.com/two")

	// Verify origin and priority on first submitted item.
	first := frontierMock.submitted[0]
	assertEqual(t, domain.FrontierOriginFeed, first.Origin)

	if first.Priority != expectedFeedPriority {
		t.Errorf("expected priority %d, got %d", expectedFeedPriority, first.Priority)
	}

	// Verify feed state was updated with success.
	if !stateStore.successCalled {
		t.Error("expected UpdateSuccess to be called")
	}

	if stateStore.lastSuccResult.ItemCount != pollerFixtureItemCount {
		t.Errorf(
			"expected item count %d, got %d",
			pollerFixtureItemCount, stateStore.lastSuccResult.ItemCount,
		)
	}
}

func TestPollFeed_NotModified(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusNotModified},
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	requireNoError(t, err)

	// No URLs should be submitted for 304.
	requireLen(t, frontierMock.submitted, 0)

	// Feed state should NOT be updated (no success, no error).
	if stateStore.successCalled {
		t.Error("expected UpdateSuccess NOT to be called for 304")
	}

	if stateStore.errorCalled {
		t.Error("expected UpdateError NOT to be called for 304")
	}
}

func TestPollFeed_FetchError(t *testing.T) {
	t.Parallel()

	fetchErr := errors.New("connection refused")
	fetcher := &mockFetcher{err: fetchErr}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Feed state error should be recorded.
	if !stateStore.errorCalled {
		t.Error("expected UpdateError to be called on fetch error")
	}

	assertEqual(t, "src-1", stateStore.lastErrSourceID)
}

func TestPollFeed_ParseError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: newOKResponse(t, "not valid xml at all"),
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error for invalid feed body, got nil")
	}

	// Feed state error should be recorded.
	if !stateStore.errorCalled {
		t.Error("expected UpdateError to be called on parse error")
	}

	// No URLs should be submitted.
	requireLen(t, frontierMock.submitted, 0)
}

func TestPollFeed_ConditionalHeaders(t *testing.T) {
	t.Parallel()

	etag := `"abc123"`
	modified := "Sat, 01 Jan 2024 00:00:00 GMT"

	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusNotModified},
	}
	stateStore := &mockFeedStateStore{
		state: &domain.FeedState{
			SourceID:     "src-1",
			FeedURL:      "https://example.com/feed.xml",
			LastETag:     &etag,
			LastModified: &modified,
		},
	}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	requireNoError(t, err)

	// Verify conditional headers were passed to the fetcher.
	if fetcher.calledETag == nil || *fetcher.calledETag != etag {
		t.Errorf("expected etag %q to be passed to fetcher", etag)
	}

	if fetcher.calledLastModified == nil || *fetcher.calledLastModified != modified {
		t.Errorf("expected last-modified %q to be passed to fetcher", modified)
	}
}

func TestPollFeed_UnexpectedStatus(t *testing.T) {
	t.Parallel()

	serverErrorCode := 500
	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: serverErrorCode},
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error for unexpected status code, got nil")
	}

	if !stateStore.errorCalled {
		t.Error("expected UpdateError to be called for unexpected status")
	}

	requireLen(t, frontierMock.submitted, 0)
}

func TestPollFeed_ETagAndModifiedPropagated(t *testing.T) {
	t.Parallel()

	etag := `"new-etag"`
	modified := "Sun, 02 Jan 2024 00:00:00 GMT"

	fetcher := &mockFetcher{
		response: &feed.FetchResponse{
			StatusCode:   http.StatusOK,
			Body:         rssFixtureForPoller,
			ETag:         &etag,
			LastModified: &modified,
		},
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	requireNoError(t, err)

	if !stateStore.successCalled {
		t.Fatal("expected UpdateSuccess to be called")
	}

	if stateStore.lastSuccResult.ETag == nil || *stateStore.lastSuccResult.ETag != etag {
		t.Errorf("expected etag %q in poll result", etag)
	}

	if stateStore.lastSuccResult.Modified == nil || *stateStore.lastSuccResult.Modified != modified {
		t.Errorf("expected modified %q in poll result", modified)
	}
}

func TestPollFeed_GetOrCreateError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{}
	stateStore := &mockFeedStateStore{
		getOrCreateErr: errors.New("database unavailable"),
	}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error when GetOrCreate fails, got nil")
	}
}

func TestPollFeed_403_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusForbidden},
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T: %v", err, err)
	}
	if pollErr.Type != feed.ErrTypeForbidden {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeForbidden)
	}
	if pollErr.Level != feed.LevelWarn {
		t.Errorf("Level = %d, want %d (LevelWarn)", pollErr.Level, feed.LevelWarn)
	}
	if !stateStore.errorCalled {
		t.Error("expected UpdateError to be called")
	}
	if stateStore.lastErrType != string(feed.ErrTypeForbidden) {
		t.Errorf("lastErrType = %q, want %q", stateStore.lastErrType, feed.ErrTypeForbidden)
	}
}

func TestPollFeed_404_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusNotFound},
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T: %v", err, err)
	}
	if pollErr.Type != feed.ErrTypeNotFound {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeNotFound)
	}
}

func TestPollFeed_429_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusTooManyRequests},
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T: %v", err, err)
	}
	if pollErr.Type != feed.ErrTypeRateLimited {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeRateLimited)
	}
}

func TestPollFeed_NetworkError_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{err: errors.New("dial tcp: no such host")}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T: %v", err, err)
	}
	if pollErr.Type != feed.ErrTypeNetwork {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeNetwork)
	}
}

func TestPollFeed_ParseError_ReturnsPollError(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		response: newOKResponse(t, "not valid xml at all"),
	}
	stateStore := &mockFeedStateStore{}
	frontierMock := &mockFrontier{}
	poller := newTestPoller(t, fetcher, stateStore, frontierMock)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var pollErr *feed.PollError
	if !errors.As(err, &pollErr) {
		t.Fatalf("expected PollError, got %T: %v", err, err)
	}
	if pollErr.Type != feed.ErrTypeParse {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeParse)
	}
}

func TestPollFeed_AutoDisable_404_AfterThreshold(t *testing.T) {
	t.Parallel()

	notFoundThreshold := 3
	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusNotFound},
	}
	stateStore := &mockFeedStateStore{
		state: &domain.FeedState{
			SourceID:          "src-1",
			FeedURL:           "https://example.com/feed.xml",
			ConsecutiveErrors: notFoundThreshold,
		},
	}
	disabler := &mockDisabler{}
	frontierMock := &mockFrontier{}
	poller := newTestPollerWithDisabler(t, fetcher, stateStore, frontierMock, disabler)

	_ = poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")

	if !disabler.disableCalled {
		t.Error("expected DisableFeed to be called after reaching threshold")
	}
	if disabler.lastReason != string(feed.ErrTypeNotFound) {
		t.Errorf("reason = %q, want %q", disabler.lastReason, feed.ErrTypeNotFound)
	}
}

func TestPollFeed_AutoDisable_410_AfterThreshold(t *testing.T) {
	t.Parallel()

	goneThreshold := 1
	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusGone},
	}
	stateStore := &mockFeedStateStore{
		state: &domain.FeedState{
			SourceID:          "src-1",
			FeedURL:           "https://example.com/feed.xml",
			ConsecutiveErrors: goneThreshold,
		},
	}
	disabler := &mockDisabler{}
	frontierMock := &mockFrontier{}
	poller := newTestPollerWithDisabler(t, fetcher, stateStore, frontierMock, disabler)

	_ = poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")

	if !disabler.disableCalled {
		t.Error("expected DisableFeed to be called for 410 after 1 error")
	}
}

func TestPollFeed_AutoDisable_429_NeverDisables(t *testing.T) {
	t.Parallel()

	largeErrorCount := 100
	fetcher := &mockFetcher{
		response: &feed.FetchResponse{StatusCode: http.StatusTooManyRequests},
	}
	stateStore := &mockFeedStateStore{
		state: &domain.FeedState{
			SourceID:          "src-1",
			FeedURL:           "https://example.com/feed.xml",
			ConsecutiveErrors: largeErrorCount,
		},
	}
	disabler := &mockDisabler{}
	frontierMock := &mockFrontier{}
	poller := newTestPollerWithDisabler(t, fetcher, stateStore, frontierMock, disabler)

	_ = poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")

	if disabler.disableCalled {
		t.Error("DisableFeed should NOT be called for rate limited errors")
	}
}

func TestPollFeed_ReEnableOnSuccess(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{response: newOKResponse(t, rssFixtureForPoller)}
	stateStore := &mockFeedStateStore{}
	disabler := &mockDisabler{}
	frontierMock := &mockFrontier{}
	poller := newTestPollerWithDisabler(t, fetcher, stateStore, frontierMock, disabler)

	err := poller.PollFeed(context.Background(), "src-1", "https://example.com/feed.xml")
	requireNoError(t, err)

	if !disabler.enableCalled {
		t.Error("expected EnableFeed to be called on success")
	}
}

// assertContainsURL verifies that at least one submitted param has the given URL.
func assertContainsURL(t *testing.T, submitted []feed.SubmitParams, url string) {
	t.Helper()

	for i := range submitted {
		if submitted[i].URL == url {
			return
		}
	}

	t.Errorf("expected submitted params to contain URL %q", url)
}
