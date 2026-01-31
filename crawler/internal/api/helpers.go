// Package api implements the HTTP API for the crawler service.
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseLimitOffset parses limit and offset query params with defaults.
func parseLimitOffset(c *gin.Context, defaultLimit, defaultOffset int) (limit, offset int) {
	limitStr := c.DefaultQuery("limit", strconv.Itoa(defaultLimit))
	offsetStr := c.DefaultQuery("offset", strconv.Itoa(defaultOffset))
	limit, _ = strconv.Atoi(limitStr)
	offset, _ = strconv.Atoi(offsetStr)
	if limit <= 0 {
		limit = defaultLimit
	}
	if offset < 0 {
		offset = defaultOffset
	}
	return limit, offset
}

// respondError sends a JSON error response.
func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

// respondNotFound sends a 404 with resource not found message.
func respondNotFound(c *gin.Context, resource string) {
	respondError(c, http.StatusNotFound, resource+" not found")
}

// respondBadRequest sends a 400 with message.
func respondBadRequest(c *gin.Context, message string) {
	respondError(c, http.StatusBadRequest, message)
}

// respondInternalError sends a 500 with message.
func respondInternalError(c *gin.Context, message string) {
	respondError(c, http.StatusInternalServerError, message)
}
