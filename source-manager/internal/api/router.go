package api

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/gosources/internal/config"
	"github.com/jonesrussell/gosources/internal/handlers"
	"github.com/jonesrussell/gosources/internal/logger"
	"github.com/jonesrussell/gosources/internal/repository"
)

const (
	corsMaxAgeHours = 12
)

// getCORSOrigins returns the list of allowed CORS origins from environment or config
func getCORSOrigins() []string {
	// Check environment variable first (comma-separated list)
	if corsOrigins := os.Getenv("CORS_ORIGINS"); corsOrigins != "" {
		origins := strings.Split(corsOrigins, ",")
		// Trim whitespace from each origin
		for i, origin := range origins {
			origins[i] = strings.TrimSpace(origin)
		}
		return origins
	}

	// Default origins
	origins := []string{"http://localhost:3000"}

	// If SOURCE_MANAGER_API_URL is set, extract host and add frontend origin
	if apiURL := os.Getenv("SOURCE_MANAGER_API_URL"); apiURL != "" {
		// Extract host from URL (e.g., http://localhost:8050 -> http://localhost:3000)
		if strings.HasPrefix(apiURL, "http://") || strings.HasPrefix(apiURL, "https://") {
			parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(apiURL, "http://"), "https://"), ":")
			if len(parts) > 0 {
				host := parts[0]
				origins = append(origins, "http://"+host+":3000")
			}
		}
	}

	return origins
}

func NewRouter(db *repository.SourceRepository, cfg *config.Config, log logger.Logger) *gin.Engine {
	router := gin.New()

	// CORS middleware - must be first
	router.Use(cors.New(cors.Config{
		AllowOrigins: getCORSOrigins(),
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Content-Length", "Accept-Encoding",
			"X-CSRF-Token", "Authorization", "accept", "origin",
			"Cache-Control", "X-Requested-With",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           corsMaxAgeHours * time.Hour,
	}))

	// Middleware
	router.Use(ginLogger(log))
	router.Use(gin.Recovery())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API v1
	v1 := router.Group("/api/v1")
	sourceHandler := handlers.NewSourceHandler(db, log)

	// Sources endpoints
	sources := v1.Group("/sources")
	sources.POST("", sourceHandler.Create)
	sources.POST("/fetch-metadata", sourceHandler.FetchMetadata)
	sources.GET("", sourceHandler.List)
	sources.GET("/:id", sourceHandler.GetByID)
	sources.PUT("/:id", sourceHandler.Update)
	sources.DELETE("/:id", sourceHandler.Delete)

	// Cities endpoint for gopost integration
	v1.GET("/cities", sourceHandler.GetCities)

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
