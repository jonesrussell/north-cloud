// Package api implements the HTTP API for the crawler service.
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseLimitOffset parses limit and offset query params with defaults.
//
//nolint:unparam // defaultLimit varies by caller intent; current callers happen to share a value.
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

// MaxPageSize is the maximum allowed page size for list endpoints.
const MaxPageSize = 250

// parseSortParams parses sort_by and sort_order query params with validation.
// allowedFields maps external names to internal column names/expressions.
// Returns the internal column name and normalized sort order.
func parseSortParams(
	c *gin.Context,
	allowedFields map[string]string,
	defaultSortBy, defaultOrder string,
) (sortBy, sortOrder string) {
	sortBy = c.DefaultQuery("sort_by", defaultSortBy)
	sortOrder = c.DefaultQuery("sort_order", defaultOrder)

	// Validate sort field against whitelist
	if column, ok := allowedFields[sortBy]; ok {
		sortBy = column
	} else {
		sortBy = allowedFields[defaultSortBy]
		if sortBy == "" {
			sortBy = defaultSortBy
		}
	}

	// Normalize sort order
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = defaultOrder
	}

	return sortBy, sortOrder
}

// clampLimit ensures limit is within valid bounds.
func clampLimit(limit, maxLimit int) int {
	if limit <= 0 || limit > maxLimit {
		return maxLimit
	}
	return limit
}
