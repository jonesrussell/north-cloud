package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

const (
	defaultLimit  = 50
	defaultOffset = 0
)

// JobsHandler handles job-related HTTP requests.
type JobsHandler struct {
	repo      *database.JobRepository
	scheduler SchedulerInterface
}

// SchedulerInterface defines the interface for scheduling jobs.
// This allows the handler to trigger job reloads when jobs are created or updated.
type SchedulerInterface interface {
	ReloadJob(jobID string) error
}

// NewJobsHandler creates a new jobs handler.
func NewJobsHandler(repo *database.JobRepository) *JobsHandler {
	return &JobsHandler{repo: repo}
}

// SetScheduler sets the scheduler for the jobs handler.
// This allows the handler to trigger immediate job reloads.
func (h *JobsHandler) SetScheduler(scheduler SchedulerInterface) {
	h.scheduler = scheduler
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

	// If job is scheduled, immediately reload it into the scheduler
	if job.ScheduleEnabled && job.ScheduleTime != nil && *job.ScheduleTime != "" {
		if h.scheduler == nil {
			// Scheduler not available - this shouldn't happen in normal operation
			c.JSON(http.StatusCreated, gin.H{
				"job":     job,
				"warning": "Job created but scheduler not available - job will be picked up on next reload",
			})
			return
		}
		if err := h.scheduler.ReloadJob(job.ID); err != nil {
			// Return error to client so they know scheduling failed
			c.JSON(http.StatusCreated, gin.H{
				"job":     job,
				"warning": fmt.Sprintf("Job created but scheduling failed: %v", err),
			})
			return
		}
		c.JSON(http.StatusCreated, job)
		return
	}

	// Schedule time provided but schedule not enabled - warn user
	if job.ScheduleTime != nil && *job.ScheduleTime != "" && !job.ScheduleEnabled {
		warningMsg := "Schedule time provided but 'Enable scheduled crawling' is not checked - " +
			"job will not run automatically. Enable the schedule checkbox to activate."
		c.JSON(http.StatusCreated, gin.H{
			"job":     job,
			"warning": warningMsg,
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
	if updateErr := h.repo.Update(c.Request.Context(), job); updateErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update job: " + updateErr.Error(),
		})
		return
	}

	// Reload job in scheduler if schedule settings changed
	if h.scheduler != nil {
		if reloadErr := h.scheduler.ReloadJob(job.ID); reloadErr != nil {
			// Log error but don't fail the request - scheduler will pick it up on next reload
			_ = reloadErr // Error is intentionally ignored
		}
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
