package redis_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	redispkg "github.com/jonesrussell/north-cloud/alert-crawler/internal/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient records calls made to Publish and allows controlled error injection.
// It satisfies redispkg.RedisClient (the exported alias of the internal interface).
type mockClient struct {
	capturedChannel string
	capturedPayload []byte
	returnErr       error
	closeErr        error
	closeCalled     bool
}

func (m *mockClient) Publish(_ context.Context, channel string, message any) error {
	m.capturedChannel = channel

	switch v := message.(type) {
	case []byte:
		m.capturedPayload = v
	case string:
		m.capturedPayload = []byte(v)
	}

	return m.returnErr
}

func (m *mockClient) Close() error {
	m.closeCalled = true
	return m.closeErr
}

// newTestPublisher builds a Publisher with an injected mock, bypassing New().
func newTestPublisher(t *testing.T, mock redispkg.RedisClient, channel string) *redispkg.Publisher {
	t.Helper()

	return redispkg.NewWithClient(mock, channel)
}

// fixtureAlert returns a minimal valid Alert for test use.
func fixtureAlert(t *testing.T) domain.Alert {
	t.Helper()

	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	return domain.Alert{
		ID:             "alert-001",
		Category:       domain.CategoryHarmReduction,
		Severity:       domain.SeverityHigh,
		Scope:          []string{"winnipeg"},
		IssuedAt:       now,
		LifecycleState: domain.LifecycleActive,
		Title:          "Test Alert",
		Summary:        "Test summary",
		ParseQuality:   domain.ParseClean,
		CrawledAt:      now,
		LastUpdatedAt:  now,
		Hazard: domain.Hazard{
			HarmReduction: &domain.HarmReductionHazard{
				HazardType: domain.HazardOpioidSupply,
				Substances: []string{"fentanyl"},
			},
		},
		Sources: []domain.SourceAttribution{
			{SourceID: "src-1", SourceName: "Test Source", URL: "https://example.com"},
		},
	}
}

// fixtureEvent builds a LifecycleEvent from the fixture alert.
func fixtureEvent(t *testing.T) domain.LifecycleEvent {
	t.Helper()

	alert := fixtureAlert(t)
	eventAt := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	return domain.LifecycleEvent{
		EventType: domain.EventCreated,
		EventAt:   eventAt,
		AlertID:   alert.ID,
		Category:  alert.Category,
		Severity:  alert.Severity,
		Scope:     alert.Scope,
		Payload:   alert,
	}
}

// TestPublish_Serializes verifies that Publish encodes the LifecycleEvent as
// valid JSON with all required top-level fields present.
func TestPublish_Serializes(t *testing.T) {
	t.Parallel()

	mock := &mockClient{}
	pub := newTestPublisher(t, mock, "community_alerts:lifecycle")
	event := fixtureEvent(t)

	publishErr := pub.Publish(context.Background(), event)
	require.NoError(t, publishErr)
	require.NotEmpty(t, mock.capturedPayload, "expected non-empty payload sent to client")

	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(mock.capturedPayload, &got), "payload must be valid JSON")

	requiredFields := []string{"event_type", "event_at", "alert_id", "category", "severity", "scope", "payload"}
	for _, field := range requiredFields {
		assert.Contains(t, got, field, "missing required JSON field: %s", field)
	}

	// Verify event_type value.
	var eventType string
	require.NoError(t, json.Unmarshal(got["event_type"], &eventType))
	assert.Equal(t, string(domain.EventCreated), eventType)

	// Verify alert_id value.
	var alertID string
	require.NoError(t, json.Unmarshal(got["alert_id"], &alertID))
	assert.Equal(t, "alert-001", alertID)

	// Verify severity value.
	var severity string
	require.NoError(t, json.Unmarshal(got["severity"], &severity))
	assert.Equal(t, string(domain.SeverityHigh), severity)

	// Verify category value.
	var category string
	require.NoError(t, json.Unmarshal(got["category"], &category))
	assert.Equal(t, string(domain.CategoryHarmReduction), category)

	// Verify scope is an array with at least one entry.
	var scope []string
	require.NoError(t, json.Unmarshal(got["scope"], &scope))
	assert.NotEmpty(t, scope)
}

