package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	infraevents "github.com/jonesrussell/north-cloud/infrastructure/events"
	"github.com/jonesrussell/north-cloud/infrastructure/indigenous"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/events"
	"github.com/jonesrussell/north-cloud/source-manager/internal/importer"
	"github.com/jonesrussell/north-cloud/source-manager/internal/metadata"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/lib/pq"
)

const (
	// pqUniqueViolation is PostgreSQL error code 23505 (unique_violation).
	pqUniqueViolation = "23505"

	// Test crawl simulation constants
	defaultTestArticlesFound = 10
	defaultTestSuccessRate   = 90
	highTestQualityScore     = 85
	mediumTestQualityScore   = 72
)

// ImportResult is the response for the import-excel endpoint.
type ImportResult struct {
	Created int                    `json:"created"`
	Updated int                    `json:"updated"`
	Errors  []importer.ImportError `json:"errors"`
}

type SourceHandler struct {
	repo      *repository.SourceRepository
	logger    infralogger.Logger
	extractor *metadata.Extractor
	publisher *events.Publisher
}

func NewSourceHandler(repo *repository.SourceRepository, log infralogger.Logger, publisher *events.Publisher) *SourceHandler {
	return &SourceHandler{
		repo:      repo,
		logger:    log,
		extractor: metadata.NewExtractor(log),
		publisher: publisher,
	}
}

// defaultRateLimit is the default rate limit when parsing fails.
const defaultRateLimit = 10

// parseRateLimit parses a rate limit string like "10/s" to an integer.
// Returns defaultRateLimit if parsing fails.
func parseRateLimit(rateLimit string) int {
	if rateLimit == "" {
		return defaultRateLimit
	}
	var rate int
	_, err := fmt.Sscanf(rateLimit, "%d", &rate)
	if err != nil || rate <= 0 {
		return defaultRateLimit
	}
	return rate
}

func (h *SourceHandler) Create(c *gin.Context) {
	var source models.Source
	if err := c.ShouldBindJSON(&source); err != nil {
		h.logger.Debug("Invalid request body",
			infralogger.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}
	source.RateLimit = models.NormalizeRateLimit(source.RateLimit)

	if err := h.validateIndigenousRegion(&source); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Create(c.Request.Context(), &source); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pqUniqueViolation {
			h.logger.Warn("Duplicate source name",
				infralogger.String("source_name", source.Name),
			)
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Source name '%s' already exists", source.Name)})
			return
		}
		h.logger.Error("Failed to create source",
			infralogger.String("source_name", source.Name),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create source"})
		return
	}

	h.logger.Info("Source created",
		infralogger.String("source_id", source.ID),
		infralogger.String("source_name", source.Name),
	)

	// Publish event asynchronously
	if h.publisher != nil {
		sourceID, _ := uuid.Parse(source.ID)
		h.publisher.PublishAsync(infraevents.SourceEvent{
			EventType: infraevents.SourceCreated,
			SourceID:  sourceID,
			Payload: infraevents.SourceCreatedPayload{
				Name:      source.Name,
				URL:       source.URL,
				RateLimit: parseRateLimit(source.RateLimit),
				MaxDepth:  source.MaxDepth,
				Enabled:   source.Enabled,
				Priority:  infraevents.PriorityNormal,
			},
		})
	}

	c.JSON(http.StatusCreated, source)
}

func (h *SourceHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	source, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Debug("Source not found",
			infralogger.String("source_id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "Source not found"})
		return
	}

	c.JSON(http.StatusOK, source)
}

// GetByIdentityKey returns a source by its identity_key (query param "identity_key").
// Used by the Source Identity Resolver. Returns 404 when no source matches.
func (h *SourceHandler) GetByIdentityKey(c *gin.Context) {
	identityKey := c.Query("identity_key")
	if identityKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identity_key query parameter is required"})
		return
	}

	source, err := h.repo.GetByIdentityKey(c.Request.Context(), identityKey)
	if err != nil {
		h.logger.Error("Failed to get source by identity_key",
			infralogger.String("identity_key", identityKey),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to lookup source"})
		return
	}
	if source == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source not found for identity_key"})
		return
	}

	c.JSON(http.StatusOK, source)
}

func (h *SourceHandler) List(c *gin.Context) {
	filter := parseListQuery(c)

	sources, err := h.repo.ListPaginated(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list sources",
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sources"})
		return
	}

	total, err := h.repo.Count(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to count sources",
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sources"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
		"total":   total,
	})
}

