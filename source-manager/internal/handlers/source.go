package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/source-manager/internal/importer"
	"github.com/jonesrussell/north-cloud/source-manager/internal/metadata"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
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
}

func NewSourceHandler(repo *repository.SourceRepository, log infralogger.Logger) *SourceHandler {
	return &SourceHandler{
		repo:      repo,
		logger:    log,
		extractor: metadata.NewExtractor(log),
	}
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

	if err := h.repo.Create(c.Request.Context(), &source); err != nil {
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

func (h *SourceHandler) List(c *gin.Context) {
	sources, err := h.repo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list sources",
			infralogger.Error(err),
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
			infralogger.String("source_id", id),
			infralogger.String("error", err.Error()),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	source.ID = id

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
	created, updated, err := h.repo.UpsertSourcesTx(c.Request.Context(), sources)
	if err != nil {
		h.logger.Error("Failed to import sources",
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import sources"})
		return
	}

	// 6. Log success and return
	h.logger.Info("Sources imported successfully",
		infralogger.Int("created", created),
		infralogger.Int("updated", updated),
		infralogger.String("filename", header.Filename),
	)

	c.JSON(http.StatusOK, ImportResult{
		Created: created,
		Updated: updated,
		Errors:  []importer.ImportError{},
	})
}
