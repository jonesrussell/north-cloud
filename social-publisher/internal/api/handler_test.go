package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/api"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishEndpoint_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{
		"type":    "social_update",
		"summary": "Test post",
		"project": "personal",
	}
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/publish", bytes.NewReader(bodyJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/api/v1/publish", handler.Publish)
	r.ServeHTTP(w, req)

	// Without a real repo, the handler hits a nil pointer recovered as 500.
	// Key assertion: request parsing succeeded (not 400).
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestPublishEndpoint_MissingType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{
		"summary": "Test post",
	}
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/publish", bytes.NewReader(bodyJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.POST("/api/v1/publish", handler.Publish)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestProtectedRoutes_RejectUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Auth: config.AuthConfig{JWTSecret: "test-secret-key-for-testing"},
	}
	router := api.NewRouter(nil, nil, cfg, infralogger.NewNop())
	testEngine := router.TestEngine()

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/status/test-id"},
		{http.MethodPost, "/api/v1/publish"},
		{http.MethodGet, "/api/v1/content"},
		{http.MethodPost, "/api/v1/retry/test-id"},
		{http.MethodGet, "/api/v1/accounts"},
		{http.MethodGet, "/api/v1/accounts/test-id"},
		{http.MethodPost, "/api/v1/accounts"},
		{http.MethodPut, "/api/v1/accounts/test-id"},
		{http.MethodDelete, "/api/v1/accounts/test-id"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, err := http.NewRequest(route.method, route.path, http.NoBody)
			require.NoError(t, err)

			testEngine.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestPublishEndpoint_ParsesScheduledAt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{
		"type":         "blog_post",
		"summary":      "Scheduled post",
		"project":      "personal",
		"scheduled_at": "2026-03-15T10:00:00Z",
	}
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/publish", bytes.NewReader(bodyJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/api/v1/publish", handler.Publish)
	r.ServeHTTP(w, req)

	// Without a real repo this panics/500s, but the key is it didn't return 400
	// (i.e., scheduled_at was parsed successfully, not rejected)
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestPublishEndpoint_InvalidScheduledAt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{
		"type":         "blog_post",
		"summary":      "Bad date",
		"project":      "personal",
		"scheduled_at": "not-a-date",
	}
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/publish", bytes.NewReader(bodyJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.POST("/api/v1/publish", handler.Publish)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListContent_NoRepo_Returns500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/api/v1/content", http.NoBody)
	require.NoError(t, err)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/api/v1/content", handler.ListContent)
	r.ServeHTTP(w, req)

	// Without a repo, this panics/500s — validates route is wired
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListContent_ParsesPaginationParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	req, err := http.NewRequest(
		http.MethodGet,
		"/api/v1/content?limit=10&offset=20&status=delivered&type=blog_post",
		http.NoBody,
	)
	require.NoError(t, err)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/api/v1/content", handler.ListContent)
	r.ServeHTTP(w, req)

	// Without repo it 500s, but confirms query parsing doesn't 400
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}

func TestRetryEndpoint_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil, infralogger.NewNop())

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/api/v1/retry/", http.NoBody)
	require.NoError(t, err)

	r := gin.New()
	r.POST("/api/v1/retry/:id", handler.Retry)
	// Route won't match empty id, so this tests 404 behavior
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
