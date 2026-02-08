package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/search/internal/domain"
	"github.com/jonesrussell/north-cloud/search/internal/service"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const trueString = "true"

// Handler holds HTTP request handlers
type Handler struct {
	searchService *service.SearchService
	logger        infralogger.Logger
}

// NewHandler creates a new handler instance
func NewHandler(searchService *service.SearchService, log infralogger.Logger) *Handler {
	return &Handler{
		searchService: searchService,
		logger:        log,
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
			h.logger.Warn("Invalid search request body",
				infralogger.Error(err),
			)
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
		h.logger.Error("Search failed",
			infralogger.Error(err),
			infralogger.String("query", req.Query),
		)

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
	return domain.SearchRequest{
		Query:      c.Query("q"),
		Filters:    parseFilters(c),
		Pagination: parsePagination(c),
		Sort:       parseSort(c),
		Options:    parseOptions(c),
	}
}

// parseFilters parses filter parameters from query string
func parseFilters(c *gin.Context) *domain.Filters {
	filters := &domain.Filters{}

	if topics := c.Query("topics"); topics != "" {
		filters.Topics = strings.Split(topics, ",")
	}
	if contentType := c.Query("content_type"); contentType != "" {
		filters.ContentType = contentType
	}
	if minQuality := c.Query("min_quality"); minQuality != "" {
		if mq, err := strconv.Atoi(minQuality); err == nil {
			filters.MinQualityScore = mq
		}
	}
	if maxQuality := c.Query("max_quality"); maxQuality != "" {
		if mq, err := strconv.Atoi(maxQuality); err == nil {
			filters.MaxQualityScore = mq
		}
	}
	if crimeRelevance := c.Query("crime_relevance"); crimeRelevance != "" {
		filters.CrimeRelevance = strings.Split(crimeRelevance, ",")
	}
	if sources := c.Query("sources"); sources != "" {
		filters.SourceNames = strings.Split(sources, ",")
	}
	if fromDate := c.Query("from_date"); fromDate != "" {
		if fd, err := time.Parse("2006-01-02", fromDate); err == nil {
			filters.FromDate = &fd
		}
	}
	if toDate := c.Query("to_date"); toDate != "" {
		if td, err := time.Parse("2006-01-02", toDate); err == nil {
			filters.ToDate = &td
		}
	}

	return filters
}

// parsePagination parses pagination parameters from query string
func parsePagination(c *gin.Context) *domain.Pagination {
	pagination := &domain.Pagination{}

	if page := c.Query("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			pagination.Page = p
		}
	}
	if size := c.Query("size"); size != "" {
		if s, err := strconv.Atoi(size); err == nil {
			pagination.Size = s
		}
	}

	return pagination
}

// parseSort parses sort parameters from query string
func parseSort(c *gin.Context) *domain.Sort {
	sort := &domain.Sort{}

	if sortField := c.Query("sort"); sortField != "" {
		sort.Field = sortField
	}
	if order := c.Query("order"); order != "" {
		sort.Order = order
	}

	return sort
}

// parseOptions parses options parameters from query string
func parseOptions(c *gin.Context) *domain.Options {
	options := &domain.Options{}

	if highlights := c.Query("highlights"); highlights != "" {
		options.IncludeHighlights = highlights == trueString
	}
	if facets := c.Query("facets"); facets != "" {
		options.IncludeFacets = facets == trueString
	}

	return options
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
