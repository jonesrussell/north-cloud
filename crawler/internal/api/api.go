// Package api implements the HTTP API for the crawler service.
package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/api/middleware"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	infragin "github.com/north-cloud/infrastructure/gin"
	infrajwt "github.com/north-cloud/infrastructure/jwt"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/monitoring"
)

const (
	readHeaderTimeout   = 10 * time.Second // Timeout for reading headers
	hoursPerDay         = 24               // Hours in a day
	minutesPerHour      = 60               // Minutes in an hour
	secondsPerMinute    = 60               // Seconds in a minute
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
	serviceVersion      = "1.0.0"
)

// SetupRouter creates and configures the Gin router with all routes
func SetupRouter(
	log logger.Interface,
	cfg config.Interface,
	jobsHandler *JobsHandler,
	discoveredLinksHandler *DiscoveredLinksHandler,
) (*gin.Engine, middleware.SecurityMiddlewareInterface) {
	// Disable Gin's default logging
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware(log))
	router.Use(corsMiddleware()) // Add CORS middleware

	// Create security middleware
	security := middleware.NewSecurityMiddleware(cfg.GetServerConfig(), log)

	// Track server start time for uptime calculation
	startTime := time.Now()
	version := "1.0.0" // Default version, can be overridden by config

	// Define public routes
	setupPublicRoutes(router, startTime, version)

	// API v1 routes (for dashboard frontend) - protected with JWT
	v1 := setupV1Routes(router, cfg)

	// Setup job routes
	setupJobRoutes(v1, jobsHandler)

	// Setup discovered links routes
	setupDiscoveredLinksRoutes(v1, discoveredLinksHandler)

	return router, security
}

// setupPublicRoutes configures public routes (no authentication required)
func setupPublicRoutes(router *gin.Engine, startTime time.Time, version string) {
	router.GET("/health", func(c *gin.Context) {
		uptime := time.Since(startTime)
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": version,
			"uptime":  formatUptime(uptime),
		})
	})

	// Memory health endpoint
	router.GET("/health/memory", func(c *gin.Context) {
		monitoring.MemoryHealthHandler(c.Writer, c.Request)
	})
}

// setupV1Routes configures API v1 routes with JWT middleware
func setupV1Routes(router *gin.Engine, cfg config.Interface) *gin.RouterGroup {
	v1 := router.Group("/api/v1")
	// Add JWT middleware if JWT secret is configured
	authCfg := cfg.GetAuthConfig()
	if authCfg != nil && authCfg.JWTSecret != "" {
		v1.Use(infrajwt.Middleware(authCfg.JWTSecret))
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

	// Articles endpoint for dashboard
	v1.GET("/articles", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"articles": []gin.H{},
		})
	})

	return v1
}

// setupJobRoutes configures job-related endpoints
func setupJobRoutes(v1 *gin.RouterGroup, jobsHandler *JobsHandler) {
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
}

// setupDiscoveredLinksRoutes configures discovered links endpoints
func setupDiscoveredLinksRoutes(v1 *gin.RouterGroup, discoveredLinksHandler *DiscoveredLinksHandler) {
	if discoveredLinksHandler != nil {
		v1.GET("/discovered-links", discoveredLinksHandler.ListDiscoveredLinks)
		v1.GET("/discovered-links/:id", discoveredLinksHandler.GetDiscoveredLink)
		v1.DELETE("/discovered-links/:id", discoveredLinksHandler.DeleteDiscoveredLink)
		v1.POST("/discovered-links/:id/create-job", discoveredLinksHandler.CreateJobFromLink)
	} else {
		// Fallback to placeholder endpoints if no handler provided
		v1.GET("/discovered-links", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"links": []gin.H{},
				"total": 0,
			})
		})
		v1.GET("/discovered-links/:id", func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Discovered links handler not available",
			})
		})
		v1.DELETE("/discovered-links/:id", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Discovered links handler not available",
			})
		})
		v1.POST("/discovered-links/:id/create-job", func(c *gin.Context) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Discovered links handler not available",
			})
		})
	}
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

// formatUptime formats a duration as a human-readable uptime string
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / hoursPerDay
	hours := int(d.Hours()) % hoursPerDay
	minutes := int(d.Minutes()) % minutesPerHour
	seconds := int(d.Seconds()) % secondsPerMinute

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
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
//
// Deprecated: Use NewServer() instead which uses the infrastructure gin package.
func StartHTTPServer(
	log logger.Interface,
	cfg config.Interface,
	jobsHandler *JobsHandler,
	discoveredLinksHandler *DiscoveredLinksHandler,
) (*http.Server, middleware.SecurityMiddlewareInterface, error) {
	router, security := SetupRouter(log, cfg, jobsHandler, discoveredLinksHandler)

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

// NewServer creates a new HTTP server using the infrastructure gin package.
func NewServer(
	_ logger.Interface,
	cfg config.Interface,
	jobsHandler *JobsHandler,
	discoveredLinksHandler *DiscoveredLinksHandler,
	infraLog infralogger.Logger,
) *infragin.Server {
	// Extract port from address
	port := extractPortFromAddress(cfg.GetServerConfig().Address)

	// Get JWT secret
	var jwtSecret string
	authCfg := cfg.GetAuthConfig()
	if authCfg != nil {
		jwtSecret = authCfg.JWTSecret
	}

	// Determine debug mode from logging config
	debug := false
	loggingCfg := cfg.GetLoggingConfig()
	if loggingCfg != nil {
		debug = loggingCfg.Debug
	}

	// Build server using infrastructure gin package
	server := infragin.NewServerBuilder("crawler", port).
		WithLogger(infraLog).
		WithDebug(debug).
		WithVersion(serviceVersion).
		WithTimeouts(defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout).
		WithRoutes(func(router *gin.Engine) {
			// Setup service-specific routes (health routes added by builder)
			setupCrawlerRoutes(router, jwtSecret, jobsHandler, discoveredLinksHandler)
		}).
		Build()

	return server
}

// extractPortFromAddress extracts the port number from an address string.
func extractPortFromAddress(address string) int {
	const (
		defaultPort = 8060
		decimalBase = 10
	)
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

// setupCrawlerRoutes configures all service-specific routes.
// Health routes are handled by the infrastructure gin package.
func setupCrawlerRoutes(
	router *gin.Engine,
	jwtSecret string,
	jobsHandler *JobsHandler,
	discoveredLinksHandler *DiscoveredLinksHandler,
) {
	// API v1 routes - protected with JWT
	v1 := infragin.ProtectedGroup(router, "/api/v1", jwtSecret)

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

	// Articles endpoint for dashboard
	v1.GET("/articles", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"articles": []gin.H{},
		})
	})

	// Setup job routes
	setupJobRoutes(v1, jobsHandler)

	// Setup discovered links routes
	setupDiscoveredLinksRoutes(v1, discoveredLinksHandler)
}
