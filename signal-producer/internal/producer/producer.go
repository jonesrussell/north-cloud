// Package producer wires the leaf packages (checkpoint, mapper, client) into
// the signal-producer's main loop. One Run(ctx) call corresponds to one
// systemd-timer firing of the binary.
package producer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/jonesrussell/north-cloud/signal-producer/internal/client"
	"github.com/jonesrussell/north-cloud/signal-producer/internal/mapper"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Named constants so the linter and reviewers don't see magic numbers.
const (
	// sourceDownThreshold is the number of consecutive empty runs after
	// which the producer emits a WARN with the source_down code (FR-019, D7).
	sourceDownThreshold = 3

	// defaultBatchSize is the Waaseyaa request batch size when the operator
	// does not override it. FR-013.
	defaultBatchSize = 50

	// defaultMinQualityScore is the ES filter threshold when the operator
	// does not override it. FR-001.
	defaultMinQualityScore = 40

	// defaultLookbackBuffer is subtracted from the checkpoint timestamp on
	// each run to forgive small clock skew.
	defaultLookbackBuffer = 5 * time.Minute

	// esQuerySize is the ES `size` parameter — the maximum hits per query.
	// 100 is a safe upper bound that fits comfortably inside one or two
	// Waaseyaa batches even at the default batch_size of 50.
	esQuerySize = 100

	// sourceDownCode is the stable log code reviewers and operators search
	// for when triaging a stalled crawl.
	sourceDownCode = "signal_producer.source_down"

	// allHitsUnmappableCode is a distinct log code emitted when ES returned
	// hits but every one failed mapping. Lets operators distinguish a
	// content-pipeline bug from a quiet source on dashboards (review F1).
	allHitsUnmappableCode = "signal_producer.all_hits_unmappable"
)

// Config is the subset of the loaded YAML/env config the producer needs at
// runtime. The internal/config package validates and produces it.
type Config struct {
	Waaseyaa      WaaseyaaConfig
	Elasticsearch ElasticsearchConfig
	Schedule      ScheduleConfig
	Checkpoint    CheckpointConfig
}

// WaaseyaaConfig groups the receiver-side knobs.
type WaaseyaaConfig struct {
	URL             string
	APIKey          string
	BatchSize       int
	MinQualityScore int
}

// ElasticsearchConfig groups the source-side knobs.
type ElasticsearchConfig struct {
	URL     string
	Indexes []string
}

// ScheduleConfig groups timing knobs.
type ScheduleConfig struct {
	LookbackBuffer time.Duration
}

// CheckpointConfig groups checkpoint persistence knobs.
type CheckpointConfig struct {
	File string
}

// ESHit is the generic shape the producer hands to the mapper. It is the same
// map[string]any the mapper consumes (we keep the alias to make signatures
// readable).
type ESHit = map[string]any

// ESClient abstracts the Elasticsearch search call so unit tests can supply a
// fake. The real implementation in cmd/main.go wraps go-elasticsearch.
type ESClient interface {
	Search(ctx context.Context, indexes []string, query map[string]any) ([]ESHit, error)
}

// Producer orchestrates one run of the signal-producer pipeline.
type Producer struct {
	cfg    Config
	es     ESClient
	client client.WaaseyaaClient
	log    infralogger.Logger
}

// New constructs a Producer. All dependencies are required; constructors are
// kept dumb so cmd/main.go owns wiring concerns.
func New(cfg Config, es ESClient, c client.WaaseyaaClient, log infralogger.Logger) *Producer {
	return &Producer{cfg: cfg, es: es, client: c, log: log}
}

// runResult collects per-run counters. They feed the run_summary log line and
// the validation tests.
type runResult struct {
	hits          int
	mapped        int
	skippedMap    int
	delivered     int
	batches       int
	maxCrawledAt  time.Time
	ingestedTotal int
	skippedRcv    int
}

// Run executes one cycle: load checkpoint → query ES → map → batch POST →
// advance checkpoint. Errors abort the run; a successful return advances the
// checkpoint and resets the source-down counter.
func (p *Producer) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("producer: context cancelled: %w", err)
	}

	cp, err := LoadCheckpoint(p.cfg.Checkpoint.File, p.log)
	if err != nil {
		return fmt.Errorf("producer: load checkpoint: %w", err)
	}

	lookback := p.cfg.Schedule.LookbackBuffer
	if lookback <= 0 {
		lookback = defaultLookbackBuffer
	}
	minScore := p.cfg.Waaseyaa.MinQualityScore
	if minScore <= 0 {
		minScore = defaultMinQualityScore
	}

	p.log.Info(
		"run_start",
		infralogger.String("event", "run_start"),
		infralogger.Time("checkpoint_ts", cp.LastSuccessfulRun),
		infralogger.Duration("lookback_buffer", lookback),
	)

	query := buildQuery(cp.LastSuccessfulRun, lookback, minScore)
	hits, err := p.es.Search(ctx, p.cfg.Elasticsearch.Indexes, query)
	if err != nil {
		return fmt.Errorf("producer: es search: %w", err)
	}

	p.log.Info(
		"es_query",
		infralogger.String("event", "es_query"),
		infralogger.Int("hits", len(hits)),
	)

	if len(hits) == 0 {
		return p.handleNoSignals(cp, 0, 0)
	}

	return p.deliverHits(ctx, hits, cp)
}

