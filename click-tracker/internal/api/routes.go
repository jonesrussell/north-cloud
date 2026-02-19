package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/handler"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/middleware"
	"github.com/north-cloud/infrastructure/monitoring"
)

// SetupRoutes configures all API routes.
// Health routes are registered by the infrastructure gin builder.
// The done channel is closed on server shutdown to stop the rate limiter goroutine.
func SetupRoutes(
	router *gin.Engine,
	clickHandler *handler.ClickHandler,
	maxClicksPerMin int,
	rateLimitWindow time.Duration,
	done <-chan struct{},
) {
	// Memory health endpoint
	router.GET("/health/memory", func(c *gin.Context) {
		monitoring.MemoryHealthHandler(c.Writer, c.Request)
	})

	// Click redirect with bot filter and rate limiting
	click := router.Group("")
	click.Use(middleware.BotFilter())
	click.Use(middleware.RateLimiter(maxClicksPerMin, rateLimitWindow, done))
	click.GET("/click", clickHandler.HandleClick)
}
