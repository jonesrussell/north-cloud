package fetcher_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
)

// Test configuration constants.
const (
	workerTestSourceID = "source-123"
	workerTestURLID    = "url-456"
	workerTestHost     = "example.com"
	workerTestURL      = "https://example.com/article"
	workerTestAgent    = "TestBot/1.0"
	workerTestWorkers  = 1
	workerTestRetries  = 3

	workerClaimRetryDelay = 10 * time.Millisecond
	workerRequestTimeout  = 5 * time.Second
)

// --- Mock implementations ---

// mockFrontier implements fetcher.FrontierClaimer for testing.
type mockFrontier struct {
	mu             sync.Mutex
	claimFunc      func(ctx context.Context) (*domain.FrontierURL, error)
	fetchedCalls   []fetchedCall
	failedCalls    []failedCall
	deadCalls      []deadCall
	claimCallCount int
}

type fetchedCall struct {
	ID     string
	Params fetcher.FetchedParams
}

type failedCall struct {
	ID         string
	LastError  string
	MaxRetries int
}

type deadCall struct {
	ID     string
	Reason string
}

func (m *mockFrontier) Claim(ctx context.Context) (*domain.FrontierURL, error) {
	m.mu.Lock()
	m.claimCallCount++
	m.mu.Unlock()

	return m.claimFunc(ctx)
}

func (m *mockFrontier) UpdateFetched(_ context.Context, id string, params fetcher.FetchedParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.fetchedCalls = append(m.fetchedCalls, fetchedCall{ID: id, Params: params})

	return nil
}

func (m *mockFrontier) UpdateFailed(_ context.Context, id, lastError string, maxRetries int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.failedCalls = append(m.failedCalls, failedCall{ID: id, LastError: lastError, MaxRetries: maxRetries})

	return nil
}

func (m *mockFrontier) UpdateDead(_ context.Context, id, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deadCalls = append(m.deadCalls, deadCall{ID: id, Reason: reason})

	return nil
}

func (m *mockFrontier) getClaimCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.claimCallCount
}

// mockHostUpdater implements fetcher.HostUpdater for testing.
type mockHostUpdater struct {
	mu    sync.Mutex
	hosts []string
}

func (m *mockHostUpdater) UpdateLastFetch(_ context.Context, host string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hosts = append(m.hosts, host)

	return nil
}

// mockRobots implements fetcher.RobotsAllower for testing.
type mockRobots struct {
	allowed bool
	err     error
}

func (m *mockRobots) IsAllowed(_ context.Context, _ string) (bool, error) {
	return m.allowed, m.err
}

// mockIndexer implements fetcher.ContentIndexer for testing.
type mockIndexer struct {
	mu       sync.Mutex
	contents []*fetcher.ExtractedContent
	err      error
}

func (m *mockIndexer) Index(_ context.Context, content *fetcher.ExtractedContent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.contents = append(m.contents, content)

	return m.err
}

// mockLogger implements fetcher.WorkerLogger for testing.
type mockLogger struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockLogger) Info(msg string, _ ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, "INFO: "+msg)
}

func (m *mockLogger) Error(msg string, _ ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, "ERROR: "+msg)
}

// --- Test helpers ---

// newTestFrontierURL creates a FrontierURL for testing.
func newTestFrontierURL(t *testing.T, urlStr string) *domain.FrontierURL {
	t.Helper()

	return &domain.FrontierURL{
		ID:       workerTestURLID,
		URL:      urlStr,
		Host:     workerTestHost,
		SourceID: workerTestSourceID,
	}
}

// newTestWorkerPool creates a WorkerPool with default test dependencies.
func newTestWorkerPool(
	t *testing.T,
	frontier fetcher.FrontierClaimer,
	robots fetcher.RobotsAllower,
	indexer fetcher.ContentIndexer,
) (*fetcher.WorkerPool, *mockHostUpdater) {
	t.Helper()

	hostUpdater := &mockHostUpdater{}
	log := &mockLogger{}

	cfg := fetcher.WorkerPoolConfig{
		WorkerCount:     workerTestWorkers,
		UserAgent:       workerTestAgent,
		MaxRetries:      workerTestRetries,
		ClaimRetryDelay: workerClaimRetryDelay,
		RequestTimeout:  workerRequestTimeout,
	}

	wp := fetcher.NewWorkerPool(
		frontier,
		hostUpdater,
		robots,
		fetcher.NewContentExtractor(),
		indexer,
		log,
		cfg,
	)

	return wp, hostUpdater
}

