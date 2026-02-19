package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/config"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/handler"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 60 * time.Second
)

// NewServer creates a new HTTP server.
func NewServer(
	clickHandler *handler.ClickHandler,
	cfg *config.Config,
	log infralogger.Logger,
) *infragin.Server {
	rateLimitWindow := time.Duration(cfg.RateLimit.WindowSeconds) * time.Second

	return infragin.NewServerBuilder(cfg.Service.Name, cfg.Service.Port).
		WithLogger(log).
		WithDebug(cfg.Service.Debug).
		WithVersion(cfg.Service.Version).
		WithTimeouts(defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout).
		WithRoutes(func(router *gin.Engine) {
			SetupRoutes(router, clickHandler, cfg.RateLimit.MaxClicksPerMinute, rateLimitWindow)
		}).
		Build()
}
