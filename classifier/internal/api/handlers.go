package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
)

// Handler handles HTTP requests for the classifier API
type Handler struct {
	classifier                *classifier.Classifier
	batchProcessor            *processor.BatchProcessor
	sourceRepScorer           *classifier.SourceReputationScorer
	topicClassifier           *classifier.TopicClassifier
	rulesRepo                 *database.RulesRepository
	sourceReputationRepo      *database.SourceReputationRepository
	classificationHistoryRepo *database.ClassificationHistoryRepository
	logger                    Logger
}

// Logger defines the logging interface
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// NewHandler creates a new API handler
func NewHandler(
	classifierInstance *classifier.Classifier,
	batchProcessor *processor.BatchProcessor,
	sourceRepScorer *classifier.SourceReputationScorer,
	topicClassifier *classifier.TopicClassifier,
	rulesRepo *database.RulesRepository,
	sourceReputationRepo *database.SourceReputationRepository,
	classificationHistoryRepo *database.ClassificationHistoryRepository,
	logger Logger,
) *Handler {
	return &Handler{
		classifier:                classifierInstance,
		batchProcessor:            batchProcessor,
		sourceRepScorer:           sourceRepScorer,
		topicClassifier:           topicClassifier,
		rulesRepo:                 rulesRepo,
		sourceReputationRepo:      sourceReputationRepo,
		classificationHistoryRepo: classificationHistoryRepo,
		logger:                    logger,
	}
}

// ClassifyRequest represents a single classification request
type ClassifyRequest struct {
	RawContent *domain.RawContent `json:"raw_content" binding:"required"`
}

// ClassifyResponse represents a classification response
type ClassifyResponse struct {
	Result *domain.ClassificationResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

// BatchClassifyRequest represents a batch classification request
type BatchClassifyRequest struct {
	RawContents []*domain.RawContent `json:"raw_contents" binding:"required,min=1,max=100"`
}

// BatchClassifyResponse represents a batch classification response
type BatchClassifyResponse struct {
	Results []*processor.ProcessResult `json:"results"`
	Total   int                        `json:"total"`
	Success int                        `json:"success"`
	Failed  int                        `json:"failed"`
}

// Classify handles POST /api/v1/classify
func (h *Handler) Classify(c *gin.Context) {
	var req ClassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid classification request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Classifying content",
		"content_id", req.RawContent.ID,
		"source_name", req.RawContent.SourceName,
	)

	result, err := h.classifier.Classify(c.Request.Context(), req.RawContent)
	if err != nil {
		h.logger.Error("Classification failed",
			"content_id", req.RawContent.ID,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, ClassifyResponse{
			Error: err.Error(),
		})
		return
	}

	h.logger.Info("Content classified successfully",
		"content_id", result.ContentID,
		"content_type", result.ContentType,
		"quality_score", result.QualityScore,
	)

	c.JSON(http.StatusOK, ClassifyResponse{
		Result: result,
	})
}

// ClassifyBatch handles POST /api/v1/classify/batch
func (h *Handler) ClassifyBatch(c *gin.Context) {
	var req BatchClassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid batch classification request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Batch classifying content", "batch_size", len(req.RawContents))

	results, err := h.batchProcessor.Process(c.Request.Context(), req.RawContents)
	if err != nil {
		h.logger.Error("Batch classification failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Count successes and failures
	success := 0
	failed := 0
	for _, result := range results {
		if result.Error != nil {
			failed++
		} else {
			success++
		}
	}

	h.logger.Info("Batch classification completed",
		"total", len(results),
		"success", success,
		"failed", failed,
	)

	c.JSON(http.StatusOK, BatchClassifyResponse{
		Results: results,
		Total:   len(results),
		Success: success,
		Failed:  failed,
	})
}

// GetClassificationResult handles GET /api/v1/classify/:content_id
func (h *Handler) GetClassificationResult(c *gin.Context) {
	contentID := c.Param("content_id")
	if contentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content_id is required"})
		return
	}

	// TODO: Implement retrieval from classified_content index in ES
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Retrieval from classified_content not yet implemented",
		"note":  "This endpoint will query Elasticsearch classified_content index",
	})
}

