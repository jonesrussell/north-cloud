package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/handlers"
	"github.com/jonesrussell/north-cloud/source-manager/internal/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Constants for router configuration.
const (
	corsMaxAgeHours     = 12
	defaultReadTimeout  = 30 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
	serviceVersion      = "1.0.0"
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

// NewServer creates a new HTTP server using the infrastructure gin package.
func NewServer(
	db *repository.SourceRepository,
	cfg *config.Config,
	_ logger.Logger,
	infraLog infralogger.Logger,
) *infragin.Server {
	sourceHandler := handlers.NewSourceHandler(db, wrapInfraLogger(infraLog))

	// Build CORS config
	corsConfig := infragin.CORSConfig{
		Enabled:          true,
		AllowedOrigins:   getCORSOrigins(cfg),
		AllowCredentials: true,
		MaxAge:           corsMaxAgeHours * time.Hour,
	}

	// Build server using infrastructure gin package
	server := infragin.NewServerBuilder("source-manager", cfg.Server.Port).
		WithLogger(infraLog).
		WithDebug(cfg.Debug).
		WithVersion(serviceVersion).
		WithTimeouts(defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout).
		WithCORS(corsConfig).
		WithRoutes(func(router *gin.Engine) {
			// HEAD /health for Docker health checks
			router.HEAD("/health", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			// Setup service-specific routes (health routes added by builder)
			setupServiceRoutes(router, sourceHandler, cfg)
		}).
		Build()

	return server
}

// setupServiceRoutes configures service-specific API routes (not health routes).
// Health routes are handled by the infrastructure gin package.
func setupServiceRoutes(router *gin.Engine, sourceHandler *handlers.SourceHandler, cfg *config.Config) {
	// Public API endpoints (no JWT required) - for internal service-to-service communication
	publicAPI := router.Group("/api/v1")
	// GET /api/v1/sources - allow crawler to list sources without auth
	publicAPI.GET("/sources", sourceHandler.List)
	// GET /api/v1/cities - allow publisher to get cities without auth
	publicAPI.GET("/cities", sourceHandler.GetCities)

	// Protected API endpoints (JWT required) - for dashboard and authenticated users
	v1 := infragin.ProtectedGroup(router, "/api/v1", cfg.Auth.JWTSecret)

	// Sources endpoints (protected - requires JWT)
	sources := v1.Group("/sources")
	sources.POST("", sourceHandler.Create)
	sources.POST("/fetch-metadata", sourceHandler.FetchMetadata)
	sources.POST("/test-crawl", sourceHandler.TestCrawl)
	sources.GET("/:id", sourceHandler.GetByID)
	sources.PUT("/:id", sourceHandler.Update)
	sources.DELETE("/:id", sourceHandler.Delete)
}

// wrapInfraLogger wraps infrastructure logger to match the service Logger interface.
// This is needed because handlers expect the local Logger interface.
func wrapInfraLogger(log infralogger.Logger) logger.Logger {
	return &infraLoggerWrapper{log: log}
}

// infraLoggerWrapper wraps infrastructure logger to implement local Logger interface.
type infraLoggerWrapper struct {
	log infralogger.Logger
}

func (w *infraLoggerWrapper) Debug(msg string, fields ...logger.Field) {
	w.log.Debug(msg, convertFields(fields)...)
}

func (w *infraLoggerWrapper) Info(msg string, fields ...logger.Field) {
	w.log.Info(msg, convertFields(fields)...)
}

func (w *infraLoggerWrapper) Warn(msg string, fields ...logger.Field) {
	w.log.Warn(msg, convertFields(fields)...)
}

func (w *infraLoggerWrapper) Error(msg string, fields ...logger.Field) {
	w.log.Error(msg, convertFields(fields)...)
}

func (w *infraLoggerWrapper) With(fields ...logger.Field) logger.Logger {
	return &infraLoggerWrapper{log: w.log.With(convertFields(fields)...)}
}

func (w *infraLoggerWrapper) Sync() error {
	return w.log.Sync()
}

// convertFields converts local logger.Field to infrastructure logger.Field.
// Since both use zap.Field (zapcore.Field), this is a no-op type conversion.
func convertFields(fields []logger.Field) []infralogger.Field {
	result := make([]infralogger.Field, len(fields))
	copy(result, fields)
	return result
}

// NewRouter is kept for backward compatibility but marked as deprecated.
//
// Deprecated: Use NewServer() instead which includes middleware setup.
func NewRouter(db *repository.SourceRepository, cfg *config.Config, log logger.Logger) *gin.Engine {
	router := gin.New()

	// CORS middleware - must be first
	router.Use(corsMiddleware(getCORSOrigins(cfg)))

	// Middleware
	router.Use(ginLogger(log))
	router.Use(gin.Recovery())

	// Health check - support both GET and HEAD for Docker healthchecks
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.HEAD("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	sourceHandler := handlers.NewSourceHandler(db, log)

	// Public API endpoints (no JWT required)
	publicAPI := router.Group("/api/v1")
	publicAPI.GET("/sources", sourceHandler.List)
	publicAPI.GET("/cities", sourceHandler.GetCities)

	// Protected API endpoints (JWT required)
	v1 := infragin.ProtectedGroup(router, "/api/v1", cfg.Auth.JWTSecret)

	sources := v1.Group("/sources")
	sources.POST("", sourceHandler.Create)
	sources.POST("/fetch-metadata", sourceHandler.FetchMetadata)
	sources.POST("/test-crawl", sourceHandler.TestCrawl)
	sources.GET("/:id", sourceHandler.GetByID)
	sources.PUT("/:id", sourceHandler.Update)
	sources.DELETE("/:id", sourceHandler.Delete)

	return router
}

// corsMiddleware creates a CORS middleware with the given origins.
func corsMiddleware(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowedOrigin := ""

		for _, allowed := range origins {
			if allowed == "*" {
				allowedOrigin = "*"
				break
			}
			if allowed == origin {
				allowedOrigin = origin
				break
			}
		}

		if allowedOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Writer.Header().Set("Access-Control-Allow-Headers",
				"Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-API-Key")
			c.Writer.Header().Set("Access-Control-Max-Age", "43200") // 12 hours
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
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
