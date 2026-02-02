package api

import (
	"context"
	"errors"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	infragin "github.com/north-cloud/infrastructure/gin"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

// Default timeout and health constants.
const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
	healthCheckTimeout  = 2 * time.Second
	serviceVersion      = "1.0.0"
	decimalBase         = 10 // Base 10 for decimal number parsing
)

// Router holds the API dependencies
type Router struct {
	repo        *database.Repository
	redisClient *redis.Client
	esClient    *elasticsearch.Client
	cfg         *config.Config
}

// NewRouter creates a new API router
func NewRouter(repo *database.Repository, redisClient *redis.Client, esClient *elasticsearch.Client, cfg *config.Config) *Router {
	return &Router{
		repo:        repo,
		redisClient: redisClient,
		esClient:    esClient,
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
			if r.repo == nil {
				return errors.New("database repository not initialized")
			}
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

	// Channels (Layer 2 custom channels with rules)
	channels := v1.Group("/channels")
	channels.GET("", r.listChannels)
	channels.POST("", r.createChannel)
	channels.GET("/:id/preview", r.previewChannel) // Preview matching articles
	channels.GET("/:id", r.getChannel)
	channels.PUT("/:id", r.updateChannel)
	channels.DELETE("/:id", r.deleteChannel)

	// Publish History
	history := v1.Group("/publish-history")
	history.GET("", r.listPublishHistory)
	history.GET("/:article_id", r.getPublishHistoryByArticle)
	history.DELETE("", r.clearAllPublishHistory)

	// Stats
	stats := v1.Group("/stats")
	stats.GET("/overview", r.getStatsOverview)
	stats.GET("/channels/active", r.getActiveChannels)
	stats.GET("/channels", r.getChannelStats)

	// Articles
	articles := v1.Group("/articles")
	articles.GET("/recent", r.getRecentArticles)

	// Metadata (topics and indexes for Routing V2)
	v1.GET("/topics", r.listTopics)
	v1.GET("/indexes", r.listIndexes)
}
