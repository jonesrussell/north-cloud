package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

const (
	defaultNearbyRadiusKm = 100.0
	maxNearbyRadiusKm     = 500.0
	defaultNearbyLimit    = 10
	maxNearbyLimit        = 50
	defaultListLimit      = 50
	maxListLimit          = 200
)

// CommunityHandler handles HTTP requests for the communities API.
type CommunityHandler struct {
	repo   *repository.CommunityRepository
	logger infralogger.Logger
}

// NewCommunityHandler creates a new CommunityHandler.
func NewCommunityHandler(repo *repository.CommunityRepository, log infralogger.Logger) *CommunityHandler {
	return &CommunityHandler{
		repo:   repo,
		logger: log,
	}
}

// List returns a paginated, filterable list of communities.
func (h *CommunityHandler) List(c *gin.Context) {
	filter := models.CommunityFilter{
		Type:     c.Query("type"),
		Province: c.Query("province"),
		Search:   c.Query("search"),
		Limit:    parseIntQuery(c, "limit", defaultListLimit),
		Offset:   parseIntQuery(c, "offset", 0),
	}
	if filter.Limit > maxListLimit {
		filter.Limit = maxListLimit
	}

	communities, err := h.repo.ListPaginated(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list communities", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list communities"})
		return
	}

	total, countErr := h.repo.Count(c.Request.Context(), filter)
	if countErr != nil {
		h.logger.Error("Failed to count communities", infralogger.Error(countErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count communities"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"communities": communities,
		"total":       total,
		"limit":       filter.Limit,
		"offset":      filter.Offset,
	})
}

// GetByID returns a single community by ID.
func (h *CommunityHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	community, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Debug("Community not found", infralogger.String("id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "Community not found"})
		return
	}

	c.JSON(http.StatusOK, community)
}

// GetBySlug returns a single community by slug.
func (h *CommunityHandler) GetBySlug(c *gin.Context) {
	slug := c.Param("slug")

	community, err := h.repo.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		h.logger.Debug("Community not found", infralogger.String("slug", slug))
		c.JSON(http.StatusNotFound, gin.H{"error": "Community not found"})
		return
	}

	c.JSON(http.StatusOK, community)
}

// Nearby returns communities within a radius of a lat/lon point.
func (h *CommunityHandler) Nearby(c *gin.Context) {
	latStr := c.Query("lat")
	lonStr := c.Query("lon")
	if latStr == "" || lonStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lon query parameters are required"})
		return
	}

	lat, latErr := strconv.ParseFloat(latStr, 64)
	lon, lonErr := strconv.ParseFloat(lonStr, 64)
	if latErr != nil || lonErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lat and lon must be valid numbers"})
		return
	}

	radiusKm := parseFloatQuery(c, "radius_km", defaultNearbyRadiusKm)
	if radiusKm > maxNearbyRadiusKm {
		radiusKm = maxNearbyRadiusKm
	}

	limit := parseIntQuery(c, "limit", defaultNearbyLimit)
	if limit > maxNearbyLimit {
		limit = maxNearbyLimit
	}

	communities, err := h.repo.FindNearby(c.Request.Context(), lat, lon, radiusKm, limit)
	if err != nil {
		h.logger.Error("Failed to find nearby communities", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find nearby communities"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"communities": communities,
		"total":       len(communities),
		"center":      gin.H{"lat": lat, "lon": lon},
		"radius_km":   radiusKm,
	})
}

// Create creates a new community.
func (h *CommunityHandler) Create(c *gin.Context) {
	var community models.Community
	if err := c.ShouldBindJSON(&community); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	if err := h.repo.Create(c.Request.Context(), &community); err != nil {
		h.logger.Error("Failed to create community", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create community"})
		return
	}

	c.JSON(http.StatusCreated, community)
}

// Update updates an existing community.
func (h *CommunityHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var community models.Community
	if err := c.ShouldBindJSON(&community); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}
	community.ID = id

	if err := h.repo.Update(c.Request.Context(), &community); err != nil {
		h.logger.Error("Failed to update community",
			infralogger.String("id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update community"})
		return
	}

	c.JSON(http.StatusOK, community)
}

// Delete removes a community by ID.
func (h *CommunityHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete community",
			infralogger.String("id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete community"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// parseIntQuery parses an integer query parameter with a default value.
func parseIntQuery(c *gin.Context, key string, defaultVal int) int {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil || parsed < 0 {
		return defaultVal
	}
	return parsed
}

// parseFloatQuery parses a float query parameter with a default value.
// Regions returns distinct province/region pairs with community counts.
func (h *CommunityHandler) Regions(c *gin.Context) {
	regions, err := h.repo.ListRegions(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list regions", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list regions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"regions": regions,
		"count":   len(regions),
	})
}

func parseFloatQuery(c *gin.Context, key string, defaultVal float64) float64 {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseFloat(val, 64)
	if err != nil || parsed < 0 {
		return defaultVal
	}
	return parsed
}
