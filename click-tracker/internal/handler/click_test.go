package handler_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/handler"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/middleware"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/storage"
	"github.com/north-cloud/infrastructure/clickurl"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	testSecret         = "test-secret-key"
	testBufferCapacity = 100
)

// expiredHoursOffset is the number of hours subtracted to create an expired timestamp.
const expiredHoursOffset = 25

// maxAgeHours is the click URL expiry window used in tests.
const maxAgeHours = 24

func setupRouter(t *testing.T) (*gin.Engine, *storage.Buffer) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	signer := clickurl.NewSigner(testSecret)
	buf := storage.NewBuffer(testBufferCapacity)
	log := infralogger.NewNop()
	maxAge := maxAgeHours * time.Hour

	h := handler.NewClickHandler(signer, buf, log, maxAge)
	r.GET("/click", h.HandleClick)

	return r, buf
}

func signedURL(
	t *testing.T,
	queryID, resultID string,
	pos, page int,
	ts int64,
	dest string,
) string {
	t.Helper()

	signer := clickurl.NewSigner(testSecret)
	params := clickurl.ClickParams{
		QueryID:        queryID,
		ResultID:       resultID,
		Position:       pos,
		Page:           page,
		Timestamp:      ts,
		DestinationURL: dest,
	}
	sig := signer.Sign(params.Message())

	return "/click?q=" + queryID +
		"&r=" + resultID +
		"&p=" + strconv.Itoa(pos) +
		"&pg=" + strconv.Itoa(page) +
		"&t=" + strconv.FormatInt(ts, 10) +
		"&u=" + url.QueryEscape(dest) +
		"&sig=" + sig
}

func TestHandleClick_ValidRedirect(t *testing.T) {
	r, buf := setupRouter(t)
	defer buf.Close()

	now := time.Now().Unix()
	dest := "https://example.com/article"
	target := signedURL(t, "q_abc", "r_doc", 3, 1, now, dest)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != dest {
		t.Fatalf("expected redirect to %s, got %q", dest, loc)
	}
}

func TestHandleClick_InvalidSignature(t *testing.T) {
	r, buf := setupRouter(t)
	defer buf.Close()

	now := time.Now().Unix()
	target := "/click?q=q_abc&r=r_doc&p=3&pg=1" +
		"&t=" + strconv.FormatInt(now, 10) +
		"&u=" + url.QueryEscape("https://example.com") +
		"&sig=000000000000"

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid signature, got %d", w.Code)
	}
}

func TestHandleClick_ExpiredTimestamp(t *testing.T) {
	r, buf := setupRouter(t)
	defer buf.Close()

	old := time.Now().Add(-expiredHoursOffset * time.Hour).Unix()
	target := signedURL(t, "q_abc", "r_doc", 3, 1, old, "https://example.com")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Fatalf("expected 410 for expired timestamp, got %d", w.Code)
	}
}

func TestHandleClick_BotSkipsStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	signer := clickurl.NewSigner(testSecret)
	buf := storage.NewBuffer(testBufferCapacity)
	defer buf.Close()
	log := infralogger.NewNop()
	maxAge := maxAgeHours * time.Hour

	// Add bot filter middleware before handler
	r.Use(middleware.BotFilter())
	h := handler.NewClickHandler(signer, buf, log, maxAge)
	r.GET("/click", h.HandleClick)

	now := time.Now().Unix()
	dest := "https://example.com/article"
	target := signedURL(t, "q_abc", "r_doc", 3, 1, now, dest)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	req.Header.Set("User-Agent", "Googlebot/2.1 (+http://www.google.com/bot.html)")
	r.ServeHTTP(w, req)

	// Should still redirect
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 for bot, got %d", w.Code)
	}

	// Buffer should be empty — bot event was not enqueued
	if buf.Len() != 0 {
		t.Fatalf("expected 0 buffered events for bot, got %d", buf.Len())
	}
}

func TestHandleClick_MissingParams(t *testing.T) {
	r, buf := setupRouter(t)
	defer buf.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click?q=abc", http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing params, got %d", w.Code)
	}
}
