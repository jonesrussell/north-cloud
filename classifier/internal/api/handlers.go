package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/classifier/internal/classifier"
	"github.com/jonesrussell/north-cloud/classifier/internal/database"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/processor"
	"github.com/jonesrussell/north-cloud/classifier/internal/storage"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	// Default confidence threshold for classification rules
	defaultMinConfidence = 0.3
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
	storage                   *storage.ElasticsearchStorage
	logger                    infralogger.Logger
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
	elasticStorage *storage.ElasticsearchStorage,
	logger infralogger.Logger,
) *Handler {
	return &Handler{
		classifier:                classifierInstance,
		batchProcessor:            batchProcessor,
		sourceRepScorer:           sourceRepScorer,
		topicClassifier:           topicClassifier,
		rulesRepo:                 rulesRepo,
		sourceReputationRepo:      sourceReputationRepo,
		classificationHistoryRepo: classificationHistoryRepo,
		storage:                   elasticStorage,
		logger:                    logger,
	}
}

// ClassifyRequest represents a single classification request
type ClassifyRequest struct {
	RawContent *domain.RawContent `binding:"required" json:"raw_content"`
}

// ClassifyResponse represents a classification response
type ClassifyResponse struct {
	Result *domain.ClassificationResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

// BatchClassifyRequest represents a batch classification request
type BatchClassifyRequest struct {
	RawContents []*domain.RawContent `binding:"required,min=1,max=100" json:"raw_contents"`
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
		h.logger.Warn("Invalid classification request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Classifying content",
		infralogger.String("content_id", req.RawContent.ID),
		infralogger.String("source_name", req.RawContent.SourceName),
	)

	result, err := h.classifier.Classify(c.Request.Context(), req.RawContent)
	if err != nil {
		h.logger.Error("Classification failed",
			infralogger.String("content_id", req.RawContent.ID),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, ClassifyResponse{
			Error: err.Error(),
		})
		return
	}

	h.logger.Info("Content classified successfully",
		infralogger.String("content_id", result.ContentID),
		infralogger.String("content_type", result.ContentType),
		infralogger.Int("quality_score", result.QualityScore),
	)

	c.JSON(http.StatusOK, ClassifyResponse{
		Result: result,
	})
}

// ClassifyBatch handles POST /api/v1/classify/batch
func (h *Handler) ClassifyBatch(c *gin.Context) {
	var req BatchClassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid batch classification request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Batch classifying content", infralogger.Int("batch_size", len(req.RawContents)))

	results, err := h.batchProcessor.Process(c.Request.Context(), req.RawContents)
	if err != nil {
		h.logger.Error("Batch classification failed", infralogger.Error(err))
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
		infralogger.Int("total", len(results)),
		infralogger.Int("success", success),
		infralogger.Int("failed", failed),
	)

	c.JSON(http.StatusOK, BatchClassifyResponse{
		Results: results,
		Total:   len(results),
		Success: success,
		Failed:  failed,
	})
}

// ReclassifyDocument handles POST /api/v1/classify/reclassify/:content_id
// Re-classifies an existing document using current classification rules
func (h *Handler) ReclassifyDocument(c *gin.Context) {
	contentID := c.Param("content_id")
	ctx := c.Request.Context()

	// Step 1: Fetch existing classified document to get source_name
	// This ensures we know which index to query for raw content
	existing, err := h.storage.GetClassifiedByID(ctx, contentID)
	if err != nil {
		h.logger.Warn("Classified document not found",
			infralogger.String("content_id", contentID),
			infralogger.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
		return
	}

	// Step 2: Fetch raw content from raw_content index
	raw, err := h.storage.GetRawContentByID(ctx, contentID, existing.SourceName)
	if err != nil {
		h.logger.Error("Failed to fetch raw content",
			infralogger.String("content_id", contentID),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch raw content"})
		return
	}

	// Step 3: Run classification with current rules
	result, err := h.classifier.Classify(ctx, raw)
	if err != nil {
		h.logger.Error("Classification failed",
			infralogger.String("content_id", contentID),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Classification failed"})
		return
	}

	// Step 4: Update classified_content index
	classifiedContent := &domain.ClassifiedContent{
		RawContent:           *raw,
		ContentType:          result.ContentType,
		ContentSubtype:       result.ContentSubtype,
		QualityScore:         result.QualityScore,
		QualityFactors:       result.QualityFactors,
		Topics:               result.Topics,
		TopicScores:          result.TopicScores,
		SourceReputation:     result.SourceReputation,
		SourceCategory:       result.SourceCategory,
		ClassifierVersion:    result.ClassifierVersion,
		ClassificationMethod: result.ClassificationMethod,
		ModelVersion:         result.ModelVersion,
		Confidence:           result.Confidence,
		Body:                 raw.RawText, // Publisher alias
		Source:               raw.URL,     // Publisher alias
	}

	err = h.storage.IndexClassifiedContent(ctx, classifiedContent)
	if err != nil {
		h.logger.Error("Failed to update classified content",
			infralogger.String("content_id", contentID),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update classified content"})
		return
	}

	// Step 5: Return updated classification result
	h.logger.Info("Document re-classified",
		infralogger.String("content_id", contentID),
		infralogger.Any("topics", result.Topics),
		infralogger.Int("quality_score", result.QualityScore),
	)

	c.JSON(http.StatusOK, ClassifyResponse{Result: result})
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
		h.logger.Error("Failed to list rules", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load rules"})
		return
	}

	// Convert to response format
	response := make([]RuleResponse, len(rules))
	for i, rule := range rules {
		response[i] = toRuleResponse(rule)
	}

	h.logger.Info("Rules listed successfully", infralogger.Int("count", len(response)))

	c.JSON(http.StatusOK, RulesListResponse{
		Rules: response,
		Total: len(response),
	})
}

// CreateRule handles POST /api/v1/rules
func (h *Handler) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid create rule request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Creating classification rule", infralogger.String("topic", req.Topic))

	// Build rule from request
	rule := &domain.ClassificationRule{
		RuleName:      fmt.Sprintf("%s_detection", req.Topic),
		RuleType:      domain.RuleTypeTopic,
		TopicName:     req.Topic,
		Keywords:      req.Keywords,
		MinConfidence: defaultMinConfidence,
		Enabled:       req.Enabled,
		Priority:      priorityStringToInt(req.Priority),
	}

	// Create in database
	if err := h.rulesRepo.Create(c.Request.Context(), rule); err != nil {
		h.logger.Error("Failed to create rule", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create rule"})
		return
	}

	// Reload rules in topic classifier
	h.reloadTopicClassifierRules(c.Request.Context())

	h.logger.Info("Rule created successfully",
		infralogger.String("id", strconv.Itoa(rule.ID)),
		infralogger.String("topic", rule.TopicName),
	)

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
	if err = c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid update rule request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Updating classification rule",
		infralogger.String("id", strconv.Itoa(ruleID)),
		infralogger.String("topic", req.Topic),
	)

	// Get existing rule
	rule, err := h.rulesRepo.GetByID(c.Request.Context(), ruleID)
	if err != nil {
		h.logger.Error("Failed to get rule",
			infralogger.String("id", strconv.Itoa(ruleID)),
			infralogger.Error(err),
		)
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
	if err = h.rulesRepo.Update(c.Request.Context(), rule); err != nil {
		h.logger.Error("Failed to update rule",
			infralogger.String("id", strconv.Itoa(ruleID)),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update rule"})
		return
	}

	// Reload rules in topic classifier
	h.reloadTopicClassifierRules(c.Request.Context())

	h.logger.Info("Rule updated successfully",
		infralogger.String("id", strconv.Itoa(ruleID)),
		infralogger.String("topic", rule.TopicName),
	)

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

	h.logger.Info("Deleting classification rule", infralogger.String("id", strconv.Itoa(ruleID)))

	// Delete from database
	if err = h.rulesRepo.Delete(c.Request.Context(), ruleID); err != nil {
		h.logger.Error("Failed to delete rule",
			infralogger.String("id", strconv.Itoa(ruleID)),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete rule"})
		return
	}

	// Reload rules in topic classifier
	h.reloadTopicClassifierRules(c.Request.Context())

	h.logger.Info("Rule deleted successfully", infralogger.String("id", strconv.Itoa(ruleID)))

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

	h.logger.Debug("Listing sources",
		infralogger.Int("page", page),
		infralogger.Int("page_size", pageSize),
	)

	// Query database with pagination
	sources, total, err := h.sourceReputationRepo.List(c.Request.Context(), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list sources", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load sources"})
		return
	}

	// Convert to response format
	response := make([]SourceReputationResponse, len(sources))
	for i, source := range sources {
		response[i] = toSourceResponse(source)
	}

	h.logger.Info("Sources listed successfully",
		infralogger.Int("count", len(response)),
		infralogger.Int("total", total),
	)

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

	h.logger.Debug("Getting source reputation", infralogger.String("source_name", sourceName))

	// Get or create source from database (creates with defaults if not found)
	source, err := h.sourceReputationRepo.GetOrCreateSource(c.Request.Context(), sourceName)
	if err != nil {
		h.logger.Error("Failed to get or create source",
			infralogger.String("source_name", sourceName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get source"})
		return
	}

	h.logger.Info("Source retrieved successfully", infralogger.String("source_name", sourceName))

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
		h.logger.Warn("Invalid update source request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Updating source",
		infralogger.String("source_name", sourceName),
		infralogger.String("category", req.Category),
	)

	// Get existing source
	source, err := h.sourceReputationRepo.GetSource(c.Request.Context(), sourceName)
	if err != nil {
		h.logger.Error("Failed to get source",
			infralogger.String("source_name", sourceName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "Source not found"})
		return
	}

	// Update category
	source.Category = req.Category

	// Update in database
	if err = h.sourceReputationRepo.UpdateSource(c.Request.Context(), source); err != nil {
		h.logger.Error("Failed to update source",
			infralogger.String("source_name", sourceName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update source"})
		return
	}

	h.logger.Info("Source updated successfully", infralogger.String("source_name", sourceName))

	c.JSON(http.StatusOK, toSourceResponse(source))
}

// GetSourceStats handles GET /api/v1/sources/:name/stats
func (h *Handler) GetSourceStats(c *gin.Context) {
	sourceName := c.Param("name")
	if sourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_name is required"})
		return
	}

	h.logger.Debug("Getting source stats", infralogger.String("source_name", sourceName))

	// Get stats from classification history
	stats, err := h.classificationHistoryRepo.GetSourceStatsByName(c.Request.Context(), sourceName)
	if err != nil {
		h.logger.Error("Failed to get source stats",
			infralogger.String("source_name", sourceName),
			infralogger.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get source stats"})
		return
	}

	h.logger.Info("Source stats retrieved successfully", infralogger.String("source_name", sourceName))

	c.JSON(http.StatusOK, stats)
}

// GetStats handles GET /api/v1/stats?date=today
func (h *Handler) GetStats(c *gin.Context) {
	h.logger.Debug("Getting overall classification stats")

	// Parse optional date parameter
	var startDate *time.Time
	dateParam := c.Query("date")
	if dateParam == "today" {
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		startDate = &start
		h.logger.Debug("Filtering stats for today", infralogger.String("start_date", startDate.Format(time.RFC3339)))
	}

	stats, err := h.classificationHistoryRepo.GetStats(c.Request.Context(), startDate)
	if err != nil {
		h.logger.Error("Failed to get stats", infralogger.Error(err))
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
		h.logger.Error("Failed to get topic stats", infralogger.Error(err))
		// Return empty topics instead of error to avoid breaking dashboard
		c.JSON(http.StatusOK, gin.H{"topics": []gin.H{}})
		return
	}

	h.logger.Info("Topic stats retrieved successfully", infralogger.Int("count", len(stats)))

	c.JSON(http.StatusOK, gin.H{"topics": stats})
}

// GetSourceDistribution handles GET /api/v1/stats/sources
func (h *Handler) GetSourceDistribution(c *gin.Context) {
	h.logger.Debug("Getting source distribution stats")

	stats, err := h.classificationHistoryRepo.GetSourceStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get source distribution", infralogger.Error(err))
		// Return empty sources instead of error to avoid breaking dashboard
		c.JSON(http.StatusOK, gin.H{"sources": []gin.H{}})
		return
	}

	h.logger.Info("Source distribution retrieved successfully", infralogger.Int("count", len(stats)))

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

// TestRule handles POST /api/v1/rules/:id/test
// It tests a rule against provided content and returns match details
func (h *Handler) TestRule(c *gin.Context) {
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

	var req TestRuleRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid test rule request", infralogger.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Debug("Testing rule",
		infralogger.String("id", ruleIDStr),
		infralogger.Int("body_length", len(req.Body)),
	)

	// Get the rule from database
	rule, err := h.rulesRepo.GetByID(c.Request.Context(), ruleID)
	if err != nil {
		h.logger.Error("Failed to get rule",
			infralogger.String("id", ruleIDStr),
			infralogger.Error(err),
		)
		c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
		return
	}

	// Test the rule using topic classifier
	match := h.topicClassifier.TestRule(rule, req.Title, req.Body)

	h.logger.Info("Rule tested",
		infralogger.String("id", ruleIDStr),
		infralogger.Bool("matched", match.Matched),
		infralogger.Float64("score", match.Score),
	)

	c.JSON(http.StatusOK, TestRuleResponse{
		Matched:         match.Matched,
		Score:           match.Score,
		Coverage:        match.Coverage,
		MatchCount:      match.MatchCount,
		UniqueMatches:   match.UniqueMatches,
		MatchedKeywords: match.MatchedKeywords,
	})
}

// reloadTopicClassifierRules reloads classification rules from the database into the topic classifier.
// This is called after any CRUD operation on rules to ensure the classifier uses the latest rules.
func (h *Handler) reloadTopicClassifierRules(ctx context.Context) {
	h.logger.Info("Reloading classification rules from database")

	// Load enabled topic rules from database
	rules, err := h.rulesRepo.List(ctx, domain.RuleTypeTopic, ptr(true))
	if err != nil {
		h.logger.Error("Failed to reload rules from database", infralogger.Error(err))
		return
	}

	// Update topic classifier with new rules
	h.topicClassifier.UpdateRules(rules)

	h.logger.Info("Classification rules reloaded successfully", infralogger.Int("count", len(rules)))
}
