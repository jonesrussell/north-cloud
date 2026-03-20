package handlers_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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

func newMockDictHandler(t *testing.T) (*handlers.DictionaryHandler, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	repo := repository.NewDictionaryRepository(db, testhelpers.NewTestLogger())
	handler := handlers.NewDictionaryHandler(repo, testhelpers.NewTestLogger())

	cleanup := func() {
		mock.ExpectClose()
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close: %v", closeErr)
		}
	}

	return handler, mock, cleanup
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

func TestDictionaryHandler_ListEntries_PagePaginationComputesOffset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockDictHandler(t)
	defer cleanup()

	router.GET("/api/v1/dictionary/entries", handler.ListEntries)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT `)+`.*`+regexp.QuoteMeta(`
		FROM dictionary_entries
		WHERE consent_public_display = TRUE
		ORDER BY lemma ASC
		LIMIT $1 OFFSET $2`)).
		WithArgs(10, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM dictionary_entries WHERE consent_public_display = TRUE`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dictionary/entries?page=2&limit=10", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if got := int(body["page"].(float64)); got != 2 {
		t.Fatalf("expected page 2, got %d", got)
	}
	if got := int(body["limit"].(float64)); got != 10 {
		t.Fatalf("expected limit 10, got %d", got)
	}
	if got := int(body["offset"].(float64)); got != 10 {
		t.Fatalf("expected offset 10, got %d", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
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

func TestDictionaryHandler_SearchEntries_LimitParamTakesPrecedence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockDictHandler(t)
	defer cleanup()

	router.GET("/api/v1/dictionary/search", handler.SearchEntries)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM (
		SELECT id FROM dictionary_entries
		WHERE consent_public_display = TRUE AND search_vector @@ plainto_tsquery('english', $1)
		UNION
		SELECT id FROM dictionary_entries
		WHERE consent_public_display = TRUE AND lemma ILIKE $1 || '%'
	) AS matched`)).
		WithArgs("makwa").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta(`WITH fts AS (
		SELECT `)+`.*`+regexp.QuoteMeta(`, ts_rank(search_vector, plainto_tsquery('english', $1)) AS rank
		FROM dictionary_entries
		WHERE consent_public_display = TRUE AND search_vector @@ plainto_tsquery('english', $1)
	), prefix AS (
		SELECT `)+`.*`+regexp.QuoteMeta(`, 0.0::float AS rank
		FROM dictionary_entries
		WHERE consent_public_display = TRUE AND lemma ILIKE $1 || '%'
			AND id NOT IN (SELECT id FROM fts)
	)
	SELECT `)+`.*`+regexp.QuoteMeta(` FROM (
		SELECT `)+`.*`+regexp.QuoteMeta(`, rank FROM fts
		UNION ALL
		SELECT `)+`.*`+regexp.QuoteMeta(`, rank FROM prefix
	) AS combined
	ORDER BY rank DESC, lemma ASC
	LIMIT $2 OFFSET $3`)).
		WithArgs("makwa", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/dictionary/search?q=makwa&limit=10&size=25",
		http.NoBody,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if got := int(body["page"].(float64)); got != 1 {
		t.Fatalf("expected page 1, got %d", got)
	}
	if got := int(body["limit"].(float64)); got != 10 {
		t.Fatalf("expected limit 10, got %d", got)
	}
	if got := int(body["size"].(float64)); got != 10 {
		t.Fatalf("expected legacy size 10, got %d", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
