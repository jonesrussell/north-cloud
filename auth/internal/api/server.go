package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/auth/internal/auth"
	"github.com/jonesrussell/north-cloud/auth/internal/config"
	"github.com/north-cloud/infrastructure/monitoring"
)

const (
	// HTTP server timeout constants
	readTimeoutSeconds  = 10
	writeTimeoutSeconds = 30
	idleTimeoutSeconds  = 120
)

// Server represents the HTTP server
type Server struct {
	config     *config.Config
	router     *gin.Engine
	server     *http.Server
	jwtManager *auth.JWTManager
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config) (*Server, error) {
	// Set Gin mode
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware())

	// Create JWT manager
	jwtConfig := cfg.GetJWTConfig()
	jwtManager := auth.NewJWTManager(jwtConfig.Secret, jwtConfig.Expiration)

	// Create auth handler
	authHandler := NewAuthHandler(cfg, jwtManager)

	// Health check (public)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Memory health endpoint
	router.GET("/health/memory", func(c *gin.Context) {
		monitoring.MemoryHealthHandler(c.Writer, c.Request)
	})

	// Auth routes
	v1 := router.Group("/api/v1")
	authGroup := v1.Group("/auth")
	authGroup.POST("/login", authHandler.Login)

	// Create HTTP server
	addr := fmt.Sprintf(":%s", cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  readTimeoutSeconds * time.Second,
		WriteTimeout: writeTimeoutSeconds * time.Second,
		IdleTimeout:  idleTimeoutSeconds * time.Second,
	}

	return &Server{
		config:     cfg,
		router:     router,
		server:     server,
		jwtManager: jwtManager,
	}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	if s.config.Debug {
		log.Printf("Starting auth service on port %s\n", s.config.Port)
	}
	return s.server.ListenAndServe()
}

// corsMiddleware adds CORS headers
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

// loggingMiddleware logs HTTP requests
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		log.Printf("[%s] %s %s %d %v\n", method, path, c.ClientIP(), statusCode, duration)
	}
}
