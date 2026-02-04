//go:build integration
// +build integration

// classifier/internal/classifier/integration_test.go
package classifier_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
)

const (
	mlServiceURL       = "http://localhost:8076"
	healthCheckTimeout = 5 * time.Second
)

// mockIntegrationLogger implements the Logger interface for integration testing.
type mockIntegrationLogger struct{}

func (m *mockIntegrationLogger) Debug(_ string, _ ...any) {}
func (m *mockIntegrationLogger) Info(_ string, _ ...any)  {}
func (m *mockIntegrationLogger) Warn(_ string, _ ...any)  {}
func (m *mockIntegrationLogger) Error(_ string, _ ...any) {}
func (m *mockIntegrationLogger) Fatal(_ string, _ ...any) {}
func (m *mockIntegrationLogger) With(_ ...any) *mockIntegrationLogger {
	return m
}
func (m *mockIntegrationLogger) Sync() error { return nil }

func TestCrimeClassifier_Integration(t *testing.T) {
	t.Helper()

	// Skip if ML service not available
	client := mlclient.NewClient(mlServiceURL)
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()

	if err := client.Health(ctx); err != nil {
		t.Skip("ML service not available, skipping integration test")
	}

	// Create classifier with ML client
	// Note: Using nil logger for simplicity in integration tests
	sc := classifier.NewCrimeClassifier(client, nil, true)

	tests := []struct {
		name            string
		title           string
		body            string
		expectRelevance string
		expectHomepage  bool
	}{
		{
			name:            "murder article",
			title:           "Man charged with murder after downtown stabbing",
			body:            "Police arrested a suspect following a fatal stabbing incident.",
			expectRelevance: "core_street_crime",
			expectHomepage:  true,
		},
		{
			name:            "restaurant article",
			title:           "New restaurant opens in downtown area",
			body:            "A new fine dining establishment has opened.",
			expectRelevance: "not_crime",
			expectHomepage:  false,
		},
		{
			name:            "shooting article",
			title:           "Police respond to shooting in north end",
			body:            "Officers responded to reports of gunfire late last night.",
			expectRelevance: "core_street_crime",
			expectHomepage:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &domain.RawContent{
				ID:      "test-" + tt.name,
				Title:   tt.title,
				RawText: tt.body,
			}

			result, err := sc.Classify(context.Background(), raw)
			if err != nil {
				t.Fatalf("classification failed: %v", err)
			}

			if result.Relevance != tt.expectRelevance {
				t.Errorf("relevance: got %s, want %s", result.Relevance, tt.expectRelevance)
			}

			if result.HomepageEligible != tt.expectHomepage {
				t.Errorf("homepage eligible: got %v, want %v", result.HomepageEligible, tt.expectHomepage)
			}
		})
	}
}
