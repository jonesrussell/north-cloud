package api

import (
	"github.com/gin-gonic/gin"
	"github.com/gopost/integration/internal/logger"
)

// NewRouter creates a new Gin router with all routes and middleware
func NewRouter(statsService *StatsService, log logger.Logger, version string) *gin.Engine {
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(loggingMiddleware(log))

	// Create handlers
	handlers := NewHandlers(statsService, log, version)

	// Health check
	router.GET("/health", handlers.Health)

	// API v1 routes
	v1 := router.Group("/api/v1")
	v1.GET("/stats", handlers.GetStats)
	v1.GET("/articles/recent", handlers.GetRecentArticles)

	return router
}
