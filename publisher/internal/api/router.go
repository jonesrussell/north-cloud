package api

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gopost/integration/internal/logger"
	infrajwt "github.com/north-cloud/infrastructure/jwt"
)

// NewRouter creates a new Gin router with all routes and middleware
func NewRouter(statsService *StatsService, log logger.Logger, version string) *gin.Engine {
	// Set Gin mode based on environment
	// Check GIN_MODE first (explicit), then APP_DEBUG (implicit)
	if ginMode := os.Getenv("GIN_MODE"); ginMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else if appDebug := os.Getenv("APP_DEBUG"); appDebug == "false" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		// Default to debug mode for development
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))

	// Create handlers
	handlers := NewHandlers(statsService, log, version)

	// Health check
	router.GET("/health", handlers.Health)

	// API v1 routes - protected with JWT
	v1 := router.Group("/api/v1")
	// Add JWT middleware if JWT secret is configured
	if jwtSecret := os.Getenv("AUTH_JWT_SECRET"); jwtSecret != "" {
		v1.Use(infrajwt.Middleware(jwtSecret))
	}
	v1.GET("/stats", handlers.GetStats)
	v1.GET("/articles/recent", handlers.GetRecentArticles)

	return router
}
