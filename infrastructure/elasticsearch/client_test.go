package elasticsearch

import (
	"context"
	"testing"
	"time"

	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/retry"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already has http://",
			input:    "http://elasticsearch:9200",
			expected: "http://elasticsearch:9200",
		},
		{
			name:     "already has https://",
			input:    "https://elasticsearch:9200",
			expected: "https://elasticsearch:9200",
		},
		{
			name:     "missing protocol",
			input:    "elasticsearch:9200",
			expected: "http://elasticsearch:9200",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "http://localhost:9200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected Config
	}{
		{
			name:   "empty config gets defaults",
			config: Config{},
			expected: Config{
				URL:        "http://localhost:9200",
				MaxRetries: 3,
				PingTimeout: 5 * time.Second,
				RetryConfig: &retry.Config{
					MaxAttempts:  5,
					InitialDelay: 2 * time.Second,
					MaxDelay:     10 * time.Second,
					Multiplier:   2.0,
				},
			},
		},
		{
			name: "partial config preserves values",
			config: Config{
				URL:        "http://custom:9200",
				MaxRetries: 5,
			},
			expected: Config{
				URL:        "http://custom:9200",
				MaxRetries: 5,
				PingTimeout: 5 * time.Second,
				RetryConfig: &retry.Config{
					MaxAttempts:  5,
					InitialDelay: 2 * time.Second,
					MaxDelay:     10 * time.Second,
					Multiplier:   2.0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				rc := tt.config.RetryConfig
				erc := tt.expected.RetryConfig
				if rc.MaxAttempts != erc.MaxAttempts {
					t.Errorf("RetryConfig.MaxAttempts = %d, want %d", rc.MaxAttempts, erc.MaxAttempts)
				}
				if rc.InitialDelay != erc.InitialDelay {
					t.Errorf("RetryConfig.InitialDelay = %v, want %v", rc.InitialDelay, erc.InitialDelay)
				}
				if rc.MaxDelay != erc.MaxDelay {
					t.Errorf("RetryConfig.MaxDelay = %v, want %v", rc.MaxDelay, erc.MaxDelay)
				}
				if rc.Multiplier != erc.Multiplier {
					t.Errorf("RetryConfig.Multiplier = %f, want %f", rc.Multiplier, erc.Multiplier)
				}
			}
		})
	}
}

func TestCreateTransport(t *testing.T) {
	tests := []struct {
		name      string
		tlsConfig *TLSConfig
		expectTLS bool
	}{
		{
			name:      "nil TLS config",
			tlsConfig: nil,
			expectTLS: false,
		},
		{
			name: "disabled TLS",
			tlsConfig: &TLSConfig{
				Enabled: false,
			},
			expectTLS: false,
		},
		{
			name: "enabled TLS",
			tlsConfig: &TLSConfig{
				Enabled:            true,
				InsecureSkipVerify: true,
			},
			expectTLS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := createTransport(tt.tlsConfig)

			if transport == nil {
				t.Fatal("transport is nil")
			}

			if tt.expectTLS {
				if transport.TLSClientConfig == nil {
					t.Error("TLSClientConfig is nil, expected non-nil")
				} else if tt.tlsConfig != nil && transport.TLSClientConfig.InsecureSkipVerify != tt.tlsConfig.InsecureSkipVerify {
					t.Errorf("InsecureSkipVerify = %v, want %v",
						transport.TLSClientConfig.InsecureSkipVerify,
						tt.tlsConfig.InsecureSkipVerify)
				}
			} else {
				// When TLS is not expected, TLSClientConfig should be nil
				if transport.TLSClientConfig != nil {
					t.Error("TLSClientConfig is not nil, expected nil")
				}
			}
		})
	}
}

// TestNewClient_InvalidURL tests that NewClient handles invalid URLs gracefully
// This is a unit test that doesn't require a running Elasticsearch instance
func TestNewClient_InvalidURL(t *testing.T) {
	ctx := context.Background()
	log, err := logger.New(logger.Config{
		Level:  "debug",
		Format: "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	cfg := Config{
		URL:        "http://invalid-host-that-does-not-exist:9200",
		MaxRetries: 1,
		PingTimeout: 1 * time.Second,
		RetryConfig: &retry.Config{
			MaxAttempts:  2, // Only 2 attempts for faster test
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     500 * time.Millisecond,
			Multiplier:   2.0,
		},
	}

	_, err = NewClient(ctx, cfg, log)
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}

	// Verify error message contains expected text
	if err != nil && !contains(err.Error(), "failed to connect") {
		t.Errorf("Expected error to contain 'failed to connect', got: %v", err)
	}
}

// contains checks if a string contains a substring
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
