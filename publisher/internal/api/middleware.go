package api

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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

	// Default origins - include dashboard frontend
	origins := []string{
		"http://localhost:3000", // Legacy dashboard frontend
		"http://localhost:3001", // Crawler frontend
		"http://localhost:3002", // Unified dashboard frontend
	}

	return origins
}

// corsMiddleware creates a CORS middleware
func corsMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: getCORSOrigins(),
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Content-Length", "Accept-Encoding",
			"X-CSRF-Token", "Authorization", "accept", "origin",
			"Cache-Control", "X-Requested-With", "X-API-Key",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           corsMaxAgeHours * time.Hour,
	})
}
