package jwt

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents JWT claims
type Claims struct {
	Sub string `json:"sub"`
	jwt.RegisteredClaims
}

// Middleware creates a JWT authentication middleware
func Middleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip auth for health check endpoints
		if c.Request.URL.Path == "/health" || strings.HasPrefix(c.Request.URL.Path, "/health/") {
			c.Next()
			return
		}

		// Extract token from Authorization header or query parameter
		// Query parameter is needed for SSE (EventSource) which can't set custom headers
		tokenString := extractToken(c)
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization"})
			c.Abort()
			return
		}

		// Parse and validate token
		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return []byte(secret), nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(*Claims); ok && token.Valid {
			// Store claims in context for use in handlers
			c.Set("claims", claims)
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
	}
}

// GetClaims extracts claims from the gin context
func GetClaims(c *gin.Context) (*Claims, bool) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, false
	}

	cl, ok := claims.(*Claims)
	return cl, ok
}

// extractToken extracts JWT token from Authorization header or query parameter.
// Returns empty string if no valid token found.
func extractToken(c *gin.Context) string {
	// Try Authorization header first (Bearer token)
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
		// Invalid header format - return empty to trigger error
		return ""
	}

	// Fallback to query parameter for SSE endpoints (EventSource can't set headers)
	return c.Query("token")
}