// parseListQuery parses limit, offset, sort_by, sort_order, search, enabled from query params.
func parseListQuery(c *gin.Context) repository.ListFilter {
	const defaultLimit = 100
	const maxLimit = 500

	limit := defaultLimit
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
			if limit > maxLimit {
				limit = maxLimit
			}
		}
	}

	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	sortBy := c.DefaultQuery("sort_by", "name")
	validSort := map[string]bool{
		"name": true, "url": true, "enabled": true, "created_at": true,
	}
	if !validSort[sortBy] {
		sortBy = "name"
	}

	sortOrder := strings.ToLower(c.DefaultQuery("sort_order", "asc"))
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	search := strings.TrimSpace(c.Query("search"))

	var enabled *bool
	if v := c.Query("enabled"); v != "" {
		switch v {
		case "true":
			t := true
			enabled = &t
		case "false":
			f := false
			enabled = &f
		}
	}

	var feedActive *bool
	if v := c.Query("feed_active"); v == "true" {
		t := true
		feedActive = &t
	}

	return repository.ListFilter{
		Limit:      limit,
		Offset:     offset,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
		Search:     search,
		Enabled:    enabled,
		FeedActive: feedActive,
	}
}

func (h *SourceHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var source models.Source
	if err := c.ShouldBindJSON(&source); err != nil {
		h.logger.Debug("Invalid request body",
			infralogger.String("source_id", id),
			infralogger.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	source.ID = id
	source.RateLimit = models.NormalizeRateLimit(source.RateLimit)

	if err := h.validateIndigenousRegion(&source); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.repo.Update(c.Request.Context(), &source); err != nil {
		h.logger.Error("Failed to update source",
			infralogger.String("source_id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update source"})
		return
	}

	h.logger.Info("Source updated",
		infralogger.String("source_id", id),
		infralogger.String("source_name", source.Name),
	)

	// Publish event asynchronously
	if h.publisher != nil {
		sourceID, _ := uuid.Parse(id)
		h.publisher.PublishAsync(infraevents.SourceEvent{
			EventType: infraevents.SourceUpdated,
			SourceID:  sourceID,
			Payload: infraevents.SourceUpdatedPayload{
				ChangedFields: []string{}, // TODO: Track changed fields
				Current: map[string]any{
					"name":       source.Name,
					"rate_limit": source.RateLimit,
					"max_depth":  source.MaxDepth,
					"enabled":    source.Enabled,
				},
			},
		})
	}

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
			infralogger.String("source_id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete source"})
		return
	}

	h.logger.Info("Source deleted",
		infralogger.String("source_id", id),
	)

	// Publish event asynchronously
	if h.publisher != nil {
		sourceID, _ := uuid.Parse(id)
		h.publisher.PublishAsync(infraevents.SourceEvent{
			EventType: infraevents.SourceDeleted,
			SourceID:  sourceID,
			Payload: infraevents.SourceDeletedPayload{
				DeletionReason: "user_requested",
			},
		})
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *SourceHandler) GetCities(c *gin.Context) {
	cities, err := h.repo.GetCities(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get cities",
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cities"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cities": cities,
		"count":  len(cities),
	})
}