// startTestServer creates an httptest.Server returning the given status and body.
func startTestServer(t *testing.T, statusCode int, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(server.Close)

	return server
}

// articleHTML is a simple HTML page for testing content extraction.
const articleHTML = `<!DOCTYPE html>
<html>
<head><title>Test Article</title></head>
<body><article><p>Test body content.</p></article></body>
</html>`

// --- Tests ---

func TestProcessURL_Success(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, http.StatusOK, articleHTML)
	furl := newTestFrontierURL(t, server.URL+"/article")

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{}

	wp, hostUpdater := newTestWorkerPool(t, frontier, robots, indexer)

	ctx := context.Background()

	err := wp.ProcessURL(ctx, furl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	verifyFetchedCalled(t, frontier)
	verifyHostUpdated(t, hostUpdater)
	verifyContentIndexed(t, indexer)
}

func TestProcessURL_RobotsBlocked(t *testing.T) {
	t.Parallel()

	furl := newTestFrontierURL(t, workerTestURL)

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: false}
	indexer := &mockIndexer{}

	wp, _ := newTestWorkerPool(t, frontier, robots, indexer)

	ctx := context.Background()

	err := wp.ProcessURL(ctx, furl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	verifyDeadCalled(t, frontier, "robots_blocked")
}

func TestProcessURL_NotFound(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, http.StatusNotFound, "not found")
	furl := newTestFrontierURL(t, server.URL+"/missing")

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{}

	wp, hostUpdater := newTestWorkerPool(t, frontier, robots, indexer)

	ctx := context.Background()

	err := wp.ProcessURL(ctx, furl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	verifyDeadCalled(t, frontier, "not_found")
	verifyHostUpdated(t, hostUpdater)
}

func TestProcessURL_ServerError(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, http.StatusInternalServerError, "error")
	furl := newTestFrontierURL(t, server.URL+"/error")

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{}

	wp, hostUpdater := newTestWorkerPool(t, frontier, robots, indexer)

	ctx := context.Background()

	err := wp.ProcessURL(ctx, furl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	verifyFailedCalled(t, frontier)
	verifyHostUpdated(t, hostUpdater)
}

func TestProcessURL_NotModified(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, http.StatusNotModified, "")
	etag := `"abc123"`
	furl := newTestFrontierURL(t, server.URL+"/cached")
	furl.ETag = &etag

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{}

	wp, hostUpdater := newTestWorkerPool(t, frontier, robots, indexer)

	ctx := context.Background()

	err := wp.ProcessURL(ctx, furl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	verifyFetchedCalled(t, frontier)
	verifyNoContentIndexed(t, indexer)
	verifyHostUpdated(t, hostUpdater)
}

func TestProcessURL_FetchError(t *testing.T) {
	t.Parallel()

	// Use an unreachable URL to cause a fetch error.
	furl := newTestFrontierURL(t, "http://192.0.2.1:1/unreachable")

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{}

	wp, hostUpdater := newTestWorkerPool(t, frontier, robots, indexer)

	// Use a short timeout context to avoid waiting for the default request timeout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := wp.ProcessURL(ctx, furl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	verifyFailedCalled(t, frontier)
	verifyHostUpdated(t, hostUpdater)
}

func TestProcessURL_TooManyRequests(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, http.StatusTooManyRequests, "rate limited")
	furl := newTestFrontierURL(t, server.URL+"/rate-limited")

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{}

	wp, hostUpdater := newTestWorkerPool(t, frontier, robots, indexer)

	ctx := context.Background()

	err := wp.ProcessURL(ctx, furl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	verifyFailedCalled(t, frontier)
	verifyHostUpdated(t, hostUpdater)
}

func TestWorker_ClaimRetry(t *testing.T) {
	t.Parallel()

	claimCount := 0

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			claimCount++

			return nil, fetcher.ErrNoURLAvailable
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{}

	wp, _ := newTestWorkerPool(t, frontier, robots, indexer)

	ctx, cancel := context.WithTimeout(context.Background(), workerClaimRetryDelay*3)
	defer cancel()

	_ = wp.Start(ctx)

	// After ~3 retry delays, the worker should have attempted at least 2 claims.
	minExpectedClaims := 2
	actualClaims := frontier.getClaimCallCount()

	if actualClaims < minExpectedClaims {
		t.Errorf("expected at least %d claim attempts, got %d", minExpectedClaims, actualClaims)
	}
}

