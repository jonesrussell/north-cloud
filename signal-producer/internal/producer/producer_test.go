package producer_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-producer/internal/client"
	"github.com/jonesrussell/north-cloud/signal-producer/internal/producer"
)

// fakeESClient is a deterministic ESClient implementation for unit tests.
type fakeESClient struct {
	hits []producer.ESHit
	err  error
	// queries records every Search call so tests can assert query shape.
	queries []map[string]any
}

func (f *fakeESClient) Search(_ context.Context, _ []string, query map[string]any) ([]producer.ESHit, error) {
	f.queries = append(f.queries, query)
	if f.err != nil {
		return nil, f.err
	}
	return f.hits, nil
}

// fakeWaaseyaa is a deterministic WaaseyaaClient. Each call returns the next
// scripted (result, err) pair; once the script is exhausted the last entry
// is reused.
type fakeWaaseyaaResp struct {
	res *client.IngestResult
	err error
}

type fakeWaaseyaa struct {
	script   []fakeWaaseyaaResp
	calls    int
	posted   []client.SignalBatch
	totalSig int
}

func (f *fakeWaaseyaa) PostSignals(_ context.Context, batch client.SignalBatch) (*client.IngestResult, error) {
	f.posted = append(f.posted, batch)
	f.totalSig += len(batch.Signals)
	idx := f.calls
	if idx >= len(f.script) {
		idx = len(f.script) - 1
	}
	f.calls++
	return f.script[idx].res, f.script[idx].err
}

// successResp is the default mock Waaseyaa response.
func successResp(ingested int) fakeWaaseyaaResp {
	return fakeWaaseyaaResp{res: &client.IngestResult{Ingested: ingested}, err: nil}
}

// makeHit returns a minimum-viable ES hit the mapper accepts. Hard-coded to
// content_type "rfp" — every caller uses the same value (unparam).
func makeHit(t *testing.T, id string, crawledAt time.Time) producer.ESHit {
	t.Helper()
	hit := producer.ESHit{
		"_id":           id,
		"title":         "Test " + id,
		"quality_score": float64(60),
		"url":           "https://example.com/" + id,
		"crawled_at":    crawledAt.UTC().Format(time.RFC3339),
		"content_type":  "rfp",
	}
	hit["rfp"] = map[string]any{
		"organization_name": "Org",
		"province":          "ON",
		"categories":        []any{"Construction"},
	}
	return hit
}

// testHarness bundles the per-test scaffolding.
type testHarness struct {
	cfg    producer.Config
	es     *fakeESClient
	wc     *fakeWaaseyaa
	cpFile string
	t      *testing.T
}

func newHarness(t *testing.T) *testHarness {
	t.Helper()
	dir := t.TempDir()
	cpFile := filepath.Join(dir, "checkpoint.json")
	cfg := producer.Config{
		Waaseyaa: producer.WaaseyaaConfig{
			URL:             "https://waaseyaa.example.com",
			APIKey:          "k",
			BatchSize:       50,
			MinQualityScore: 40,
		},
		Elasticsearch: producer.ElasticsearchConfig{
			URL:     "http://localhost:9200",
			Indexes: []string{"*_classified_content"},
		},
		Schedule:   producer.ScheduleConfig{LookbackBuffer: 5 * time.Minute},
		Checkpoint: producer.CheckpointConfig{File: cpFile},
	}
	return &testHarness{
		cfg:    cfg,
		es:     &fakeESClient{},
		wc:     &fakeWaaseyaa{},
		cpFile: cpFile,
		t:      t,
	}
}

func (h *testHarness) seedCheckpoint(cp producer.Checkpoint) {
	h.t.Helper()
	if err := producer.SaveCheckpoint(h.cpFile, cp); err != nil {
		h.t.Fatalf("seedCheckpoint: %v", err)
	}
}

