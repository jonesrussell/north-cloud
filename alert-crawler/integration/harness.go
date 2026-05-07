//go:build integration

// Package integration contains end-to-end acceptance tests for alert-crawler.
// Tests in this package require real Elasticsearch, Redis, and SQLite.
// They are excluded from the default unit-test run and execute only when the
// ALERT_CRAWLER_INTEGRATION env var is set to "true".
//
// Run with:
//
//	ALERT_CRAWLER_INTEGRATION=true go test -tags integration ./integration/...
package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/observability"
	infraredis "github.com/jonesrussell/north-cloud/alert-crawler/internal/redis"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/runner"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/scope"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/severity"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const (
	defaultESURL     = "http://localhost:9200"
	defaultRedisURL  = "redis://localhost:6379"
	integrationEnv   = "ALERT_CRAWLER_INTEGRATION"
	esURLEnv         = "ALERT_CRAWLER_INTEGRATION_ES_URL"
	redisURLEnv      = "ALERT_CRAWLER_INTEGRATION_REDIS_URL"
	alertsIndex      = "community_alerts"
	lifecycleChannel = "community_alerts:lifecycle"

	// harnessRSSTimeout is the HTTP timeout used by the harness RSS fetcher.
	harnessRSSTimeout = 10 * time.Second
	// harnessDefaultExpiry is the default alert expiry wired into NewRunner.
	harnessDefaultExpiry = 72 * time.Hour
)

// WithIntegration skips the test if ALERT_CRAWLER_INTEGRATION != "true"
// or if testing.Short() is set.
func WithIntegration(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv(integrationEnv) != "true" {
		t.Skipf("skipping integration test: %s is not set to 'true'", integrationEnv)
	}
}

// esURL returns the Elasticsearch URL from env or the default.
func esURL() string {
	if u := os.Getenv(esURLEnv); u != "" {
		return u
	}

	return defaultESURL
}

// redisAddr parses the Redis URL from env and returns host:port.
// Supports redis://host:port and bare host:port forms.
func redisAddr() string {
	rawURL := os.Getenv(redisURLEnv)
	if rawURL == "" {
		rawURL = defaultRedisURL
	}

	// Strip "redis://" prefix if present.
	const prefix = "redis://"
	if len(rawURL) > len(prefix) && rawURL[:len(prefix)] == prefix {
		return rawURL[len(prefix):]
	}

	return rawURL
}

// Harness holds all wired dependencies for one integration test.
// Obtain via NewHarness; call Cleanup when the test is done.
type Harness struct {
	t         *testing.T
	Indexer   *elasticsearch.Indexer
	Publisher *infraredis.Publisher
	Store     *catalogue.Store
	Resolver  *scope.Resolver
	SevTable  severity.Table
	Metrics   *observability.Metrics
	Fetcher   *rss.Client
	dbPath    string
	sub       *Subscriber
}

// NewHarness bootstraps a Harness ready for one integration test.
// It deletes and recreates the community_alerts index for isolation.
func NewHarness(t *testing.T) *Harness {
	t.Helper()

	ctx := context.Background()

	// --- Elasticsearch ---
	ix := elasticsearch.New(elasticsearch.Config{
		BaseURL: esURL(),
		Index:   alertsIndex,
	})
	require.NoError(t, deleteIndex(ctx, esURL(), alertsIndex), "delete index for isolation")
	require.NoError(t, ix.EnsureIndex(ctx), "recreate index")

	// --- SQLite catalogue ---
	dbPath := t.TempDir() + "/alerts.db"
	st, err := catalogue.Open(ctx, dbPath)
	require.NoError(t, err, "open catalogue")

	// --- Redis publisher ---
	addr := redisAddr()
	pub, err := infraredis.New(infraredis.Config{
		Address: addr,
		Channel: lifecycleChannel,
	})
	require.NoError(t, err, "connect redis publisher")

	// --- Subscriber (for event assertions) ---
	sub, err := NewSubscriber(addr, lifecycleChannel)
	require.NoError(t, err, "connect redis subscriber")

	// --- Scope resolver + severity table ---
	res := scope.New()
	sevTable := severity.NewTable(map[string]domain.Severity{
		"fentanyl":        domain.SeverityHigh,
		"carfentanil":     domain.SeverityCritical,
		"methamphetamine": domain.SeverityHigh,
		"benzodiazepine":  domain.SeverityMedium,
	})

	// --- Logger + metrics ---
	log, logErr := infralogger.New(infralogger.Config{Level: "error", Format: "console"})
	require.NoError(t, logErr, "create logger")
	metrics := observability.New(log)

	// --- RSS fetcher ---
	fetcher := rss.New(rss.WithTimeout(harnessRSSTimeout))

	return &Harness{
		t:         t,
		Indexer:   ix,
		Publisher: pub,
		Store:     st,
		Resolver:  res,
		SevTable:  sevTable,
		Metrics:   metrics,
		Fetcher:   fetcher,
		dbPath:    dbPath,
		sub:       sub,
	}
}

// Cleanup closes all clients. Call via t.Cleanup(h.Cleanup).
func (h *Harness) Cleanup() {
	h.t.Helper()

	if h.sub != nil {
		h.sub.Close()
	}

	if h.Publisher != nil {
		_ = h.Publisher.Close()
	}

	if h.Store != nil {
		_ = h.Store.Close()
	}
}

// NewRunner wires a runner.Runner from the harness for the given sources.
func (h *Harness) NewRunner(sources []domain.AlertSource) *runner.Runner {
	h.t.Helper()

	sevTable := h.SevTable

	return runner.New(runner.Dependencies{
		Fetch:         h.Fetcher,
		Store:         h.Store,
		Indexer:       h.Indexer,
		Pub:           h.Publisher,
		Resolver:      h.Resolver,
		SevInfer:      func(hazard domain.Hazard) domain.Severity { return severity.Infer(hazard, sevTable) },
		Metrics:       h.Metrics,
		Sources:       sources,
		DefaultExpiry: harnessDefaultExpiry,
	})
}

// QueryActiveAlerts returns all currently active alerts from ES.
func (h *Harness) QueryActiveAlerts(ctx context.Context) ([]domain.Alert, error) {
	return h.Indexer.QueryActive(ctx, "")
}

// WaitForEvent returns the next lifecycle event from the subscriber within timeout.
// The second return value is false if the timeout elapsed without an event.
func (h *Harness) WaitForEvent(t *testing.T, timeout time.Duration) (domain.LifecycleEvent, bool) {
	t.Helper()

	return h.sub.Receive(timeout)
}

// deleteIndex issues a DELETE request against the given ES index.
// A 404 response is treated as success (index already absent).
func deleteIndex(ctx context.Context, baseURL, index string) error {
	url := fmt.Sprintf("%s/%s", baseURL, index)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("build DELETE request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE index: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	return fmt.Errorf("DELETE index returned %d", resp.StatusCode)
}