// TestPublish_ChannelHonored verifies that Publish sends the event to the
// channel name that was configured at construction time.
func TestPublish_ChannelHonored(t *testing.T) {
	t.Parallel()

	const wantChannel = "indigenous:alerts"

	mock := &mockClient{}
	pub := newTestPublisher(t, mock, wantChannel)
	event := fixtureEvent(t)

	publishErr := pub.Publish(context.Background(), event)
	require.NoError(t, publishErr)
	assert.Equal(t, wantChannel, mock.capturedChannel)
}

// TestPublish_PropagatesError verifies that a client-level error is wrapped and
// returned to the caller so the runner can update metrics.
func TestPublish_PropagatesError(t *testing.T) {
	t.Parallel()

	sentinelErr := errors.New("redis unavailable")
	mock := &mockClient{returnErr: sentinelErr}
	pub := newTestPublisher(t, mock, "indigenous:alerts")
	event := fixtureEvent(t)

	publishErr := pub.Publish(context.Background(), event)
	require.Error(t, publishErr)
	assert.ErrorIs(t, publishErr, sentinelErr, "original error must be reachable via errors.Is")
}

// TestPublish_ContextRejected verifies that passing a nil context returns
// ErrNilContext immediately without forwarding to the client.
func TestPublish_ContextRejected(t *testing.T) {
	t.Parallel()

	mock := &mockClient{}
	pub := newTestPublisher(t, mock, "indigenous:alerts")
	event := fixtureEvent(t)

	//nolint:staticcheck // intentional nil context to exercise guard clause
	publishErr := pub.Publish(nil, event)
	require.Error(t, publishErr)
	require.ErrorIs(t, publishErr, redispkg.ErrNilContext)
	assert.Empty(t, mock.capturedChannel, "client must not be called when context is nil")
}

// TestPublish_MarshalError verifies that a JSON marshal failure (e.g. nil Hazard)
// is returned as a wrapped error before the client is ever called.
func TestPublish_MarshalError(t *testing.T) {
	t.Parallel()

	mock := &mockClient{}
	pub := newTestPublisher(t, mock, "indigenous:alerts")

	// An Alert with a nil Hazard.HarmReduction causes MarshalJSON to fail.
	badEvent := domain.LifecycleEvent{
		EventType: domain.EventCreated,
		EventAt:   time.Now().UTC(),
		AlertID:   "bad-001",
		Category:  domain.CategoryHarmReduction,
		Severity:  domain.SeverityHigh,
		Scope:     []string{"winnipeg"},
		Payload: domain.Alert{
			Hazard: domain.Hazard{HarmReduction: nil}, // triggers marshal error
		},
	}

	publishErr := pub.Publish(context.Background(), badEvent)
	require.Error(t, publishErr)
	assert.Empty(t, mock.capturedChannel, "client must not be called when marshal fails")
}

// TestClose_Delegates verifies that Publisher.Close forwards to the underlying client.
func TestClose_Delegates(t *testing.T) {
	t.Parallel()

	mock := &mockClient{}
	pub := newTestPublisher(t, mock, "indigenous:alerts")

	require.NoError(t, pub.Close())
	assert.True(t, mock.closeCalled)
}

// TestClose_PropagatesError verifies that a client Close error is wrapped and returned.
func TestClose_PropagatesError(t *testing.T) {
	t.Parallel()

	sentinelErr := errors.New("close failed")
	mock := &mockClient{closeErr: sentinelErr}
	pub := newTestPublisher(t, mock, "indigenous:alerts")

	closeErr := pub.Close()
	require.Error(t, closeErr)
	assert.ErrorIs(t, closeErr, sentinelErr)
}

// TestPublish_CancelledContext verifies that a cancelled context is forwarded
// to the client (Publisher does not short-circuit on cancellation itself) and
// that any client-side error is wrapped and returned to the caller.
func TestPublish_CancelledContext(t *testing.T) {
	t.Parallel()

	const wantChannel = "indigenous:alerts"

	sentinelErr := errors.New("context deadline exceeded")
	mock := &mockClient{returnErr: sentinelErr}
	pub := newTestPublisher(t, mock, wantChannel)
	event := fixtureEvent(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before Publish

	publishErr := pub.Publish(ctx, event)
	// Publisher must forward the context to the client — mock still receives the call.
	assert.Equal(t, wantChannel, mock.capturedChannel, "client must be called even with a cancelled context")
	require.Error(t, publishErr)
	assert.ErrorIs(t, publishErr, sentinelErr)
}
