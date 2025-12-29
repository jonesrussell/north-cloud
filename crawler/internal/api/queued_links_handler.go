package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

const (
	defaultQueuedLinksLimit  = 50
	defaultQueuedLinksOffset = 0
)

// QueuedLinksHandler handles queued link-related HTTP requests.
type QueuedLinksHandler struct {
	repo      *database.QueuedLinkRepository
	jobRepo   *database.JobRepository
	scheduler SchedulerInterface
}

// NewQueuedLinksHandler creates a new queued links handler.
func NewQueuedLinksHandler(repo *database.QueuedLinkRepository, jobRepo *database.JobRepository) *QueuedLinksHandler {
	return &QueuedLinksHandler{
		repo:    repo,
		jobRepo: jobRepo,
	}
}

// SetScheduler sets the scheduler for the queued links handler.
func (h *QueuedLinksHandler) SetScheduler(scheduler SchedulerInterface) {
	h.scheduler = scheduler
}

// ListQueuedLinks handles GET /api/v1/queued-links
func (h *QueuedLinksHandler) ListQueuedLinks(c *gin.Context) {
	// Get query parameters
	status := c.Query("status")
	sourceID := c.Query("source_id")
	sourceName := c.Query("source_name")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort", "priority")
	sortOrder := c.DefaultQuery("order", "desc")
	limitStr := c.DefaultQuery("limit", strconv.Itoa(defaultQueuedLinksLimit))
	offsetStr := c.DefaultQuery("offset", strconv.Itoa(defaultQueuedLinksOffset))

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = defaultQueuedLinksLimit
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = defaultQueuedLinksOffset
	}

	// Build filters
	filters := database.ListFilters{
		Status:     status,
		SourceID:   sourceID,
		SourceName: sourceName,
		Search:     search,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
		Limit:      limit,
		Offset:     offset,
	}

	// Get queued links from database
	links, err := h.repo.List(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve queued links",
		})
		return
	}

	// Get total count
	total, err := h.repo.Count(c.Request.Context(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get total count",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"links": links,
		"total": total,
	})
}

// GetQueuedLink handles GET /api/v1/queued-links/:id
func (h *QueuedLinksHandler) GetQueuedLink(c *gin.Context) {
	id := c.Param("id")

	link, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Queued link not found",
		})
		return
	}

	c.JSON(http.StatusOK, link)
}

// DeleteQueuedLink handles DELETE /api/v1/queued-links/:id
func (h *QueuedLinksHandler) DeleteQueuedLink(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Queued link not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Queued link deleted successfully",
	})
}

// CreateJobFromLinkRequest represents a request to create a job from a queued link.
type CreateJobFromLinkRequest struct {
	SourceID        string `json:"source_id"`
	SourceName      string `json:"source_name"`
	ScheduleTime    string `json:"schedule_time"`
	ScheduleEnabled bool   `json:"schedule_enabled"`
}

// CreateJobFromLink handles POST /api/v1/queued-links/:id/create-job
func (h *QueuedLinksHandler) CreateJobFromLink(c *gin.Context) {
	id := c.Param("id")

	// Get queued link
	link, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Queued link not found",
		})
		return
	}

	// Parse request body
	var req CreateJobFromLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Use queued link values as defaults
	sourceID := req.SourceID
	if sourceID == "" {
		sourceID = link.SourceID
	}
	sourceName := req.SourceName
	if sourceName == "" {
		sourceName = link.SourceName
	}

	// Create job domain object
	job := &domain.Job{
		ID:              uuid.New().String(),
		SourceID:        sourceID,
		URL:             link.URL,
		ScheduleEnabled: req.ScheduleEnabled,
		Status:          "pending",
	}

	// Set nullable string fields as pointers
	if sourceName != "" {
		job.SourceName = &sourceName
	}
	if req.ScheduleTime != "" {
		scheduleTime := req.ScheduleTime
		job.ScheduleTime = &scheduleTime
	}

	// Save to database
	if err := h.jobRepo.Create(c.Request.Context(), job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create job: " + err.Error(),
		})
		return
	}

	// Update link status to processing
	if updateErr := h.repo.UpdateStatus(c.Request.Context(), id, "processing"); updateErr != nil {
		// Log but don't fail the request
		_ = updateErr
	}

	// Note: IntervalScheduler polls database automatically, no manual reload needed

	c.JSON(http.StatusCreated, job)
}
