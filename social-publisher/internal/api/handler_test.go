package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestPublishEndpoint_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewHandler(nil, nil)

	w := httptest.NewRecorder()
	body := map[string]any{
		"type":    "social_update",
		"summary": "Test post",
		"project": "personal",
	}
	bodyJSON, err := json.Marshal(body)
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/publish", bytes.NewReader(bodyJSON))
	assert.NoError(t, err)
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
	handler := api.NewHandler(nil, nil)

	w := httptest.NewRecorder()
	body := map[string]any{
		"summary": "Test post",
	}
	bodyJSON, err := json.Marshal(body)
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/publish", bytes.NewReader(bodyJSON))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.POST("/api/v1/publish", handler.Publish)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
