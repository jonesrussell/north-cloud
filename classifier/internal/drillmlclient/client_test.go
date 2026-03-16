package drillmlclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Extract_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing or wrong API key")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}

		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": `[{"hole_id":"DDH-24-001","commodity":"gold","intercept_m":12.5,"grade":3.2,"unit":"g/t"}]`},
			},
			"usage": map[string]any{
				"input_tokens":  500,
				"output_tokens": 50,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(server.URL, "test-key", "claude-haiku-4-5", 4000)
	results, err := c.Extract("DDH-24-001 returned 12.5m @ 3.2 g/t Au")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].HoleID != "DDH-24-001" {
		t.Errorf("HoleID = %q, want DDH-24-001", results[0].HoleID)
	}
}

func TestClient_Extract_EmptyArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": `[]`},
			},
			"usage": map[string]any{"input_tokens": 500, "output_tokens": 5},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(server.URL, "test-key", "claude-haiku-4-5", 4000)
	results, err := c.Extract("No drill results here")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestClient_Extract_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	c := New(server.URL, "test-key", "claude-haiku-4-5", 4000)
	_, err := c.Extract("some body text")
	if err == nil {
		t.Error("expected error for 429 response")
	}
}

func TestClient_Extract_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": `not valid json`},
			},
			"usage": map[string]any{"input_tokens": 500, "output_tokens": 10},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(server.URL, "test-key", "claude-haiku-4-5", 4000)
	_, err := c.Extract("some body")
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}
