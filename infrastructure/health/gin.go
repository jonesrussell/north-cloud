package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GinHandler returns a Gin handler for the health check endpoint.
func (c *Checker) GinHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		checkCtx, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
		defer cancel()

		status, results := c.Check(checkCtx)

		response := gin.H{
			"status":    status,
			"checks":    results,
			"timestamp": time.Now().Format(time.RFC3339),
		}

		statusCode := http.StatusOK
		if status == StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		}

		ctx.JSON(statusCode, response)
	}
}

// GinLivenessHandler returns a simple Gin liveness handler.
func GinLivenessHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status": "alive",
		})
	}
}

// GinReadinessHandler returns a Gin readiness handler using the checker.
func GinReadinessHandler(checker *Checker) gin.HandlerFunc {
	return checker.GinHandler()
}

// SimpleGinHandler returns a simple health handler that always returns healthy.
// Use this when no specific health checks are needed.
func SimpleGinHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	}
}

// RegisterRoutes registers health check routes on a Gin router.
func RegisterRoutes(router *gin.Engine, checker *Checker) {
	router.GET("/health", checker.GinHandler())
	router.GET("/health/live", GinLivenessHandler())
	router.GET("/health/ready", GinReadinessHandler(checker))
}

// RegisterSimpleRoutes registers simple health routes (always healthy).
func RegisterSimpleRoutes(router *gin.Engine) {
	router.GET("/health", SimpleGinHandler())
}
