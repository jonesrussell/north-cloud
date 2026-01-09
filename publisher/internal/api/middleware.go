package api

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const (
	corsMaxAgeHours = 12
)

// corsMiddleware creates a CORS middleware
func corsMiddleware(origins []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: origins,
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