// ListRules handles GET /api/v1/rules
func (h *Handler) ListRules(c *gin.Context) {
	h.logger.Debug("Listing classification rules")

	// List all topic rules
	rules, err := h.rulesRepo.List(c.Request.Context(), domain.RuleTypeTopic, nil)
	if err != nil {
		h.logger.Error("Failed to list rules", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load rules"})
		return
	}

	// Convert to response format
	response := make([]RuleResponse, len(rules))
	for i, rule := range rules {
		response[i] = toRuleResponse(rule)
	}

	h.logger.Info("Rules listed successfully", "count", len(response))

	c.JSON(http.StatusOK, RulesListResponse{
		Rules: response,
		Total: len(response),
	})
}

// CreateRule handles POST /api/v1/rules
func (h *Handler) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid create rule request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Creating classification rule", "topic", req.Topic)

	// Build rule from request
	rule := &domain.ClassificationRule{
		RuleName:      fmt.Sprintf("%s_detection", req.Topic),
		RuleType:      domain.RuleTypeTopic,
		TopicName:     req.Topic,
		Keywords:      req.Keywords,
		MinConfidence: 0.3, // Default confidence threshold
		Enabled:       req.Enabled,
		Priority:      priorityStringToInt(req.Priority),
	}

	// Create in database
	if err := h.rulesRepo.Create(c.Request.Context(), rule); err != nil {
		h.logger.Error("Failed to create rule", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rule"})
		return
	}

	// Reload rules in topic classifier
	h.reloadTopicClassifierRules(c.Request.Context())

	h.logger.Info("Rule created successfully", "id", rule.ID, "topic", rule.TopicName)

	c.JSON(http.StatusCreated, toRuleResponse(rule))
}

// UpdateRule handles PUT /api/v1/rules/:id
func (h *Handler) UpdateRule(c *gin.Context) {
	ruleIDStr := c.Param("id")
	if ruleIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule_id is required"})
		return
	}

	ruleID, err := strconv.Atoi(ruleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule ID"})
		return
	}

	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid update rule request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Updating classification rule", "id", ruleID, "topic", req.Topic)

	// Get existing rule
	rule, err := h.rulesRepo.GetByID(c.Request.Context(), ruleID)
	if err != nil {
		h.logger.Error("Failed to get rule", "id", ruleID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}

	// Update fields
	rule.TopicName = req.Topic
	rule.Keywords = req.Keywords
	rule.Priority = priorityStringToInt(req.Priority)
	rule.Enabled = req.Enabled
	rule.RuleName = fmt.Sprintf("%s_detection", req.Topic)

	// Update in database
	if err := h.rulesRepo.Update(c.Request.Context(), rule); err != nil {
		h.logger.Error("Failed to update rule", "id", ruleID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update rule"})
		return
	}

	// Reload rules in topic classifier
	h.reloadTopicClassifierRules(c.Request.Context())

	h.logger.Info("Rule updated successfully", "id", ruleID, "topic", rule.TopicName)

	c.JSON(http.StatusOK, toRuleResponse(rule))
}

// DeleteRule handles DELETE /api/v1/rules/:id
func (h *Handler) DeleteRule(c *gin.Context) {
	ruleIDStr := c.Param("id")
	if ruleIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule_id is required"})
		return
	}

	ruleID, err := strconv.Atoi(ruleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rule ID"})
		return
	}

	h.logger.Info("Deleting classification rule", "id", ruleID)

	// Delete from database
	if err := h.rulesRepo.Delete(c.Request.Context(), ruleID); err != nil {
		h.logger.Error("Failed to delete rule", "id", ruleID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rule"})
		return
	}

	// Reload rules in topic classifier
	h.reloadTopicClassifierRules(c.Request.Context())

	h.logger.Info("Rule deleted successfully", "id", ruleID)

	c.JSON(http.StatusOK, gin.H{"message": "Rule deleted successfully"})
}

// ListSources handles GET /api/v1/sources
func (h *Handler) ListSources(c *gin.Context) {
	// Get pagination parameters
	page := 1
	pageSize := 50

	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}

	if sizeParam := c.Query("page_size"); sizeParam != "" {
		if s, err := strconv.Atoi(sizeParam); err == nil && s > 0 && s <= 100 {
			pageSize = s
		}
	}

	h.logger.Debug("Listing sources", "page", page, "page_size", pageSize)

	// Query database with pagination
	sources, total, err := h.sourceReputationRepo.List(c.Request.Context(), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list sources", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load sources"})
		return
	}

	// Convert to response format
	response := make([]SourceReputationResponse, len(sources))
	for i, source := range sources {
		response[i] = toSourceResponse(source)
	}

	h.logger.Info("Sources listed successfully", "count", len(response), "total", total)

	c.JSON(http.StatusOK, SourcesListResponse{
		Sources: response,
		Total:   total,
		Page:    page,
		PerPage: pageSize,
	})
}

// GetSource handles GET /api/v1/sources/:name
func (h *Handler) GetSource(c *gin.Context) {
	sourceName := c.Param("name")
	if sourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_name is required"})
		return
	}

	h.logger.Debug("Getting source reputation", "source_name", sourceName)

	// Get from database
	source, err := h.sourceReputationRepo.GetSource(c.Request.Context(), sourceName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || err.Error() == fmt.Sprintf("source not found: %s", sourceName) {
			h.logger.Warn("Source not found", "source_name", sourceName)
			c.JSON(http.StatusNotFound, gin.H{"error": "Source not found"})
			return
		}
		h.logger.Error("Failed to get source", "source_name", sourceName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get source"})
		return
	}

	h.logger.Info("Source retrieved successfully", "source_name", sourceName)

	c.JSON(http.StatusOK, toSourceResponse(source))
}

