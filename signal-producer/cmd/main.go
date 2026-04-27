// Command signal-producer is the one-shot binary that drains classified
// content from Elasticsearch, maps each hit to the Waaseyaa /api/signals
// schema, and POSTs batches.
//
// One process invocation = one timer firing. systemd records the unit failure
// when this process exits non-zero (FR-016).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"

	"github.com/jonesrussell/north-cloud/signal-producer/internal/client"
	spconfig "github.com/jonesrussell/north-cloud/signal-producer/internal/config"
	"github.com/jonesrussell/north-cloud/signal-producer/internal/producer"
)

// defaultConfigPath is consulted when CONFIG_PATH is unset.
const defaultConfigPath = "config.yml"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		// Logger may not have been built yet; fall back to stderr.
		fmt.Fprintf(os.Stderr, "signal-producer: %v\n", err)
		os.Exit(1)
	}
}

// run wires dependencies and invokes producer.Run. Returning an error makes
// main() exit non-zero so systemd records the unit as failed (FR-016).
func run(ctx context.Context) error {
	// infraconfig.GetConfigPath internally reads CONFIG_PATH and falls back
	// to the supplied default. os.Getenv usage is only allowed in cmd/main.go.
	configPath := infraconfig.GetConfigPath(defaultConfigPath)
	// Allow CONFIG_PATH="-" to skip YAML and use env-only config (useful in
	// minimal containers).
	yamlPath := configPath
	if configPath == "-" {
		yamlPath = ""
	} else if _, statErr := os.Stat(configPath); errors.Is(statErr, os.ErrNotExist) {
		// No file on disk — use env-only.
		yamlPath = ""
	}

	cfg, err := spconfig.Load(yamlPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := infralogger.New(infralogger.Config{Level: os.Getenv("LOG_LEVEL")})
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	esClient, err := newESClient(cfg.Elasticsearch.URL)
	if err != nil {
		return fmt.Errorf("build es client: %w", err)
	}

	waaseyaaClient, err := client.New(client.Config{
		BaseURL: cfg.Waaseyaa.URL,
		APIKey:  cfg.Waaseyaa.APIKey,
		Logger:  log,
	})
	if err != nil {
		return fmt.Errorf("build waaseyaa client: %w", err)
	}

	p := producer.New(toProducerConfig(cfg), &esSearcher{client: esClient}, waaseyaaClient, log)

	if runErr := p.Run(ctx); runErr != nil {
		log.Error("run failed", infralogger.Error(runErr))
		return runErr
	}
	return nil
}

// toProducerConfig translates the load-time config into the runtime shape the
// producer expects. This decouples the YAML schema from the producer's
// internal field names.
func toProducerConfig(c *spconfig.Config) producer.Config {
	return producer.Config{
		Waaseyaa: producer.WaaseyaaConfig{
			URL:             c.Waaseyaa.URL,
			APIKey:          c.Waaseyaa.APIKey,
			BatchSize:       c.Waaseyaa.BatchSize,
			MinQualityScore: c.Waaseyaa.MinQualityScore,
		},
		Elasticsearch: producer.ElasticsearchConfig{
			URL:     c.Elasticsearch.URL,
			Indexes: c.Elasticsearch.Indexes,
		},
		Schedule: producer.ScheduleConfig{
			LookbackBuffer: c.Schedule.LookbackBuffer,
		},
		Checkpoint: producer.CheckpointConfig{
			File: c.Checkpoint.File,
		},
	}
}

// newESClient returns a configured go-elasticsearch client. We intentionally
// keep this thin (no infra retry wrapper) because each producer run is a
// one-shot and the operator can rerun via the timer.
func newESClient(esURL string) (*es.Client, error) {
	cfg := es.Config{
		Addresses: []string{esURL},
		Transport: &http.Transport{},
	}
	c, err := es.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch.NewClient: %w", err)
	}
	return c, nil
}

// esSearcher adapts *es.Client to the producer.ESClient interface.
type esSearcher struct {
	client *es.Client
}

// Search executes a search across the configured indexes and returns the
// hits as []map[string]any. The mapper consumes a flat hit shape, so we
// merge the ES `_source` fields with the synthetic `_id`.
func (s *esSearcher) Search(ctx context.Context, indexes []string, query map[string]any) ([]producer.ESHit, error) {
	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("es search: marshal query: %w", err)
	}
	req := esapi.SearchRequest{
		Index: indexes,
		Body:  strings.NewReader(string(body)),
	}
	res, err := req.Do(ctx, s.client)
	if err != nil {
		return nil, fmt.Errorf("es search: do: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		raw, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("es search: status=%s body=%s", res.Status(), string(raw))
	}
	return decodeHits(res.Body)
}

// searchEnvelope mirrors the subset of the ES search response we actually
// read. We avoid pulling in the official typedapi just for this.
type searchEnvelope struct {
	Hits struct {
		Hits []struct {
			ID     string         `json:"_id"`
			Source map[string]any `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// decodeHits flattens the ES response into the [_id+_source merged] map shape
// the mapper expects. Always returns a non-nil slice.
func decodeHits(r io.Reader) ([]producer.ESHit, error) {
	var env searchEnvelope
	if err := json.NewDecoder(r).Decode(&env); err != nil {
		return nil, fmt.Errorf("es search: decode response: %w", err)
	}
	out := make([]producer.ESHit, 0, len(env.Hits.Hits))
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

