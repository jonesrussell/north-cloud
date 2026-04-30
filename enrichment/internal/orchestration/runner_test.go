package orchestration

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
	"github.com/jonesrussell/north-cloud/enrichment/internal/callback"
	"github.com/jonesrussell/north-cloud/enrichment/internal/enricher"
)

func TestEnqueueReturnsBeforeSlowEnrichmentCompletes(t *testing.T) {
	t.Parallel()

	block := make(chan struct{})
	done := make(chan struct{})
	registry := fakeRegistry{
		enrichers: map[string]enricher.Enricher{
			enricher.TypeCompanyIntel: fakeEnricher{
				enrichmentType: enricher.TypeCompanyIntel,
				run: func(context.Context, api.EnrichmentRequest) (enricher.Result, error) {
					close(done)
					<-block
					return successResult(t, enricher.TypeCompanyIntel), nil
				},
			},
		},
	}
	runner := NewRunner(Config{
		Registry: registry,
		Callback: &recordingCallback{},
		Logger:   discardLogger(t),
		Timeout:  time.Second,
	})

	if err := runner.Enqueue(context.Background(), validRequest(t, enricher.TypeCompanyIntel)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("enrichment did not start")
	}

	close(block)
}

func TestRunnerSkipsUnknownTypes(t *testing.T) {
	t.Parallel()

	callbacks := &recordingCallback{}
	runner := NewRunner(Config{
		Registry: fakeRegistry{},
		Callback: callbacks,
		Logger:   discardLogger(t),
	})

	if err := runner.Enqueue(context.Background(), validRequest(t, "market_news")); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	callbacks.waitForCount(t, 0)
}

func TestRunnerIsolatesEnrichmentErrors(t *testing.T) {
	t.Parallel()

	callbacks := &recordingCallback{}
	searchErr := errors.New("search failed")
	registry := fakeRegistry{
		enrichers: map[string]enricher.Enricher{
			enricher.TypeCompanyIntel: fakeEnricher{
				enrichmentType: enricher.TypeCompanyIntel,
				run: func(apiCtx context.Context, request api.EnrichmentRequest) (enricher.Result, error) {
					return enricher.Result{
						LeadID: request.LeadID,
						Type:   enricher.TypeCompanyIntel,
						Status: enricher.StatusError,
						Error:  searchErr.Error(),
					}, searchErr
				},
			},
			enricher.TypeTechStack: fakeEnricher{
				enrichmentType: enricher.TypeTechStack,
				run: func(context.Context, api.EnrichmentRequest) (enricher.Result, error) {
					return successResult(t, enricher.TypeTechStack), nil
				},
			},
		},
	}
	runner := NewRunner(Config{
		Registry: registry,
		Callback: callbacks,
		Logger:   discardLogger(t),
	})

	if err := runner.Enqueue(context.Background(), validRequest(t, enricher.TypeCompanyIntel, enricher.TypeTechStack)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	results := callbacks.waitForCount(t, 2)
	statuses := map[string]string{}
	for _, item := range results {
		statuses[item.Type] = item.Status
	}
	if statuses[enricher.TypeCompanyIntel] != enricher.StatusError {
		t.Fatalf("company status = %q, want %q", statuses[enricher.TypeCompanyIntel], enricher.StatusError)
	}
	if statuses[enricher.TypeTechStack] != enricher.StatusSuccess {
		t.Fatalf("tech status = %q, want %q", statuses[enricher.TypeTechStack], enricher.StatusSuccess)
	}
}

func TestRunnerDoesNotLeakAPIKeyToCallbackResult(t *testing.T) {
	t.Parallel()

	callbacks := &recordingCallback{}
	registry := fakeRegistry{
		enrichers: map[string]enricher.Enricher{
			enricher.TypeHiring: fakeEnricher{
				enrichmentType: enricher.TypeHiring,
				run: func(context.Context, api.EnrichmentRequest) (enricher.Result, error) {
					return successResult(t, enricher.TypeHiring), nil
				},
			},
		},
	}
	runner := NewRunner(Config{
		Registry: registry,
		Callback: callbacks,
		Logger:   discardLogger(t),
	})

	request := validRequest(t, enricher.TypeHiring)
	if err := runner.Enqueue(context.Background(), request); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	results := callbacks.waitForCount(t, 1)
	if results[0].Error == request.CallbackAPIKey {
		t.Fatal("callback result leaked API key")
	}
}

type fakeRegistry struct {
	enrichers map[string]enricher.Enricher
}

func (r fakeRegistry) Lookup(enrichmentType string) (enricher.Enricher, bool) {
	item, ok := r.enrichers[enrichmentType]
	return item, ok
}

type fakeEnricher struct {
	enrichmentType string
	run            func(context.Context, api.EnrichmentRequest) (enricher.Result, error)
}

func (e fakeEnricher) Type() string {
	return e.enrichmentType
}

func (e fakeEnricher) Enrich(ctx context.Context, request api.EnrichmentRequest) (enricher.Result, error) {
	return e.run(ctx, request)
}

type recordingCallback struct {
	mu      sync.Mutex
	results []callback.EnrichmentResult
}

func (c *recordingCallback) SendEnrichment(
	_ context.Context,
	_ string,
	_ string,
	result callback.EnrichmentResult,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, result)
	return nil
}

func (c *recordingCallback) waitForCount(t *testing.T, want int) []callback.EnrichmentResult {
	t.Helper()

	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		c.mu.Lock()
		if len(c.results) == want {
			out := append([]callback.EnrichmentResult(nil), c.results...)
			c.mu.Unlock()
			return out
		}
		got := len(c.results)
		c.mu.Unlock()

		if want == 0 && got == 0 {
			return nil
		}

		select {
		case <-deadline:
			t.Fatalf("callback count = %d, want %d", got, want)
		case <-ticker.C:
		}
	}
}

func validRequest(t *testing.T, requestedTypes ...string) api.EnrichmentRequest {
	t.Helper()

	return api.EnrichmentRequest{
		LeadID:         "lead-123",
		CompanyName:    "Acme Mining",
		Domain:         "acme.example",
		Sector:         "mining",
		RequestedTypes: requestedTypes,
		CallbackURL:    "https://waaseyaa.example/callback",
		CallbackAPIKey: "super-secret",
	}
}

func successResult(t *testing.T, enrichmentType string) enricher.Result {
	t.Helper()

	return enricher.Result{
		LeadID:     "lead-123",
		Type:       enrichmentType,
		Status:     enricher.StatusSuccess,
		Confidence: 0.9,
		Data:       map[string]any{"ok": true},
	}
}

func discardLogger(t *testing.T) *slog.Logger {
	t.Helper()

	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
