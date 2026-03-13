package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/source-manager/internal/handlers"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
)

func TestCommunityHandler_ImportWebsites_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := handlers.NewCommunityHandler(nil, testhelpers.NewTestLogger())
	router.POST("/api/v1/communities/import-websites", handler.ImportWebsites)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/communities/import-websites",
		strings.NewReader(`not-valid-json`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCommunityHandler_ImportWebsites_EmptyUpdates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := handlers.NewCommunityHandler(nil, testhelpers.NewTestLogger())
	router.POST("/api/v1/communities/import-websites", handler.ImportWebsites)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/communities/import-websites",
		strings.NewReader(`{"updates":[]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty updates, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCommunityHandler_ImportWebsites_NullUpdates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := handlers.NewCommunityHandler(nil, testhelpers.NewTestLogger())
	router.POST("/api/v1/communities/import-websites", handler.ImportWebsites)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/communities/import-websites",
		strings.NewReader(`{}`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing updates, got %d: %s", w.Code, w.Body.String())
	}
}
