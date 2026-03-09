package fetcher_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
)

// mockRenderer implements fetcher.PageRenderer for testing.
type mockRenderer struct {
	page      *fetcher.RenderedPage
	err       error
	callCount int
}

func (m *mockRenderer) Render(_ context.Context, _ string) (*fetcher.RenderedPage, error) {
	m.callCount++
	return m.page, m.err
}

// mockModeResolver implements fetcher.SourceRenderModeResolver for testing.
type mockModeResolver struct {
	mode string
	err  error
}

func (m *mockModeResolver) GetRenderMode(_ context.Context, _ string) (string, error) {
	return m.mode, m.err
}

// buildRenderPool creates a WorkerPool wired with a renderer and mode resolver.
func buildRenderPool(
	t *testing.T,
	frontier fetcher.FrontierClaimer,
	renderer fetcher.PageRenderer,
	resolver fetcher.SourceRenderModeResolver,
) *fetcher.WorkerPool {
	t.Helper()

	cfg := fetcher.WorkerPoolConfig{
		WorkerCount:     workerTestWorkers,
		UserAgent:       workerTestAgent,
		MaxRetries:      workerTestRetries,
		ClaimRetryDelay: workerClaimRetryDelay,
		RequestTimeout:  workerRequestTimeout,
		Renderer:        renderer,
		ModeResolver:    resolver,
	}

	return fetcher.NewWorkerPool(
		frontier,
		&mockHostUpdater{},
		&mockRobots{allowed: true},
		fetcher.NewContentExtractor(),
		&mockIndexer{},
		&mockLogger{},
		cfg,
	)
}

// TestProcessURL_DynamicRender_UsesRenderer verifies that a dynamic source
// routes through the Playwright render worker instead of plain HTTP.
func TestProcessURL_DynamicRender_UsesRenderer(t *testing.T) {
	t.Parallel()

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return nil, fetcher.ErrNoURLAvailable
		},
	}

	renderer := &mockRenderer{
		page: &fetcher.RenderedPage{
			HTML:       articleHTML,
			FinalURL:   workerTestURL,
			StatusCode: http.StatusOK,
		},
	}

	resolver := &mockModeResolver{mode: "dynamic"}

	pool := buildRenderPool(t, frontier, renderer, resolver)

	furl := &domain.FrontierURL{
		ID:       workerTestURLID,
		URL:      workerTestURL,
		Host:     workerTestHost,
		SourceID: workerTestSourceID,
	}

	if err := pool.ProcessURL(context.Background(), furl); err != nil {
		t.Fatalf("ProcessURL returned error: %v", err)
	}

	if renderer.callCount != 1 {
		t.Errorf("expected renderer called once, got %d", renderer.callCount)
	}
}

// TestProcessURL_StaticRender_UsesHTTP verifies that a static source
// uses plain HTTP and does NOT invoke the render worker.
func TestProcessURL_StaticRender_UsesHTTP(t *testing.T) {
	t.Parallel()

	server := startTestServer(t, http.StatusOK, articleHTML)

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return nil, fetcher.ErrNoURLAvailable
		},
	}

	renderer := &mockRenderer{
		err: errors.New("renderer must not be called for static sources"),
	}

	resolver := &mockModeResolver{mode: "static"}

	pool := buildRenderPool(t, frontier, renderer, resolver)

	furl := &domain.FrontierURL{
		ID:       workerTestURLID,
		URL:      server.URL + "/article",
		Host:     workerTestHost,
		SourceID: workerTestSourceID,
	}

	if err := pool.ProcessURL(context.Background(), furl); err != nil {
		t.Fatalf("ProcessURL returned error: %v", err)
	}

	if renderer.callCount != 0 {
		t.Errorf("expected renderer NOT called for static source, got %d calls", renderer.callCount)
	}
}

// TestProcessURL_RendererError_MarksURLFailed verifies that a render worker
// failure causes the URL to be marked as failed in the frontier.
func TestProcessURL_RendererError_MarksURLFailed(t *testing.T) {
	t.Parallel()

	frontier := &mockFrontier{
		claimFunc: func(_ context.Context) (*domain.FrontierURL, error) {
			return nil, fetcher.ErrNoURLAvailable
		},
	}

	renderer := &mockRenderer{
		err: errors.New("playwright timeout"),
	}

	resolver := &mockModeResolver{mode: "dynamic"}

	pool := buildRenderPool(t, frontier, renderer, resolver)

	furl := &domain.FrontierURL{
		ID:       workerTestURLID,
		URL:      workerTestURL,
		Host:     workerTestHost,
		SourceID: workerTestSourceID,
	}

	if err := pool.ProcessURL(context.Background(), furl); err != nil {
		t.Fatalf("ProcessURL returned error: %v", err)
	}

	frontier.mu.Lock()
	failedCount := len(frontier.failedCalls)
	frontier.mu.Unlock()

	if failedCount != 1 {
		t.Fatalf("expected 1 failed call, got %d", failedCount)
	}
}
