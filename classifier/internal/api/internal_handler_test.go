//nolint:testpackage // Testing internal handler in same package for access to types
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	infragin "github.com/north-cloud/infrastructure/gin"
)

const testInternalSecret = "test-secret-123"

// setupInternalTestRouter creates a router with the internal extract endpoint.
// Uses setupTestHandler from handlers_test.go which creates a real classifier
// with in-memory dependencies.
func setupInternalTestRouter(internalSecret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := setupTestHandler()

	// Also register via SetupRoutes to test route registration
	cfg := &config.Config{
		Auth: config.AuthConfig{
			InternalSecret: internalSecret,
		},
	}
	SetupRoutes(router, handler, cfg)

	return router
}

// setupInternalTestRouterDirect creates a router with only the internal endpoint
// for more focused testing.
func setupInternalTestRouterDirect(internalSecret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := setupTestHandler()

	internal := router.Group("/api/internal/v1")
	if internalSecret != "" {
		internal.Use(infragin.InternalAuthMiddleware(internalSecret))
	}
	internal.POST("/extract", handler.InternalExtract)

	return router
}

func TestInternalExtract_Success(t *testing.T) {
	router := setupInternalTestRouterDirect("")

	body := `{"html": "<html><head><title>Test Article</title></head><body><p>This is a test article ` +
		`about technology and software development. It contains enough words to be meaningful for ` +
		`classification purposes. The article discusses various aspects of modern software ` +
		`engineering practices and methodologies.</p></body></html>", "url": "https://example.com/test-article", ` +
		`"source_name": "test-source", "title": "Test Article"}`

	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp InternalExtractResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify title preference: request title should be used
	if resp.Title != "Test Article" {
		t.Errorf("expected title %q, got %q", "Test Article", resp.Title)
	}

	// Verify content_type is populated
	if resp.ContentType == "" {
		t.Error("expected non-empty content_type")
	}

	// Verify topics is not nil (should be empty array, not null)
	if resp.Topics == nil {
		t.Error("expected topics to be non-nil")
	}

	// Verify topic_scores is not nil
	if resp.TopicScores == nil {
		t.Error("expected topic_scores to be non-nil")
	}

	// Verify quality_score is a reasonable value
	if resp.QualityScore < 0 || resp.QualityScore > 100 {
		t.Errorf("expected quality_score between 0-100, got %d", resp.QualityScore)
	}

	// Verify body contains extracted text, not raw HTML
	if strings.Contains(resp.Body, "<html>") || strings.Contains(resp.Body, "<body>") || strings.Contains(resp.Body, "<p>") {
		t.Errorf("body should contain extracted text, not HTML tags: %s", resp.Body[:min(len(resp.Body), 200)])
	}
	if !strings.Contains(resp.Body, "test article") {
		t.Errorf("body should contain article text, got: %s", resp.Body[:min(len(resp.Body), 200)])
	}

	// Verify word_count is populated
	if resp.WordCount == 0 {
		t.Error("expected non-zero word_count for content with text")
	}
}

func TestInternalExtract_MissingHTML(t *testing.T) {
	router := setupInternalTestRouterDirect("")

	body := `{
		"url": "https://example.com/test-article"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing html, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInternalExtract_MissingURL(t *testing.T) {
	router := setupInternalTestRouterDirect("")

	body := `{
		"html": "<html><body>Content</body></html>"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing url, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInternalExtract_EmptyBody(t *testing.T) {
	router := setupInternalTestRouterDirect("")

	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInternalExtract_InvalidJSON(t *testing.T) {
	router := setupInternalTestRouterDirect("")

	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInternalExtract_DefaultSourceName(t *testing.T) {
	router := setupInternalTestRouterDirect("")

	body := `{
		"html": "<html><body><p>Some content here for testing purposes.</p></body></html>",
		"url": "https://example.com/test"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// The handler defaults source_name to "pipelinex" when not provided
	// We can verify the response is well-formed
	var resp InternalExtractResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ContentType == "" {
		t.Error("expected non-empty content_type")
	}
}

func TestInternalExtract_AuthRequired(t *testing.T) {
	router := setupInternalTestRouterDirect(testInternalSecret)

	body := `{
		"html": "<html><body>Content</body></html>",
		"url": "https://example.com/test"
	}`

	// Request without the X-Internal-Secret header
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 without auth header, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInternalExtract_AuthInvalidSecret(t *testing.T) {
	router := setupInternalTestRouterDirect(testInternalSecret)

	body := `{
		"html": "<html><body>Content</body></html>",
		"url": "https://example.com/test"
	}`

	// Request with wrong secret
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", "wrong-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 with wrong secret, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInternalExtract_AuthValidSecret(t *testing.T) {
	router := setupInternalTestRouterDirect(testInternalSecret)

	body := `{
		"html": "<html><body><p>Content for auth test.</p></body></html>",
		"url": "https://example.com/test"
	}`

	// Request with correct secret
	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", testInternalSecret)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 with valid secret, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInternalExtract_ResponseShape(t *testing.T) {
	router := setupInternalTestRouterDirect("")

	body := `{"html": "<html><head><meta property=\"og:title\" content=\"OG Title\"><meta property=\"og:description\" ` +
		`content=\"OG Description\"></head><body><p>Article content for response shape test with enough words ` +
		`to be meaningful.</p></body></html>", "url": "https://example.com/test-article", ` +
		`"source_name": "test-source", "title": "My Test Title"}`

	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse as raw JSON to verify all expected fields exist
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify all required top-level fields exist
	requiredFields := []string{
		"title", "author", "published_date", "body",
		"word_count", "quality_score", "topics", "topic_scores",
		"content_type", "og",
	}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field %q in response", field)
		}
	}

	// Verify OG is an object (fields may be omitted if empty due to omitempty)
	if _, ok := raw["og"].(map[string]any); !ok {
		t.Fatal("expected og to be an object")
	}

	// Verify title preference
	if raw["title"] != "My Test Title" {
		t.Errorf("expected title %q, got %v", "My Test Title", raw["title"])
	}

	// Verify topics is an array, not null
	if topics, ok := raw["topics"].([]any); !ok {
		t.Fatal("expected topics to be an array")
	} else {
		_ = topics // may be empty, that's fine
	}

	// Verify topic_scores is an object, not null
	if _, ok := raw["topic_scores"].(map[string]any); !ok {
		t.Fatal("expected topic_scores to be an object")
	}
}

func TestInternalExtract_RouteRegisteredViaSetupRoutes(t *testing.T) {
	// Verify the route is registered when using SetupRoutes (the legacy path)
	router := setupInternalTestRouter("")

	body := `{
		"html": "<html><body><p>Content</p></body></html>",
		"url": "https://example.com/test"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/internal/v1/extract", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should get 200 (route exists and processes), not 404
	if w.Code == http.StatusNotFound {
		t.Error("expected route /api/internal/v1/extract to be registered, got 404")
	}
}
