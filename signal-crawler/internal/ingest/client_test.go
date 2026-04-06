package ingest_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_PostSignal(t *testing.T) {
	var captured *http.Request
	var body []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := ingest.New(srv.URL, "test-api-key")
	sig := adapter.Signal{
		SignalType: "hn_mention",
		ExternalID: "hn-123",
		SourceName: "hn",
		Label:      "Government RFP: IT Services",
		SourceURL:  "https://buyandsell.gc.ca/123",
	}

	err := client.Post(context.Background(), sig)
	require.NoError(t, err)

	assert.Equal(t, "/api/signals", captured.URL.Path)
	assert.Equal(t, "test-api-key", captured.Header.Get("X-Api-Key"))
	assert.Equal(t, "application/json", captured.Header.Get("Content-Type"))

	var payload struct {
		Signals []map[string]any `json:"signals"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Len(t, payload.Signals, 1)
	assert.Equal(t, "Government RFP: IT Services", payload.Signals[0]["label"])
	assert.Equal(t, "hn_mention", payload.Signals[0]["signal_type"])
	assert.Equal(t, "hn-123", payload.Signals[0]["external_id"])
	assert.Equal(t, "hn", payload.Signals[0]["source"])
}

func TestClient_PostFunding(t *testing.T) {
	var captured *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := ingest.New(srv.URL, "test-api-key")
	sig := adapter.Signal{
		SignalType:    "funding_win",
		ExternalID:    "otf-456",
		SourceName:    "funding",
		Label:         "NSERC Grant Awarded",
		SourceURL:     "https://nserc.ca/456",
		FundingStatus: "awarded",
	}

	err := client.Post(context.Background(), sig)
	require.NoError(t, err)

	// All signals go to the same endpoint now.
	assert.Equal(t, "/api/signals", captured.URL.Path)
}

func TestClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal failure"}`))
	}))
	defer srv.Close()

	client := ingest.New(srv.URL, "test-api-key")
	sig := adapter.Signal{
		SignalType: "job_posting",
		ExternalID: "ro-789",
		SourceName: "remoteok",
		Label:      "Some Signal",
		SourceURL:  "https://example.com/789",
	}

	err := client.Post(context.Background(), sig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "internal failure")
}
