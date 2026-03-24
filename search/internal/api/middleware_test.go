//nolint:testpackage // tests unexported helper functions
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/search/internal/config"
)

// ---------------------------------------------------------------------------
// testNoopLogger — satisfies infralogger.Logger for tests
// ---------------------------------------------------------------------------

type testNoopLogger struct{}

func (testNoopLogger) Debug(_ string, _ ...infralogger.Field) {}
func (testNoopLogger) Info(_ string, _ ...infralogger.Field)  {}
func (testNoopLogger) Warn(_ string, _ ...infralogger.Field)  {}
func (testNoopLogger) Error(_ string, _ ...infralogger.Field) {}
func (testNoopLogger) Fatal(_ string, _ ...infralogger.Field) {}

func (l testNoopLogger) With(_ ...infralogger.Field) infralogger.Logger {
	return l
}

func (testNoopLogger) Sync() error { return nil }

func newTestLogger() infralogger.Logger {
	return testNoopLogger{}
}

// ---------------------------------------------------------------------------
// isOriginAllowed
// ---------------------------------------------------------------------------

func TestIsOriginAllowed_Wildcard(t *testing.T) {
	t.Helper()

	if !isOriginAllowed("http://example.com", []string{"*"}) {
		t.Error("wildcard should allow any origin")
	}
}

func TestIsOriginAllowed_ExactMatch(t *testing.T) {
	t.Helper()

	origins := []string{"http://localhost:3000", "https://northcloud.one"}

	if !isOriginAllowed("https://northcloud.one", origins) {
		t.Error("exact match should be allowed")
	}
}

func TestIsOriginAllowed_NotAllowed(t *testing.T) {
	t.Helper()

	origins := []string{"http://localhost:3000"}

	if isOriginAllowed("http://evil.com", origins) {
		t.Error("non-matching origin should not be allowed")
	}
}

func TestIsOriginAllowed_EmptyList(t *testing.T) {
	t.Helper()

	if isOriginAllowed("http://example.com", []string{}) {
		t.Error("empty list should reject all origins")
	}
}

// ---------------------------------------------------------------------------
// boolToString
// ---------------------------------------------------------------------------

func TestBoolToString_True(t *testing.T) {
	t.Helper()

	if boolToString(true) != "true" {
		t.Errorf("expected %q, got %q", "true", boolToString(true))
	}
}

func TestBoolToString_False(t *testing.T) {
	t.Helper()

	if boolToString(false) != "false" {
		t.Errorf("expected %q, got %q", "false", boolToString(false))
	}
}

// ---------------------------------------------------------------------------
// intToString
// ---------------------------------------------------------------------------

func TestIntToString(t *testing.T) {
	t.Helper()

	if intToString(42) != "42" {
		t.Errorf("expected %q, got %q", "42", intToString(42))
	}
}

func TestIntToString_Zero(t *testing.T) {
	t.Helper()

	if intToString(0) != "0" {
		t.Errorf("expected %q, got %q", "0", intToString(0))
	}
}

// ---------------------------------------------------------------------------
// joinStrings
// ---------------------------------------------------------------------------

func TestJoinStrings_Multiple(t *testing.T) {
	t.Helper()

	result := joinStrings([]string{"GET", "POST", "OPTIONS"})
	expected := "GET, POST, OPTIONS"

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestJoinStrings_Single(t *testing.T) {
	t.Helper()

	result := joinStrings([]string{"GET"})

	if result != "GET" {
		t.Errorf("expected %q, got %q", "GET", result)
	}
}

func TestJoinStrings_Empty(t *testing.T) {
	t.Helper()

	result := joinStrings([]string{})

	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// CORSMiddleware
// ---------------------------------------------------------------------------

func TestCORSMiddleware_Disabled(t *testing.T) {
	t.Helper()

	cfg := &config.CORSConfig{Enabled: false}
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "http://evil.com")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when CORS disabled, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS headers should not be set when disabled")
	}
}

func TestCORSMiddleware_AllowedOrigin(t *testing.T) {
	t.Helper()

	cfg := &config.CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           600,
	}
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "http://localhost:3000")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("expected origin header, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("expected credentials header to be true")
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "GET, POST" {
		t.Errorf("expected methods header, got %q", w.Header().Get("Access-Control-Allow-Methods"))
	}
	if w.Header().Get("Access-Control-Max-Age") != "600" {
		t.Errorf("expected max-age=600, got %q", w.Header().Get("Access-Control-Max-Age"))
	}
}

func TestCORSMiddleware_ForbiddenOrigin(t *testing.T) {
	t.Helper()

	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"http://localhost:3000"},
	}
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "http://evil.com")
	router.ServeHTTP(w, req)

	if w.Code != httpStatusForbidden {
		t.Errorf("expected 403 for forbidden origin, got %d", w.Code)
	}
}

func TestCORSMiddleware_PreflightRequest(t *testing.T) {
	t.Helper()

	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         300,
	}
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.OPTIONS("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/test", http.NoBody)
	req.Header.Set("Origin", "http://example.com")
	router.ServeHTTP(w, req)

	if w.Code != httpStatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", w.Code)
	}
}

func TestCORSMiddleware_NoOriginHeader(t *testing.T) {
	t.Helper()

	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
	}
	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	// No Origin header set
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when no origin (wildcard fallback), got %d", w.Code)
	}
	// When no origin is sent, the middleware defaults to "*" which matches "*" in allowed list
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected origin=* fallback, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

// ---------------------------------------------------------------------------
// RecoveryMiddleware
// ---------------------------------------------------------------------------

func TestRecoveryMiddleware_PanicRecovery(t *testing.T) {
	t.Helper()

	log := newTestLogger()
	router := gin.New()
	router.Use(RecoveryMiddleware(log))
	router.GET("/panic", func(_ *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/panic", http.NoBody)
	router.ServeHTTP(w, req)

	if w.Code != httpStatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", w.Code)
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	t.Helper()

	log := newTestLogger()
	router := gin.New()
	router.Use(RecoveryMiddleware(log))
	router.GET("/ok", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ok", http.NoBody)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// LoggerMiddleware
// ---------------------------------------------------------------------------

func TestLoggerMiddleware_LogsRequest(t *testing.T) {
	t.Helper()

	log := newTestLogger()
	router := gin.New()
	router.Use(LoggerMiddleware(log))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
