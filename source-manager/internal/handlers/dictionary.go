package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
)

const (
	defaultDictLimit = 50
	maxDictLimit     = 200
	minSearchLen     = 2
	attributionValue = "Ojibwe People's Dictionary, University of Minnesota"
	attributionKey   = "X-Attribution"
)

// DictionaryHandler handles HTTP requests for the dictionary API.
type DictionaryHandler struct {
	repo   *repository.DictionaryRepository
	logger infralogger.Logger
}

// NewDictionaryHandler creates a new DictionaryHandler.
func NewDictionaryHandler(
	repo *repository.DictionaryRepository, log infralogger.Logger,
) *DictionaryHandler {
	return &DictionaryHandler{
		repo:   repo,
		logger: log,
	}
}

// ListEntries handles GET /api/v1/dictionary/entries.
// Only returns entries where consent_public_display = true.
func (h *DictionaryHandler) ListEntries(c *gin.Context) {
	c.Header(attributionKey, attributionValue)

	filter := models.DictionaryEntryFilter{
		Limit:  parseIntQuery(c, "limit", defaultDictLimit),
		Offset: parseIntQuery(c, "offset", 0),
	}
	if filter.Limit > maxDictLimit {
		filter.Limit = maxDictLimit
	}

	entries, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list dictionary entries", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list dictionary entries"})
		return
	}

	total, countErr := h.repo.Count(c.Request.Context(), filter)
	if countErr != nil {
		h.logger.Error("Failed to count dictionary entries", infralogger.Error(countErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count dictionary entries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}

// GetEntry handles GET /api/v1/dictionary/words/:id.
// Returns 404 if not found or consent_public_display is false.
func (h *DictionaryHandler) GetEntry(c *gin.Context) {
	c.Header(attributionKey, attributionValue)

	id := c.Param("id")

	entry, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get dictionary entry",
			infralogger.String("id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get dictionary entry"})
		return
	}

	if entry == nil || !entry.ConsentPublicDisplay {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dictionary entry not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"entry": entry})
}

// SearchEntries handles GET /api/v1/dictionary/search?q=<query>.
// Returns entries matching full-text search (English definitions) or prefix match (Ojibwe lemma),
// with consent_public_display = true filtering and proper pagination.
func (h *DictionaryHandler) SearchEntries(c *gin.Context) {
	c.Header(attributionKey, attributionValue)

	q := c.Query("q")
	if len(q) < minSearchLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query must be at least 2 characters"})
		return
	}

	page := parseIntQuery(c, "page", 1)
	size := parseIntQuery(c, "size", defaultDictLimit)

	entries, total, searchErr := h.repo.SearchWithCount(c.Request.Context(), q, page, size)
	if searchErr != nil {
		h.logger.Error("Failed to search dictionary entries",
			infralogger.String("query", q),
			infralogger.Error(searchErr),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search dictionary entries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
		"total":   total,
		"page":    page,
		"size":    size,
		"query":   q,
	})
}
