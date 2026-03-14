package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/services"
)

const (
	defaultTravelMaxMinutes = 60
	maxTravelMaxMinutes     = 180
	defaultTravelRadiusKm   = 100.0
	maxTravelRadiusKm       = 500.0
	defaultTransportMode    = "car"
)

// validTransportModes lists the allowed transport modes for OSRM.
var validTransportModes = map[string]bool{
	"car":     true,
	"bicycle": true,
	"foot":    true,
}

// TravelTimeHandler handles HTTP requests for travel time matrix computation.
type TravelTimeHandler struct {
	service *services.TravelTimeService
	logger  infralogger.Logger
}

// NewTravelTimeHandler creates a new TravelTimeHandler.
func NewTravelTimeHandler(svc *services.TravelTimeService, log infralogger.Logger) *TravelTimeHandler {
	return &TravelTimeHandler{
		service: svc,
		logger:  log,
	}
}

// GetTravelTime handles GET /api/v1/communities/:id/travel-time.
func (h *TravelTimeHandler) GetTravelTime(c *gin.Context) {
	communityID := c.Param("id")

	mode := c.DefaultQuery("mode", defaultTransportMode)
	if !validTransportModes[mode] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid mode: must be car, bicycle, or foot",
		})
		return
	}

	maxMinutes := min(
		parseIntQuery(c, "max_minutes", defaultTravelMaxMinutes),
		maxTravelMaxMinutes,
	)
	radiusKm := min(
		parseFloatQuery(c, "radius_km", defaultTravelRadiusKm),
		maxTravelRadiusKm,
	)

	results, err := h.service.ComputeMatrix(c.Request.Context(), communityID, radiusKm, maxMinutes, mode)
	if err != nil {
		h.logger.Error("Failed to compute travel time matrix",
			infralogger.String("community_id", communityID),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compute travel time matrix"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"origin_id":    communityID,
		"mode":         mode,
		"max_minutes":  maxMinutes,
		"radius_km":    radiusKm,
		"destinations": results,
		"count":        len(results),
	})
}
