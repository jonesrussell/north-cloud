package render_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Render_Success(t *testing.T) {
	expectedHTML := "<html><body>rendered content</body></html>"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/render", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req map[string]string
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com", req["url"])
		assert.Equal(t, "networkidle", req["wait_for"])

		resp := map[string]string{"html": expectedHTML, "url": "https://example.com"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := render.New(srv.URL)
	html, err := client.Render(context.Background(), "https://example.com")

	require.NoError(t, err)
	assert.Equal(t, expectedHTML, html)
}

func TestClient_Render_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "renderer busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := render.New(srv.URL)
	_, err := client.Render(context.Background(), "https://example.com")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestClient_Render_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := render.New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Render(ctx, "https://example.com")
	require.Error(t, err)
}
