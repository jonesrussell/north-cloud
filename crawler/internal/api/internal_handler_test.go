//nolint:testpackage // Testing internal handler in same package for access to types
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const testHTML = `<!DOCTYPE html>
<html>
<head>
	<title>Test Article Title</title>
	<meta name="description" content="A test article description">
	<meta name="author" content="Jane Smith">
	<meta property="og:title" content="OG Test Title">
	<meta property="og:description" content="OG test description">
	<meta property="og:image" content="https://example.com/image.jpg">
	<meta property="og:type" content="article">
	<meta property="og:url" content="https://example.com/test-article">
	<meta property="og:site_name" content="Test Site">
</head>
<body>
	<nav>Navigation content that should be stripped</nav>
	<header>Header content that should be stripped</header>
	<article>
		<h1>Test Article Title</h1>
		<p>This is the main article content that should be extracted.</p>
		<p>It has multiple paragraphs of text.</p>
	</article>
	<footer>Footer content that should be stripped</footer>
	<script>var x = "should be stripped";</script>
	<style>.should-be-stripped { display: none; }</style>
</body>
</html>`

func setupTestRouter(internalSecret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewInternalHandler(infralogger.NewNop())

	internal := router.Group("/api/internal/v1")
	if internalSecret != "" {
		internal.Use(infragin.InternalAuthMiddleware(internalSecret))
	}
	internal.POST("/fetch", handler.Fetch)

	return router
}

// doFetchRequest sends a fetch request and returns the decoded response.
func doFetchRequest(t *testing.T, router *gin.Engine, targetURL string) fetchResponse {
	t.Helper()
	body := `{"url": "` + targetURL + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp fetchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func TestFetchHandler_Success(t *testing.T) {
	// Start test server serving known HTML
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, testHTML)
	}))
	defer ts.Close()

	router := setupTestRouter("")

	resp := doFetchRequest(t, router, ts.URL)

	// Verify URL fields
	if resp.URL != ts.URL {
		t.Errorf("expected url %q, got %q", ts.URL, resp.URL)
	}
	if resp.FinalURL == "" {
		t.Error("expected non-empty final_url")
	}

	// Verify status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status_code 200, got %d", resp.StatusCode)
	}

	// Verify content type
	if !strings.Contains(resp.ContentType, "text/html") {
		t.Errorf("expected content_type to contain text/html, got %q", resp.ContentType)
	}

	// Verify HTML is returned
	if resp.HTML == "" {
		t.Error("expected non-empty html")
	}

	verifySuccessContentExtraction(t, &resp)
	verifySuccessOGMetadata(t, &resp)

	// Verify duration is non-negative (may be 0 when mock server responds in < 1ms)
	if resp.DurationMS < 0 {
		t.Errorf("expected non-negative duration_ms, got %d", resp.DurationMS)
	}
}

func verifySuccessContentExtraction(t *testing.T, resp *fetchResponse) {
	t.Helper()

	if resp.Title != "Test Article Title" {
		t.Errorf("expected title %q, got %q", "Test Article Title", resp.Title)
	}
	if resp.Description != "A test article description" {
		t.Errorf("expected description %q, got %q", "A test article description", resp.Description)
	}
	if resp.Author != "Jane Smith" {
		t.Errorf("expected author %q, got %q", "Jane Smith", resp.Author)
	}
	if !strings.Contains(resp.Body, "main article content") {
		t.Errorf("expected body to contain 'main article content', got %q", resp.Body)
	}
	if strings.Contains(resp.Body, "Navigation content") {
		t.Error("expected nav content to be stripped from body")
	}
	if strings.Contains(resp.Body, "should be stripped") {
		t.Error("expected script/style content to be stripped from body")
	}
}

func verifySuccessOGMetadata(t *testing.T, resp *fetchResponse) {
	t.Helper()

	if resp.OG == nil {
		t.Fatal("expected og metadata to be present")
	}
	if resp.OG.Title != "OG Test Title" {
		t.Errorf("expected og.title %q, got %q", "OG Test Title", resp.OG.Title)
	}
	if resp.OG.Description != "OG test description" {
		t.Errorf("expected og.description %q, got %q", "OG test description", resp.OG.Description)
	}
	if resp.OG.Image != "https://example.com/image.jpg" {
		t.Errorf("expected og.image %q, got %q", "https://example.com/image.jpg", resp.OG.Image)
	}
	if resp.OG.Type != "article" {
		t.Errorf("expected og.type %q, got %q", "article", resp.OG.Type)
	}
	if resp.OG.URL != "https://example.com/test-article" {
		t.Errorf("expected og.url %q, got %q", "https://example.com/test-article", resp.OG.URL)
	}
	if resp.OG.SiteName != "Test Site" {
		t.Errorf("expected og.site_name %q, got %q", "Test Site", resp.OG.SiteName)
	}
}

