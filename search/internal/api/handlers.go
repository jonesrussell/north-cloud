package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/search/internal/domain"
	"github.com/jonesrussell/north-cloud/search/internal/logger"
	"github.com/jonesrussell/north-cloud/search/internal/service"
)

// Handler holds HTTP request handlers
type Handler struct {
	searchService *service.SearchService
	logger        *logger.Logger
}

// NewHandler creates a new handler instance
func NewHandler(searchService *service.SearchService, logger *logger.Logger) *Handler {
	return &Handler{
		searchService: searchService,
		logger:        logger,
	}
}

// Search handles search requests (both GET and POST)
func (h *Handler) Search(c *gin.Context) {
	var req domain.SearchRequest

	// Support both GET and POST
	if c.Request.Method == http.MethodGet {
		req = h.parseQueryParams(c)
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			h.logger.Warn("Invalid search request body", "error", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:     "Invalid request body: " + err.Error(),
				Code:      "INVALID_REQUEST",
				Timestamp: time.Now(),
			})
			return
		}
	}

	// Execute search
	result, err := h.searchService.Search(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Search failed", "error", err, "query", req.Query)

		// Determine error type
		statusCode := http.StatusInternalServerError
		errorCode := "SEARCH_ERROR"
		if strings.Contains(err.Error(), "validation") {
			statusCode = http.StatusBadRequest
			errorCode = "VALIDATION_ERROR"
		}

		c.JSON(statusCode, ErrorResponse{
			Error:     err.Error(),
			Code:      errorCode,
			Timestamp: time.Now(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// parseQueryParams parses search parameters from query string (GET requests)
func (h *Handler) parseQueryParams(c *gin.Context) domain.SearchRequest {
	req := domain.SearchRequest{
		Query:      c.Query("q"),
		Filters:    &domain.Filters{},
		Pagination: &domain.Pagination{},
		Sort:       &domain.Sort{},
		Options:    &domain.Options{},
	}

	// Parse filters
	if topics := c.Query("topics"); topics != "" {
		req.Filters.Topics = strings.Split(topics, ",")
	}
	if contentType := c.Query("content_type"); contentType != "" {
		req.Filters.ContentType = contentType
	}
	if minQuality := c.Query("min_quality"); minQuality != "" {
		if mq, err := strconv.Atoi(minQuality); err == nil {
			req.Filters.MinQualityScore = mq
		}
	}
	if maxQuality := c.Query("max_quality"); maxQuality != "" {
		if mq, err := strconv.Atoi(maxQuality); err == nil {
			req.Filters.MaxQualityScore = mq
		}
	}
	if isCrime := c.Query("is_crime_related"); isCrime != "" {
		val := isCrime == "true"
		req.Filters.IsCrimeRelated = &val
	}
	if sources := c.Query("sources"); sources != "" {
		req.Filters.SourceNames = strings.Split(sources, ",")
	}
	if fromDate := c.Query("from_date"); fromDate != "" {
		if fd, err := time.Parse("2006-01-02", fromDate); err == nil {
			req.Filters.FromDate = &fd
		}
	}
	if toDate := c.Query("to_date"); toDate != "" {
		if td, err := time.Parse("2006-01-02", toDate); err == nil {
			req.Filters.ToDate = &td
		}
	}

	// Parse pagination
	if page := c.Query("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			req.Pagination.Page = p
		}
	}
	if size := c.Query("size"); size != "" {
		if s, err := strconv.Atoi(size); err == nil {
			req.Pagination.Size = s
		}
	}

	// Parse sort
	if sortField := c.Query("sort"); sortField != "" {
		req.Sort.Field = sortField
	}
	if order := c.Query("order"); order != "" {
		req.Sort.Order = order
	}

	// Parse options
	if highlights := c.Query("highlights"); highlights != "" {
		req.Options.IncludeHighlights = highlights == "true"
	}
	if facets := c.Query("facets"); facets != "" {
		req.Options.IncludeFacets = facets == "true"
	}

	return req
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(c *gin.Context) {
	status := h.searchService.HealthCheck(c.Request.Context())

	if status.Status != "healthy" {
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}

	c.JSON(http.StatusOK, status)
}

// ReadinessCheck handles readiness check requests
func (h *Handler) ReadinessCheck(c *gin.Context) {
	// For now, same as health check
	// In production, might check additional criteria (e.g., warm-up complete)
	h.HealthCheck(c)
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error     string    `json:"error"`
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}
