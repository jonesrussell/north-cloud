package orchestration

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
	"github.com/jonesrussell/north-cloud/enrichment/internal/callback"
	"github.com/jonesrussell/north-cloud/enrichment/internal/enricher"
)

const defaultRunTimeout = 30 * time.Second

// CallbackSender is the callback client surface used by the runner.
type CallbackSender interface {
	SendEnrichment(ctx context.Context, callbackURL string, apiKey string, result callback.EnrichmentResult) error
}

// Runner processes accepted enrichment requests outside the HTTP response path.
type Runner struct {
	registry Registry
	callback CallbackSender
	logger   *slog.Logger
	timeout  time.Duration
}

// Registry is the subset of enricher.Registry needed for lookup.
type Registry interface {
	Lookup(enrichmentType string) (enricher.Enricher, bool)
}

// Config customizes runner construction.
type Config struct {
	Registry Registry
	Callback CallbackSender
	Logger   *slog.Logger
	Timeout  time.Duration
}

// NewRunner creates an asynchronous enrichment runner.
func NewRunner(cfg Config) *Runner {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultRunTimeout
	}

	return &Runner{
		registry: cfg.Registry,
		callback: cfg.Callback,
		logger:   logger,
		timeout:  timeout,
	}
}

// Enqueue starts background enrichment and returns immediately after scheduling work.
func (r *Runner) Enqueue(ctx context.Context, request api.EnrichmentRequest) error {
	if r.registry == nil {
		return fmt.Errorf("enrichment registry is required")
	}
	if r.callback == nil {
		return fmt.Errorf("callback client is required")
	}

	runCtx := context.WithoutCancel(ctx)
	go r.run(runCtx, request)
	return nil
}

func (r *Runner) run(parent context.Context, request api.EnrichmentRequest) {
	ctx, cancel := context.WithTimeout(parent, r.timeout)
	defer cancel()

	var wg sync.WaitGroup
	for _, requestedType := range request.RequestedTypes {
		item, ok := r.registry.Lookup(requestedType)
		if !ok {
			r.logger.Warn("unknown enrichment type skipped",
				slog.String("lead_id", request.LeadID),
				slog.String("type", requestedType))
			continue
		}

		wg.Add(1)
		go func(item enricher.Enricher) {
			defer wg.Done()
			r.runOne(ctx, request, item)
		}(item)
	}
	wg.Wait()
}

func (r *Runner) runOne(ctx context.Context, request api.EnrichmentRequest, item enricher.Enricher) {
	result, err := item.Enrich(ctx, request)
	if err != nil {
		r.logger.Error("enrichment failed",
			slog.String("lead_id", request.LeadID),
			slog.String("type", item.Type()),
			slog.Any("error", err))
	}

	if sendErr := r.callback.SendEnrichment(ctx, request.CallbackURL, request.CallbackAPIKey, toCallbackResult(result)); sendErr != nil {
		r.logger.Error("enrichment callback failed",
			slog.String("lead_id", request.LeadID),
			slog.String("type", item.Type()),
			slog.Any("error", sendErr))
	}
}

func toCallbackResult(result enricher.Result) callback.EnrichmentResult {
	return callback.EnrichmentResult{
		LeadID:     result.LeadID,
		Type:       result.Type,
		Status:     result.Status,
		Confidence: result.Confidence,
		Data:       result.Data,
		Error:      result.Error,
	}
}