// FetchMetadata extracts metadata from a URL for form prefilling
func (h *SourceHandler) FetchMetadata(c *gin.Context) {
	var request struct {
		URL string `binding:"required" json:"url"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Debug("Invalid request body",
			infralogger.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required", "details": err.Error()})
		return
	}

	h.logger.Info("Fetching metadata",
		infralogger.String("url", request.URL),
	)

	metadataResp, err := h.extractor.Extract(c.Request.Context(), request.URL)
	if err != nil {
		h.logger.Error("Failed to extract metadata",
			infralogger.String("url", request.URL),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to extract metadata", "details": err.Error()})
		return
	}

	h.logger.Info("Metadata extracted successfully",
		infralogger.String("url", request.URL),
		infralogger.String("name", metadataResp.Name),
	)

	c.JSON(http.StatusOK, metadataResp)
}

// TestCrawl performs a test crawl without saving to database
// This allows users to preview what articles will be extracted before creating a source
func (h *SourceHandler) TestCrawl(c *gin.Context) {
	var request struct {
		URL       string         `binding:"required" json:"url"`
		Selectors map[string]any `json:"selectors"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Debug("Invalid request body",
			infralogger.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	h.logger.Info("Test crawl requested",
		infralogger.String("url", request.URL),
	)

	// For now, return a simulated response
	// In a full implementation, this would actually crawl the URL and extract articles
	response := gin.H{
		"articles_found": defaultTestArticlesFound,
		"success_rate":   defaultTestSuccessRate,
		"warnings": []string{
			"No author selector matched on 2 articles",
		},
		"sample_articles": []gin.H{
			{
				"title":          "Sample Article 1",
				"body":           "This is a sample article extracted from the test crawl...",
				"url":            request.URL + "/article-1",
				"published_date": "2026-01-02T10:00:00Z",
				"author":         "John Doe",
				"quality_score":  highTestQualityScore,
			},
			{
				"title":          "Sample Article 2",
				"body":           "Another sample article demonstrating the crawl results...",
				"url":            request.URL + "/article-2",
				"published_date": "2026-01-02T09:30:00Z",
				"author":         "",
				"quality_score":  mediumTestQualityScore,
			},
		},
	}

	h.logger.Info("Test crawl completed",
		infralogger.String("url", request.URL),
		infralogger.Int("articles_found", defaultTestArticlesFound),
	)

	c.JSON(http.StatusOK, response)
}

// FeedDisableRequest is the request body for disabling a feed.
type FeedDisableRequest struct {
	Reason string `binding:"required" json:"reason"`
}

// DisableFeed marks a source's feed as disabled.
func (h *SourceHandler) DisableFeed(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source ID"})
		return
	}

	var req FeedDisableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.repo.DisableFeed(c.Request.Context(), id, req.Reason); err != nil {
		h.logger.Error("Failed to disable feed",
			infralogger.String("source_id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable feed"})
		return
	}

	h.logger.Info("Feed disabled",
		infralogger.String("source_id", id),
		infralogger.String("reason", req.Reason),
	)

	c.JSON(http.StatusOK, gin.H{"message": "Feed disabled", "source_id": id, "reason": req.Reason})
}

// EnableFeed clears a source's feed disabled state.
func (h *SourceHandler) EnableFeed(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid source ID"})
		return
	}

	if err := h.repo.EnableFeed(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to enable feed",
			infralogger.String("source_id", id),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable feed"})
		return
	}

	h.logger.Info("Feed enabled", infralogger.String("source_id", id))

	c.JSON(http.StatusOK, gin.H{"message": "Feed enabled", "source_id": id})
}

// publishImportEvents publishes SourceCreated for created sources and SourceUpdated for updated sources.
// Created events are published first so the crawler creates jobs before rescheduling.
func (h *SourceHandler) publishImportEvents(createdList, updatedList []*models.Source) {
	if h.publisher == nil {
		return
	}
	for _, s := range createdList {
		sourceID, parseErr := uuid.Parse(s.ID)
		if parseErr != nil {
			continue
		}
		h.publisher.PublishAsync(infraevents.SourceEvent{
			EventType: infraevents.SourceCreated,
			SourceID:  sourceID,
			Payload: infraevents.SourceCreatedPayload{
				Name:      s.Name,
				URL:       s.URL,
				RateLimit: parseRateLimit(s.RateLimit),
				MaxDepth:  s.MaxDepth,
				Enabled:   s.Enabled,
				Priority:  infraevents.PriorityNormal,
			},
		})
	}
	for _, s := range updatedList {
		sourceID, parseErr := uuid.Parse(s.ID)
		if parseErr != nil {
			continue
		}
		h.publisher.PublishAsync(infraevents.SourceEvent{
			EventType: infraevents.SourceUpdated,
			SourceID:  sourceID,
			Payload: infraevents.SourceUpdatedPayload{
				ChangedFields: []string{"rate_limit", "max_depth", "url", "enabled"},
			},
		})
	}
	h.logger.Info("Published import events",
		infralogger.Int("source_created", len(createdList)),
		infralogger.Int("source_updated", len(updatedList)),
	)
}

