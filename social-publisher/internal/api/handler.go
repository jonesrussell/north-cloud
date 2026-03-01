package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/database"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	"github.com/jonesrussell/north-cloud/social-publisher/internal/orchestrator"
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
	Type        string               `json:"type"    binding:"required"`
	Title       string               `json:"title"`
	Body        string               `json:"body"`
	Summary     string               `json:"summary"`
	URL         string               `json:"url"`
	Images      []string             `json:"images"`
	Tags        []string             `json:"tags"`
	Project     string               `json:"project"`
	Targets     []domain.TargetConfig `json:"targets"`
	ScheduledAt string               `json:"scheduled_at"`
	Metadata    map[string]string    `json:"metadata"`
	Source      string               `json:"source"`
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
		ContentID: contentID,
		Type:      domain.ContentType(req.Type),
		Title:     req.Title,
		Body:      req.Body,
		Summary:   req.Summary,
		URL:       req.URL,
		Images:    req.Images,
		Tags:      req.Tags,
		Project:   req.Project,
		Targets:   req.Targets,
		Metadata:  req.Metadata,
		Source:    req.Source,
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

	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "manual retry is not yet implemented",
	})
}
