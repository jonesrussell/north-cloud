package gin

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

const internalAuthHeader = "X-Internal-Secret"

// InternalAuthMiddleware validates requests using a shared secret header.
// Used for internal service-to-service communication (not JWT).
func InternalAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provided := c.GetHeader(internalAuthHeader)
		if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid internal auth"})
			c.Abort()
			return
		}
		c.Next()
	}
}
