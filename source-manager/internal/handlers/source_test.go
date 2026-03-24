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
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockSourceHandler(t *testing.T) (*handlers.SourceHandler, sqlmock.Sqlmock, func()) {
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

func TestSourceHandler_Create_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources", handler.Create)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_Create_InvalidSourceType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources", handler.Create)

	body := `{"name":"Test","url":"https://example.com","type":"invalid_type"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid source type")
}

func TestSourceHandler_Create_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources", handler.Create)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO sources")).
		WithArgs(
			sqlmock.AnyArg(), // id
			"Test Source", "https://example.com", "10s", 0,
			sqlmock.AnyArg(), // time
			sqlmock.AnyArg(), // selectors
			false,
			sqlmock.AnyArg(), // feed_url
			sqlmock.AnyArg(), // sitemap_url
			sqlmock.AnyArg(), // ingestion_mode
			sqlmock.AnyArg(), // feed_poll_interval_minutes
			sqlmock.AnyArg(), // allow_source_discovery
			sqlmock.AnyArg(), // identity_key
			sqlmock.AnyArg(), // extraction_profile
			sqlmock.AnyArg(), // template_hint
			sqlmock.AnyArg(), // render_mode
			sqlmock.AnyArg(), // type
			sqlmock.AnyArg(), // indigenous_region
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"name":"Test Source","url":"https://example.com","rate_limit":"10"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp models.Source
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.ID)
	assert.Equal(t, "Test Source", resp.Name)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_GetByID_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.GET("/api/v1/sources/:id", handler.GetByID)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WithArgs("not-found-id").
		WillReturnRows(sqlmock.NewRows(nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/not-found-id", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_GetByID_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.GET("/api/v1/sources/:id", handler.GetByID)

	now := time.Now()
	selectorsJSON := []byte(`{"article":{"title":"h1"},"list":{},"page":{}}`)
	timeJSON := []byte(`["09:00"]`)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, url")).
		WithArgs("src-123").
		WillReturnRows(
			sqlmock.NewRows([]string{
				"id", "name", "url", "rate_limit", "max_depth",
				"time", "selectors", "enabled",
				"feed_url", "sitemap_url", "ingestion_mode", "feed_poll_interval_minutes",
				"feed_disabled_at", "feed_disable_reason",
				"allow_source_discovery", "identity_key", "extraction_profile", "template_hint",
				"render_mode", "type", "indigenous_region",
				"disabled_at", "disable_reason",
				"created_at", "updated_at",
			}).AddRow(
				"src-123", "My Source", "https://example.com", "5s", 3,
				timeJSON, selectorsJSON, true,
				nil, nil, "crawl", 0,
				nil, nil,
				false, nil, nil, nil,
				"static", "news", nil,
				nil, nil,
				now, now,
			),
		)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/src-123", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.Source
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "src-123", resp.ID)
	assert.Equal(t, "My Source", resp.Name)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_Delete_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.DELETE("/api/v1/sources/:id", handler.Delete)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM sources WHERE id = $1")).
		WithArgs("del-id").
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sources/del-id", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_Delete_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.DELETE("/api/v1/sources/:id", handler.Delete)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM sources WHERE id = $1")).
		WithArgs("fail-id").
		WillReturnResult(sqlmock.NewResult(0, 0))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sources/fail-id", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_Update_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.PUT("/api/v1/sources/:id", handler.Update)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/sources/some-id", strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_Update_InvalidSourceType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.PUT("/api/v1/sources/:id", handler.Update)

	body := `{"name":"Test","url":"https://example.com","type":"bad_type"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sources/some-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_GetByIdentityKey_MissingParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.GET("/api/v1/sources/by-identity-key", handler.GetByIdentityKey)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/by-identity-key", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "identity_key")
}

func TestSourceHandler_GetCities_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.GET("/api/v1/cities", handler.GetCities)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT name as source_name")).
		WillReturnRows(
			sqlmock.NewRows([]string{"source_name"}).
				AddRow("Sudbury").
				AddRow("Timmins"),
		)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cities", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	cities, ok := resp["cities"].([]any)
	require.True(t, ok)
	assert.Len(t, cities, 2)
	assert.InDelta(t, 2, resp["count"], 0.001)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSourceHandler_FetchMetadata_MissingURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/fetch-metadata", handler.FetchMetadata)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/fetch-metadata", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_TestCrawl_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/test-crawl", handler.TestCrawl)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/test-crawl", strings.NewReader(`invalid`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_TestCrawl_MissingURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/test-crawl", handler.TestCrawl)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/test-crawl", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_DisableFeed_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/disable-feed", handler.DisableFeed)

	body := `{"reason":"test reason"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/not-a-uuid/disable-feed", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid source ID")
}

func TestSourceHandler_EnableFeed_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/enable-feed", handler.EnableFeed)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/not-a-uuid/enable-feed", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_DisableSource_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/disable", handler.DisableSource)

	body := `{"reason":"maintenance"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/bad-id/disable", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_EnableSource_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/:id/enable", handler.EnableSource)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/bad-id/enable", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_BatchCreate_EmptySources(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/batch", handler.BatchCreate)

	body := `{"sources":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "empty")
}

func TestSourceHandler_BatchCreate_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/batch", handler.BatchCreate)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/batch", strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_ImportExcel_NoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/import-excel", handler.ImportExcel)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/import-excel", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_ImportIndigenous_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, _, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.POST("/api/v1/sources/import-indigenous", handler.ImportIndigenous)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources/import-indigenous",
		strings.NewReader(`not-json`),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSourceHandler_ListIndigenous_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Use closed DB for error path
	handler := newClosedDBSourceHandler(t)
	router.GET("/api/v1/sources/indigenous", handler.ListIndigenous)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources/indigenous", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// newClosedDBSourceHandler creates a SourceHandler backed by a closed DB for error-path testing.
func newClosedDBSourceHandler(t *testing.T) *handlers.SourceHandler {
	t.Helper()
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	db.Close()

	repo := repository.NewSourceRepository(db, testhelpers.NewTestLogger())
	return handlers.NewSourceHandler(repo, testhelpers.NewTestLogger(), nil)
}

func TestSourceHandler_List_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler, mock, cleanup := newMockSourceHandler(t)
	defer cleanup()

	router.GET("/api/v1/sources", handler.List)

	now := time.Now()
	selectorsJSON := []byte(`{"article":{},"list":{},"page":{}}`)
	timeJSON := []byte(`[]`)

	mock.ExpectQuery("SELECT id, name, url").
		WillReturnRows(
			sqlmock.NewRows([]string{
				"id", "name", "url", "rate_limit", "max_depth",
				"time", "selectors", "enabled",
				"feed_url", "sitemap_url", "ingestion_mode", "feed_poll_interval_minutes",
				"feed_disabled_at", "feed_disable_reason",
				"allow_source_discovery", "identity_key", "extraction_profile", "template_hint",
				"render_mode", "type", "indigenous_region",
				"disabled_at", "disable_reason",
				"created_at", "updated_at",
			}).AddRow(
				"id-1", "Source 1", "https://example.com", "1s", 2,
				timeJSON, selectorsJSON, true,
				nil, nil, "", 0,
				nil, nil,
				false, nil, nil, nil,
				"", "news", nil,
				nil, nil,
				now, now,
			),
		)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM sources WHERE 1=1")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources?limit=10&page=1", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.InDelta(t, 1, resp["total"], 0.001)

	require.NoError(t, mock.ExpectationsWereMet())
}
