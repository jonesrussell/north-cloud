package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
)

func TestHealthReturnsOK(t *testing.T) {
	t.Parallel()

	handler := New(discardLogger(t), nil).Handler()
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	assertJSONField(t, response.Body.Bytes(), "status", "ok")
	assertJSONField(t, response.Body.Bytes(), "service", "enrichment")
}

func TestEnrichRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	response := postEnrich(t, New(discardLogger(t), nil).Handler(), strings.NewReader(`{"lead_id":`))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	assertJSONField(t, response.Body.Bytes(), "error", "invalid JSON request body")
}

func TestEnrichRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	body := `{"lead_id":"","company_name":"","requested_types":[],"callback_url":"","callback_api_key":""}`
	response := postEnrich(t, New(discardLogger(t), nil).Handler(), strings.NewReader(body))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	var payload api.ErrorResponse
	decodeJSON(t, response.Body.Bytes(), &payload)
	if len(payload.Fields) != 5 {
		t.Fatalf("field count = %d, want 5: %#v", len(payload.Fields), payload.Fields)
	}
}

func TestEnrichRejectsInvalidCallbackURL(t *testing.T) {
	t.Parallel()

	request := validRequest(t)
	request.CallbackURL = "ftp://waaseyaa.example/callback"

	response := postEnrichJSON(t, New(discardLogger(t), nil).Handler(), request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if strings.Contains(response.Body.String(), request.CallbackAPIKey) {
		t.Fatal("response body leaked callback API key")
	}
}

func TestEnrichRejectsEmptyRequestedType(t *testing.T) {
	t.Parallel()

	request := validRequest(t)
	request.RequestedTypes = []string{"company_intel", " "}

	response := postEnrichJSON(t, New(discardLogger(t), nil).Handler(), request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestEnrichAcceptsValidRequest(t *testing.T) {
	t.Parallel()

	runner := &recordingRunner{}
	request := validRequest(t)
	response := postEnrichJSON(t, New(discardLogger(t), runner).Handler(), request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusAccepted, response.Body.String())
	}
	assertJSONField(t, response.Body.Bytes(), "status", "accepted")
	assertJSONField(t, response.Body.Bytes(), "lead_id", request.LeadID)
	if runner.leadID != request.LeadID {
		t.Fatalf("runner lead id = %q, want %q", runner.leadID, request.LeadID)
	}
}

func TestEnrichRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	body := `{"lead_id":"lead-1","company_name":"Acme","requested_types":["company_intel"],"callback_url":"https://waaseyaa.example/callback","callback_api_key":"secret","unexpected":true}`
	response := postEnrich(t, New(discardLogger(t), nil).Handler(), strings.NewReader(body))

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

type recordingRunner struct {
	leadID string
}

func (r *recordingRunner) Enqueue(_ context.Context, request api.EnrichmentRequest) error {
	r.leadID = request.LeadID
	return nil
}

func validRequest(t *testing.T) api.EnrichmentRequest {
	t.Helper()

	return api.EnrichmentRequest{
		LeadID:         "lead-123",
		CompanyName:    "Acme Mining",
		Domain:         "acme.example",
		Sector:         "mining",
		RequestedTypes: []string{"company_intel"},
		Signals:        map[string]any{"source": "waaseyaa"},
		CallbackURL:    "https://waaseyaa.example/api/enrichment-callback",
		CallbackAPIKey: "super-secret",
	}
}

func postEnrichJSON(t *testing.T, handler http.Handler, request api.EnrichmentRequest) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	return postEnrich(t, handler, &body)
}

func postEnrich(t *testing.T, handler http.Handler, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/enrich", body)
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	return response
}

func assertJSONField(t *testing.T, body []byte, field string, want string) {
	t.Helper()

	var payload map[string]any
	decodeJSON(t, body, &payload)
	got, ok := payload[field].(string)
	if !ok {
		t.Fatalf("field %q missing or not string in %s", field, string(body))
	}
	if got != want {
		t.Fatalf("field %q = %q, want %q", field, got, want)
	}
}

func decodeJSON(t *testing.T, body []byte, target any) {
	t.Helper()

	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("decode JSON %s: %v", string(body), err)
	}
}

func discardLogger(t *testing.T) *slog.Logger {
	t.Helper()

	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
