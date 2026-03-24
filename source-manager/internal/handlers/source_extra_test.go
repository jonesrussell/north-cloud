package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/source-manager/internal/handlers"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockSourceHandlerExtra(t *testing.T) (*handlers.SourceHandler, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	repo := repository.NewSourceRepository(db, testhelpers.NewTestLogger())
	handler := handlers.NewSourceHandler(repo, testhelpers.NewTestLogger(), nil)

	cleanup := func() {
		mock.ExpectClose()
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close: %v", closeErr)
		}
	}

	return handler, mock, cleanup
}

func sourceListCols() []string {
	return []string{
		"id", "name", "url", "rate_limit", "max_depth",
		"time", "selectors", "enabled",
		"feed_url", "sitemap_url", "ingestion_mode", "feed_poll_interval_minutes",
		"feed_disabled_at", "feed_disable_reason",
		"allow_source_discovery", "identity_key", "extraction_profile", "template_hint",
		"render_mode", "type", "indigenous_region",
		"disabled_at", "disable_reason",
		"created_at", "updated_at",
	}
}

func addSourceRowExtra(rows *sqlmock.Rows, id, name string) {
	now := time.Now()
	rows.AddRow(
		id, name, "https://example.com", "1s", 2,
		[]byte(`["09:00"]`),
		[]byte(`{"article":{"title":"h1"},"list":{},"page":{}}`),
		true,
		nil, nil, "", 0,
		nil, nil,
		false, nil, nil, nil,
		"static", "news", nil,
		nil, nil,
		now, now,
	)
}

func TestSourceHandler_List_WithSearchParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.GET("/api/v1/sources", handler.List)

	rows := sqlmock.NewRows(sourceListCols())
	addSourceRowExtra(rows, "id-1", "CBC News")

	mock.ExpectQuery("SELECT id, name, url").
		WithArgs("%cbc%", 100, 0).
		WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs("%cbc%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources?search=cbc", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.InDelta(t, 1, resp["total"], 0.001)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_List_WithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.GET("/api/v1/sources", handler.List)

	rows := sqlmock.NewRows(sourceListCols())
	addSourceRowExtra(rows, "id-2", "Source Page 2")

	mock.ExpectQuery("SELECT id, name, url").
		WithArgs(10, 10).
		WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources?limit=10&page=2", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.InDelta(t, 25, resp["total"], 0.001)
	assert.InDelta(t, 2, resp["page"], 0.001)
	assert.InDelta(t, 10, resp["per_page"], 0.001)
	assert.InDelta(t, 3, resp["total_pages"], 0.001)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_List_WithEnabledFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.GET("/api/v1/sources", handler.List)

	rows := sqlmock.NewRows(sourceListCols())
	addSourceRowExtra(rows, "id-1", "Enabled Source")

	mock.ExpectQuery("SELECT id, name, url").
		WithArgs(true, 100, 0).
		WillReturnRows(rows)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*)")).
		WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources?enabled=true", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_List_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := newClosedDBSourceHandler(t)

	router.GET("/api/v1/sources", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSourceHandler_GetCities_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := newClosedDBSourceHandler(t)

	router.GET("/api/v1/cities", handler.GetCities)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cities", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSourceHandler_Update_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.PUT("/api/v1/sources/:id", handler.Update)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs(
			"upd-id",
			"Updated Source", "https://updated.com", "5s", 3,
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			true,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// GetByID after update
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WithArgs("upd-id").
		WillReturnRows(
			sqlmock.NewRows(sourceListCols()).AddRow(
				"upd-id", "Updated Source", "https://updated.com", "5s", 3,
				[]byte(`["09:00"]`),
				[]byte(`{"article":{"title":"h1"},"list":{},"page":{}}`),
				true,
				nil, nil, "crawl", 0,
				nil, nil,
				false, nil, nil, nil,
				"static", "news", nil,
				nil, nil,
				now, now,
			),
		)

	body := `{"name":"Updated Source","url":"https://updated.com","rate_limit":"5","max_depth":3,"enabled":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sources/upd-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_DisableFeed_MissingBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/disable-feed", handler.DisableFeed)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources/550e8400-e29b-41d4-a716-446655440000/disable-feed",
		http.NoBody,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_DisableFeed_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/disable-feed", handler.DisableFeed)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs("550e8400-e29b-41d4-a716-446655440000", "not_found").
		WillReturnResult(sqlmock.NewResult(0, 1))

	body := `{"reason":"not_found"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources/550e8400-e29b-41d4-a716-446655440000/disable-feed",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_EnableFeed_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/enable-feed", handler.EnableFeed)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs("550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources/550e8400-e29b-41d4-a716-446655440000/enable-feed",
		http.NoBody,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_DisableSource_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/disable", handler.DisableSource)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs("550e8400-e29b-41d4-a716-446655440000", "maintenance").
		WillReturnResult(sqlmock.NewResult(0, 1))

	body := `{"reason":"maintenance"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources/550e8400-e29b-41d4-a716-446655440000/disable",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_EnableSource_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/enable", handler.EnableSource)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs("550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources/550e8400-e29b-41d4-a716-446655440000/enable",
		http.NoBody,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_DisableFeed_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/disable-feed", handler.DisableFeed)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs("550e8400-e29b-41d4-a716-446655440000", "not_found").
		WillReturnResult(sqlmock.NewResult(0, 0))

	body := `{"reason":"not_found"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources/550e8400-e29b-41d4-a716-446655440000/disable-feed",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_EnableFeed_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/enable-feed", handler.EnableFeed)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE sources")).
		WithArgs("550e8400-e29b-41d4-a716-446655440000").
		WillReturnResult(sqlmock.NewResult(0, 0))

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources/550e8400-e29b-41d4-a716-446655440000/enable-feed",
		http.NoBody,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_GetByIdentityKey_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandlerExtra(t)
	defer cleanup()

	router.GET("/api/v1/sources/by-identity-key", handler.GetByIdentityKey)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WithArgs("no-match").
		WillReturnRows(sqlmock.NewRows(sourceListCols()))

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sources/by-identity-key?identity_key=no-match",
		http.NoBody,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}
