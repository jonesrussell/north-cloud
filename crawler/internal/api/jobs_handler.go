package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
)

const (
	defaultLimit  = 50
	defaultOffset = 0
)

// JobsHandler handles job-related HTTP requests.
type JobsHandler struct {
	repo          database.JobRepositoryInterface
	executionRepo database.ExecutionRepositoryInterface
	scheduler     SchedulerInterface
}

// SchedulerInterface defines the interface for the scheduler.
// This allows the handler to interact with the scheduler for job control.
type SchedulerInterface interface {
	CancelJob(jobID string) error
	GetMetrics() scheduler.SchedulerMetrics
}

// NewJobsHandler creates a new jobs handler.
func NewJobsHandler(
	repo database.JobRepositoryInterface,
	executionRepo database.ExecutionRepositoryInterface,
) *JobsHandler {
	return &JobsHandler{
		repo:          repo,
		executionRepo: executionRepo,
	}
}

// SetScheduler sets the scheduler for the jobs handler.
func (h *JobsHandler) SetScheduler(sched SchedulerInterface) {
	h.scheduler = sched
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

	// Validate job ID
	if id == "" || id == "undefined" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

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

	// Set defaults
	maxRetries := 3
	if req.MaxRetries != nil {
		maxRetries = *req.MaxRetries
	}

	retryBackoff := 60
	if req.RetryBackoffSeconds != nil {
		retryBackoff = *req.RetryBackoffSeconds
	}

	intervalType := "minutes"
	if req.IntervalType != "" {
		intervalType = req.IntervalType
	}

	// Determine initial status
	status := "pending"
	if req.IntervalMinutes != nil && req.ScheduleEnabled {
		status = "scheduled"
	}

	// Create job domain object
	job := &domain.Job{
		ID:                  uuid.New().String(),
		SourceID:            req.SourceID,
		URL:                 req.URL,
		IntervalMinutes:     req.IntervalMinutes,
		IntervalType:        intervalType,
		ScheduleEnabled:     req.ScheduleEnabled,
		MaxRetries:          maxRetries,
		RetryBackoffSeconds: retryBackoff,
		Status:              status,
		Metadata:            req.Metadata,
	}

	// Set nullable string fields as pointers
	if req.SourceName != "" {
		sourceName := req.SourceName
		job.SourceName = &sourceName
	}

	// Legacy cron support (deprecated)
	if req.ScheduleTime != "" {
		scheduleTime := req.ScheduleTime
		job.ScheduleTime = &scheduleTime
	}

	// Save to database (trigger will calculate next_run_at)
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

	// Interval-based scheduling updates
	if req.IntervalMinutes != nil {
		job.IntervalMinutes = req.IntervalMinutes
	}
	if req.IntervalType != "" {
		job.IntervalType = req.IntervalType
	}
	if req.ScheduleEnabled != nil {
		job.ScheduleEnabled = *req.ScheduleEnabled
	}

	// Retry configuration updates
	if req.MaxRetries != nil {
		job.MaxRetries = *req.MaxRetries
	}
	if req.RetryBackoffSeconds != nil {
		job.RetryBackoffSeconds = *req.RetryBackoffSeconds
	}

	// Legacy cron support (deprecated)
	if req.ScheduleTime != "" {
		scheduleTime := req.ScheduleTime
		job.ScheduleTime = &scheduleTime
	}

	if req.Status != "" {
		job.Status = req.Status
	}

	if req.Metadata != nil {
		job.Metadata = req.Metadata
	}

	// Save changes (trigger will recalculate next_run_at if needed)
	if updateErr := h.repo.Update(c.Request.Context(), job); updateErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update job: " + updateErr.Error(),
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

// PauseJob handles POST /api/v1/jobs/:id/pause
func (h *JobsHandler) PauseJob(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.PauseJob(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Get updated job
	job, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Job paused but failed to retrieve updated status",
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// ResumeJob handles POST /api/v1/jobs/:id/resume
func (h *JobsHandler) ResumeJob(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.ResumeJob(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Get updated job
	job, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Job resumed but failed to retrieve updated status",
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// CancelJob handles POST /api/v1/jobs/:id/cancel
func (h *JobsHandler) CancelJob(c *gin.Context) {
	id := c.Param("id")

	// Get job to check status
	job, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	// If job is currently running, cancel via scheduler
	if job.Status == "running" && h.scheduler != nil {
		if cancelErr := h.scheduler.CancelJob(id); cancelErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to cancel running job: " + cancelErr.Error(),
			})
			return
		}
	}

	// Update job status in database
	if updateErr := h.repo.CancelJob(c.Request.Context(), id); updateErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": updateErr.Error(),
		})
		return
	}

	// Get updated job
	job, err = h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Job cancelled but failed to retrieve updated status",
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// RetryJob handles POST /api/v1/jobs/:id/retry
func (h *JobsHandler) RetryJob(c *gin.Context) {
	id := c.Param("id")

	// Get job to check status
	job, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	// Only failed jobs can be retried
	if job.Status != "failed" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Only failed jobs can be retried",
		})
		return
	}

	// Reset job for retry
	job.Status = "pending"
	job.CurrentRetryCount = 0
	job.ErrorMessage = nil
	job.CompletedAt = nil

	if updateErr := h.repo.Update(c.Request.Context(), job); updateErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retry job: " + updateErr.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// GetJobExecutions handles GET /api/v1/jobs/:id/executions
func (h *JobsHandler) GetJobExecutions(c *gin.Context) {
	id := c.Param("id")

	// Validate job ID
	if id == "" || id == "undefined" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

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

	executions, err := h.executionRepo.ListByJobID(c.Request.Context(), id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve executions",
		})
		return
	}

	total, err := h.executionRepo.CountByJobID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get total count",
		})
		return
	}

	c.JSON(http.StatusOK, ExecutionsListResponse{
		Executions: executions,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
	})
}

// GetJobStats handles GET /api/v1/jobs/:id/stats
func (h *JobsHandler) GetJobStats(c *gin.Context) {
	id := c.Param("id")

	// Validate job ID
	if id == "" || id == "undefined" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

	stats, err := h.executionRepo.GetJobStats(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve job statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetExecution handles GET /api/v1/executions/:id
func (h *JobsHandler) GetExecution(c *gin.Context) {
	id := c.Param("id")

	execution, err := h.executionRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Execution not found",
		})
		return
	}

	c.JSON(http.StatusOK, execution)
}

// GetSchedulerMetrics handles GET /api/v1/scheduler/metrics
func (h *JobsHandler) GetSchedulerMetrics(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	metrics := h.scheduler.GetMetrics()

	response := SchedulerMetricsResponse{}
	response.Jobs.Scheduled = metrics.JobsScheduled
	response.Jobs.Running = metrics.JobsRunning
	response.Jobs.Completed = metrics.JobsCompleted
	response.Jobs.Failed = metrics.JobsFailed
	response.Jobs.Cancelled = metrics.JobsCancelled
	response.Executions.Total = metrics.TotalExecutions
	response.Executions.AverageDurationMs = metrics.AverageDurationMs
	response.Executions.SuccessRate = metrics.SuccessRate
	response.LastCheckAt = metrics.LastCheckAt
	response.LastMetricsUpdate = metrics.LastMetricsUpdate
	response.StaleLocksCleared = metrics.StaleLocksCleared

	c.JSON(http.StatusOK, response)
}
