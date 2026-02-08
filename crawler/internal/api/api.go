// Package api implements the HTTP API for the crawler service.
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/admin"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
	serviceVersion      = "1.0.0"
)

// setupJobRoutes configures job-related endpoints
func setupJobRoutes(v1 *gin.RouterGroup, jobsHandler *JobsHandler) {
	if jobsHandler != nil {
		// Aggregate endpoints (before :id to avoid route conflict)
		v1.GET("/jobs/status-counts", jobsHandler.GetJobStatusCounts)

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

		// Scheduler metrics and distribution
		v1.GET("/scheduler/metrics", jobsHandler.GetSchedulerMetrics)
		v1.GET("/scheduler/distribution", jobsHandler.GetSchedulerDistribution)
		v1.POST("/scheduler/rebalance", jobsHandler.PostSchedulerRebalance)
		v1.POST("/scheduler/rebalance/preview", jobsHandler.PostSchedulerRebalancePreview)
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

// setupLogRoutes configures log streaming endpoints
func setupLogRoutes(v1 *gin.RouterGroup, logsHandler *LogsHandler, logsV2Handler *LogsStreamV2Handler) {
	if logsHandler != nil {
		v1.GET("/jobs/:id/logs", logsHandler.GetLogsMetadata)
		v1.GET("/jobs/:id/logs/stream", logsHandler.StreamLogs)
		v1.GET("/jobs/:id/logs/download", logsHandler.DownloadLogs)
		v1.GET("/jobs/:id/logs/view", logsHandler.ViewLogs)
	}
	// V2 streaming endpoint (Redis Streams-backed)
	if logsV2Handler != nil {
		v1.GET("/jobs/:id/logs/stream/v2", logsV2Handler.Stream)
	}
}

// setupMigrationRoutes configures Phase 3 migration endpoints
func setupMigrationRoutes(v1 *gin.RouterGroup, migrationHandler *MigrationHandler) {
	if migrationHandler != nil {
		v1.POST("/jobs/migrate", migrationHandler.RunMigration)
		v1.GET("/jobs/migration-stats", migrationHandler.GetStats)
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

// NewServer creates a new HTTP server using the infrastructure gin package.
func NewServer(
	cfg config.Interface,
	jobsHandler *JobsHandler,
	discoveredLinksHandler *DiscoveredLinksHandler,
	logsHandler *LogsHandler, // Optional - pass nil to disable log streaming
	logsV2Handler *LogsStreamV2Handler, // Optional - pass nil to disable v2 log streaming
	executionRepo database.ExecutionRepositoryInterface,
	infraLog infralogger.Logger,
	sseHandler *SSEHandler, // Optional - pass nil to disable SSE
	migrationHandler *MigrationHandler, // Optional - pass nil to disable migration endpoints
	syncHandler *admin.SyncEnabledSourcesHandler, // Optional - pass nil to disable sync endpoint
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
			setupCrawlerRoutes(
				router, jwtSecret, jobsHandler, discoveredLinksHandler,
				logsHandler, logsV2Handler, executionRepo, sseHandler,
				migrationHandler, syncHandler,
			)
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
	logsHandler *LogsHandler,
	logsV2Handler *LogsStreamV2Handler,
	executionRepo database.ExecutionRepositoryInterface,
	sseHandler *SSEHandler,
	migrationHandler *MigrationHandler,
	syncHandler *admin.SyncEnabledSourcesHandler,
) {
	// API v1 routes - protected with JWT
	v1 := infragin.ProtectedGroup(router, "/api/v1", jwtSecret)

	// Stats endpoint for dashboard
	v1.GET("/stats", func(c *gin.Context) {
		crawledToday := int64(0)
		indexedToday := int64(0)

		if executionRepo != nil {
			crawled, indexed, err := executionRepo.GetTodayStats(c.Request.Context())
			if err == nil {
				crawledToday = crawled
				indexedToday = indexed
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"totalArticles":   0,
			"successRate":     0,
			"avgResponseTime": 0,
			"crawled":         0,
			"failed":          0,
			"pending":         0,
			"activeSources":   0,
			"totalSources":    0,
			"crawled_today":   crawledToday,
			"indexed_today":   indexedToday,
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

	// API v2 routes (minimal: run-now only; same JWT protection)
	v2 := infragin.ProtectedGroup(router, "/api/v2", jwtSecret)
	v2.POST("/jobs/:id/force-run", jobsHandler.ForceRun)

	// Setup log routes
	setupLogRoutes(v1, logsHandler, logsV2Handler)

	// Setup discovered links routes
	setupDiscoveredLinksRoutes(v1, discoveredLinksHandler)

	// Setup migration routes (Phase 3)
	setupMigrationRoutes(v1, migrationHandler)

	// Admin: sync enabled sources to crawler jobs
	if syncHandler != nil {
		v1.POST("/admin/sync-enabled-sources", syncHandler.SyncEnabledSources)
	}

	// Setup SSE routes (protected with JWT)
	if sseHandler != nil {
		// Note: SSE endpoints use /api prefix (not /api/v1) per frontend expectations
		sseGroup := infragin.ProtectedGroup(router, "/api", jwtSecret)
		sseGroup.GET("/crawler/events", sseHandler.HandleCrawlerEvents)
		sseGroup.GET("/health/events", sseHandler.HandleHealthEvents)
		sseGroup.GET("/metrics/events", sseHandler.HandleMetricsEvents)
	}
}
