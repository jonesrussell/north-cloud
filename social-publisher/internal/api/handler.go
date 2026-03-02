package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultLimit = 50
	maxLimit     = 100
)

// Handler implements the HTTP API endpoints for social publishing.
type Handler struct {
	repo *database.Repository
	orch *orchestrator.Orchestrator
	log  infralogger.Logger
}

// NewHandler creates a new API handler.
func NewHandler(repo *database.Repository, orch *orchestrator.Orchestrator, log infralogger.Logger) *Handler {
	return &Handler{repo: repo, orch: orch, log: log}
}

// PublishRequest is the JSON body for the publish endpoint.
type PublishRequest struct {
	Type        string                `binding:"required"  json:"type"`
	Title       string                `json:"title"`
	Body        string                `json:"body"`
	Summary     string                `json:"summary"`
	URL         string                `json:"url"`
	Images      []string              `json:"images"`
	Tags        []string              `json:"tags"`
	Project     string                `json:"project"`
	Targets     []domain.TargetConfig `json:"targets"`
	ScheduledAt *time.Time            `json:"scheduled_at"`
	Metadata    map[string]string     `json:"metadata"`
	Source      string                `json:"source"`
}

// Publish accepts content for social media publishing.
func (h *Handler) Publish(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contentID := uuid.New().String()
	if req.Source == "" {
		req.Source = "api"
	}

	msg := &domain.PublishMessage{
		ContentID:   contentID,
		Type:        domain.ContentType(req.Type),
		Title:       req.Title,
		Body:        req.Body,
		Summary:     req.Summary,
		URL:         req.URL,
		Images:      req.Images,
		Tags:        req.Tags,
		Project:     req.Project,
		Targets:     req.Targets,
		ScheduledAt: req.ScheduledAt,
		Metadata:    req.Metadata,
		Source:      req.Source,
	}

	if err := h.repo.CreateContent(c.Request.Context(), msg); err != nil {
		h.log.Error("Failed to store content",
			infralogger.Error(err),
			infralogger.String("content_id", contentID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store content"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"content_id": contentID,
		"status":     "accepted",
	})
}

// Status returns delivery status for a given content item.
func (h *Handler) Status(c *gin.Context) {
	contentID := c.Param("id")
	if contentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content_id is required"})
		return
	}

	deliveries, err := h.repo.GetDeliveriesByContentID(c.Request.Context(), contentID)
	if err != nil {
		h.log.Error("Failed to fetch deliveries",
			infralogger.Error(err),
			infralogger.String("content_id", contentID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch deliveries"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content_id": contentID,
		"deliveries": deliveries,
	})
}

// Retry re-queues a failed delivery for another attempt.
func (h *Handler) Retry(c *gin.Context) {
	deliveryID := c.Param("id")
	if deliveryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "delivery_id is required"})
		return
	}

	delivery, err := h.repo.GetDeliveryByID(c.Request.Context(), deliveryID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "delivery not found"})
		return
	}

	if delivery.Status != domain.StatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "only failed deliveries can be retried",
		})
		return
	}

	if retryErr := h.repo.ResetDeliveryForRetry(c.Request.Context(), deliveryID); retryErr != nil {
		h.log.Error("Failed to reset delivery for retry",
			infralogger.Error(retryErr),
			infralogger.String("delivery_id", deliveryID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue retry"})
		return
	}

	updated, err := h.repo.GetDeliveryByID(c.Request.Context(), deliveryID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"delivery_id": deliveryID, "status": "retrying"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// ListContent returns a paginated list of content items with delivery summaries.
func (h *Handler) ListContent(c *gin.Context) {
	filter := domain.ContentListFilter{
		Limit:  defaultLimit,
		Offset: 0,
		Status: c.Query("status"),
		Type:   c.Query("type"),
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, parseErr := strconv.Atoi(limitStr); parseErr == nil && limit > 0 && limit <= maxLimit {
			filter.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, parseErr := strconv.Atoi(offsetStr); parseErr == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	items, err := h.repo.ListContent(c.Request.Context(), filter)
	if err != nil {
		h.log.Error("Failed to list content", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list content"})
		return
	}

	total, err := h.repo.CountContent(c.Request.Context(), filter)
	if err != nil {
		h.log.Error("Failed to count content", infralogger.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count content"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"count":  len(items),
		"total":  total,
		"offset": filter.Offset,
		"limit":  filter.Limit,
	})
}
