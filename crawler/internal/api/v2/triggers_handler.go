package v2

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/triggers"
)

// TriggersHandlerV2 handles V2 trigger-related HTTP requests.
type TriggersHandlerV2 struct {
	webhookHandler *triggers.WebhookHandler
	triggerRouter  *triggers.Router
}

// NewTriggersHandlerV2 creates a new V2 triggers handler.
func NewTriggersHandlerV2(router *triggers.Router) *TriggersHandlerV2 {
	return &TriggersHandlerV2{
		triggerRouter: router,
	}
}

// SetWebhookHandler sets the webhook handler.
func (h *TriggersHandlerV2) SetWebhookHandler(wh *triggers.WebhookHandler) {
	h.webhookHandler = wh
}

// HandleWebhook handles POST /api/v2/triggers/webhook
// This is the main webhook trigger endpoint.
func (h *TriggersHandlerV2) HandleWebhook(c *gin.Context) {
	if h.webhookHandler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Webhook handler not available",
		})
		return
	}

	// Delegate to the webhook handler
	h.webhookHandler.ServeHTTP(c.Writer, c.Request)
}

// HandleWebhookPath handles POST /api/v2/triggers/webhook/*path
// This allows path-based webhook matching.
func (h *TriggersHandlerV2) HandleWebhookPath(c *gin.Context) {
	if h.webhookHandler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Webhook handler not available",
		})
		return
	}

	// Delegate to the webhook handler with the full path
	h.webhookHandler.ServeHTTP(c.Writer, c.Request)
}

// ListWebhooks handles GET /api/v2/triggers/webhooks
// Returns all registered webhook patterns.
func (h *TriggersHandlerV2) ListWebhooks(c *gin.Context) {
	if h.triggerRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Trigger router not available",
		})
		return
	}

	patterns := h.triggerRouter.GetRegisteredWebhooks()

	c.JSON(http.StatusOK, gin.H{
		"webhooks": patterns,
		"count":    len(patterns),
	})
}

// ListChannels handles GET /api/v2/triggers/channels
// Returns all registered Pub/Sub channels.
func (h *TriggersHandlerV2) ListChannels(c *gin.Context) {
	if h.triggerRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Trigger router not available",
		})
		return
	}

	channels := h.triggerRouter.GetRegisteredChannels()

	c.JSON(http.StatusOK, gin.H{
		"channels": channels,
		"count":    len(channels),
	})
}

// RegisterWebhookRequest represents a request to register a webhook trigger.
type RegisterWebhookRequest struct {
	JobID   string `binding:"required" json:"job_id"`
	Pattern string `binding:"required" json:"pattern"`
}

// RegisterWebhook handles POST /api/v2/triggers/webhooks
// Registers a new webhook trigger for a job.
func (h *TriggersHandlerV2) RegisterWebhook(c *gin.Context) {
	if h.triggerRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Trigger router not available",
		})
		return
	}

	var req RegisterWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	if err := h.triggerRouter.RegisterWebhookTrigger(req.JobID, req.Pattern); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to register webhook: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Webhook trigger registered",
		"job_id":  req.JobID,
		"pattern": req.Pattern,
	})
}

// RegisterChannelRequest represents a request to register a channel trigger.
type RegisterChannelRequest struct {
	JobID   string `binding:"required" json:"job_id"`
	Channel string `binding:"required" json:"channel"`
}

// RegisterChannel handles POST /api/v2/triggers/channels
// Registers a new Pub/Sub channel trigger for a job.
func (h *TriggersHandlerV2) RegisterChannel(c *gin.Context) {
	if h.triggerRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Trigger router not available",
		})
		return
	}

	var req RegisterChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	if err := h.triggerRouter.RegisterChannelTrigger(c.Request.Context(), req.JobID, req.Channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to register channel: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Channel trigger registered",
		"job_id":  req.JobID,
		"channel": req.Channel,
	})
}

// UnregisterTriggerRequest represents a request to unregister triggers for a job.
type UnregisterTriggerRequest struct {
	JobID string `binding:"required" json:"job_id"`
}

// UnregisterJob handles DELETE /api/v2/triggers/jobs/:id
// Unregisters all triggers for a job.
func (h *TriggersHandlerV2) UnregisterJob(c *gin.Context) {
	jobID := c.Param("id")

	if h.triggerRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Trigger router not available",
		})
		return
	}

	h.triggerRouter.UnregisterJob(jobID)

	c.JSON(http.StatusOK, gin.H{
		"message": "All triggers unregistered for job",
		"job_id":  jobID,
	})
}

// TriggerStatus handles GET /api/v2/triggers/status
// Returns the status of the trigger system.
func (h *TriggersHandlerV2) TriggerStatus(c *gin.Context) {
	if h.triggerRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":  "Trigger router not available",
			"status": "unavailable",
		})
		return
	}

	running := h.triggerRouter.IsRunning()
	webhooks := h.triggerRouter.GetRegisteredWebhooks()
	channels := h.triggerRouter.GetRegisteredChannels()

	status := "running"
	if !running {
		status = "stopped"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          status,
		"webhooks_count":  len(webhooks),
		"channels_count":  len(channels),
		"pubsub_enabled":  h.triggerRouter.IsPubSubEnabled(),
		"webhook_enabled": true,
	})
}
