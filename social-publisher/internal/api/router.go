package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/config"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
)

// Router wires API routes to the infrastructure HTTP server.
type Router struct {
	handler         *Handler
	accountsHandler *AccountsHandler
	repo            *database.Repository
	cfg             *config.Config
}

// NewRouter creates a new API router.
func NewRouter(
	repo *database.Repository, orch *orchestrator.Orchestrator, cfg *config.Config, log logger.Logger,
) *Router {
	return &Router{
		handler:         NewHandler(repo, orch, log),
		accountsHandler: NewAccountsHandler(repo, cfg.Encryption.Key, log),
		repo:            repo,
		cfg:             cfg,
	}
}

// NewServer builds an infrastructure gin.Server with routes and health checks.
func (r *Router) NewServer(log logger.Logger, port int) *infragin.Server {
	const healthCheckTimeout = 2 * time.Second

	return infragin.NewServerBuilder("social-publisher", port).
		WithLogger(log).
		WithDebug(r.cfg.Debug).
		WithVersion("0.1.0").
		WithDatabaseHealthCheck(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
			defer cancel()
			return r.repo.Ping(ctx)
		}).
		WithRoutes(func(router *gin.Engine) {
			r.setupRoutes(router)
		}).
		Build()
}

// TestEngine creates a Gin engine with routes configured, for use in tests.
func (r *Router) TestEngine() *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	r.setupRoutes(engine)
	return engine
}

func (r *Router) setupRoutes(router *gin.Engine) {
	v1 := infragin.ProtectedGroup(router, "/api/v1", r.cfg.Auth.JWTSecret)

	v1.POST("/publish", r.handler.Publish)
	v1.GET("/content", r.handler.ListContent)
	v1.GET("/status/:id", r.handler.Status)
	v1.POST("/retry/:id", r.handler.Retry)

	accounts := v1.Group("/accounts")
	accounts.GET("", r.accountsHandler.List)
	accounts.GET("/:id", r.accountsHandler.Get)
	accounts.POST("", r.accountsHandler.Create)
	accounts.PUT("/:id", r.accountsHandler.Update)
	accounts.DELETE("/:id", r.accountsHandler.Delete)
}
