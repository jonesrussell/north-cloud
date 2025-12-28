package api

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	infrajwt "github.com/north-cloud/infrastructure/jwt"
)

// Router holds the API dependencies
type Router struct {
	repo *database.Repository
}

// NewRouter creates a new API router
func NewRouter(repo *database.Repository) *Router {
	return &Router{repo: repo}
}

// SetupRoutes configures all API routes with middleware
func (r *Router) SetupRoutes() *gin.Engine {
	// Set Gin mode based on environment
	if ginMode := os.Getenv("GIN_MODE"); ginMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else if appDebug := os.Getenv("APP_DEBUG"); appDebug == "false" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware()) // Defined in middleware.go

	// Health check (public, no auth)
	router.GET("/health", r.healthCheck)

	// API v1 routes - protected with JWT
	v1 := router.Group("/api/v1")

	// Add JWT middleware if JWT secret is configured
	if jwtSecret := os.Getenv("AUTH_JWT_SECRET"); jwtSecret != "" {
		v1.Use(infrajwt.Middleware(jwtSecret))
	}

	// Sources
	sources := v1.Group("/sources")
	sources.GET("", r.listSources)
	sources.POST("", r.createSource)
	sources.GET("/:id", r.getSource)
	sources.PUT("/:id", r.updateSource)
	sources.DELETE("/:id", r.deleteSource)

	// Channels
	channels := v1.Group("/channels")
	channels.GET("", r.listChannels)
	channels.POST("", r.createChannel)
	channels.GET("/:id", r.getChannel)
	channels.PUT("/:id", r.updateChannel)
	channels.DELETE("/:id", r.deleteChannel)

	// Routes
	routes := v1.Group("/routes")
	routes.GET("", r.listRoutes)
	routes.POST("", r.createRoute)
	routes.GET("/:id", r.getRoute)
	routes.PUT("/:id", r.updateRoute)
	routes.DELETE("/:id", r.deleteRoute)

	// Publish History
	history := v1.Group("/publish-history")
	history.GET("", r.listPublishHistory)
	history.GET("/:article_id", r.getPublishHistoryByArticle)

	// Stats
	stats := v1.Group("/stats")
	stats.GET("/overview", r.getStatsOverview)
	stats.GET("/channels", r.getChannelStats)
	stats.GET("/routes", r.getRouteStats)

	return router
}

const (
	httpStatusOK = 200
)

// healthCheck returns the service health status
func (r *Router) healthCheck(c *gin.Context) {
	c.JSON(httpStatusOK, gin.H{
		"status":  "healthy",
		"service": "publisher",
		"version": "1.0.0",
	})
}
