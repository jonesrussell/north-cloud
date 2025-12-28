package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		if errors.Is(err, models.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Source with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create source",
		})
		return
	}

	c.JSON(http.StatusCreated, source)
}

// getSource retrieves a source by ID
// GET /api/v1/sources/:id
func (r *Router) getSource(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid source ID format",
		})
		return
	}

	source, err := r.repo.GetSourceByID(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Source not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get source",
		})
		return
	}

	c.JSON(http.StatusOK, source)
}

// updateSource updates a source
// PUT /api/v1/sources/:id
func (r *Router) updateSource(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid source ID format",
		})
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
		if errors.Is(validateErr, models.ErrNoFieldsToUpdate) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "At least one field must be provided for update",
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": validateErr.Error(),
		})
		return
	}

	source, err := r.repo.UpdateSource(ctx, id, &req)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Source not found",
			})
			return
		}
		if errors.Is(err, models.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Source with this name already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update source",
		})
		return
	}

	c.JSON(http.StatusOK, source)
}

// deleteSource deletes a source
// DELETE /api/v1/sources/:id
func (r *Router) deleteSource(c *gin.Context) {
	ctx := c.Request.Context()

	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid source ID format",
		})
		return
	}

	err = r.repo.DeleteSource(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Source not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete source",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Source deleted successfully",
	})
}
