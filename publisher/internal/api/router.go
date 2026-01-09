package api

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

// Default timeout and health constants.
const (
	defaultReadTimeout   = 30 * time.Second
	defaultWriteTimeout  = 60 * time.Second
	defaultIdleTimeout   = 120 * time.Second
	httpStatusOK         = 200
	healthStatusHealthy  = "healthy"
	healthStatusDegraded = "degraded"
	healthCheckTimeout   = 2 * time.Second
	serviceVersion       = "1.0.0"
	decimalBase          = 10 // Base 10 for decimal number parsing
)

// Router holds the API dependencies
type Router struct {
	repo        *database.Repository
	redisClient *redis.Client
	cfg         *config.Config
}

// NewRouter creates a new API router
func NewRouter(repo *database.Repository, redisClient *redis.Client, cfg *config.Config) *Router {
	return &Router{
		repo:        repo,
		redisClient: redisClient,
		cfg:         cfg,
	}
}

// NewServer creates a new HTTP server using the infrastructure gin package.
func (r *Router) NewServer(log logger.Logger) *infragin.Server {
	// Build CORS config
	corsConfig := infragin.CORSConfig{
		Enabled:          true,
		AllowedOrigins:   r.cfg.Server.CORSOrigins,
		AllowCredentials: true,
	}

	// Determine port from address
	port := extractPort(r.cfg.Server.Address)

	// Build server using infrastructure gin package
	server := infragin.NewServerBuilder("publisher", port).
		WithLogger(log).
		WithDebug(r.cfg.Debug).
		WithVersion(serviceVersion).
		WithTimeouts(defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout).
		WithCORS(corsConfig).
		WithDatabaseHealthCheck(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
			defer cancel()
			return r.repo.Ping(ctx)
		}).
		WithRedisHealthCheck(func() error {
			if r.redisClient == nil {
				return nil // No Redis client configured, not an error
			}
			ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
			defer cancel()
			return r.redisClient.Ping(ctx).Err()
		}).
		WithRoutes(func(router *gin.Engine) {
			// Setup service-specific routes (health routes added by builder)
			r.setupServiceRoutes(router)
		}).
		Build()

	return server
}

// extractPort extracts the port number from an address string like ":8070" or "localhost:8070"
func extractPort(address string) int {
	const defaultPort = 8070
	if address == "" {
		return defaultPort
	}

	// Find the last colon
	for i := len(address) - 1; i >= 0; i-- {
		if address[i] != ':' {
			continue
		}
		portStr := address[i+1:]
		port := 0
		for _, c := range portStr {
			if c < '0' || c > '9' {
				return defaultPort
			}
			port = port*decimalBase + int(c-'0')
		}
		if port > 0 {
			return port
		}
		break
	}

	return defaultPort
}

// setupServiceRoutes configures service-specific API routes (not health routes).
// Health routes are handled by the infrastructure gin package.
func (r *Router) setupServiceRoutes(router *gin.Engine) {
	// API v1 routes - protected with JWT
	v1 := infragin.ProtectedGroup(router, "/api/v1", r.cfg.Auth.JWTSecret)

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
	channels.GET("/:id/test-publish", r.testPublish) // More specific route before :id
	channels.GET("/:id", r.getChannel)
	channels.PUT("/:id", r.updateChannel)
	channels.DELETE("/:id", r.deleteChannel)

	// Routes
	routes := v1.Group("/routes")
	routes.GET("", r.listRoutes)
	routes.GET("/preview", r.previewRoute) // More specific route before :id
	routes.POST("", r.createRoute)
	routes.GET("/:id", r.getRoute)
	routes.PUT("/:id", r.updateRoute)
	routes.DELETE("/:id", r.deleteRoute)

	// Publish History
	history := v1.Group("/publish-history")
	history.GET("", r.listPublishHistory)
	history.GET("/:article_id", r.getPublishHistoryByArticle)
	history.DELETE("", r.clearAllPublishHistory)

	// Stats
	stats := v1.Group("/stats")
	stats.GET("/overview", r.getStatsOverview)
	stats.GET("/channels/active", r.getActiveChannels) // More specific route first
	stats.GET("/channels", r.getChannelStats)
	stats.GET("/routes", r.getRouteStats)

	// Articles
	articles := v1.Group("/articles")
	articles.GET("/recent", r.getRecentArticles)
}

// SetupRoutes is kept for backward compatibility but marked as deprecated.
//
// Deprecated: Use NewServer() instead which includes middleware setup.
func (r *Router) SetupRoutes() *gin.Engine {
	// Set Gin mode based on config
	if !r.cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware(r.cfg.Server.CORSOrigins)) // Defined in middleware.go

	// Health check (public, no auth)
	router.GET("/health", r.healthCheck)

	// Setup service routes
	r.setupServiceRoutes(router)

	return router
}

// healthCheck returns the service health status (kept for backward compatibility)
func (r *Router) healthCheck(c *gin.Context) {
	health := gin.H{
		"status":  healthStatusHealthy,
		"service": "publisher",
		"version": serviceVersion,
	}

	// Check database connection with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), healthCheckTimeout)
	defer cancel()

	dbConnected := true
	if err := r.repo.Ping(ctx); err != nil {
		dbConnected = false
		health["status"] = healthStatusDegraded
	}
	health["database"] = gin.H{
		"connected": dbConnected,
	}

	// Check Redis connection
	redisHealth := r.checkRedisHealth(ctx)
	health["redis"] = redisHealth

	// Update status if Redis is not connected
	if connected, ok := redisHealth["connected"].(bool); ok && !connected {
		if health["status"] == healthStatusHealthy {
			health["status"] = healthStatusDegraded
		}
	}

	c.JSON(httpStatusOK, health)
}

// checkRedisHealth checks Redis connection and returns health info
func (r *Router) checkRedisHealth(ctx context.Context) gin.H {
	if r.redisClient == nil {
		return gin.H{
			"connected": false,
			"error":     "Redis client not initialized",
		}
	}

	redisConnected, redisErr := checkRedisConnection(ctx, r.redisClient)
	redisHealth := gin.H{
		"connected": redisConnected,
	}
	if redisErr != nil {
		redisHealth["error"] = redisErr.Error()
	}

	return redisHealth
}

// checkRedisConnection tests Redis connectivity
func checkRedisConnection(ctx context.Context, client *redis.Client) (bool, error) {
	if client == nil {
		return false, nil
	}
	err := client.Ping(ctx).Err()
	if err != nil {
		return false, err
	}
	return true, nil
}
