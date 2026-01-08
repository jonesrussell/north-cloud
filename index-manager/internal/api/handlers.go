package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/index-manager/internal/domain"
	"github.com/jonesrussell/north-cloud/index-manager/internal/service"
)

const errDocumentNotFound = "document not found"

// Handler handles HTTP requests for the index manager API
type Handler struct {
	indexService    *service.IndexService
	documentService *service.DocumentService
	logger          Logger
}

// Logger defines the logging interface
type Logger interface {
	Info(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Debug(msg string, keysAndValues ...any)
}

// NewHandler creates a new API handler
func NewHandler(indexService *service.IndexService, documentService *service.DocumentService, logger Logger) *Handler {
	return &Handler{
		indexService:    indexService,
		documentService: documentService,
		logger:          logger,
	}
}

// HealthCheck handles GET /api/v1/health
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
		h.logger.Warn("Invalid create index request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Creating index",
		"index_name", req.IndexName,
		"index_type", req.IndexType,
		"source_name", req.SourceName,
	)

	index, err := h.indexService.CreateIndex(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create index",
			"index_name", req.IndexName,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Index created successfully",
		"index_name", index.Name,
		"index_type", index.Type,
	)

	c.JSON(http.StatusCreated, index)
}

// ListIndices handles GET /api/v1/indexes
func (h *Handler) ListIndices(c *gin.Context) {
	indexType := c.Query("type")
	sourceName := c.Query("source")

	h.logger.Debug("Listing indices",
		"index_type", indexType,
		"source_name", sourceName,
	)

	indices, err := h.indexService.ListIndices(c.Request.Context(), indexType, sourceName)
	if err != nil {
		h.logger.Error("Failed to list indices", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"indices": indices,
		"count":   len(indices),
	})
}

// GetIndex handles GET /api/v1/indexes/:index_name
func (h *Handler) GetIndex(c *gin.Context) {
	indexName := c.Param("index_name")

	h.logger.Debug("Getting index", "index_name", indexName)

	index, err := h.indexService.GetIndex(c.Request.Context(), indexName)
	if err != nil {
		h.logger.Error("Failed to get index",
			"index_name", indexName,
			"error", err,
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "index not found"})
		return
	}

	c.JSON(http.StatusOK, index)
}

