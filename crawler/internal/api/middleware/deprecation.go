// Package middleware provides HTTP middleware for the crawler API.
package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// DeprecationHeader indicates that the API version is deprecated.
	DeprecationHeader = "Deprecation"

	// SunsetHeader indicates when the API will be removed.
	SunsetHeader = "Sunset"

	// LinkHeader provides the link to the newer API version.
	LinkHeader = "Link"
)

// V1DeprecationMiddleware adds deprecation headers to V1 API responses.
// This signals to clients that they should migrate to V2 API.
func V1DeprecationMiddleware() gin.HandlerFunc {
	// Calculate sunset date (6 months from now, adjust as needed)
	// Note: In production, this should be a fixed date from configuration
	sunsetDate := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	return func(c *gin.Context) {
		// Add deprecation headers
		c.Header(DeprecationHeader, "true")
		c.Header(SunsetHeader, sunsetDate.Format(time.RFC1123))
		c.Header(LinkHeader, `</api/v2>; rel="successor-version"`)

		c.Next()
	}
}

// SchedulerV2FeatureFlag is a middleware that checks if V2 scheduler is enabled.
// When disabled, V2 endpoints return a 503 Service Unavailable.
func SchedulerV2FeatureFlag(enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "V2 scheduler is not enabled",
				"message": "Set SCHEDULER_V2_ENABLED=true to enable V2 features",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
