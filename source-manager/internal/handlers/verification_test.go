package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/source-manager/internal/handlers"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
)

func TestVerificationHandler_ListPending_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := handlers.NewVerificationHandler(nil, testhelpers.NewTestLogger())
	router.GET("/api/v1/verification/pending", handler.ListPending)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/verification/pending?type=invalid", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerificationHandler_Verify_MissingType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := handlers.NewVerificationHandler(nil, testhelpers.NewTestLogger())
	router.POST("/api/v1/verification/:id/verify", handler.Verify)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/verification/abc/verify", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerificationHandler_Reject_MissingType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := handlers.NewVerificationHandler(nil, testhelpers.NewTestLogger())
	router.POST("/api/v1/verification/:id/reject", handler.Reject)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/verification/abc/reject", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}
