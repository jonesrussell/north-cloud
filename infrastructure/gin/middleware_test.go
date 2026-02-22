package gin_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ginpkg "github.com/gin-gonic/gin"
	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
)

func TestRequestIDLoggerMiddleware_GeneratesID(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	router.ServeHTTP(w, req)

	reqID := w.Header().Get("X-Request-ID")
	if reqID == "" {
		t.Fatal("X-Request-ID response header is empty, want a generated ID")
	}

	// Generated IDs should be 32 hex chars (16 random bytes encoded)
	const expectedLen = 32
	if len(reqID) != expectedLen {
		t.Errorf("generated request ID length = %d, want %d", len(reqID), expectedLen)
	}
}

func TestRequestIDLoggerMiddleware_PreservesExistingID(t *testing.T) {
	t.Parallel()

	const inboundID = "trace-from-upstream-abc123"

	log := logger.NewNop()
	router := ginpkg.New()
	router.Use(infragin.RequestIDLoggerMiddleware(log))

	var gotGinCtxID string
	router.GET("/test", func(c *ginpkg.Context) {
		if v, ok := c.Get("request_id"); ok {
			gotGinCtxID, _ = v.(string)
		}
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Request-ID", inboundID)
	router.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != inboundID {
		t.Errorf("response X-Request-ID = %q, want %q", got, inboundID)
	}
	if gotGinCtxID != inboundID {
		t.Errorf("gin context request_id = %q, want %q", gotGinCtxID, inboundID)
	}
}

func TestRequestIDLoggerMiddleware_RejectsOversizedID(t *testing.T) {
	t.Parallel()

	oversizedID := strings.Repeat("x", 200)
	router := newTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Request-ID", oversizedID)
	router.ServeHTTP(w, req)

	gotID := w.Header().Get("X-Request-ID")
	if gotID == oversizedID {
		t.Error("middleware accepted oversized X-Request-ID, want it to generate a new one")
	}
	if gotID == "" {
		t.Fatal("X-Request-ID response header is empty after rejecting oversized ID")
	}
}

func TestRequestIDLoggerMiddleware_SetsResponseHeader(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	router.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID not set on response")
	}
}

func TestRequestIDLoggerMiddleware_StoresLoggerInContext(t *testing.T) {
	t.Parallel()

	log := logger.NewNop()
	router := ginpkg.New()
	router.Use(infragin.RequestIDLoggerMiddleware(log))

	var gotLogger logger.Logger
	router.GET("/test", func(c *ginpkg.Context) {
		gotLogger = logger.FromContext(c.Request.Context())
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	router.ServeHTTP(w, req)

	if gotLogger == nil {
		t.Fatal("logger.FromContext returned nil inside handler, want enriched logger")
	}
}

func TestRequestIDLoggerMiddleware_UniqueIDs(t *testing.T) {
	t.Parallel()

	router := newTestRouter(t)

	const iterations = 100
	ids := make(map[string]bool, iterations)

	for range iterations {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		router.ServeHTTP(w, req)

		id := w.Header().Get("X-Request-ID")
		if ids[id] {
			t.Fatalf("duplicate request ID generated: %s", id)
		}
		ids[id] = true
	}
}

// newTestRouter creates a gin.Engine with RequestIDLoggerMiddleware and a simple GET /test route.
func newTestRouter(t *testing.T) *ginpkg.Engine {
	t.Helper()

	log := logger.NewNop()
	router := ginpkg.New()
	router.Use(infragin.RequestIDLoggerMiddleware(log))
	router.GET("/test", func(c *ginpkg.Context) {
		c.String(http.StatusOK, "ok")
	})

	return router
}
