package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

// BandOfficeHandler handles HTTP requests for the band office API.
type BandOfficeHandler struct {
	repo   *repository.BandOfficeRepository
	logger infralogger.Logger
}

// NewBandOfficeHandler creates a new BandOfficeHandler.
func NewBandOfficeHandler(repo *repository.BandOfficeRepository, log infralogger.Logger) *BandOfficeHandler {
	return &BandOfficeHandler{
		repo:   repo,
		logger: log,
	}
}

// GetByCommunity returns the band office for a community.
func (h *BandOfficeHandler) GetByCommunity(c *gin.Context) {
	communityID := c.Param("id")

	office, err := h.repo.GetByCommunity(c.Request.Context(), communityID)
	if err != nil {
		h.logger.Debug("Band office not found",
			infralogger.String("community_id", communityID),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "Band office not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"band_office": office})
}

// Upsert creates or updates the band office for a community.
func (h *BandOfficeHandler) Upsert(c *gin.Context) {
	communityID := c.Param("id")

	var office models.BandOffice
	if err := c.ShouldBindJSON(&office); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}
	office.CommunityID = communityID

	if err := h.repo.Upsert(c.Request.Context(), &office); err != nil {
		h.logger.Error("Failed to upsert band office",
			infralogger.String("community_id", communityID),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save band office"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"band_office": office})
}

// Update modifies an existing band office.
func (h *BandOfficeHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var office models.BandOffice
	if err := c.ShouldBindJSON(&office); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}
	office.ID = id

	if err := h.repo.Update(c.Request.Context(), &office); err != nil {
		h.logger.Error("Failed to update band office",
			infralogger.String("id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update band office"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"band_office": office})
}
