package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/handler"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/middleware"
)

// SetupRoutes configures all API routes.
// Health routes (/health, /health/memory) are registered by the infrastructure gin builder.
// The done channel is closed on server shutdown to stop the rate limiter goroutine.
func SetupRoutes(
	router *gin.Engine,
	clickHandler *handler.ClickHandler,
	maxClicksPerMin int,
	rateLimitWindow time.Duration,
	done <-chan struct{},
) {
	// Click redirect with bot filter and rate limiting
	click := router.Group("")
	click.Use(middleware.BotFilter())
	click.Use(middleware.RateLimiter(maxClicksPerMin, rateLimitWindow, done))
	click.GET("/click", clickHandler.HandleClick)
}
