package render_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/render"
)

func TestClientRender(t *testing.T) {
	t.Helper()

	expectedHTML := "<html><body>rendered content</body></html>"
	expectedRenderTimeMs := 1500
	expectedStatusCode := 200

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/render" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)

			return
		}

		var req render.RenderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		if req.URL == "" {
			t.Error("URL is empty")
		}

		resp := render.RenderResponse{
			HTML:         expectedHTML,
			FinalURL:     req.URL,
			RenderTimeMs: expectedRenderTimeMs,
			StatusCode:   expectedStatusCode,
		}

		w.Header().Set("Content-Type", "application/json")

		if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
			t.Errorf("failed to encode response: %v", encodeErr)
		}
	}))
	defer server.Close()

	client := render.NewClient(server.URL)

	result, err := client.Render(context.Background(), "https://example.com/article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.HTML != expectedHTML {
		t.Errorf("HTML = %q, want %q", result.HTML, expectedHTML)
	}

	if result.FinalURL != "https://example.com/article" {
		t.Errorf("FinalURL = %q, want %q", result.FinalURL, "https://example.com/article")
	}

	if result.RenderTimeMs != expectedRenderTimeMs {
		t.Errorf("RenderTimeMs = %d, want %d", result.RenderTimeMs, expectedRenderTimeMs)
	}

	if result.StatusCode != expectedStatusCode {
		t.Errorf("StatusCode = %d, want %d", result.StatusCode, expectedStatusCode)
	}
}

func TestClientRenderError(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		resp := render.ErrorResponse{Error: "navigation timeout"}
		if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
			t.Errorf("failed to encode error response: %v", encodeErr)
		}
	}))
	defer server.Close()

	client := render.NewClient(server.URL)

	_, err := client.Render(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
