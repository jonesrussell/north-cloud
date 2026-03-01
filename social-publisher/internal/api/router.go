package api

import (
	"context"

	"github.com/gin-gonic/gin"

	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/config"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
)

// Router wires API routes to the infrastructure HTTP server.
type Router struct {
	handler *Handler
	repo    *database.Repository
	cfg     *config.Config
}

// NewRouter creates a new API router.
func NewRouter(
	repo *database.Repository, orch *orchestrator.Orchestrator, cfg *config.Config, log logger.Logger,
) *Router {
	return &Router{
		handler: NewHandler(repo, orch, log),
		repo:    repo,
		cfg:     cfg,
	}
}

// NewServer builds an infrastructure gin.Server with routes and health checks.
func (r *Router) NewServer(log logger.Logger, port int) *infragin.Server {
	return infragin.NewServerBuilder("social-publisher", port).
		WithLogger(log).
		WithDebug(r.cfg.Debug).
		WithVersion("0.1.0").
		WithDatabaseHealthCheck(func() error {
			return r.repo.Ping(context.TODO())
		}).
		WithRoutes(func(router *gin.Engine) {
			r.setupRoutes(router)
		}).
		Build()
}

func (r *Router) setupRoutes(router *gin.Engine) {
	v1 := router.Group("/api/v1")

	v1.POST("/publish", r.handler.Publish)
	v1.GET("/status/:id", r.handler.Status)
	v1.POST("/retry/:id", r.handler.Retry)
}