func (h *testHarness) loadCheckpoint() producer.Checkpoint {
	h.t.Helper()
	data, err := os.ReadFile(h.cpFile)
	if err != nil {
		h.t.Fatalf("read checkpoint: %v", err)
	}
	var cp producer.Checkpoint
	if unmarshalErr := json.Unmarshal(data, &cp); unmarshalErr != nil {
		h.t.Fatalf("unmarshal checkpoint: %v", unmarshalErr)
	}
	return cp
}

func (h *testHarness) run() *producer.Producer {
	h.t.Helper()
	log, _ := infralogger.New(infralogger.Config{Level: "fatal"})
	return producer.New(h.cfg, h.es, h.wc, log)
}

// --- Tests ---

func TestRun_HappyPath(t *testing.T) {
	h := newHarness(t)
	now := time.Now().UTC().Truncate(time.Second)
	h.seedCheckpoint(producer.Checkpoint{
		LastSuccessfulRun: now.Add(-time.Hour),
		LastBatchSize:     0,
		ConsecutiveEmpty:  2, // should reset on success
	})
	for i := range 5 {
		h.es.hits = append(h.es.hits, makeHit(t, fmt.Sprintf("id%d", i), now.Add(-time.Duration(5-i)*time.Minute)))
	}
	h.wc.script = []fakeWaaseyaaResp{successResp(5)}

	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.wc.calls != 1 {
		t.Errorf("expected 1 POST, got %d", h.wc.calls)
	}
	if h.wc.totalSig != 5 {
		t.Errorf("expected 5 signals delivered, got %d", h.wc.totalSig)
	}
	cp := h.loadCheckpoint()
	if cp.ConsecutiveEmpty != 0 {
		t.Errorf("ConsecutiveEmpty = %d, want 0 after success", cp.ConsecutiveEmpty)
	}
	if cp.LastBatchSize != 5 {
		t.Errorf("LastBatchSize = %d, want 5", cp.LastBatchSize)
	}
	if !cp.LastSuccessfulRun.After(now.Add(-2 * time.Hour)) {
		t.Errorf("LastSuccessfulRun did not advance: %v", cp.LastSuccessfulRun)
	}
}

func TestRun_EmptyResult(t *testing.T) {
	h := newHarness(t)
	original := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	h.seedCheckpoint(producer.Checkpoint{LastSuccessfulRun: original, LastBatchSize: 7})

	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.wc.calls != 0 {
		t.Errorf("expected 0 POSTs on empty result, got %d", h.wc.calls)
	}
	cp := h.loadCheckpoint()
	if !cp.LastSuccessfulRun.Equal(original) {
		t.Errorf("timestamp advanced on empty run: was %v now %v", original, cp.LastSuccessfulRun)
	}
	if cp.ConsecutiveEmpty != 1 {
		t.Errorf("ConsecutiveEmpty = %d, want 1", cp.ConsecutiveEmpty)
	}
}

func TestRun_SourceDownThreshold(t *testing.T) {
	h := newHarness(t)
	original := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	// Pre-set ConsecutiveEmpty=2 so this empty run hits the threshold (3).
	h.seedCheckpoint(producer.Checkpoint{LastSuccessfulRun: original, ConsecutiveEmpty: 2})

	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	cp := h.loadCheckpoint()
	if cp.ConsecutiveEmpty != 3 {
		t.Errorf("ConsecutiveEmpty = %d, want 3", cp.ConsecutiveEmpty)
	}

	// Fourth empty run: counter increments but WARN must NOT re-fire (we
	// can't easily inspect log output here without a fake logger; the
	// behavior is encoded in handleEmptyRun's `== threshold` equality
	// check). Verify the counter keeps going up.
	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run again: %v", err)
	}
	cp = h.loadCheckpoint()
	if cp.ConsecutiveEmpty != 4 {
		t.Errorf("ConsecutiveEmpty = %d after 4th empty run, want 4", cp.ConsecutiveEmpty)
	}
}