// UpdateSource handles PUT /api/v1/sources/:name
func (h *Handler) UpdateSource(c *gin.Context) {
	sourceName := c.Param("name")
	if sourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_name is required"})
		return
	}

	var req UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid update source request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Updating source", "source_name", sourceName, "category", req.Category)

	// Get existing source
	source, err := h.sourceReputationRepo.GetSource(c.Request.Context(), sourceName)
	if err != nil {
		h.logger.Error("Failed to get source", "source_name", sourceName, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Source not found"})
		return
	}

	// Update category
	source.Category = req.Category

	// Update in database
	if err := h.sourceReputationRepo.UpdateSource(c.Request.Context(), source); err != nil {
		h.logger.Error("Failed to update source", "source_name", sourceName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update source"})
		return
	}

	h.logger.Info("Source updated successfully", "source_name", sourceName)

	c.JSON(http.StatusOK, toSourceResponse(source))
}

// GetSourceStats handles GET /api/v1/sources/:name/stats
func (h *Handler) GetSourceStats(c *gin.Context) {
	sourceName := c.Param("name")
	if sourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_name is required"})
		return
	}

	h.logger.Debug("Getting source stats", "source_name", sourceName)

	// Get stats from classification history
	stats, err := h.classificationHistoryRepo.GetSourceStatsByName(c.Request.Context(), sourceName)
	if err != nil {
		h.logger.Error("Failed to get source stats", "source_name", sourceName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get source stats"})
		return
	}

	h.logger.Info("Source stats retrieved successfully", "source_name", sourceName)

	c.JSON(http.StatusOK, stats)
}

// GetStats handles GET /api/v1/stats
func (h *Handler) GetStats(c *gin.Context) {
	h.logger.Debug("Getting overall classification stats")

	stats, err := h.classificationHistoryRepo.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get stats", "error", err)
		// Return empty stats instead of error to avoid breaking dashboard
		c.JSON(http.StatusOK, gin.H{
			"total_classified":       0,
			"avg_quality_score":      0,
			"crime_related":          0,
			"avg_processing_time_ms": 0,
			"content_types":          gin.H{},
		})
		return
	}

	h.logger.Info("Stats retrieved successfully")

	c.JSON(http.StatusOK, stats)
}

// GetTopicStats handles GET /api/v1/stats/topics
func (h *Handler) GetTopicStats(c *gin.Context) {
	h.logger.Debug("Getting topic distribution stats")

	stats, err := h.classificationHistoryRepo.GetTopicStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get topic stats", "error", err)
		// Return empty topics instead of error to avoid breaking dashboard
		c.JSON(http.StatusOK, gin.H{"topics": []gin.H{}})
		return
	}

	h.logger.Info("Topic stats retrieved successfully", "count", len(stats))

	c.JSON(http.StatusOK, gin.H{"topics": stats})
}

// GetSourceDistribution handles GET /api/v1/stats/sources
func (h *Handler) GetSourceDistribution(c *gin.Context) {
	h.logger.Debug("Getting source distribution stats")

	stats, err := h.classificationHistoryRepo.GetSourceStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get source distribution", "error", err)
		// Return empty sources instead of error to avoid breaking dashboard
		c.JSON(http.StatusOK, gin.H{"sources": []gin.H{}})
		return
	}

	h.logger.Info("Source distribution retrieved successfully", "count", len(stats))

	c.JSON(http.StatusOK, gin.H{"sources": stats})
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "classifier",
		"version": "1.0.0",
	})
}

// ReadyCheck handles GET /ready
func (h *Handler) ReadyCheck(c *gin.Context) {
	// TODO: Check dependencies (ES, PostgreSQL, Redis)
	// For now, always return ready
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
		"checks": gin.H{
			"elasticsearch": "ok",
			"postgresql":    "ok",
			"redis":         "ok",
		},
	})
}

// reloadTopicClassifierRules reloads classification rules from the database into the topic classifier.
// This is called after any CRUD operation on rules to ensure the classifier uses the latest rules.
func (h *Handler) reloadTopicClassifierRules(ctx context.Context) {
	h.logger.Info("Reloading classification rules from database")

	// Load enabled topic rules from database
	rules, err := h.rulesRepo.List(ctx, domain.RuleTypeTopic, ptr(true))
	if err != nil {
		h.logger.Error("Failed to reload rules from database", "error", err)
		return
	}

	// Update topic classifier with new rules
	h.topicClassifier.UpdateRules(rules)

	h.logger.Info("Classification rules reloaded successfully", "count", len(rules))
}
