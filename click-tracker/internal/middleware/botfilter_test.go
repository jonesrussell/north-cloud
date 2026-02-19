package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/middleware"
)

func TestBotFilter_AllowsNormalUA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BotFilter())
	r.GET("/click", func(c *gin.Context) {
		isBot, exists := c.Get("is_bot")
		if exists && isBot == true {
			c.String(http.StatusOK, "bot")
			return
		}
		c.String(http.StatusOK, "human")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click", http.NoBody)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	r.ServeHTTP(w, req)

	if w.Body.String() != "human" {
		t.Fatalf("expected 'human' for normal UA, got %q", w.Body.String())
	}
}

func TestBotFilter_FlagsGooglebot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BotFilter())
	r.GET("/click", func(c *gin.Context) {
		isBot, exists := c.Get("is_bot")
		if exists && isBot == true {
			c.String(http.StatusOK, "bot")
			return
		}
		c.String(http.StatusOK, "human")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click", http.NoBody)
	req.Header.Set("User-Agent", "Googlebot/2.1 (+http://www.google.com/bot.html)")
	r.ServeHTTP(w, req)

	if w.Body.String() != "bot" {
		t.Fatalf("expected 'bot' for Googlebot, got %q", w.Body.String())
	}
}

func TestBotFilter_FlagsMissingUA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BotFilter())
	r.GET("/click", func(c *gin.Context) {
		isBot, exists := c.Get("is_bot")
		if exists && isBot == true {
			c.String(http.StatusOK, "bot")
			return
		}
		c.String(http.StatusOK, "human")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/click", http.NoBody)
	// No User-Agent header
	r.ServeHTTP(w, req)

	if w.Body.String() != "bot" {
		t.Fatalf("expected 'bot' for missing UA, got %q", w.Body.String())
	}
}
