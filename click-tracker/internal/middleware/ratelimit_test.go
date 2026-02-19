package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/middleware"
)

const testRateLimit = 3

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	done := make(chan struct{})
	defer close(done)

	r := gin.New()
	r.Use(middleware.RateLimiter(testRateLimit, time.Minute, done))
	r.GET("/click", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click?q=q1", http.NoBody)
	req.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	done := make(chan struct{})
	defer close(done)

	r := gin.New()
	r.Use(middleware.RateLimiter(testRateLimit, time.Minute, done))
	r.GET("/click", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for i := range testRateLimit {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/click?q=q1", http.NoBody)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}

	// This should be rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click?q=q1", http.NoBody)
	req.RemoteAddr = "1.2.3.4:1234"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	done := make(chan struct{})
	defer close(done)

	r := gin.New()
	r.Use(middleware.RateLimiter(1, time.Minute, done))
	r.GET("/click", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// First IP uses its one allowed request
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/click?q=q1", http.NoBody)
	req1.RemoteAddr = "1.1.1.1:1234"
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("IP1: expected 200, got %d", w1.Code)
	}

	// Second IP should still be allowed
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/click?q=q1", http.NoBody)
	req2.RemoteAddr = "2.2.2.2:1234"
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("IP2: expected 200, got %d", w2.Code)
	}
}
