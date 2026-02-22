package gin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/north-cloud/infrastructure/logger"
)

// Buffer size constants for middleware operations.
const (
	maxAgeBufSize      = 10  // Buffer size for max age string conversion
	requestIDByteLen   = 16  // Number of random bytes for request ID (produces 32 hex chars)
	maxRequestIDLength = 128 // Maximum length for inbound X-Request-ID header
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

		// Include request ID if present
		if reqID, exists := c.Get("request_id"); exists {
			if id, ok := reqID.(string); ok {
				fields = append(fields, logger.String("request_id", id))
			}
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
//
// Deprecated: Use RequestIDLoggerMiddleware instead, which also stores
// a request-scoped logger in the Go context.
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

// RequestIDLoggerMiddleware generates a request ID and stores a request-scoped
// logger (with request_id field) in both the Gin context and the Go context.
// This allows downstream handlers to retrieve an enriched logger via
// logger.FromContext(c.Request.Context()).
func RequestIDLoggerMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" || len(requestID) > maxRequestIDLength {
			requestID = generateRequestID()
		}

		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		// Store logger with request_id in Go context so downstream handlers
		// can retrieve it via logger.FromContext(c.Request.Context())
		reqLog := log.With(logger.String("request_id", requestID))
		ctx := logger.WithContext(c.Request.Context(), reqLog)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// generateRequestID creates a unique request ID using cryptographic randomness.
func generateRequestID() string {
	b := make([]byte, requestIDByteLen)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp if crypto/rand fails (should never happen)
		now := time.Now().UnixNano()
		for i := requestIDByteLen - 1; i >= 0; i-- {
			b[i] = byte(now)
			now >>= 8
		}
	}
	return hex.EncodeToString(b)
}
