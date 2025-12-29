// Package api implements the HTTP API for the crawler service.
package api

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/api/middleware"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	infrajwt "github.com/north-cloud/infrastructure/jwt"
)

const (
	readHeaderTimeout = 10 * time.Second // Timeout for reading headers
)

// SetupRouter creates and configures the Gin router with all routes
func SetupRouter(
	log logger.Interface,
	cfg config.Interface,
	jobsHandler *JobsHandler,
	queuedLinksHandler *QueuedLinksHandler,
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

	// API v1 routes (for dashboard frontend) - protected with JWT
	v1 := router.Group("/api/v1")
	// Add JWT middleware if JWT secret is configured
	if jwtSecret := os.Getenv("AUTH_JWT_SECRET"); jwtSecret != "" {
		v1.Use(infrajwt.Middleware(jwtSecret))
	}
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
	if jobsHandler != nil {
		// Basic CRUD
		v1.GET("/jobs", jobsHandler.ListJobs)
		v1.POST("/jobs", jobsHandler.CreateJob)
		v1.GET("/jobs/:id", jobsHandler.GetJob)
		v1.PUT("/jobs/:id", jobsHandler.UpdateJob)
		v1.DELETE("/jobs/:id", jobsHandler.DeleteJob)

		// Job control operations (new)
		v1.POST("/jobs/:id/pause", jobsHandler.PauseJob)
		v1.POST("/jobs/:id/resume", jobsHandler.ResumeJob)
		v1.POST("/jobs/:id/cancel", jobsHandler.CancelJob)
		v1.POST("/jobs/:id/retry", jobsHandler.RetryJob)

		// Job execution history (new)
		v1.GET("/jobs/:id/executions", jobsHandler.GetJobExecutions)
		v1.GET("/jobs/:id/stats", jobsHandler.GetJobStats)
		v1.GET("/executions/:id", jobsHandler.GetExecution)

		// Scheduler metrics (new)
		v1.GET("/scheduler/metrics", jobsHandler.GetSchedulerMetrics)
	} else {
		// Fallback to placeholder endpoints if no handler provided
		v1.GET("/jobs", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"jobs": []gin.H{},
			})
		})
		v1.POST("/jobs", func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{
				"id":      "job-1",
				"status":  "pending",
				"message": "Job created successfully",
			})
		})
	}

	// Articles endpoint for dashboard
	v1.GET("/articles", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"articles": []gin.H{},
		})
	})

	// Queued links endpoints for dashboard
	if queuedLinksHandler != nil {
		v1.GET("/queued-links", queuedLinksHandler.ListQueuedLinks)
		v1.GET("/queued-links/:id", queuedLinksHandler.GetQueuedLink)
		v1.DELETE("/queued-links/:id", queuedLinksHandler.DeleteQueuedLink)
		v1.POST("/queued-links/:id/create-job", queuedLinksHandler.CreateJobFromLink)
	}

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

// StartHTTPServer starts the HTTP server with the given configuration
func StartHTTPServer(
	log logger.Interface,
	cfg config.Interface,
	jobsHandler *JobsHandler,
	queuedLinksHandler *QueuedLinksHandler,
) (*http.Server, middleware.SecurityMiddlewareInterface, error) {
	router, security := SetupRouter(log, cfg, jobsHandler, queuedLinksHandler)

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