func TestRun_BatchExactlyAtLimit(t *testing.T) {
	h := newHarness(t)
	h.cfg.Waaseyaa.BatchSize = 3
	now := time.Now().UTC().Truncate(time.Second)
	h.seedCheckpoint(producer.Checkpoint{LastSuccessfulRun: now.Add(-time.Hour)})
	for i := range 3 {
		h.es.hits = append(h.es.hits, makeHit(t, fmt.Sprintf("id%d", i), now.Add(-time.Duration(3-i)*time.Minute)))
	}
	h.wc.script = []fakeWaaseyaaResp{successResp(3)}

	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.wc.calls != 1 {
		t.Errorf("expected 1 POST at exact batch boundary, got %d", h.wc.calls)
	}
}

func TestRun_TwoBatchesBothSucceed(t *testing.T) {
	h := newHarness(t)
	h.cfg.Waaseyaa.BatchSize = 3
	now := time.Now().UTC().Truncate(time.Second)
	h.seedCheckpoint(producer.Checkpoint{LastSuccessfulRun: now.Add(-time.Hour)})
	for i := range 4 {
		h.es.hits = append(h.es.hits, makeHit(t, fmt.Sprintf("id%d", i), now.Add(-time.Duration(4-i)*time.Minute)))
	}
	h.wc.script = []fakeWaaseyaaResp{successResp(3), successResp(1)}

	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.wc.calls != 2 {
		t.Errorf("expected 2 POSTs, got %d", h.wc.calls)
	}
	cp := h.loadCheckpoint()
	if cp.LastBatchSize != 4 {
		t.Errorf("LastBatchSize = %d, want 4", cp.LastBatchSize)
	}
}

func TestRun_FirstBatchSucceedsSecondFails(t *testing.T) {
	h := newHarness(t)
	h.cfg.Waaseyaa.BatchSize = 3
	now := time.Now().UTC().Truncate(time.Second)
	original := now.Add(-time.Hour)
	h.seedCheckpoint(producer.Checkpoint{LastSuccessfulRun: original})
	for i := range 4 {
		h.es.hits = append(h.es.hits, makeHit(t, fmt.Sprintf("id%d", i), now.Add(-time.Duration(4-i)*time.Minute)))
	}
	clientErr := fmt.Errorf("post: %w", client.ErrClientResponse())
	h.wc.script = []fakeWaaseyaaResp{
		successResp(3),
		{res: nil, err: clientErr},
	}

	err := h.run().Run(context.Background())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, client.ErrClientResponse()) {
		t.Errorf("expected wrapped errClient, got %v", err)
	}
	cp := h.loadCheckpoint()
	if !cp.LastSuccessfulRun.Equal(original) {
		t.Errorf("checkpoint advanced on partial failure: was %v now %v", original, cp.LastSuccessfulRun)
	}
}

func TestRun_MapperErrorOnOneHit(t *testing.T) {
	h := newHarness(t)
	now := time.Now().UTC().Truncate(time.Second)
	h.seedCheckpoint(producer.Checkpoint{LastSuccessfulRun: now.Add(-time.Hour)})
	for i := range 4 {
		h.es.hits = append(h.es.hits, makeHit(t, fmt.Sprintf("id%d", i), now.Add(-time.Duration(4-i)*time.Minute)))
	}
	// Add a malformed hit (missing required title).
	bad := makeHit(t, "bad", now)
	delete(bad, "title")
	h.es.hits = append(h.es.hits, bad)
	h.wc.script = []fakeWaaseyaaResp{successResp(4)}

	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.wc.totalSig != 4 {
		t.Errorf("expected 4 delivered (1 skipped), got %d", h.wc.totalSig)
	}
}

// makeUnmappableHit returns an ES hit guaranteed to fail mapping (missing
// the required title field — same trick as TestRun_MapperErrorOnOneHit).
func makeUnmappableHit(t *testing.T, id string, crawledAt time.Time) producer.ESHit {
	t.Helper()
	hit := makeHit(t, id, crawledAt)
	delete(hit, "title")
	return hit
}

