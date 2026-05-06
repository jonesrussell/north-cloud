package catalogue_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// stubESClient is a test double for ESActiveAlertQuerier.
type stubESClient struct {
	alerts []domain.Alert
}

func (c *stubESClient) QueryActiveAlertIDs(_ context.Context) ([]domain.Alert, error) {
	return c.alerts, nil
}

func makeTestAlert(id, sourceID string) domain.Alert {
	return domain.Alert{
		ID:             id,
		Category:       domain.CategoryHarmReduction,
		Severity:       domain.SeverityMedium,
		Scope:          []string{"test-scope"},
		IssuedAt:       time.Now().UTC(),
		LifecycleState: domain.LifecycleActive,
		Title:          "Test Alert " + id,
		Summary:        "Test summary",
		ParseQuality:   domain.ParseClean,
		CrawledAt:      time.Now().UTC(),
		LastUpdatedAt:  time.Now().UTC(),
		Sources: []domain.SourceAttribution{
			{
				SourceID:   sourceID,
				SourceName: "Test Source",
				URL:        "https://example.com/feed",
			},
		},
	}
}

func TestRebuildFromES_Idempotent(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)
	ctx := context.Background()

	alerts := []domain.Alert{
		makeTestAlert("alert-001", "src-rebuild"),
		makeTestAlert("alert-002", "src-rebuild"),
	}

	stub := &stubESClient{alerts: alerts}

	// First rebuild.
	require.NoError(t, s.RebuildFromES(ctx, stub))

	for _, a := range alerts {
		got, err := s.LookupAlert(ctx, "src-rebuild", a.ID)
		require.NoError(t, err, "alert %s should exist after rebuild", a.ID)
		assert.True(t, got.IsActive)
	}

	// Second rebuild — idempotent; no error, existing rows replaced in place.
	require.NoError(t, s.RebuildFromES(ctx, stub))

	for _, a := range alerts {
		got, err := s.LookupAlert(ctx, "src-rebuild", a.ID)
		require.NoError(t, err, "alert %s should still exist after second rebuild", a.ID)
		assert.True(t, got.IsActive)
	}
}

func TestRebuildFromES_EmptyResult(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)

	stub := &stubESClient{alerts: []domain.Alert{}}

	err := s.RebuildFromES(context.Background(), stub)
	assert.NoError(t, err, "rebuild with empty ES result must succeed")
}

func TestRebuildFromES_AlertWithNoSources(t *testing.T) {
	t.Parallel()

	s := openMemStore(t)
	ctx := context.Background()

	// Alert with no sources — sourceIDFromAlert falls back to empty string.
	alert := domain.Alert{
		ID:             "alert-nosrc",
		Category:       domain.CategoryHarmReduction,
		Severity:       domain.SeverityLow,
		Scope:          []string{"scope"},
		IssuedAt:       time.Now().UTC(),
		LifecycleState: domain.LifecycleActive,
		Title:          "No Sources",
		Summary:        "summary",
		ParseQuality:   domain.ParseClean,
		CrawledAt:      time.Now().UTC(),
		LastUpdatedAt:  time.Now().UTC(),
		Sources:        nil, // no attribution
	}

	stub := &stubESClient{alerts: []domain.Alert{alert}}

	require.NoError(t, s.RebuildFromES(ctx, stub))

	// Alert inserted with source_id = "".
	got, err := s.LookupAlert(ctx, "", alert.ID)
	require.NoError(t, err)
	assert.Equal(t, alert.ID, got.AlertID)
}
