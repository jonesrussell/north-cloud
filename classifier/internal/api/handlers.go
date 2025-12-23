package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
)

// Handler handles HTTP requests for the classifier API
type Handler struct {
	classifier      *classifier.Classifier
	batchProcessor  *processor.BatchProcessor
	sourceRepScorer *classifier.SourceReputationScorer
	topicClassifier *classifier.TopicClassifier
	logger          Logger
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
	logger Logger,
) *Handler {
	return &Handler{
		classifier:      classifierInstance,
		batchProcessor:  batchProcessor,
		sourceRepScorer: sourceRepScorer,
		topicClassifier: topicClassifier,
		logger:          logger,
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

// RuleResponse represents a classification rule response
type RuleResponse struct {
	ID            int      `json:"id"`
	RuleName      string   `json:"rule_name"`
	RuleType      string   `json:"rule_type"`
	TopicName     string   `json:"topic_name,omitempty"`
	Keywords      []string `json:"keywords,omitempty"`
	MinConfidence float64  `json:"min_confidence"`
	Enabled       bool     `json:"enabled"`
	Priority      int      `json:"priority"`
}

// CreateRuleRequest represents a request to create a rule
type CreateRuleRequest struct {
	RuleName      string   `json:"rule_name" binding:"required"`
	RuleType      string   `json:"rule_type" binding:"required"`
	TopicName     string   `json:"topic_name"`
	Keywords      []string `json:"keywords"`
	MinConfidence float64  `json:"min_confidence"`
	Enabled       bool     `json:"enabled"`
	Priority      int      `json:"priority"`
}

// ListRules handles GET /api/v1/rules
func (h *Handler) ListRules(c *gin.Context) {
	// TODO: Implement retrieval from database
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Rule listing not yet implemented",
		"note":  "This endpoint will query classification_rules table",
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

	// TODO: Implement database insertion
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Rule creation not yet implemented",
		"note":  "This endpoint will insert into classification_rules table",
	})
}

// UpdateRule handles PUT /api/v1/rules/:id
func (h *Handler) UpdateRule(c *gin.Context) {
	ruleID := c.Param("id")
	if ruleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule_id is required"})
		return
	}

	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid update rule request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement database update
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Rule update not yet implemented",
		"note":  "This endpoint will update classification_rules table",
	})
}

// DeleteRule handles DELETE /api/v1/rules/:id
func (h *Handler) DeleteRule(c *gin.Context) {
	ruleID := c.Param("id")
	if ruleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule_id is required"})
		return
	}

	// TODO: Implement database deletion
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Rule deletion not yet implemented",
		"note":  "This endpoint will delete from classification_rules table",
	})
}

// SourceReputationResponse represents a source reputation response
type SourceReputationResponse struct {
	SourceName          string  `json:"source_name"`
	Category            string  `json:"category"`
	ReputationScore     int     `json:"reputation_score"`
	TotalArticles       int     `json:"total_articles"`
	AverageQualityScore float64 `json:"average_quality_score"`
	SpamCount           int     `json:"spam_count"`
	Rank                string  `json:"rank"`
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

	// TODO: Implement database query with pagination
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Source listing not yet implemented",
		"note":  "This endpoint will query source_reputation table",
		"pagination": gin.H{
			"page":      page,
			"page_size": pageSize,
		},
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

	result, err := h.sourceRepScorer.Score(c.Request.Context(), sourceName)
	if err != nil {
		h.logger.Error("Failed to get source reputation",
			"source_name", sourceName,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// TODO: Get full source details from database
	// For now, return just the scoring result
	c.JSON(http.StatusOK, gin.H{
		"source_name":      sourceName,
		"reputation_score": result.Score,
		"category":         result.Category,
		"rank":             result.Rank,
		"note":             "Full source details will be retrieved from database",
	})
}

// UpdateSource handles PUT /api/v1/sources/:name
func (h *Handler) UpdateSource(c *gin.Context) {
	sourceName := c.Param("name")
	if sourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_name is required"})
		return
	}

	// TODO: Implement database update
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Source update not yet implemented",
		"note":  "This endpoint will update source_reputation table",
	})
}

// GetSourceStats handles GET /api/v1/sources/:name/stats
func (h *Handler) GetSourceStats(c *gin.Context) {
	sourceName := c.Param("name")
	if sourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_name is required"})
		return
	}

	// TODO: Implement statistics retrieval from classification_history
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Source statistics not yet implemented",
		"note":  "This endpoint will query classification_history table",
	})
}

// GetStats handles GET /api/v1/stats
func (h *Handler) GetStats(c *gin.Context) {
	// TODO: Implement overall statistics
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Overall statistics not yet implemented",
		"note":  "This endpoint will aggregate classification_history data",
	})
}

// GetTopicStats handles GET /api/v1/stats/topics
func (h *Handler) GetTopicStats(c *gin.Context) {
	// TODO: Implement topic distribution statistics
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Topic statistics not yet implemented",
		"note":  "This endpoint will aggregate topic distribution from classification_history",
	})
}

// GetSourceDistribution handles GET /api/v1/stats/sources
func (h *Handler) GetSourceDistribution(c *gin.Context) {
	// TODO: Implement source reputation distribution
	// For now, return not implemented
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Source distribution not yet implemented",
		"note":  "This endpoint will aggregate source_reputation data",
	})
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
