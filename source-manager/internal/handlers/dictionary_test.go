package handlers_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/source-manager/internal/handlers"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
)

// newTestDictHandler creates a DictionaryHandler with a repo backed by a closed DB.
// DB calls return errors instead of panicking.
func newTestDictHandler(t *testing.T) *handlers.DictionaryHandler {
	t.Helper()

	// Open and immediately close a DB to get a non-nil *sql.DB that returns errors.
	db, err := sql.Open("postgres", "host=localhost port=0 dbname=none sslmode=disable")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.Close()

	repo := repository.NewDictionaryRepository(db, testhelpers.NewTestLogger())
	return handlers.NewDictionaryHandler(repo, testhelpers.NewTestLogger())
}

func TestDictionaryHandler_SearchEntries_EmptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := newTestDictHandler(t)
	router.GET("/api/v1/dictionary/search", handler.SearchEntries)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/search", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty query, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDictionaryHandler_SearchEntries_ShortQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := newTestDictHandler(t)
	router.GET("/api/v1/dictionary/search", handler.SearchEntries)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/search?q=a", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for short query, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDictionaryHandler_SearchEntries_AttributionHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := newTestDictHandler(t)
	router.GET("/api/v1/dictionary/search", handler.SearchEntries)

	// Short query triggers 400 but header should still be set
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/search?q=x", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	attr := w.Header().Get("X-Attribution")
	expected := "Ojibwe People's Dictionary, University of Minnesota"
	if attr != expected {
		t.Errorf("expected X-Attribution %q, got %q", expected, attr)
	}
}

func TestDictionaryHandler_ListEntries_ClosedDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := newTestDictHandler(t)
	router.GET("/api/v1/dictionary/entries", handler.ListEntries)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/entries", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Closed DB returns error, handler should respond 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for closed DB, got %d: %s", w.Code, w.Body.String())
	}

	attr := w.Header().Get("X-Attribution")
	if attr == "" {
		t.Error("expected X-Attribution header to be set")
	}
}

func TestDictionaryHandler_GetEntry_ClosedDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := newTestDictHandler(t)
	router.GET("/api/v1/dictionary/words/:id", handler.GetEntry)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/dictionary/words/00000000-0000-0000-0000-000000000000",
		http.NoBody,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for closed DB, got %d: %s", w.Code, w.Body.String())
	}

	attr := w.Header().Get("X-Attribution")
	if attr == "" {
		t.Error("expected X-Attribution header to be set")
	}
}

func TestDictionaryHandler_GetEntryByEntryID_ClosedDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := newTestDictHandler(t)
	router.GET("/api/v1/dictionary/entries/:id", handler.GetEntryByEntryID)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/dictionary/entries/00000000-0000-0000-0000-000000000000",
		http.NoBody,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for closed DB, got %d: %s", w.Code, w.Body.String())
	}

	attr := w.Header().Get("X-Attribution")
	if attr == "" {
		t.Error("expected X-Attribution header to be set")
	}
}

func TestDictionaryHandler_SearchEntries_ValidQuery_ClosedDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := newTestDictHandler(t)
	router.GET("/api/v1/dictionary/search", handler.SearchEntries)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/search?q=makwa", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Closed DB returns error
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 for closed DB, got %d: %s", w.Code, w.Body.String())
	}

	attr := w.Header().Get("X-Attribution")
	if attr == "" {
		t.Error("expected X-Attribution header to be set")
	}
}
