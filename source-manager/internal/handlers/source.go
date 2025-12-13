package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/gosources/internal/logger"
	"github.com/jonesrussell/gosources/internal/models"
	"github.com/jonesrussell/gosources/internal/repository"
)

type SourceHandler struct {
	repo   *repository.SourceRepository
	logger logger.Logger
}

func NewSourceHandler(repo *repository.SourceRepository, log logger.Logger) *SourceHandler {
	return &SourceHandler{
		repo:   repo,
		logger: log,
	}
}

func (h *SourceHandler) Create(c *gin.Context) {
	var source models.Source
	if err := c.ShouldBindJSON(&source); err != nil {
		h.logger.Debug("Invalid request body",
			logger.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if err := h.repo.Create(c.Request.Context(), &source); err != nil {
		h.logger.Error("Failed to create source",
			logger.String("source_name", source.Name),
			logger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create source"})
		return
	}

	h.logger.Info("Source created",
		logger.String("source_id", source.ID),
		logger.String("source_name", source.Name),
	)

	c.JSON(http.StatusCreated, source)
}

func (h *SourceHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	source, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Debug("Source not found",
			logger.String("source_id", id),
			logger.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "Source not found"})
		return
	}

	c.JSON(http.StatusOK, source)
}

func (h *SourceHandler) List(c *gin.Context) {
	sources, err := h.repo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list sources",
			logger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sources"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
		"count":   len(sources),
	})
}

func (h *SourceHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var source models.Source
	if err := c.ShouldBindJSON(&source); err != nil {
		h.logger.Debug("Invalid request body",
			logger.String("source_id", id),
			logger.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	source.ID = id

	if err := h.repo.Update(c.Request.Context(), &source); err != nil {
		h.logger.Error("Failed to update source",
			logger.String("source_id", id),
			logger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update source"})
		return
	}

	h.logger.Info("Source updated",
		logger.String("source_id", id),
		logger.String("source_name", source.Name),
	)

	// Fetch updated source
	updated, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, source)
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *SourceHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete source",
			logger.String("source_id", id),
			logger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete source"})
		return
	}

	h.logger.Info("Source deleted",
		logger.String("source_id", id),
	)

	c.JSON(http.StatusNoContent, nil)
}

func (h *SourceHandler) GetCities(c *gin.Context) {
	cities, err := h.repo.GetCities(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get cities",
			logger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cities"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cities": cities,
		"count":  len(cities),
	})
}