// DeleteIndex handles DELETE /api/v1/indexes/:index_name
func (h *Handler) DeleteIndex(c *gin.Context) {
	indexName := c.Param("index_name")

	h.logger.Info("Deleting index", "index_name", indexName)

	if err := h.indexService.DeleteIndex(c.Request.Context(), indexName); err != nil {
		h.logger.Error("Failed to delete index",
			"index_name", indexName,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Index deleted successfully", "index_name", indexName)
	c.JSON(http.StatusOK, gin.H{"message": "index deleted successfully"})
}

// GetIndexHealth handles GET /api/v1/indexes/:index_name/health
func (h *Handler) GetIndexHealth(c *gin.Context) {
	indexName := c.Param("index_name")

	health, err := h.indexService.GetIndexHealth(c.Request.Context(), indexName)
	if err != nil {
		h.logger.Error("Failed to get index health",
			"index_name", indexName,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"index_name": indexName,
		"health":     health,
	})
}

// CreateIndexesForSource handles POST /api/v1/sources/:source_name/indexes
func (h *Handler) CreateIndexesForSource(c *gin.Context) {
	sourceName := c.Param("source_name")

	var req struct {
		IndexTypes []domain.IndexType `json:"index_types,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		h.logger.Warn("Invalid request body", "error", err)
		// Continue with default index types
	}

	h.logger.Info("Creating indexes for source",
		"source_name", sourceName,
		"index_types", req.IndexTypes,
	)

	indices, err := h.indexService.CreateIndexesForSource(c.Request.Context(), sourceName, req.IndexTypes)
	if err != nil {
		h.logger.Error("Failed to create indexes for source",
			"source_name", sourceName,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Indexes created for source",
		"source_name", sourceName,
		"count", len(indices),
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

	h.logger.Debug("Listing indexes for source", "source_name", sourceName)

	indices, err := h.indexService.ListIndices(c.Request.Context(), "", sourceName)
	if err != nil {
		h.logger.Error("Failed to list indexes for source",
			"source_name", sourceName,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"source_name": sourceName,
		"indices":     indices,
		"count":       len(indices),
	})
}

// DeleteIndexesForSource handles DELETE /api/v1/sources/:source_name/indexes
func (h *Handler) DeleteIndexesForSource(c *gin.Context) {
	sourceName := c.Param("source_name")

	h.logger.Info("Deleting indexes for source", "source_name", sourceName)

	if err := h.indexService.DeleteIndexesForSource(c.Request.Context(), sourceName); err != nil {
		h.logger.Error("Failed to delete indexes for source",
			"source_name", sourceName,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Indexes deleted for source", "source_name", sourceName)
	c.JSON(http.StatusOK, gin.H{"message": "indexes deleted successfully"})
}

// BulkCreateIndexes handles POST /api/v1/indexes/bulk/create
func (h *Handler) BulkCreateIndexes(c *gin.Context) {
	var req domain.BulkCreateIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid bulk create request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Bulk creating indexes", "count", len(req.Indexes))

	var results []*domain.Index
	var errors []string

	for _, indexReq := range req.Indexes {
		index, err := h.indexService.CreateIndex(c.Request.Context(), &indexReq)
		if err != nil {
			h.logger.Warn("Failed to create index in bulk",
				"index_name", indexReq.IndexName,
				"error", err,
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
		h.logger.Warn("Invalid bulk delete request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Bulk deleting indexes", "count", len(req.IndexNames))

	var deleted []string
	var errors []string

	for _, indexName := range req.IndexNames {
		if err := h.indexService.DeleteIndex(c.Request.Context(), indexName); err != nil {
			h.logger.Warn("Failed to delete index in bulk",
				"index_name", indexName,
				"error", err,
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
		h.logger.Error("Failed to get stats", "error", err)
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
			h.logger.Warn("Invalid query documents request", "error", jsonErr)
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
			"index_name", indexName,
			"error", err,
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
		"index_name", indexName,
		"document_id", documentID,
	)

	document, err := h.documentService.GetDocument(c.Request.Context(), indexName, documentID)
	if err != nil {
		h.logger.Error("Failed to get document",
			"index_name", indexName,
			"document_id", documentID,
			"error", err,
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
		h.logger.Warn("Invalid update document request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set the document ID from URL parameter
	doc.ID = documentID

	h.logger.Info("Updating document",
		"index_name", indexName,
		"document_id", documentID,
	)

	if err := h.documentService.UpdateDocument(c.Request.Context(), indexName, documentID, &doc); err != nil {
		h.logger.Error("Failed to update document",
			"index_name", indexName,
			"document_id", documentID,
			"error", err,
		)
		statusCode := http.StatusInternalServerError
		if err.Error() == errDocumentNotFound || err.Error() == fmt.Sprintf("index %s does not exist", indexName) {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Document updated successfully",
		"index_name", indexName,
		"document_id", documentID,
	)

	c.JSON(http.StatusOK, gin.H{"message": "document updated successfully"})
}

// DeleteDocument handles DELETE /api/v1/indexes/:index_name/documents/:document_id
func (h *Handler) DeleteDocument(c *gin.Context) {
	indexName := c.Param("index_name")
	documentID := c.Param("document_id")

	h.logger.Info("Deleting document",
		"index_name", indexName,
		"document_id", documentID,
	)

	if err := h.documentService.DeleteDocument(c.Request.Context(), indexName, documentID); err != nil {
		h.logger.Error("Failed to delete document",
			"index_name", indexName,
			"document_id", documentID,
			"error", err,
		)
		statusCode := http.StatusInternalServerError
		if err.Error() == errDocumentNotFound || err.Error() == fmt.Sprintf("index %s does not exist", indexName) {
			statusCode = http.StatusNotFound
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Document deleted successfully",
		"index_name", indexName,
		"document_id", documentID,
	)

	c.JSON(http.StatusOK, gin.H{"message": "document deleted successfully"})
}

// BulkDeleteDocuments handles POST /api/v1/indexes/:index_name/documents/bulk-delete
func (h *Handler) BulkDeleteDocuments(c *gin.Context) {
	indexName := c.Param("index_name")

	var req domain.BulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid bulk delete request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Bulk deleting documents",
		"index_name", indexName,
		"count", len(req.DocumentIDs),
	)

	if err := h.documentService.BulkDeleteDocuments(c.Request.Context(), indexName, req.DocumentIDs); err != nil {
		h.logger.Error("Failed to bulk delete documents",
			"index_name", indexName,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Documents bulk deleted successfully",
		"index_name", indexName,
		"count", len(req.DocumentIDs),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "documents deleted successfully",
		"count":   len(req.DocumentIDs),
	})
}
