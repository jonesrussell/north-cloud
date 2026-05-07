//go:build integration

package runner_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	smokeDefaultESURL    = "http://localhost:9200"
	smokeDefaultRedisURL = "localhost:6379"
	smokeAlertsIndex     = "community_alerts_runner_smoke"
	smokeLifecycleChan   = "community_alerts:lifecycle:smoke"
	smokePollTimeout     = 10 * time.Second
	smokePollInterval    = 30 * time.Minute
	smokeDefaultExpiry   = 72 * time.Hour
)

// smokeEnv returns an env var value or the given default.
func smokeEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

// TestRunnerIntegrationSmoke wires a real Runner with real ES + Redis and
// runs one poll cycle against a fixture httptest server. It asserts that Run
// returns without error. Detailed assertions live in the integration/ package;
// this test exists as a CI hook that exercises the runner wiring directly.
func TestRunnerIntegrationSmoke(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if os.Getenv("ALERT_CRAWLER_INTEGRATION") != "true" {
		t.Skip("skipping integration test: ALERT_CRAWLER_INTEGRATION is not 'true'")
	}

	ctx := context.Background()
	esBaseURL := smokeEnv("ALERT_CRAWLER_INTEGRATION_ES_URL", smokeDefaultESURL)
	redisAddr := smokeEnv("ALERT_CRAWLER_INTEGRATION_REDIS_URL", smokeDefaultRedisURL)

	feedBody := buildSmokeFeed()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(feedBody))
	}))
	defer srv.Close()

	ix := elasticsearch.New(elasticsearch.Config{
		BaseURL: esBaseURL,
		Index:   smokeAlertsIndex,
	})
	require.NoError(t, ix.EnsureIndex(ctx))

	dbPath := t.TempDir() + "/smoke.db"
	st, stErr := catalogue.Open(ctx, dbPath)
	require.NoError(t, stErr)
	defer func() { _ = st.Close() }()

	pub, pubErr := infraredis.New(infraredis.Config{
		Address: redisAddr,
		Channel: smokeLifecycleChan,
	})
	require.NoError(t, pubErr)
	defer func() { _ = pub.Close() }()

	log, logErr := infralogger.New(infralogger.Config{Level: "error", Format: "console"})
	require.NoError(t, logErr)
	metrics := observability.New(log)

	sevTable := severity.NewTable(map[string]domain.Severity{
		"fentanyl": domain.SeverityHigh,
	})

	r := runner.New(runner.Dependencies{
		Fetch:    rss.New(rss.WithTimeout(smokePollTimeout)),
		Store:    st,
		Indexer:  ix,
		Pub:      pub,
		Resolver: scope.New(),
		SevInfer: func(hazard domain.Hazard) domain.Severity {
			return severity.Infer(hazard, sevTable)
		},
		Metrics: metrics,
		Sources: []domain.AlertSource{
			{
				ID:                  "runner-smoke",
				Name:                "Runner Smoke",
				FeedURL:             srv.URL,
				Enabled:             true,
				PollInterval:        smokePollInterval,
				DefaultScope:        []string{"treaty:1"},
				DefaultCategory:     domain.CategoryHarmReduction,
				DefaultExpiry:       smokeDefaultExpiry,
				AcquisitionStrategy: domain.AcquisitionRSS,
			},
		},
		DefaultExpiry: smokeDefaultExpiry,
	})

	require.NoError(t, r.Run(ctx), "smoke poll cycle must succeed")
}

// buildSmokeFeed returns a minimal RSS 2.0 body for the smoke test.
func buildSmokeFeed() string {
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<rss version="2.0"><channel><title>Smoke</title>` +
		`<item>` +
		`<title>Fentanyl smoke advisory</title>` +
		`<link>https://safersites.example.ca/alerts/smoke-001</link>` +
		`<pubDate>Mon, 06 Jan 2025 12:00:00 -0600</pubDate>` +
		`<description>Fentanyl detected. Lab source: FTIR.</description>` +
		`</item>` +
		`</channel></rss>`
}
