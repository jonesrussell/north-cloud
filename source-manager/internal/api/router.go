package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/handlers"
	"github.com/jonesrussell/north-cloud/source-manager/internal/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	infrajwt "github.com/north-cloud/infrastructure/jwt"
	"github.com/north-cloud/infrastructure/monitoring"
)

const (
	corsMaxAgeHours = 12
)

// getCORSOrigins returns the list of allowed CORS origins from config, with dynamic origins based on API URL
func getCORSOrigins(cfg *config.Config) []string {
	origins := make([]string, 0, len(cfg.Server.CORSOrigins))
	// Use CORS origins from config
	origins = append(origins, cfg.Server.CORSOrigins...)

	// If SOURCE_MANAGER_API_URL is set, extract host and add frontend origins dynamically
	if cfg.Server.APIURL == "" {
		return origins
	}

	// Extract host from URL (e.g., http://localhost:8050 -> http://localhost:3000)
	host := extractHostFromURL(cfg.Server.APIURL)
	if host == "" {
		return origins
	}

	// Add dynamic origins if not already present
	dynamicOrigins := []string{
		"http://" + host + ":3000",
		"http://" + host + ":3001",
		"http://" + host + ":3002",
	}

	for _, dynOrigin := range dynamicOrigins {
		if !contains(origins, dynOrigin) {
			origins = append(origins, dynOrigin)
		}
	}

	return origins
}

// extractHostFromURL extracts the host from a URL string
func extractHostFromURL(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return ""
	}

	// Remove protocol prefix
	withoutProtocol := strings.TrimPrefix(strings.TrimPrefix(url, "http://"), "https://")
	parts := strings.Split(withoutProtocol, ":")
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

// contains checks if a string slice contains a specific string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func NewRouter(db *repository.SourceRepository, cfg *config.Config, log logger.Logger) *gin.Engine {
	router := gin.New()

	// CORS middleware - must be first
	router.Use(cors.New(cors.Config{
		AllowOrigins: getCORSOrigins(cfg),
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Content-Length", "Accept-Encoding",
			"X-CSRF-Token", "Authorization", "accept", "origin",
			"Cache-Control", "X-Requested-With", "X-API-Key",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           corsMaxAgeHours * time.Hour,
	}))

	// Middleware
	router.Use(ginLogger(log))
	router.Use(gin.Recovery())

	// Health check - support both GET and HEAD for Docker healthchecks
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Memory health endpoint
	router.GET("/health/memory", func(c *gin.Context) {
		monitoring.MemoryHealthHandler(c.Writer, c.Request)
	})
	router.HEAD("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	sourceHandler := handlers.NewSourceHandler(db, log)

	// Public API endpoints (no JWT required) - for internal service-to-service communication
	// These are registered directly on the router, not in a group with JWT middleware
	publicAPI := router.Group("/api/v1")
	// GET /api/v1/sources - allow crawler to list sources without auth
	publicAPI.GET("/sources", sourceHandler.List)
	// GET /api/v1/cities - allow publisher to get cities without auth
	publicAPI.GET("/cities", sourceHandler.GetCities)

	// Protected API endpoints (JWT required) - for dashboard and authenticated users
	v1 := router.Group("/api/v1")
	// Add JWT middleware if JWT secret is configured
	if cfg.Auth.JWTSecret != "" {
		v1.Use(infrajwt.Middleware(cfg.Auth.JWTSecret))
	}

	// Sources endpoints (protected - requires JWT)
	sources := v1.Group("/sources")
	sources.POST("", sourceHandler.Create)
	sources.POST("/fetch-metadata", sourceHandler.FetchMetadata)
	sources.POST("/test-crawl", sourceHandler.TestCrawl)
	sources.GET("/:id", sourceHandler.GetByID)
	sources.PUT("/:id", sourceHandler.Update)
	sources.DELETE("/:id", sourceHandler.Delete)

	return router
}

func ginLogger(log logger.Logger) gin.HandlerFunc {
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
			logger.Int("status_code", statusCode),
			logger.String("client_ip", c.ClientIP()),
			logger.Duration("duration", duration),
		)
	}
}
