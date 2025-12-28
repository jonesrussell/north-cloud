//nolint:dupl // Similar structure to channels_handler.go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// listSources returns all sources
// GET /api/v1/sources?enabled_only=true
func (r *Router) listSources(c *gin.Context) {
	ctx := c.Request.Context()

	const queryTrue = "true"
	// Parse query parameters
	enabledOnly := c.Query("enabled_only") == queryTrue

	sources, err := r.repo.ListSources(ctx, enabledOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list sources",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
		"count":   len(sources),
	})
}

// createSource creates a new source
// POST /api/v1/sources
func (r *Router) createSource(c *gin.Context) {
	ctx := c.Request.Context()

	var req models.SourceCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": err.Error(),
		})
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	source, err := r.repo.CreateSource(ctx, &req)
	if err != nil {
		handleRepositoryError(c, err, "source", "create")
		return
	}

	c.JSON(http.StatusCreated, source)
}

// getSource retrieves a source by ID
// GET /api/v1/sources/:id
func (r *Router) getSource(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := parseUUID(c, "id", "source")
	if !ok {
		return
	}

	source, err := r.repo.GetSourceByID(ctx, id)
	if err != nil {
		handleRepositoryError(c, err, "source", "get")
		return
	}

	c.JSON(http.StatusOK, source)
}

// updateSource updates a source
// PUT /api/v1/sources/:id
func (r *Router) updateSource(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := parseUUID(c, "id", "source")
	if !ok {
		return
	}

	var req models.SourceUpdateRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request payload",
			"details": bindErr.Error(),
		})
		return
	}

	// Validate request
	if validateErr := req.Validate(); validateErr != nil {
		handleValidationError(c, validateErr)
		return
	}

	source, err := r.repo.UpdateSource(ctx, id, &req)
	if err != nil {
		handleRepositoryError(c, err, "source", "update")
		return
	}

	c.JSON(http.StatusOK, source)
}

// deleteSource deletes a source
// DELETE /api/v1/sources/:id
func (r *Router) deleteSource(c *gin.Context) {
	ctx := c.Request.Context()

	id, ok := parseUUID(c, "id", "source")
	if !ok {
		return
	}

	err := r.repo.DeleteSource(ctx, id)
	if err != nil {
		handleRepositoryError(c, err, "source", "delete")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Source deleted successfully",
	})
}
