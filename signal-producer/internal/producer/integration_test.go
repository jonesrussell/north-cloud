//go:build integration

package producer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-producer/internal/client"
)

// esURL returns the ES base URL for the integration test, defaulting to the
// dev cluster if the harness env var is not set.
func esURL() string {
	if v := os.Getenv("INTEGRATION_ES_URL"); v != "" {
		return v
	}
	return "http://localhost:9200"
}

// integrationESSearcher is a real ES adapter that satisfies producer.ESClient.
// It is intentionally minimal — no retries, no auth — because the integration
// harness only runs against a clean local dev cluster.
type integrationESSearcher struct {
	base string
	hc   *http.Client
}

func (s *integrationESSearcher) Search(ctx context.Context, indexes []string, query map[string]any) ([]ESHit, error) {
	body, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	idxPath := strings.Join(indexes, ",")
	u := fmt.Sprintf("%s/%s/_search", s.base, url.PathEscape(idxPath))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("es status %d: %s", resp.StatusCode, string(raw))
	}
	var env struct {
		Hits struct {
			Hits []struct {
				ID     string         `json:"_id"`
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	out := make([]ESHit, 0, len(env.Hits.Hits))
	for _, h := range env.Hits.Hits {
		hit := make(map[string]any, len(h.Source)+1)
		for k, v := range h.Source {
			hit[k] = v
		}
		hit["_id"] = h.ID
		out = append(out, hit)
	}
	return out, nil
}

func setupTestES(t *testing.T) (string, *integrationESSearcher) {
	t.Helper()
	base := esURL()
	idx := fmt.Sprintf("signal-producer-it-%d", time.Now().UnixNano())

	// Create the index.
	createReq, _ := http.NewRequest(http.MethodPut, base+"/"+idx, nil)
	resp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Skipf("ES unreachable at %s: %v", base, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Skipf("ES create index returned %d", resp.StatusCode)
	}

	t.Cleanup(func() {
		delReq, _ := http.NewRequest(http.MethodDelete, base+"/"+idx, nil)
		if r, err := http.DefaultClient.Do(delReq); err == nil {
			_ = r.Body.Close()
		}
	})

	return idx, &integrationESSearcher{base: base, hc: http.DefaultClient}
}

func seedFixtures(t *testing.T, idx string) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	docs := []map[string]any{
		{
			"_id":           "rfp-001",
			"title":         "Bridge construction services",
			"quality_score": 78,
			"url":           "https://canadabuys.example/rfp-001",
			"crawled_at":    now.Add(-30 * time.Minute).Format(time.RFC3339),
			"content_type":  "rfp",
			"rfp": map[string]any{
				"organization_name": "Government of Canada",
				"province":          "ON",
				"categories":        []any{"Construction"},
			},
		},
		{
			"_id":           "sig-001",
			"title":         "Need: web developer",
			"quality_score": 55,
			"url":           "https://example.org/sig-001",
			"crawled_at":    now.Add(-10 * time.Minute).Format(time.RFC3339),
			"content_type":  "need_signal",
			"need_signal": map[string]any{
				"organization_name": "Example Co",
				"province":          "BC",
				"sector":            "Tech",
				"signal_type":       "rfq",
			},
		},
	}
	for _, doc := range docs {
		body, _ := json.Marshal(doc)
		id := doc["_id"].(string)
		req, _ := http.NewRequest(http.MethodPut, esURL()+"/"+idx+"/_doc/"+id+"?refresh=wait_for", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 400 {
			t.Fatalf("seed status %d", resp.StatusCode)
		}
	}
}

// startMockWaaseyaa returns an httptest.Server stand-in plus a thread-safe
// captured list of received bodies.
func startMockWaaseyaa(t *testing.T) (*httptest.Server, func() []client.SignalBatch) {
	t.Helper()
	var (
		mu       sync.Mutex
		captured []client.SignalBatch
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch client.SignalBatch
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		captured = append(captured, batch)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(client.IngestResult{Ingested: len(batch.Signals)})
	}))
	t.Cleanup(srv.Close)
	get := func() []client.SignalBatch {
		mu.Lock()
		defer mu.Unlock()
		out := make([]client.SignalBatch, len(captured))
		copy(out, captured)
		return out
	}
	return srv, get
}

func TestProducer_EndToEnd(t *testing.T) {
	idx, esClient := setupTestES(t)
	seedFixtures(t, idx)

	srv, getBatches := startMockWaaseyaa(t)

	dir := t.TempDir()
	cpFile := filepath.Join(dir, "checkpoint.json")
	cfg := Config{
		Waaseyaa: WaaseyaaConfig{
			URL:             srv.URL,
			APIKey:          "test-key",
			BatchSize:       50,
			MinQualityScore: 40,
		},
		Elasticsearch: ElasticsearchConfig{
			URL:     esURL(),
			Indexes: []string{idx},
		},
		Schedule:   ScheduleConfig{LookbackBuffer: 5 * time.Minute},
		Checkpoint: CheckpointConfig{File: cpFile},
	}

	wc, err := client.New(client.Config{
		BaseURL:  srv.URL,
		APIKey:   "test-key",
		Logger:   mustLogger(t),
		Backoffs: []time.Duration{},
	})
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}

	p := New(cfg, esClient, wc, mustLogger(t))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := p.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	batches := getBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch posted, got %d", len(batches))
	}
	if len(batches[0].Signals) != 2 {
		t.Errorf("expected 2 signals in batch, got %d", len(batches[0].Signals))
	}

	if _, statErr := os.Stat(cpFile); statErr != nil {
		t.Fatalf("checkpoint not written: %v", statErr)
	}
	data, _ := os.ReadFile(cpFile)
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		t.Fatalf("checkpoint parse: %v", err)
	}
	if cp.LastBatchSize != 2 {
		t.Errorf("LastBatchSize = %d, want 2", cp.LastBatchSize)
	}
	if cp.LastSuccessfulRun.IsZero() {
		t.Error("LastSuccessfulRun is zero")
	}
}

func mustLogger(t *testing.T) infralogger.Logger {
	t.Helper()
	l, err := infralogger.New(infralogger.Config{Level: "info"})
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	return l
}