func TestRun_AllHitsUnmappable_IncrementsCounter(t *testing.T) {
	h := newHarness(t)
	now := time.Now().UTC().Truncate(time.Second)
	original := now.Add(-time.Hour)
	h.seedCheckpoint(producer.Checkpoint{LastSuccessfulRun: original, ConsecutiveEmpty: 0})
	for i := range 3 {
		h.es.hits = append(h.es.hits, makeUnmappableHit(t, fmt.Sprintf("bad%d", i), now.Add(-time.Duration(3-i)*time.Minute)))
	}

	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.wc.calls != 0 {
		t.Errorf("expected 0 POSTs when all hits unmappable, got %d", h.wc.calls)
	}
	cp := h.loadCheckpoint()
	if cp.ConsecutiveEmpty != 1 {
		t.Errorf("ConsecutiveEmpty = %d, want 1 after all-unmappable run", cp.ConsecutiveEmpty)
	}
	if !cp.LastSuccessfulRun.Equal(original) {
		t.Errorf("timestamp advanced on all-unmappable run: was %v now %v", original, cp.LastSuccessfulRun)
	}
}

func TestRun_AllHitsUnmappable_FiresSourceDownAtThreshold(t *testing.T) {
	h := newHarness(t)
	now := time.Now().UTC().Truncate(time.Second)
	original := now.Add(-time.Hour)
	// Pre-seed counter at 2 so this run hits the threshold equality (3).
	h.seedCheckpoint(producer.Checkpoint{LastSuccessfulRun: original, ConsecutiveEmpty: 2})
	for i := range 2 {
		h.es.hits = append(h.es.hits, makeUnmappableHit(t, fmt.Sprintf("bad%d", i), now.Add(-time.Duration(2-i)*time.Minute)))
	}

	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.wc.calls != 0 {
		t.Errorf("expected 0 POSTs, got %d", h.wc.calls)
	}
	cp := h.loadCheckpoint()
	if cp.ConsecutiveEmpty != 3 {
		t.Errorf("ConsecutiveEmpty = %d at threshold, want 3", cp.ConsecutiveEmpty)
	}
	if !cp.LastSuccessfulRun.Equal(original) {
		t.Errorf("timestamp advanced at threshold: was %v now %v", original, cp.LastSuccessfulRun)
	}

	// Fourth run — counter still advances, WARN must NOT re-fire (the `==`
	// equality in handleNoSignals guards against re-spam, mirroring the
	// pure-empty TestRun_SourceDownThreshold case).
	h.es.hits = h.es.hits[:0]
	h.es.hits = append(h.es.hits, makeUnmappableHit(t, "bad-x", now))
	if err := h.run().Run(context.Background()); err != nil {
		t.Fatalf("Run again: %v", err)
	}
	cp = h.loadCheckpoint()
	if cp.ConsecutiveEmpty != 4 {
		t.Errorf("ConsecutiveEmpty = %d after 4th unmappable run, want 4", cp.ConsecutiveEmpty)
	}
}

func TestRun_ContextCancelled(t *testing.T) {
	h := newHarness(t)
	h.seedCheckpoint(producer.Checkpoint{LastSuccessfulRun: time.Now().UTC().Add(-time.Hour)})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := h.run().Run(ctx)
	if err == nil {
		t.Fatalf("expected error from cancelled ctx")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if len(h.es.queries) != 0 {
		t.Errorf("expected no ES calls, got %d", len(h.es.queries))
	}
}

func TestBuildQuery_Shape(t *testing.T) {
	cp := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	q := producer.BuildQuery(cp, 5*time.Minute, 40)
	bs, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal query: %v", err)
	}
	s := string(bs)
	for _, want := range []string{
		`"crawled_at":{"gte":"2026-04-27T09:55:00Z"}`,
		`"content_type":["rfp","need_signal"]`,
		`"quality_score":{"gte":40}`,
		`"size":100`,
	} {
		if !contains(s, want) {
			t.Errorf("query missing %q\nfull: %s", want, s)
		}
	}
}

// contains is a tiny strings.Contains alias to keep imports minimal.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
