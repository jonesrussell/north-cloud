package osrm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/services/osrm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger(t *testing.T) infralogger.Logger {
	t.Helper()
	return infralogger.NewNop()
}

func TestGetTravelTime(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{
			"code": "Ok",
			"durations": [[0, 2700], [2700, 0]],
			"distances": [[0, 45000], [45000, 0]]
		}`))
		if writeErr != nil {
			t.Errorf("failed to write response: %v", writeErr)
		}
	}))
	defer server.Close()

	log := newTestLogger(t)
	client := osrm.NewClient(server.URL, log)

	result, err := client.GetTravelTime(context.Background(), 48.0, -79.0, 48.5, -79.5, "car")
	require.NoError(t, err)
	assert.Equal(t, 2700, result.DurationSeconds)
	assert.Equal(t, 45000, result.DistanceMeters)
}

func TestGetTravelTimeServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr := w.Write([]byte("internal error"))
		if writeErr != nil {
			t.Errorf("failed to write response: %v", writeErr)
		}
	}))
	defer server.Close()

	log := newTestLogger(t)
	client := osrm.NewClient(server.URL, log)

	_, err := client.GetTravelTime(context.Background(), 48.0, -79.0, 48.5, -79.5, "car")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestNewClientDefaultURL(t *testing.T) {
	t.Parallel()

	log := newTestLogger(t)
	client := osrm.NewClient("", log)
	// Client was created successfully with default URL
	assert.NotNil(t, client)
}

func TestGetTravelTimeOSRMError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{"code": "InvalidQuery"}`))
		if writeErr != nil {
			t.Errorf("failed to write response: %v", writeErr)
		}
	}))
	defer server.Close()

	log := newTestLogger(t)
	client := osrm.NewClient(server.URL, log)

	_, err := client.GetTravelTime(context.Background(), 48.0, -79.0, 48.5, -79.5, "car")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OSRM error code")
}

func TestGetTravelTimeDurationsOnly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte(`{
			"code": "Ok",
			"durations": [[0, 1800], [1800, 0]]
		}`))
		if writeErr != nil {
			t.Errorf("failed to write response: %v", writeErr)
		}
	}))
	defer server.Close()

	log := newTestLogger(t)
	client := osrm.NewClient(server.URL, log)

	result, err := client.GetTravelTime(context.Background(), 48.0, -79.0, 48.5, -79.5, "car")
	require.NoError(t, err)
	assert.Equal(t, 1800, result.DurationSeconds)
	assert.Equal(t, 0, result.DistanceMeters)
}