// ImportExcel handles bulk import of sources from an Excel file.
func (h *SourceHandler) ImportExcel(c *gin.Context) {
	// 1. Extract file from multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.logger.Debug("No file in request",
			infralogger.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// 2. Validate file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
		h.logger.Debug("Invalid file extension",
			infralogger.String("filename", header.Filename),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "File must be .xlsx format"})
		return
	}

	h.logger.Info("Processing Excel import",
		infralogger.String("filename", header.Filename),
		infralogger.Int64("size", header.Size),
	)

	// 3. Parse and validate all rows
	rows, validationErrors := importer.ParseExcelFile(file)
	if len(validationErrors) > 0 {
		h.logger.Debug("Validation errors in Excel file",
			infralogger.Int("error_count", len(validationErrors)),
		)
		c.JSON(http.StatusBadRequest, ImportResult{Errors: validationErrors})
		return
	}

	// 4. Convert to models
	sources := make([]*models.Source, 0, len(rows))
	for _, row := range rows {
		source, convErr := importer.ToSource(row)
		if convErr != nil {
			// This shouldn't happen if validation passed, but handle it
			h.logger.Error("Failed to convert row to source",
				infralogger.Int("row", row.Row),
				infralogger.Error(convErr),
			)
			c.JSON(http.StatusBadRequest, ImportResult{
				Errors: []importer.ImportError{{Row: row.Row, Error: convErr.Error()}},
			})
			return
		}
		sources = append(sources, source)
	}

	// 5. Upsert in transaction
	createdList, updatedList, err := h.repo.UpsertSourcesTx(c.Request.Context(), sources)
	if err != nil {
		h.logger.Error("Failed to import sources",
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import sources"})
		return
	}

	// 6. Publish events: created first, then updated (ordering for crawler job creation before reschedule)
	h.publishImportEvents(createdList, updatedList)

	// 7. Log success and return
	h.logger.Info("Sources imported successfully",
		infralogger.Int("created", len(createdList)),
		infralogger.Int("updated", len(updatedList)),
		infralogger.String("filename", header.Filename),
	)

	c.JSON(http.StatusOK, ImportResult{
		Created: len(createdList),
		Updated: len(updatedList),
		Errors:  []importer.ImportError{},
	})
}

// ImportIndigenous handles bulk import of global indigenous sources from a JSON array.
func (h *SourceHandler) ImportIndigenous(c *gin.Context) {
	indigenousSources, err := importer.ParseIndigenousSources(c.Request.Body)
	if err != nil {
		h.logger.Debug("Failed to parse indigenous sources JSON", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
		return
	}

	// Validate all sources before upserting.
	var validationErrors []importer.ImportError
	for i, src := range indigenousSources {
		if errMsg := importer.ValidateIndigenousSource(src); errMsg != "" {
			validationErrors = append(validationErrors, importer.ImportError{Row: i + 1, Error: errMsg})
		}
	}
	if len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, ImportResult{Errors: validationErrors})
		return
	}

	// Convert to models.
	sources := make([]*models.Source, 0, len(indigenousSources))
	for i, src := range indigenousSources {
		model, convErr := importer.IndigenousSourceToModel(src)
		if convErr != nil {
			c.JSON(http.StatusBadRequest, ImportResult{
				Errors: []importer.ImportError{{Row: i + 1, Error: convErr.Error()}},
			})
			return
		}
		sources = append(sources, model)
	}

	// Upsert in transaction.
	createdList, updatedList, err := h.repo.UpsertSourcesTx(c.Request.Context(), sources)
	if err != nil {
		h.logger.Error("Failed to import indigenous sources", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import sources"})
		return
	}

	h.publishImportEvents(createdList, updatedList)

	h.logger.Info("Indigenous sources imported",
		infralogger.Int("created", len(createdList)),
		infralogger.Int("updated", len(updatedList)),
	)

	c.JSON(http.StatusOK, ImportResult{
		Created: len(createdList),
		Updated: len(updatedList),
		Errors:  []importer.ImportError{},
	})
}

// ListIndigenous returns sources with indigenous_region IS NOT NULL.
func (h *SourceHandler) ListIndigenous(c *gin.Context) {
	filter := parseListQuery(c)
	filter.IndigenousOnly = true

	sources, err := h.repo.ListPaginated(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list indigenous sources", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list indigenous sources"})
		return
	}

	total, countErr := h.repo.Count(c.Request.Context(), filter)
	if countErr != nil {
		h.logger.Error("Failed to count indigenous sources", infralogger.Error(countErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list indigenous sources"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
		"total":   total,
	})
}

// validateIndigenousRegion normalizes and validates the indigenous_region field on a source.
// If the region is set, it must be a valid canonical slug. The pointer is updated in place.
func (h *SourceHandler) validateIndigenousRegion(source *models.Source) error {
	if source.IndigenousRegion == nil {
		return nil
	}
	normalized, err := indigenous.NormalizeRegionSlug(*source.IndigenousRegion)
	if err != nil {
		return fmt.Errorf("invalid indigenous_region: %w", err)
	}
	if normalized == "" {
		source.IndigenousRegion = nil
	} else {
		source.IndigenousRegion = &normalized
	}
	return nil
}
