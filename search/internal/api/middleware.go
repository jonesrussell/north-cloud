package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/search/internal/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	httpStatusForbidden           = 403
	httpStatusNoContent           = 204
	httpStatusInternalServerError = 500
)

// LoggerMiddleware logs HTTP requests
func LoggerMiddleware(log infralogger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Log request
		log.Info("HTTP request",
			infralogger.String("method", c.Request.Method),
			infralogger.String("path", c.Request.URL.Path),
			infralogger.Int("status", c.Writer.Status()),
			infralogger.Int64("duration_ms", duration.Milliseconds()),
			infralogger.String("client_ip", c.ClientIP()),
			infralogger.String("user_agent", c.Request.UserAgent()),
		)
	}
}

// CORSMiddleware handles CORS
func CORSMiddleware(cfg *config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		// Set CORS headers
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		// Check if origin is allowed
		if !isOriginAllowed(origin, cfg.AllowedOrigins) {
			c.AbortWithStatus(httpStatusForbidden)
			return
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", boolToString(cfg.AllowCredentials))
		c.Writer.Header().Set("Access-Control-Allow-Methods", joinStrings(cfg.AllowedMethods, ", "))
		c.Writer.Header().Set("Access-Control-Allow-Headers", joinStrings(cfg.AllowedHeaders, ", "))
		c.Writer.Header().Set("Access-Control-Max-Age", intToString(cfg.MaxAge))

		// Handle preflight requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(httpStatusNoContent)
			return
		}

		c.Next()
	}
}

// RecoveryMiddleware handles panics
func RecoveryMiddleware(log infralogger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error("Panic recovered",
					infralogger.Any("error", err),
					infralogger.String("path", c.Request.URL.Path),
					infralogger.String("method", c.Request.Method),
				)

				c.JSON(httpStatusInternalServerError, ErrorResponse{
					Error:     "Internal server error",
					Code:      "INTERNAL_ERROR",
					Timestamp: time.Now(),
				})
			}
		}()

		c.Next()
	}
}

// isOriginAllowed checks if an origin is in the allowed list
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// Helper functions
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func intToString(i int) string {
	return strconv.Itoa(i)
}

func joinStrings(strs []string, sep string) string {
	result := ""
	var resultSb117 strings.Builder
	for i, str := range strs {
		if i > 0 {
			resultSb117.WriteString(sep)
		}
		resultSb117.WriteString(str)
	}
	result += resultSb117.String()
	return result
}
