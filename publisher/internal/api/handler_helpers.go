package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
)

// parseUUID parses a UUID from a gin.Context parameter
func parseUUID(c *gin.Context, paramName, entityType string) (uuid.UUID, bool) {
	idParam := c.Param(paramName)
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid " + entityType + " ID format",
		})
		return uuid.Nil, false
	}
	return id, true
}

// handleRepositoryError handles common repository errors
func handleRepositoryError(c *gin.Context, err error, entityType, operation string) {
	if errors.Is(err, models.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": entityType + " not found",
		})
		return
	}
	if errors.Is(err, models.ErrAlreadyExists) {
		c.JSON(http.StatusConflict, gin.H{
			"error": entityType + " with this name already exists",
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": "Failed to " + operation + " " + entityType,
	})
}

// handleValidationError handles validation errors
func handleValidationError(c *gin.Context, err error) {
	if errors.Is(err, models.ErrNoFieldsToUpdate) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "At least one field must be provided for update",
		})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"error": err.Error(),
	})
}
