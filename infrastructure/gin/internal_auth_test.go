package gin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ginpkg "github.com/gin-gonic/gin"
	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
)

func TestInternalAuthMiddleware_RejectsMissingHeader(t *testing.T) {
	t.Parallel()

	router := newInternalAuthRouter("test-secret")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestInternalAuthMiddleware_RejectsWrongSecret(t *testing.T) {
	t.Parallel()

	router := newInternalAuthRouter("test-secret")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Internal-Secret", "wrong-secret")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestInternalAuthMiddleware_AllowsCorrectSecret(t *testing.T) {
	t.Parallel()

	router := newInternalAuthRouter("test-secret")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Internal-Secret", "test-secret")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestInternalAuthMiddleware_AbortsChainOnFailure(t *testing.T) {
	t.Parallel()

	handlerCalled := false
	router := ginpkg.New()
	router.Use(infragin.InternalAuthMiddleware("test-secret"))
	router.GET("/test", func(c *ginpkg.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, ginpkg.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	router.ServeHTTP(w, req)

	if handlerCalled {
		t.Error("handler was called despite missing secret, want it to be aborted")
	}
}

// newInternalAuthRouter creates a gin.Engine with InternalAuthMiddleware and a simple GET /test route.
func newInternalAuthRouter(secret string) *ginpkg.Engine {
	ginpkg.SetMode(ginpkg.TestMode)
	router := ginpkg.New()
	router.Use(infragin.InternalAuthMiddleware(secret))
	router.GET("/test", func(c *ginpkg.Context) {
		c.JSON(http.StatusOK, ginpkg.H{"ok": true})
	})
	return router
}
