package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
	"github.com/jonesrussell/north-cloud/index-manager/internal/service"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const errDocumentNotFound = "document not found"

// Pagination constants
const (
	defaultLimit     = 50
	maxLimit         = 100
	defaultSortBy    = "name"
	defaultSortOrder = "asc"
)

// Handler handles HTTP requests for the index manager API
type Handler struct {
	indexService       *service.IndexService
	documentService    *service.DocumentService
	aggregationService *service.AggregationService
	logger             infralogger.Logger
}

// NewHandler creates a new API handler
func NewHandler(
	indexService *service.IndexService,
	documentService *service.DocumentService,
	aggregationService *service.AggregationService,
	logger infralogger.Logger,
) *Handler {
	return &Handler{
		indexService:       indexService,
		documentService:    documentService,
		aggregationService: aggregationService,
		logger:             logger,
	}
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(c *gin.Context) {
	health := &domain.HealthStatus{
		Status:    "healthy",
		Version:   "1.0.0",
		Checks:    make(map[string]string),
		Timestamp: "",
	}

	// TODO: Add actual health checks (ES connection, DB connection)
	health.Checks["elasticsearch"] = "ok"
	health.Checks["database"] = "ok"

	c.JSON(http.StatusOK, health)
}

// CreateIndex handles POST /api/v1/indexes
func (h *Handler) CreateIndex(c *gin.Context) {
	var req domain.CreateIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid create index request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Creating index",
		infralogger.String("index_name", req.IndexName),
		infralogger.String("index_type", string(req.IndexType)),
		infralogger.String("source_name", req.SourceName),
	)

	index, err := h.indexService.CreateIndex(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create index",
			infralogger.String("index_name", req.IndexName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Index created successfully",
		infralogger.String("index_name", index.Name),
		infralogger.String("index_type", string(index.Type)),
	)

	c.JSON(http.StatusCreated, index)
}

// ListIndices handles GET /api/v1/indexes with pagination, filtering, and sorting
func (h *Handler) ListIndices(c *gin.Context) {
	// Parse filters
	req := &domain.ListIndicesRequest{
		Type:       c.Query("type"),
		SourceName: c.Query("source"),
		Search:     c.Query("search"),
		Health:     c.Query("health"),
		SortBy:     c.DefaultQuery("sortBy", defaultSortBy),
		SortOrder:  c.DefaultQuery("sortOrder", defaultSortOrder),
	}

	// Parse pagination
	req.Limit = defaultLimit
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, parseErr := strconv.Atoi(limitStr); parseErr == nil && limit > 0 && limit <= maxLimit {
			req.Limit = limit
		}
	}

	req.Offset = 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, parseErr := strconv.Atoi(offsetStr); parseErr == nil && offset >= 0 {
			req.Offset = offset
		}
	}

	// Validate sortBy
	validSortFields := map[string]bool{
		"name": true, "document_count": true, "size": true, "health": true, "type": true,
	}
	if !validSortFields[req.SortBy] {
		req.SortBy = defaultSortBy
	}

	// Validate sortOrder
	if req.SortOrder != "asc" && req.SortOrder != "desc" {
		req.SortOrder = defaultSortOrder
	}

	h.logger.Debug("Listing indices",
		infralogger.String("type", req.Type),
		infralogger.String("source", req.SourceName),
		infralogger.String("search", req.Search),
		infralogger.String("health", req.Health),
		infralogger.Int("limit", req.Limit),
		infralogger.Int("offset", req.Offset),
		infralogger.String("sort_by", req.SortBy),
		infralogger.String("sort_order", req.SortOrder),
	)

	response, err := h.indexService.ListIndices(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to list indices", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetIndex handles GET /api/v1/indexes/:index_name
func (h *Handler) GetIndex(c *gin.Context) {
	indexName := c.Param("index_name")

	h.logger.Debug("Getting index", infralogger.String("index_name", indexName))

	index, err := h.indexService.GetIndex(c.Request.Context(), indexName)
	if err != nil {
		h.logger.Error("Failed to get index",
			infralogger.String("index_name", indexName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "index not found"})
		return
	}

	c.JSON(http.StatusOK, index)
}

// DeleteIndex handles DELETE /api/v1/indexes/:index_name
func (h *Handler) DeleteIndex(c *gin.Context) {
	indexName := c.Param("index_name")

	h.logger.Info("Deleting index", infralogger.String("index_name", indexName))

	if err := h.indexService.DeleteIndex(c.Request.Context(), indexName); err != nil {
		h.logger.Error("Failed to delete index",
			infralogger.String("index_name", indexName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Index deleted successfully", infralogger.String("index_name", indexName))
	c.JSON(http.StatusOK, gin.H{"message": "index deleted successfully"})
}

// GetIndexHealth handles GET /api/v1/indexes/:index_name/health
func (h *Handler) GetIndexHealth(c *gin.Context) {
	indexName := c.Param("index_name")

	health, err := h.indexService.GetIndexHealth(c.Request.Context(), indexName)
	if err != nil {
		h.logger.Error("Failed to get index health",
			infralogger.String("index_name", indexName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"index_name": indexName,
		"health":     health,
	})
}

// MigrateIndex handles POST /api/v1/indexes/:index_name/migrate
func (h *Handler) MigrateIndex(c *gin.Context) {
	indexName := c.Param("index_name")

	h.logger.Info("Migrating index", infralogger.String("index_name", indexName))

	result, err := h.indexService.MigrateIndex(c.Request.Context(), indexName)
	if err != nil {
		h.logger.Error("Failed to migrate index",
			infralogger.String("index_name", indexName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// CreateIndexesForSource handles POST /api/v1/sources/:source_name/indexes
func (h *Handler) CreateIndexesForSource(c *gin.Context) {
	sourceName := c.Param("source_name")

	var req struct {
		IndexTypes []domain.IndexType `json:"index_types,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		h.logger.Warn("Invalid request body", infralogger.Error(err))
		// Continue with default index types
	}

	h.logger.Info("Creating indexes for source",
		infralogger.String("source_name", sourceName),
		infralogger.Any("index_types", req.IndexTypes),
	)

	indices, err := h.indexService.CreateIndexesForSource(c.Request.Context(), sourceName, req.IndexTypes)
	if err != nil {
		h.logger.Error("Failed to create indexes for source",
			infralogger.String("source_name", sourceName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Indexes created for source",
		infralogger.String("source_name", sourceName),
		infralogger.Int("count", len(indices)),
	)

	c.JSON(http.StatusCreated, gin.H{
		"source_name": sourceName,
		"indices":     indices,
		"count":       len(indices),
	})
}

// ListIndexesForSource handles GET /api/v1/sources/:source_name/indexes
func (h *Handler) ListIndexesForSource(c *gin.Context) {
	sourceName := c.Param("source_name")

	h.logger.Debug("Listing indexes for source", infralogger.String("source_name", sourceName))

	// Use a high limit to return all indexes for a source
	const allIndexesLimit = 1000
	response, err := h.indexService.ListIndices(c.Request.Context(), &domain.ListIndicesRequest{
		SourceName: sourceName,
		Limit:      allIndexesLimit,
		Offset:     0,
		SortBy:     "name",
		SortOrder:  "asc",
	})
	if err != nil {
		h.logger.Error("Failed to list indexes for source",
			infralogger.String("source_name", sourceName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"source_name": sourceName,
		"indices":     response.Indices,
		"count":       response.Total,
	})
}

// DeleteIndexesForSource handles DELETE /api/v1/sources/:source_name/indexes
func (h *Handler) DeleteIndexesForSource(c *gin.Context) {
	sourceName := c.Param("source_name")

	h.logger.Info("Deleting indexes for source", infralogger.String("source_name", sourceName))

	if err := h.indexService.DeleteIndexesForSource(c.Request.Context(), sourceName); err != nil {
		h.logger.Error("Failed to delete indexes for source",
			infralogger.String("source_name", sourceName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Indexes deleted for source", infralogger.String("source_name", sourceName))
	c.JSON(http.StatusOK, gin.H{"message": "indexes deleted successfully"})
}

// BulkCreateIndexes handles POST /api/v1/indexes/bulk/create
func (h *Handler) BulkCreateIndexes(c *gin.Context) {
	var req domain.BulkCreateIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid bulk create request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Bulk creating indexes", infralogger.Int("count", len(req.Indexes)))

	results := make([]*domain.Index, 0, len(req.Indexes))
	errors := make([]string, 0, len(req.Indexes))

	for _, indexReq := range req.Indexes {
		index, err := h.indexService.CreateIndex(c.Request.Context(), &indexReq)
		if err != nil {
			h.logger.Warn("Failed to create index in bulk",
				infralogger.String("index_name", indexReq.IndexName),
				infralogger.Error(err),
			)
			errors = append(errors, err.Error())
			continue
		}
		results = append(results, index)
	}

	response := gin.H{
		"created": results,
		"count":   len(results),
		"total":   len(req.Indexes),
	}
	if len(errors) > 0 {
		response["errors"] = errors
		response["failed"] = len(errors)
	}

	statusCode := http.StatusCreated
	if len(errors) == len(req.Indexes) {
		statusCode = http.StatusInternalServerError
	} else if len(errors) > 0 {
		statusCode = http.StatusMultiStatus
	}

	c.JSON(statusCode, response)
}

// BulkDeleteIndexes handles DELETE /api/v1/indexes/bulk/delete
func (h *Handler) BulkDeleteIndexes(c *gin.Context) {
	var req domain.BulkDeleteIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid bulk delete request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Bulk deleting indexes", infralogger.Int("count", len(req.IndexNames)))

	deleted := make([]string, 0, len(req.IndexNames))
	errors := make([]string, 0, len(req.IndexNames))

	for _, indexName := range req.IndexNames {
		if err := h.indexService.DeleteIndex(c.Request.Context(), indexName); err != nil {
			h.logger.Warn("Failed to delete index in bulk",
				infralogger.String("index_name", indexName),
				infralogger.Error(err),
			)
			errors = append(errors, err.Error())
			continue
		}
		deleted = append(deleted, indexName)
	}

	response := gin.H{
		"deleted": deleted,
		"count":   len(deleted),
		"total":   len(req.IndexNames),
	}
	if len(errors) > 0 {
		response["errors"] = errors
		response["failed"] = len(errors)
	}

	statusCode := http.StatusOK
	if len(errors) == len(req.IndexNames) {
		statusCode = http.StatusInternalServerError
	} else if len(errors) > 0 {
		statusCode = http.StatusMultiStatus
	}

	c.JSON(statusCode, response)
}

// GetStats handles GET /api/v1/stats
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.indexService.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get stats", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// QueryDocuments handles GET /api/v1/indexes/:index_name/documents
func (h *Handler) QueryDocuments(c *gin.Context) {
	indexName := c.Param("index_name")

	var req domain.DocumentQueryRequest
	if bindErr := c.ShouldBindQuery(&req); bindErr != nil {
		// Try to bind from JSON body if query params fail
		if jsonErr := c.ShouldBindJSON(&req); jsonErr != nil {
			h.logger.Warn("Invalid query documents request", infralogger.Error(jsonErr))
			c.JSON(http.StatusBadRequest, gin.H{"error": jsonErr.Error()})
			return
		}
	}

	// Parse query parameters manually if binding failed
	if req.Query == "" {
		req.Query = c.Query("query")
	}
	//nolint:nestif // Complex nested pagination parsing logic
	if req.Pagination == nil {
		page := 1
		size := 20
		if pageStr := c.Query("page"); pageStr != "" {
			if parsedPage, parseErr := strconv.Atoi(pageStr); parseErr == nil {
				page = parsedPage
			}
		}
		if sizeStr := c.Query("size"); sizeStr != "" {
			if parsedSize, parseErr := strconv.Atoi(sizeStr); parseErr == nil {
				size = parsedSize
			}
		}
		req.Pagination = &domain.DocumentPagination{
			Page: page,
			Size: size,
		}
	}
	if req.Sort == nil {
		req.Sort = &domain.DocumentSort{
			Field: c.DefaultQuery("sort_field", "relevance"),
			Order: c.DefaultQuery("sort_order", "desc"),
		}
	}

	response, err := h.documentService.QueryDocuments(c.Request.Context(), indexName, &req)
	if err != nil {
		h.logger.Error("Failed to query documents",
			infralogger.String("index_name", indexName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetDocument handles GET /api/v1/indexes/:index_name/documents/:document_id
func (h *Handler) GetDocument(c *gin.Context) {
	indexName := c.Param("index_name")
	documentID := c.Param("document_id")

	h.logger.Debug("Getting document",
		infralogger.String("index_name", indexName),
		infralogger.String("document_id", documentID),
	)

	document, err := h.documentService.GetDocument(c.Request.Context(), indexName, documentID)
	if err != nil {
		h.logger.Error("Failed to get document",
			infralogger.String("index_name", indexName),
			infralogger.String("document_id", documentID),
			infralogger.Error(err),
		)
		statusCode := http.StatusInternalServerError
		if err.Error() == errDocumentNotFound || err.Error() == fmt.Sprintf("index %s does not exist", indexName) {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, document)
}

// UpdateDocument handles PUT /api/v1/indexes/:index_name/documents/:document_id
func (h *Handler) UpdateDocument(c *gin.Context) {
	indexName := c.Param("index_name")
	documentID := c.Param("document_id")

	var doc domain.Document
	if err := c.ShouldBindJSON(&doc); err != nil {
		h.logger.Warn("Invalid update document request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set the document ID from URL parameter
	doc.ID = documentID

	h.logger.Info("Updating document",
		infralogger.String("index_name", indexName),
		infralogger.String("document_id", documentID),
	)

	if err := h.documentService.UpdateDocument(c.Request.Context(), indexName, documentID, &doc); err != nil {
		h.logger.Error("Failed to update document",
			infralogger.String("index_name", indexName),
			infralogger.String("document_id", documentID),
			infralogger.Error(err),
		)
		statusCode := http.StatusInternalServerError
		if err.Error() == errDocumentNotFound || err.Error() == fmt.Sprintf("index %s does not exist", indexName) {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Document updated successfully",
		infralogger.String("index_name", indexName),
		infralogger.String("document_id", documentID),
	)

	c.JSON(http.StatusOK, gin.H{"message": "document updated successfully"})
}

// DeleteDocument handles DELETE /api/v1/indexes/:index_name/documents/:document_id
func (h *Handler) DeleteDocument(c *gin.Context) {
	indexName := c.Param("index_name")
	documentID := c.Param("document_id")

	h.logger.Info("Deleting document",
		infralogger.String("index_name", indexName),
		infralogger.String("document_id", documentID),
	)

	if err := h.documentService.DeleteDocument(c.Request.Context(), indexName, documentID); err != nil {
		h.logger.Error("Failed to delete document",
			infralogger.String("index_name", indexName),
			infralogger.String("document_id", documentID),
			infralogger.Error(err),
		)
		statusCode := http.StatusInternalServerError
		if err.Error() == errDocumentNotFound || err.Error() == fmt.Sprintf("index %s does not exist", indexName) {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Document deleted successfully",
		infralogger.String("index_name", indexName),
		infralogger.String("document_id", documentID),
	)

	c.JSON(http.StatusOK, gin.H{"message": "document deleted successfully"})
}

// BulkDeleteDocuments handles POST /api/v1/indexes/:index_name/documents/bulk-delete
func (h *Handler) BulkDeleteDocuments(c *gin.Context) {
	indexName := c.Param("index_name")

	var req domain.BulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid bulk delete request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Bulk deleting documents",
		infralogger.String("index_name", indexName),
		infralogger.Int("count", len(req.DocumentIDs)),
	)

	if err := h.documentService.BulkDeleteDocuments(c.Request.Context(), indexName, req.DocumentIDs); err != nil {
		h.logger.Error("Failed to bulk delete documents",
			infralogger.String("index_name", indexName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Documents bulk deleted successfully",
		infralogger.String("index_name", indexName),
		infralogger.Int("count", len(req.DocumentIDs)),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "documents deleted successfully",
		"count":   len(req.DocumentIDs),
	})
}

// GetCrimeAggregation handles GET /api/v1/aggregations/crime
func (h *Handler) GetCrimeAggregation(c *gin.Context) {
	req := h.parseAggregationRequest(c)

	result, err := h.aggregationService.GetCrimeAggregation(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get crime aggregation", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetLocationAggregation handles GET /api/v1/aggregations/location
func (h *Handler) GetLocationAggregation(c *gin.Context) {
	req := h.parseAggregationRequest(c)

	result, err := h.aggregationService.GetLocationAggregation(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get location aggregation", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetOverviewAggregation handles GET /api/v1/aggregations/overview
func (h *Handler) GetOverviewAggregation(c *gin.Context) {
	req := h.parseAggregationRequest(c)

	result, err := h.aggregationService.GetOverviewAggregation(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get overview aggregation", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetMiningAggregation handles GET /api/v1/aggregations/mining
func (h *Handler) GetMiningAggregation(c *gin.Context) {
	req := h.parseAggregationRequest(c)

	result, err := h.aggregationService.GetMiningAggregation(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to get mining aggregation", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// parseAggregationRequest extracts filters from query parameters
func (h *Handler) parseAggregationRequest(c *gin.Context) *domain.AggregationRequest {
	req := &domain.AggregationRequest{
		Filters: &domain.DocumentFilters{},
	}

	// Parse crime filters
	if v := c.QueryArray("crime_relevance"); len(v) > 0 {
		req.Filters.CrimeRelevance = v
	}
	if v := c.QueryArray("crime_sub_labels"); len(v) > 0 {
		req.Filters.CrimeSubLabels = v
	}
	if v := c.QueryArray("crime_types"); len(v) > 0 {
		req.Filters.CrimeTypes = v
	}

	// Parse location filters
	if v := c.QueryArray("cities"); len(v) > 0 {
		req.Filters.Cities = v
	}
	if v := c.QueryArray("provinces"); len(v) > 0 {
		req.Filters.Provinces = v
	}
	if v := c.QueryArray("countries"); len(v) > 0 {
		req.Filters.Countries = v
	}

	// Parse source filter
	if v := c.QueryArray("sources"); len(v) > 0 {
		req.Filters.Sources = v
	}

	// Parse quality filters
	if minQ := c.Query("min_quality"); minQ != "" {
		if val, err := strconv.Atoi(minQ); err == nil {
			req.Filters.MinQualityScore = val
		}
	}

	return req
}
