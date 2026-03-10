package api

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/events"
	"github.com/jonesrussell/north-cloud/source-manager/internal/handlers"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
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
	communityRepo *repository.CommunityRepository,
	personRepo *repository.PersonRepository,
	bandOfficeRepo *repository.BandOfficeRepository,
	cfg *config.Config,
	infraLog infralogger.Logger,
	publisher *events.Publisher,
) *infragin.Server {
	sourceHandler := handlers.NewSourceHandler(db, infraLog, publisher)
	communityHandler := handlers.NewCommunityHandler(communityRepo, infraLog)
	personHandler := handlers.NewPersonHandler(personRepo, infraLog)
	bandOfficeHandler := handlers.NewBandOfficeHandler(bandOfficeRepo, infraLog)
	linkerHandler := handlers.NewLinkerHandler(communityRepo, db, infraLog)

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
			// Setup service-specific routes (health routes added by builder)
			setupServiceRoutes(router, sourceHandler, communityHandler, personHandler, bandOfficeHandler, linkerHandler, cfg)
		}).
		Build()

	return server
}

// setupServiceRoutes configures service-specific API routes (not health routes).
// Health routes are handled by the infrastructure gin package.
func setupServiceRoutes(
	router *gin.Engine,
	sourceHandler *handlers.SourceHandler,
	communityHandler *handlers.CommunityHandler,
	personHandler *handlers.PersonHandler,
	bandOfficeHandler *handlers.BandOfficeHandler,
	linkerHandler *handlers.LinkerHandler,
	cfg *config.Config,
) {
	// Public API endpoints (no JWT required) - for internal service-to-service communication
	publicAPI := router.Group("/api/v1")
	// GET /api/v1/sources - allow crawler to list sources without auth
	publicAPI.GET("/sources", sourceHandler.List)
	// GET /api/v1/sources/indigenous - sources with indigenous_region set
	publicAPI.GET("/sources/indigenous", sourceHandler.ListIndigenous)
	// GET /api/v1/cities - allow publisher to get cities without auth
	publicAPI.GET("/cities", sourceHandler.GetCities)

	// Communities — public read endpoints (service-to-service, Minoo sync)
	publicCommunities := publicAPI.Group("/communities")
	publicCommunities.GET("", communityHandler.List)
	publicCommunities.GET("/nearby", communityHandler.Nearby)
	publicCommunities.GET("/by-slug/:slug", communityHandler.GetBySlug)
	publicCommunities.GET("/:id", communityHandler.GetByID)

	// Protected API endpoints (JWT required) - for dashboard and authenticated users
	v1 := infragin.ProtectedGroup(router, "/api/v1", cfg.Auth.JWTSecret)

	// Sources endpoints (protected - requires JWT)
	sources := v1.Group("/sources")
	sources.POST("", sourceHandler.Create)
	sources.POST("/fetch-metadata", sourceHandler.FetchMetadata)
	sources.POST("/test-crawl", sourceHandler.TestCrawl)
	sources.POST("/import-excel", sourceHandler.ImportExcel)
	sources.POST("/import-indigenous", sourceHandler.ImportIndigenous)
	sources.GET("/by-identity", sourceHandler.GetByIdentityKey)
	sources.GET("/:id", sourceHandler.GetByID)
	sources.PUT("/:id", sourceHandler.Update)
	sources.DELETE("/:id", sourceHandler.Delete)
	sources.PATCH("/:id/feed-disable", sourceHandler.DisableFeed)
	sources.PATCH("/:id/feed-enable", sourceHandler.EnableFeed)

	// Communities endpoints (protected - requires JWT for mutations)
	communities := v1.Group("/communities")
	communities.POST("", communityHandler.Create)
	communities.POST("/link-sources", linkerHandler.LinkSources)
	communities.PUT("/:id", communityHandler.Update)
	communities.DELETE("/:id", communityHandler.Delete)

	// People — public read endpoints (nested under communities)
	publicCommunities.GET("/:id/people", personHandler.ListByCommunity)
	publicCommunities.GET("/:id/band-office", bandOfficeHandler.GetByCommunity)

	// People — public by-ID lookup
	publicAPI.GET("/people/:id", personHandler.GetByID)

	// People — protected mutation endpoints
	communities.POST("/:id/people", personHandler.Create)
	communities.POST("/:id/band-office", bandOfficeHandler.Upsert)

	// People/band-office direct mutations (protected)
	v1.PUT("/people/:id", personHandler.Update)
	v1.DELETE("/people/:id", personHandler.Delete)
	v1.PUT("/band-offices/:id", bandOfficeHandler.Update)
}
