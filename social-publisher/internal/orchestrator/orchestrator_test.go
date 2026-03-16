package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/adapters"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_ProcessJob_Success(t *testing.T) {
	mock := adapters.NewMockAdapter("test")
	orch := orchestrator.NewOrchestrator(
		map[string]domain.PlatformAdapter{"test": mock},
		nil, nil,
	)

	msg := &domain.PublishMessage{
		ContentID: "test-1",
		Summary:   "Hello world",
		URL:       "https://example.com",
	}

	result, err := orch.ProcessJob(context.Background(), "test", msg)
	require.NoError(t, err)
	assert.NotEmpty(t, result.PlatformID)
	assert.Equal(t, 1, mock.PublishCount())
}

func TestOrchestrator_ProcessJob_UnknownPlatform(t *testing.T) {
	orch := orchestrator.NewOrchestrator(
		map[string]domain.PlatformAdapter{},
		nil, nil,
	)

	msg := &domain.PublishMessage{ContentID: "test-1", Summary: "Hello"}
	_, err := orch.ProcessJob(context.Background(), "unknown", msg)
	assert.Error(t, err)
}

func TestOrchestrator_ProcessJob_ValidationError(t *testing.T) {
	mock := adapters.NewMockAdapter("test")
	orch := orchestrator.NewOrchestrator(
		map[string]domain.PlatformAdapter{"test": mock},
		nil, nil,
	)

	msg := &domain.PublishMessage{ContentID: "test-1", Summary: ""}
	_, err := orch.ProcessJob(context.Background(), "test", msg)
	assert.Error(t, err)
}

func TestBackoff_Calculation(t *testing.T) {
	tests := []struct {
		attempts int
		expected time.Duration
		valid    bool
	}{
		{0, 30 * time.Second, true},
		{1, 2 * time.Minute, true},
		{2, 10 * time.Minute, true},
		{3, 0, false},
	}

	for _, tc := range tests {
		result, ok := orchestrator.NextRetryAt(tc.attempts)
		assert.Equal(t, tc.valid, ok)
		if ok {
			assert.InDelta(t, tc.expected.Seconds(), time.Until(result).Seconds(), 2)
		}
	}
}
