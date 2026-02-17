package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/frontier"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultFrontierListLimit  = 50
	defaultFrontierListOffset = 0
)

// FrontierHandler handles frontier-related HTTP requests.
type FrontierHandler struct {
	repo *database.FrontierRepository
	log  infralogger.Logger
}

// NewFrontierHandler creates a new frontier handler.
func NewFrontierHandler(repo *database.FrontierRepository, log infralogger.Logger) *FrontierHandler {
	return &FrontierHandler{
		repo: repo,
		log:  log,
	}
}

// List handles GET /api/v1/frontier
func (h *FrontierHandler) List(c *gin.Context) {
	limit, offset := parseLimitOffset(c, defaultFrontierListLimit, defaultFrontierListOffset)

	filters := database.FrontierFilters{
		Status:   c.Query("status"),
		SourceID: c.Query("source_id"),
		Host:     c.Query("host"),
		Origin:   c.Query("origin"),
		Search:   c.Query("search"),
		SortBy:   c.DefaultQuery("sort_by", "priority"),
		Limit:    limit,
		Offset:   offset,
	}

	urls, total, err := h.repo.List(c.Request.Context(), filters)
	if err != nil {
		respondInternalError(c, "Failed to retrieve frontier URLs")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"urls":  urls,
		"total": total,
	})
}

// Stats handles GET /api/v1/frontier/stats
func (h *FrontierHandler) Stats(c *gin.Context) {
	stats, err := h.repo.Stats(c.Request.Context())
	if err != nil {
		respondInternalError(c, "Failed to retrieve frontier stats")
		return
	}

	c.JSON(http.StatusOK, stats)
}

// submitRequest represents the JSON body for POST /api/v1/frontier/submit.
type submitRequest struct {
	URL      string `binding:"required" json:"url"`
	SourceID string `binding:"required" json:"source_id"`
	Origin   string `json:"origin"`
	Priority int    `json:"priority"`
}

// Submit handles POST /api/v1/frontier/submit
func (h *FrontierHandler) Submit(c *gin.Context) {
	var req submitRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		respondBadRequest(c, "Invalid request: "+bindErr.Error())
		return
	}

	params, normalizeErr := buildSubmitParams(req)
	if normalizeErr != nil {
		respondBadRequest(c, "Invalid URL: "+normalizeErr.Error())
		return
	}

	if submitErr := h.repo.Submit(c.Request.Context(), params); submitErr != nil {
		respondInternalError(c, "Failed to submit URL to frontier")
		return
	}

	h.log.Info("URL submitted to frontier",
		infralogger.String("url", req.URL),
		infralogger.String("source_id", req.SourceID),
		infralogger.String("origin", params.Origin),
	)

	c.JSON(http.StatusCreated, gin.H{
		"message": "URL submitted to frontier",
	})
}

// buildSubmitParams normalizes the URL and constructs SubmitParams from a submit request.
func buildSubmitParams(req submitRequest) (database.SubmitParams, error) {
	normalizedURL, normalizeErr := frontier.NormalizeURL(req.URL)
	if normalizeErr != nil {
		return database.SubmitParams{}, normalizeErr
	}

	urlHash, hashErr := frontier.URLHash(req.URL)
	if hashErr != nil {
		return database.SubmitParams{}, hashErr
	}

	host, hostErr := frontier.ExtractHost(req.URL)
	if hostErr != nil {
		return database.SubmitParams{}, hostErr
	}

	origin := req.Origin
	if origin == "" {
		origin = "manual"
	}

	priority := req.Priority
	if priority <= 0 {
		priority = defaultSubmitPriority
	}

	return database.SubmitParams{
		URL:      normalizedURL,
		URLHash:  urlHash,
		Host:     host,
		SourceID: req.SourceID,
		Origin:   origin,
		Priority: priority,
	}, nil
}

// defaultSubmitPriority is the default priority for manually submitted URLs.
const defaultSubmitPriority = 5

// Delete handles DELETE /api/v1/frontier/:id
func (h *FrontierHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		respondNotFound(c, "Frontier URL")
		return
	}

	c.Status(http.StatusNoContent)
}
