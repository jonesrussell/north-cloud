package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/gocrawl/internal/database"
	"github.com/jonesrussell/gocrawl/internal/domain"
)

const (
	defaultLimit  = 50
	defaultOffset = 0
)

// JobsHandler handles job-related HTTP requests.
type JobsHandler struct {
	repo *database.JobRepository
}

// NewJobsHandler creates a new jobs handler.
func NewJobsHandler(repo *database.JobRepository) *JobsHandler {
	return &JobsHandler{repo: repo}
}

// ListJobs handles GET /api/v1/jobs
func (h *JobsHandler) ListJobs(c *gin.Context) {
	// Get query parameters
	status := c.Query("status")
	limitStr := c.DefaultQuery("limit", strconv.Itoa(defaultLimit))
	offsetStr := c.DefaultQuery("offset", strconv.Itoa(defaultOffset))

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = defaultLimit
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = defaultOffset
	}

	// Get jobs from database
	jobs, err := h.repo.List(c.Request.Context(), status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve jobs",
		})
		return
	}

	// Get total count
	total, err := h.repo.Count(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get total count",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":  jobs,
		"total": total,
	})
}

// GetJob handles GET /api/v1/jobs/:id
func (h *JobsHandler) GetJob(c *gin.Context) {
	id := c.Param("id")

	job, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// CreateJob handles POST /api/v1/jobs
func (h *JobsHandler) CreateJob(c *gin.Context) {
	var req CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Create job domain object
	job := &domain.Job{
		ID:              uuid.New().String(),
		SourceID:        req.SourceID,
		URL:             req.URL,
		ScheduleEnabled: req.ScheduleEnabled,
		Status:          "pending",
	}

	// Set nullable string fields as pointers
	if req.SourceName != "" {
		sourceName := req.SourceName
		job.SourceName = &sourceName
	}
	if req.ScheduleTime != "" {
		scheduleTime := req.ScheduleTime
		job.ScheduleTime = &scheduleTime
	}

	// Save to database
	if err := h.repo.Create(c.Request.Context(), job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create job: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, job)
}

// UpdateJob handles PUT /api/v1/jobs/:id
func (h *JobsHandler) UpdateJob(c *gin.Context) {
	id := c.Param("id")

	var req UpdateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Get existing job
	job, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	// Update fields if provided
	if req.SourceID != "" {
		job.SourceID = req.SourceID
	}
	if req.SourceName != "" {
		sourceName := req.SourceName
		job.SourceName = &sourceName
	}
	if req.URL != "" {
		job.URL = req.URL
	}
	if req.ScheduleTime != "" {
		scheduleTime := req.ScheduleTime
		job.ScheduleTime = &scheduleTime
	}
	if req.ScheduleEnabled != nil {
		job.ScheduleEnabled = *req.ScheduleEnabled
	}
	if req.Status != "" {
		job.Status = req.Status
	}

	// Save changes
	if err := h.repo.Update(c.Request.Context(), job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update job: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// DeleteJob handles DELETE /api/v1/jobs/:id
func (h *JobsHandler) DeleteJob(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Job deleted successfully",
	})
}
