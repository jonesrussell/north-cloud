package elasticsearch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/infrastructure/elasticsearch"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/infrastructure/retry"
)

func TestNewClient_NormalizesURLWithoutProtocol(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := elasticsearch.Config{
		URL:         server.Listener.Addr().String(),
		MaxRetries:  1,
		PingTimeout: time.Second,
		RetryConfig: &retry.Config{
			MaxAttempts:  1,
			InitialDelay: time.Millisecond,
			MaxDelay:     time.Millisecond,
			Multiplier:   1,
		},
	}

	client, err := elasticsearch.NewClient(context.Background(), cfg, testLogger(t))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
}

func TestNewClient_UsesTLSConfig(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := elasticsearch.Config{
		URL:         server.URL,
		MaxRetries:  1,
		PingTimeout: time.Second,
		TLS: &elasticsearch.TLSConfig{
			Enabled:            true,
			InsecureSkipVerify: true,
		},
		RetryConfig: &retry.Config{
			MaxAttempts:  1,
			InitialDelay: time.Millisecond,
			MaxDelay:     time.Millisecond,
			Multiplier:   1,
		},
	}

	client, err := elasticsearch.NewClient(context.Background(), cfg, testLogger(t))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   elasticsearch.Config
		expected elasticsearch.Config
	}{
		{
			name:   "empty config gets defaults",
			config: elasticsearch.Config{},
			expected: elasticsearch.Config{
				URL:         "http://localhost:9200",
				MaxRetries:  elasticsearch.DefaultMaxRetries,
				PingTimeout: elasticsearch.DefaultPingTimeout,
				RetryConfig: &retry.Config{
					MaxAttempts:  elasticsearch.DefaultRetryMaxAttempts,
					InitialDelay: elasticsearch.DefaultRetryInitialDelay,
					MaxDelay:     elasticsearch.DefaultRetryMaxDelay,
					Multiplier:   elasticsearch.DefaultRetryMultiplier,
				},
			},
		},
		{
			name: "partial config preserves values",
			config: elasticsearch.Config{
				URL:        "http://custom:9200",
				MaxRetries: 5,
			},
			expected: elasticsearch.Config{
				URL:         "http://custom:9200",
				MaxRetries:  5,
				PingTimeout: elasticsearch.DefaultPingTimeout,
				RetryConfig: &retry.Config{
					MaxAttempts:  elasticsearch.DefaultRetryMaxAttempts,
					InitialDelay: elasticsearch.DefaultRetryInitialDelay,
					MaxDelay:     elasticsearch.DefaultRetryMaxDelay,
					Multiplier:   elasticsearch.DefaultRetryMultiplier,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.config.SetDefaults()

			if tt.config.URL != tt.expected.URL {
				t.Errorf("URL = %q, want %q", tt.config.URL, tt.expected.URL)
			}
			if tt.config.MaxRetries != tt.expected.MaxRetries {
				t.Errorf("MaxRetries = %d, want %d", tt.config.MaxRetries, tt.expected.MaxRetries)
			}
			if tt.config.PingTimeout != tt.expected.PingTimeout {
				t.Errorf("PingTimeout = %v, want %v", tt.config.PingTimeout, tt.expected.PingTimeout)
			}
			if tt.config.RetryConfig == nil {
				t.Error("RetryConfig is nil, expected non-nil")
			} else {
				assertRetryConfig(t, tt.config.RetryConfig, tt.expected.RetryConfig)
			}
		})
	}
}

func TestNewClient_InvalidURL(t *testing.T) {
	t.Parallel()

	cfg := elasticsearch.Config{
		URL:         "http://invalid-host-that-does-not-exist:9200",
		MaxRetries:  1,
		PingTimeout: time.Second,
		RetryConfig: &retry.Config{
			MaxAttempts:  2,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     500 * time.Millisecond,
			Multiplier:   2.0,
		},
	}

	_, err := elasticsearch.NewClient(context.Background(), cfg, testLogger(t))
	if err == nil {
		t.Fatal("Expected error for invalid URL, got nil")
	}

	if !contains(err.Error(), "failed to connect") {
		t.Errorf("Expected error to contain 'failed to connect', got: %v", err)
	}
}

func assertRetryConfig(t *testing.T, got, want *retry.Config) {
	t.Helper()

	if got.MaxAttempts != want.MaxAttempts {
		t.Errorf("RetryConfig.MaxAttempts = %d, want %d", got.MaxAttempts, want.MaxAttempts)
	}
	if got.InitialDelay != want.InitialDelay {
		t.Errorf("RetryConfig.InitialDelay = %v, want %v", got.InitialDelay, want.InitialDelay)
	}
	if got.MaxDelay != want.MaxDelay {
		t.Errorf("RetryConfig.MaxDelay = %v, want %v", got.MaxDelay, want.MaxDelay)
	}
	if got.Multiplier != want.Multiplier {
		t.Errorf("RetryConfig.Multiplier = %f, want %f", got.Multiplier, want.Multiplier)
	}
}

func testLogger(t *testing.T) logger.Logger {
	t.Helper()

	log, err := logger.New(logger.Config{
		Level:  "debug",
		Format: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return log
}

func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