// handleNoSignals is the shared path for "this run delivered zero signals":
// either ES returned no hits, or every hit failed mapping. In both cases we
// increment ConsecutiveEmpty, fire the source_down WARN at the threshold
// equality (FR-019, D7), and persist the counter without advancing the
// timestamp. totalHits and skippedMap are passed through to run_summary so
// operators can distinguish "ES empty" (totalHits=0) from "all unmappable"
// (totalHits>0, skippedMap==totalHits).
func (p *Producer) handleNoSignals(cp Checkpoint, totalHits, skippedMap int) error {
	cp.ConsecutiveEmpty++
	if skippedMap > 0 && skippedMap == totalHits {
		// Distinct log code so dashboards can split the content-pipeline-bug
		// case from the quiet-source case (review F1).
		p.log.Info(
			"all hits failed mapping",
			infralogger.String("event", "all_hits_unmappable"),
			infralogger.String("code", allHitsUnmappableCode),
			infralogger.Int("total_hits", totalHits),
			infralogger.Int("skipped", skippedMap),
		)
	}
	if cp.ConsecutiveEmpty == sourceDownThreshold {
		p.log.Warn(
			"source appears down",
			infralogger.String("event", "source_down"),
			infralogger.String("code", sourceDownCode),
			infralogger.Int("consecutive_empty", cp.ConsecutiveEmpty),
			infralogger.Time("last_success_at", cp.LastSuccessfulRun),
		)
	}
	// Persist the counter even on empty runs so the threshold survives
	// process restarts. Timestamp is intentionally NOT advanced.
	if saveErr := SaveCheckpoint(p.cfg.Checkpoint.File, cp); saveErr != nil {
		return fmt.Errorf("producer: save checkpoint after empty run: %w", saveErr)
	}
	p.log.Info(
		"run_summary",
		infralogger.String("event", "run_summary"),
		infralogger.Int("total_hits", totalHits),
		infralogger.Int("total_signals", 0),
		infralogger.Int("skipped", skippedMap),
		infralogger.Int("batches", 0),
	)
	return nil
}

// deliverHits maps hits, batches the resulting signals, posts each batch, and
// advances the checkpoint after every batch succeeds.
func (p *Producer) deliverHits(ctx context.Context, hits []ESHit, cp Checkpoint) error {
	res := runResult{hits: len(hits)}
	signals := make([]any, 0, len(hits))
	for _, hit := range hits {
		signal, mapErr := mapper.MapHit(hit)
		if mapErr != nil {
			p.log.Warn(
				"mapper skipped hit",
				infralogger.String("event", "mapper_error"),
				infralogger.Error(mapErr),
			)
			res.skippedMap++
			continue
		}
		signals = append(signals, signal)
		res.mapped++
		// Track max crawled_at on the *delivered* hits — see SaveCheckpoint
		// callers below: only after every batch returns 2xx do we advance.
		if ts, ok := crawledAt(hit); ok && ts.After(res.maxCrawledAt) {
			res.maxCrawledAt = ts
		}
	}

	if len(signals) == 0 {
		// All hits failed to map — operationally indistinguishable from a
		// quiet source. Reuse the empty-run helper so ConsecutiveEmpty
		// advances and the source_down WARN fires at threshold (review B1).
		return p.handleNoSignals(cp, res.hits, res.skippedMap)
	}

	batchSize := p.cfg.Waaseyaa.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	if err := p.postAllBatches(ctx, signals, batchSize, &res); err != nil {
		return err
	}

	// All batches delivered. Advance the checkpoint to the max crawled_at of
	// delivered hits (NOT time.Now — protects against lookback-buffer drift).
	cp.LastSuccessfulRun = res.maxCrawledAt
	cp.LastBatchSize = res.delivered
	cp.ConsecutiveEmpty = 0
	if saveErr := SaveCheckpoint(p.cfg.Checkpoint.File, cp); saveErr != nil {
		return fmt.Errorf("producer: save checkpoint after success: %w", saveErr)
	}

	p.log.Info(
		"run_summary",
		infralogger.String("event", "run_summary"),
		infralogger.Int("total_hits", res.hits),
		infralogger.Int("total_signals", res.delivered),
		infralogger.Int("skipped", res.skippedMap),
		infralogger.Int("batches", res.batches),
		infralogger.Int("ingested", res.ingestedTotal),
	)
	return nil
}

