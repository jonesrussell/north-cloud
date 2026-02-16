package mltransport_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/mltransport"
)

// testResponse is a simple response struct for test assertions.
type testResponse struct {
	Result string `json:"result"`
}

func TestDoClassify_ReturnsLatencyAndSize(t *testing.T) {
	want := testResponse{Result: "ok"}
	respBody, marshalErr := json.Marshal(want)
	if marshalErr != nil {
		t.Fatalf("marshal test response: %v", marshalErr)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write(respBody); writeErr != nil {
			t.Errorf("write response: %v", writeErr)
		}
	}))
	defer srv.Close()

	req := &mltransport.ClassifyRequest{Title: "Test", Body: "Test body"}
	var got testResponse

	latencyMs, responseSizeBytes, err := mltransport.DoClassify(
		context.Background(), srv.URL, req, &got,
	)
	if err != nil {
		t.Fatalf("DoClassify returned unexpected error: %v", err)
	}

	if latencyMs < 0 {
		t.Errorf("expected latencyMs >= 0, got %d", latencyMs)
	}

	if responseSizeBytes != len(respBody) {
		t.Errorf("expected responseSizeBytes=%d, got %d", len(respBody), responseSizeBytes)
	}

	if got.Result != want.Result {
		t.Errorf("expected result=%q, got %q", want.Result, got.Result)
	}
}

func TestDoClassify_ErrorReturnsLatency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, writeErr := w.Write([]byte("internal error")); writeErr != nil {
			t.Errorf("write response: %v", writeErr)
		}
	}))
	defer srv.Close()

	req := &mltransport.ClassifyRequest{Title: "Test", Body: "Test body"}
	var got testResponse

	latencyMs, _, err := mltransport.DoClassify(context.Background(), srv.URL, req, &got)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}

	if latencyMs < 0 {
		t.Errorf("expected latencyMs >= 0 even on error, got %d", latencyMs)
	}
}