func TestFetchPage_ConditionalHeaders(t *testing.T) {
	t.Parallel()

	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(articleHTML))
	}))
	defer server.Close()

	etag := `"test-etag"`
	lastModified := "Mon, 01 Jan 2024 00:00:00 GMT"

	furl := newTestFrontierURL(t, server.URL+"/conditional")
	furl.ETag = &etag
	furl.LastModified = &lastModified

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{}

	wp, _ := newTestWorkerPool(t, frontier, robots, indexer)

	ctx := context.Background()

	err := wp.ProcessURL(ctx, furl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	verifyHeader(t, receivedHeaders, "If-None-Match", etag)
	verifyHeader(t, receivedHeaders, "If-Modified-Since", lastModified)
	verifyHeader(t, receivedHeaders, "User-Agent", workerTestAgent)
}

func TestFetchPage_UserAgentSet(t *testing.T) {
	t.Parallel()

	var receivedUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(articleHTML))
	}))
	defer server.Close()

	furl := newTestFrontierURL(t, server.URL+"/ua-test")

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{}

	wp, _ := newTestWorkerPool(t, frontier, robots, indexer)

	ctx := context.Background()

	err := wp.ProcessURL(ctx, furl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedUA != workerTestAgent {
		t.Errorf("expected User-Agent %q, got %q", workerTestAgent, receivedUA)
	}
}

func TestProcessURL_IndexerError(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, http.StatusOK, articleHTML)
	furl := newTestFrontierURL(t, server.URL+"/article")

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return furl, nil
		},
	}
	robots := &mockRobots{allowed: true}
	indexer := &mockIndexer{err: errors.New("elasticsearch unavailable")}

	wp, _ := newTestWorkerPool(t, frontier, robots, indexer)

	ctx := context.Background()

	err := wp.ProcessURL(ctx, furl)
	if err == nil {
		t.Fatal("expected error from indexer, got nil")
	}

	expectedMsg := "index content"
	if !errors.Is(err, indexer.err) && err.Error() != fmt.Sprintf("%s: %s", expectedMsg, indexer.err.Error()) {
		t.Errorf("expected error containing %q, got %q", expectedMsg, err.Error())
	}
}

// --- Verification helpers ---

func verifyFetchedCalled(t *testing.T, frontier *mockFrontier) {
	t.Helper()

	frontier.mu.Lock()
	defer frontier.mu.Unlock()

	if len(frontier.fetchedCalls) == 0 {
		t.Error("expected UpdateFetched to be called, but it was not")
	}
}

func verifyFailedCalled(t *testing.T, frontier *mockFrontier) {
	t.Helper()

	frontier.mu.Lock()
	defer frontier.mu.Unlock()

	if len(frontier.failedCalls) == 0 {
		t.Error("expected UpdateFailed to be called, but it was not")
	}
}

func verifyDeadCalled(t *testing.T, frontier *mockFrontier, expectedReason string) {
	t.Helper()

	frontier.mu.Lock()
	defer frontier.mu.Unlock()

	if len(frontier.deadCalls) == 0 {
		t.Fatalf("expected UpdateDead to be called, but it was not")
	}

	if frontier.deadCalls[0].Reason != expectedReason {
		t.Errorf("expected dead reason %q, got %q", expectedReason, frontier.deadCalls[0].Reason)
	}
}

func verifyHostUpdated(t *testing.T, hostUpdater *mockHostUpdater) {
	t.Helper()

	hostUpdater.mu.Lock()
	defer hostUpdater.mu.Unlock()

	if len(hostUpdater.hosts) == 0 {
		t.Error("expected UpdateLastFetch to be called, but it was not")
		return
	}

	if hostUpdater.hosts[0] != workerTestHost {
		t.Errorf("expected host %q, got %q", workerTestHost, hostUpdater.hosts[0])
	}
}

func verifyContentIndexed(t *testing.T, indexer *mockIndexer) {
	t.Helper()

	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if len(indexer.contents) == 0 {
		t.Error("expected Index to be called, but it was not")
	}
}

func verifyNoContentIndexed(t *testing.T, indexer *mockIndexer) {
	t.Helper()

	indexer.mu.Lock()
	defer indexer.mu.Unlock()

	if len(indexer.contents) != 0 {
		t.Errorf("expected no Index calls for 304, got %d", len(indexer.contents))
	}
}

func verifyHeader(t *testing.T, headers http.Header, key, expected string) {
	t.Helper()

	actual := headers.Get(key)
	if actual != expected {
		t.Errorf("header %q: expected %q, got %q", key, expected, actual)
	}
}