// postAllBatches splits signals into chunks of batchSize and posts each one.
// On the first non-retryable client error or retry-exhausted server error it
// returns immediately so the caller can refuse to advance the checkpoint
// (FR-005).
func (p *Producer) postAllBatches(ctx context.Context, signals []any, batchSize int, res *runResult) error {
	for start := 0; start < len(signals); start += batchSize {
		end := start + batchSize
		if end > len(signals) {
			end = len(signals)
		}
		batch := signals[start:end]
		ingest, postErr := p.client.PostSignals(ctx, client.SignalBatch{Signals: batch})
		if postErr != nil {
			p.logBatchError(postErr, len(batch))
			return fmt.Errorf("producer: post batch: %w", postErr)
		}
		res.batches++
		res.delivered += len(batch)
		if ingest != nil {
			res.ingestedTotal += ingest.Ingested
			res.skippedRcv += ingest.Skipped
		}
		p.logBatchPost(len(batch), ingest)
	}
	return nil
}

// logBatchError emits the single ERROR line that records why this run will
// fail. Kept separate so cognitive complexity in postAllBatches stays low.
func (p *Producer) logBatchError(err error, size int) {
	switch {
	case errors.Is(err, client.ErrClientResponse()):
		p.log.Error(
			"batch_post failed (client error, non-retryable)",
			infralogger.String("event", "batch_post_failed"),
			infralogger.Int("batch_size", size),
			infralogger.Error(err),
		)
	default:
		p.log.Error(
			"batch_post failed",
			infralogger.String("event", "batch_post_failed"),
			infralogger.Int("batch_size", size),
			infralogger.Error(err),
		)
	}
}

// logBatchPost emits the per-batch INFO line. NFR-006: ≤ 5 INFO lines per
// batch — this is the single info call inside the batch loop.
func (p *Producer) logBatchPost(size int, ingest *client.IngestResult) {
	if ingest == nil {
		p.log.Info(
			"batch_post",
			infralogger.String("event", "batch_post"),
			infralogger.Int("batch_size", size),
		)
		return
	}
	p.log.Info(
		"batch_post",
		infralogger.String("event", "batch_post"),
		infralogger.Int("batch_size", size),
		infralogger.Int("ingested", ingest.Ingested),
		infralogger.Int("skipped", ingest.Skipped),
		infralogger.Int("leads_created", ingest.LeadsCreated),
		infralogger.Int("leads_matched", ingest.LeadsMatched),
	)
}

// crawledAt extracts the RFC3339 timestamp from an ES hit, used to compute the
// next checkpoint watermark.
func crawledAt(hit ESHit) (time.Time, bool) {
	raw, ok := hit["crawled_at"]
	if !ok {
		return time.Time{}, false
	}
	str, ok := raw.(string)
	if !ok {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// buildQuery returns the Elasticsearch DSL body for the search call. The
// shape mirrors issue #592 exactly. Lookback is `checkpoint - lookbackBuffer`
// to forgive clock skew between the producer host and ES.
func buildQuery(checkpoint time.Time, lookbackBuffer time.Duration, minQualityScore int) map[string]any {
	gte := checkpoint.Add(-lookbackBuffer).UTC().Format(time.RFC3339)
	return map[string]any{
		"size": esQuerySize,
		"sort": []any{
			map[string]any{"crawled_at": "asc"},
		},
		"query": map[string]any{
			"bool": map[string]any{
				"must": []any{
					map[string]any{
						"range": map[string]any{
							"crawled_at": map[string]any{"gte": gte},
						},
					},
					map[string]any{
						"terms": map[string]any{
							"content_type": []any{"rfp", "need_signal"},
						},
					},
				},
				"filter": []any{
					map[string]any{
						"range": map[string]any{
							"quality_score": map[string]any{"gte": minQualityScore},
						},
					},
				},
			},
		},
	}
}

// ValidateConfig is a small helper exposed for cmd/main.go and tests so the
// URL parse rules and integer bounds live in one place.
func ValidateConfig(cfg Config) error {
	if _, err := url.ParseRequestURI(cfg.Waaseyaa.URL); err != nil {
		return fmt.Errorf("producer: invalid waaseyaa.url: %w", err)
	}
	if cfg.Waaseyaa.APIKey == "" {
		return errors.New("producer: waaseyaa.api_key is required")
	}
	if _, err := url.ParseRequestURI(cfg.Elasticsearch.URL); err != nil {
		return fmt.Errorf("producer: invalid elasticsearch.url: %w", err)
	}
	if len(cfg.Elasticsearch.Indexes) == 0 {
		return errors.New("producer: elasticsearch.indexes must not be empty")
	}
	if cfg.Checkpoint.File == "" {
		return errors.New("producer: checkpoint.file is required")
	}
	return nil
}
