package bootstrap

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/pipeline/internal/api"
	"github.com/jonesrussell/north-cloud/pipeline/internal/config"
	"github.com/jonesrussell/north-cloud/pipeline/internal/database"
	"github.com/jonesrussell/north-cloud/pipeline/internal/service"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
	healthCheckTimeout  = 2 * time.Second
	serviceVersion      = "1.0.0"
)

// SetupHTTPServer creates the HTTP server with all handlers wired.
func SetupHTTPServer(
	cfg *config.Config,
	db *database.Connection,
	log infralogger.Logger,
) *infragin.Server {
	repo := database.NewRepository(db.DB)
	pipelineSvc := service.NewPipelineService(repo, log)

	ingestHandler := api.NewIngestHandler(pipelineSvc)
	funnelHandler := api.NewFunnelHandler(pipelineSvc)

	server := infragin.NewServerBuilder("pipeline", cfg.Service.Port).
		WithLogger(log).
		WithDebug(cfg.Service.Debug).
		WithVersion(serviceVersion).
		WithTimeouts(defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout).
		WithDatabaseHealthCheck(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
			defer cancel()
			return db.Ping(ctx)
		}).
		WithRoutes(func(router *gin.Engine) {
			api.SetupRoutes(router, ingestHandler, funnelHandler, cfg.Auth.JWTSecret)
		}).
		Build()

	return server
}
