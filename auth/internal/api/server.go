package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/auth/internal/auth"
	"github.com/jonesrussell/north-cloud/auth/internal/config"
	"github.com/north-cloud/infrastructure/health"
	"github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/monitoring"
)

const (
	// HTTP server timeout constants
	readTimeoutSeconds  = 10
	writeTimeoutSeconds = 30
	idleTimeoutSeconds  = 120
)

// NewServer creates a new HTTP server.
func NewServer(cfg *config.Config, log logger.Logger) (*http.Server, error) {
	// Set Gin mode
	if !cfg.Service.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))

	// Create JWT manager
	jwtConfig := cfg.GetJWTConfig()
	jwtManager := auth.NewJWTManager(jwtConfig.Secret, jwtConfig.Expiration)

	// Create auth handler
	authHandler := NewAuthHandler(cfg, jwtManager, log)

	// Health check routes
	router.GET("/health", health.SimpleGinHandler())
	router.GET("/health/memory", func(c *gin.Context) {
		monitoring.MemoryHealthHandler(c.Writer, c.Request)
	})

	// Auth routes
	v1 := router.Group("/api/v1")
	authGroup := v1.Group("/auth")
	authGroup.POST("/login", authHandler.Login)

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.Address(),
		Handler:      router,
		ReadTimeout:  readTimeoutSeconds * time.Second,
		WriteTimeout: writeTimeoutSeconds * time.Second,
		IdleTimeout:  idleTimeoutSeconds * time.Second,
	}

	return server, nil
}

// corsMiddleware adds CORS headers.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// loggingMiddleware logs HTTP requests using the infrastructure logger.
func loggingMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		log.Info("HTTP request",
			logger.String("method", method),
			logger.String("path", path),
			logger.String("client_ip", c.ClientIP()),
			logger.Int("status", statusCode),
			logger.Duration("duration", duration),
		)
	}
}
