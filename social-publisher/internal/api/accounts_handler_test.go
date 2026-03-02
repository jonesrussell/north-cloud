package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/api"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAccount_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewAccountsHandler(nil, "", infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{"name": "test-x"}
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewReader(bodyJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.POST("/api/v1/accounts", handler.Create)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateAccount_ValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := api.NewAccountsHandler(nil, "", infralogger.NewNop())

	w := httptest.NewRecorder()
	body := map[string]any{
		"name":     "personal-x",
		"platform": "x",
		"project":  "personal",
	}
	bodyJSON, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewReader(bodyJSON))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/api/v1/accounts", handler.Create)
	r.ServeHTTP(w, req)

	// Without a real repo, panics/500s — but not 400 (proves validation passed)
	assert.NotEqual(t, http.StatusBadRequest, w.Code)
}