func TestFetchHandler_Redirect(t *testing.T) {
	// Start a test server that redirects
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Redirected</title></head><body>Final page</body></html>`)
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusFound)
	}))
	defer redirectServer.Close()

	router := setupTestRouter("")

	body := `{"url": "` + redirectServer.URL + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp fetchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Original URL should be the redirect server
	if resp.URL != redirectServer.URL {
		t.Errorf("expected url %q, got %q", redirectServer.URL, resp.URL)
	}

	// Final URL should be the final server
	if !strings.HasPrefix(resp.FinalURL, finalServer.URL) {
		t.Errorf("expected final_url to start with %q, got %q", finalServer.URL, resp.FinalURL)
	}

	if resp.Title != "Redirected" {
		t.Errorf("expected title %q, got %q", "Redirected", resp.Title)
	}
}

func TestFetchHandler_NonHTML(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"key": "value"}`)
	}))
	defer ts.Close()

	router := setupTestRouter("")

	body := `{"url": "` + ts.URL + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp fetchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Non-HTML should not have extracted content
	if resp.Title != "" {
		t.Errorf("expected empty title for non-HTML, got %q", resp.Title)
	}
	if resp.Body != "" {
		t.Errorf("expected empty body for non-HTML, got %q", resp.Body)
	}
	if resp.OG != nil {
		t.Error("expected nil OG metadata for non-HTML")
	}

	// Raw content should still be in HTML field
	if resp.HTML == "" {
		t.Error("expected non-empty html field even for non-HTML responses")
	}
}

func TestFetchHandler_InvalidRequest(t *testing.T) {
	router := setupTestRouter("")

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "missing url",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty url",
			body:       `{"url": ""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid json",
			body:       `{invalid`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid url format",
			body:       `{"url": "not-a-url"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			var resp map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if _, ok := resp["error"]; !ok {
				t.Error("expected error field in response")
			}
		})
	}
}

func TestFetchHandler_UnreachableHost(t *testing.T) {
	router := setupTestRouter("")

	// Use a URL that will definitely fail to connect (RFC 5737 TEST-NET)
	body := `{"url": "http://192.0.2.1:1", "timeout": 1}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if _, ok := resp["error"]; !ok {
		t.Error("expected error field in response")
	}
}

func TestFetchHandler_CustomTimeout(t *testing.T) {
	// Server that responds slowly
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>Slow page</body></html>`)
	}))
	defer ts.Close()

	router := setupTestRouter("")

	// Request with 1s timeout should fail
	body := `{"url": "` + ts.URL + `", "timeout": 1}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status 502 for timeout, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFetchHandler_TimeoutCapped(t *testing.T) {
	// Verify that timeout > 30s is capped at 30s
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Quick</title></head><body>Fast page</body></html>`)
	}))
	defer ts.Close()

	router := setupTestRouter("")

	// Request with 60s timeout should be capped to 30s but still succeed
	body := `{"url": "` + ts.URL + `", "timeout": 60}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp fetchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Title != "Quick" {
		t.Errorf("expected title %q, got %q", "Quick", resp.Title)
	}
}

func TestFetchHandler_AuthRequired(t *testing.T) {
	secret := "test-internal-secret-12345"
	router := setupTestRouter(secret)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Auth Test</title></head><body>Protected</body></html>`)
	}))
	defer ts.Close()

	body := `{"url": "` + ts.URL + `"}`

	t.Run("missing secret header returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("wrong secret returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Secret", "wrong-secret")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("correct secret returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Secret", secret)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp fetchResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Title != "Auth Test" {
			t.Errorf("expected title %q, got %q", "Auth Test", resp.Title)
		}
	})
}

func TestFetchHandler_NoOGMetadata(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>No OG</title></head><body>Simple page</body></html>`)
	}))
	defer ts.Close()

	router := setupTestRouter("")

	body := `{"url": "` + ts.URL + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp fetchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// OG should be nil when no OG metadata present
	if resp.OG != nil {
		t.Errorf("expected nil OG metadata, got %+v", resp.OG)
	}
}

func TestFetchHandler_MainFallback(t *testing.T) {
	// Test that <main> is used when no <article>
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><nav>Nav</nav><main><p>Main content here</p></main><footer>Foot</footer></body></html>`)
	}))
	defer ts.Close()

	router := setupTestRouter("")

	body := `{"url": "` + ts.URL + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp fetchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !strings.Contains(resp.Body, "Main content here") {
		t.Errorf("expected body to contain 'Main content here', got %q", resp.Body)
	}
	if strings.Contains(resp.Body, "Nav") {
		t.Error("expected nav to be stripped from main content")
	}
}

func TestFetchHandler_OGTitleFallback(t *testing.T) {
	// When no <title>, og:title should be used
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><meta property="og:title" content="OG Fallback Title"></head><body>Content</body></html>`)
	}))
	defer ts.Close()

	router := setupTestRouter("")

	body := `{"url": "` + ts.URL + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp fetchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Title != "OG Fallback Title" {
		t.Errorf("expected title %q, got %q", "OG Fallback Title", resp.Title)
	}
}

func TestFetchHandler_ServerError(t *testing.T) {
	// Test that non-200 status codes from the target are relayed
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `<html><head><title>Error</title></head><body>Server error</body></html>`)
	}))
	defer ts.Close()

	router := setupTestRouter("")

	body := `{"url": "` + ts.URL + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/fetch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// The fetch endpoint should return 200 with the target's status code in the response
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp fetchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status_code 500, got %d", resp.StatusCode)
	}
}
