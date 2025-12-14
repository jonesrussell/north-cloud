// Package api implements the HTTP API for the search service.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/gocrawl/internal/api/middleware"
	"github.com/jonesrussell/gocrawl/internal/config"
	"github.com/jonesrussell/gocrawl/internal/logger"
)

// SearchManager defines the interface for search operations.
type SearchManager interface {
	// Search performs a search query.
	Search(ctx context.Context, index string, query map[string]any) ([]any, error)

	// Count returns the number of documents matching a query.
	Count(ctx context.Context, index string, query map[string]any) (int64, error)

	// Aggregate performs an aggregation query.
	Aggregate(ctx context.Context, index string, aggs map[string]any) (map[string]any, error)

	// Close closes any resources held by the search manager.
	Close() error
}

// Constants
const (
	readHeaderTimeout = 10 * time.Second // Timeout for reading headers
	DefaultMaxResults = 10
	DefaultTimeout    = 30 * time.Second
	DefaultRetries    = 3
	defaultSearchSize = 10
)

// SetupRouter creates and configures the Gin router with all routes
func SetupRouter(
	log logger.Interface,
	searchManager SearchManager,
	cfg config.Interface,
) (*gin.Engine, middleware.SecurityMiddlewareInterface) {
	// Disable Gin's default logging
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware(log))
	router.Use(corsMiddleware()) // Add CORS middleware

	// Create security middleware
	security := middleware.NewSecurityMiddleware(cfg.GetServerConfig(), log)

	// Define public routes
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API v1 routes (for dashboard frontend)
	v1 := router.Group("/api/v1")
	// Stats endpoint for dashboard
	v1.GET("/stats", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"totalArticles":   0,
			"successRate":     0,
			"avgResponseTime": 0,
			"crawled":         0,
			"failed":          0,
			"pending":         0,
			"activeSources":   0,
			"totalSources":    0,
		})
	})

	// Jobs endpoints for dashboard
	v1.GET("/jobs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"jobs": []gin.H{},
		})
	})

	v1.POST("/jobs", func(c *gin.Context) {
		var job map[string]any
		if err := c.ShouldBindJSON(&job); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"id":      "job-1",
			"status":  "pending",
			"message": "Job created successfully",
		})
	})

	v1.GET("/jobs/:id", func(c *gin.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, gin.H{
			"id":     id,
			"status": "pending",
		})
	})

	v1.DELETE("/jobs/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Job deleted successfully",
		})
	})

	// Articles endpoint for dashboard
	v1.GET("/articles", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"articles": []gin.H{},
		})
	})

	// Define protected routes
	protected := router.Group("")
	protected.Use(security.Middleware())
	protected.POST("/search", handleSearch(searchManager))

	return router, security
}

// loggingMiddleware creates a middleware that logs HTTP requests
func loggingMiddleware(log logger.Interface) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		log.Info("HTTP Request",
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"status", statusCode,
			"latency", latency,
		)
	}
}

// corsMiddleware adds CORS headers to allow frontend access
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, "+
				"Authorization, accept, origin, Cache-Control, X-Requested-With, X-API-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// handleSearch creates a handler for search requests
func handleSearch(searchManager SearchManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request payload",
			})
			return
		}

		if req.Query == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Query cannot be empty",
			})
			return
		}

		// Set default size if not provided
		if req.Size == 0 {
			req.Size = defaultSearchSize
		}

		// Create search query
		query := map[string]any{
			"query": map[string]any{
				"match": map[string]any{
					"content": req.Query,
				},
			},
			"size": req.Size,
		}

		// Perform search
		results, err := searchManager.Search(c.Request.Context(), req.Index, query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Search failed",
			})
			return
		}

		// Get total count
		total, err := searchManager.Count(c.Request.Context(), req.Index, query)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to get total count",
			})
			return
		}

		// Return response
		response := SearchResponse{
			Results: results,
			Total:   int(total),
		}
		c.JSON(http.StatusOK, response)
	}
}

// StartHTTPServer starts the HTTP server with the given configuration
func StartHTTPServer(
	log logger.Interface,
	searchManager SearchManager,
	cfg config.Interface,
) (*http.Server, middleware.SecurityMiddlewareInterface, error) {
	router, security := SetupRouter(log, searchManager, cfg)

	srv := &http.Server{
		Addr:              cfg.GetServerConfig().Address,
		Handler:           router,
		ReadTimeout:       cfg.GetServerConfig().ReadTimeout,
		WriteTimeout:      cfg.GetServerConfig().WriteTimeout,
		IdleTimeout:       cfg.GetServerConfig().IdleTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	return srv, security, nil
}
