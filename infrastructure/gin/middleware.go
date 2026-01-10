package gin

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/north-cloud/infrastructure/logger"
)

// Buffer size constants for middleware operations.
const (
	maxAgeBufSize    = 10 // Buffer size for max age string conversion
	requestIDBufSize = 16 // Buffer size for request ID (hex timestamp)
)

// LoggerMiddleware creates a Gin middleware for structured HTTP request logging.
// It logs method, path, status, duration, and client IP using the infrastructure logger.
func LoggerMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		method := c.Request.Method

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		// Build log fields
		fields := []logger.Field{
			logger.String("method", method),
			logger.String("path", path),
			logger.Int("status", statusCode),
			logger.Duration("duration", duration),
			logger.String("client_ip", c.ClientIP()),
		}

		// Add query if present
		if query != "" {
			fields = append(fields, logger.String("query", query))
		}

		// Add user agent for non-health endpoints
		if !strings.HasPrefix(path, "/health") {
			fields = append(fields, logger.String("user_agent", c.Request.UserAgent()))
		}

		// Add error information to the single log entry (avoid double-logging)
		if len(c.Errors) > 0 {
			errorMessages := make([]string, len(c.Errors))
			for i, err := range c.Errors {
				errorMessages[i] = err.Err.Error()
			}
			fields = append(fields, logger.Strings("errors", errorMessages))
		}

		// Log the request once with all context
		if len(c.Errors) > 0 {
			log.Error("HTTP request with errors", fields...)
		} else {
			log.Info("HTTP request", fields...)
		}
	}
}

// CORSMiddleware creates a Gin middleware for handling Cross-Origin Resource Sharing.
// It supports configurable origins, methods, and headers.
func CORSMiddleware(cfg CORSConfig) gin.HandlerFunc {
	// Apply defaults if not set
	cfg.SetDefaults()

	// Pre-compute joined strings for headers
	allowedMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowedHeaders := strings.Join(cfg.AllowedHeaders, ", ")
	allowCredentials := "false"
	if cfg.AllowCredentials {
		allowCredentials = "true"
	}

	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")

		// Determine the allowed origin to return
		allowedOrigin := determineAllowedOrigin(origin, cfg.AllowedOrigins)
		if allowedOrigin == "" {
			// Origin not allowed, continue without CORS headers
			c.Next()
			return
		}

		// Set CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", allowCredentials)
		c.Writer.Header().Set("Access-Control-Allow-Methods", allowedMethods)
		c.Writer.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		c.Writer.Header().Set("Access-Control-Max-Age", formatMaxAge(cfg.MaxAge))

		// Handle preflight requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// determineAllowedOrigin checks if the origin is in the allowed list.
// Returns the origin to use in the response, or empty string if not allowed.
func determineAllowedOrigin(origin string, allowedOrigins []string) string {
	// If no origin header, allow the request (same-origin)
	if origin == "" {
		return "*"
	}

	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return "*"
		}
		if allowed == origin {
			return origin
		}
	}

	return ""
}

// formatMaxAge converts a duration to seconds string for the Max-Age header.
func formatMaxAge(d time.Duration) string {
	seconds := int(d.Seconds())
	// Use a simple conversion to avoid importing strconv
	if seconds <= 0 {
		return "0"
	}

	// Convert to string manually to avoid import
	result := make([]byte, 0, maxAgeBufSize)
	for seconds > 0 {
		result = append([]byte{byte('0' + seconds%10)}, result...)
		seconds /= 10
	}
	return string(result)
}

// RecoveryMiddleware creates a Gin middleware for panic recovery with logging.
// It catches panics, logs them with the infrastructure logger, and returns a 500 error.
func RecoveryMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				log.Error("Panic recovered",
					logger.Any("error", err),
					logger.String("path", c.Request.URL.Path),
					logger.String("method", c.Request.Method),
					logger.String("client_ip", c.ClientIP()),
				)

				// Return 500 error
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":   "Internal server error",
					"code":    "INTERNAL_ERROR",
					"message": "An unexpected error occurred",
				})
			}
		}()

		c.Next()
	}
}

// RequestIDMiddleware adds a unique request ID to each request context.
// The ID is either taken from X-Request-ID header or generated.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		c.Next()
	}
}

// generateRequestID creates a simple unique request ID.
// Uses timestamp + random component for uniqueness.
func generateRequestID() string {
	// Simple timestamp-based ID
	// Format: unix_nano as hex
	now := time.Now().UnixNano()
	const hexDigits = "0123456789abcdef"
	result := make([]byte, requestIDBufSize)
	for i := requestIDBufSize - 1; i >= 0; i-- {
		result[i] = hexDigits[now&0xf]
		now >>= 4
	}
	return string(result)
}
