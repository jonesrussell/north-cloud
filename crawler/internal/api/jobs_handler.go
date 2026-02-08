package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	defaultLimit  = 50
	defaultOffset = 0
	undefinedID   = "undefined"

	// Job status values (for goconst)
	statusPending   = "pending"
	statusScheduled = "scheduled"
	statusRunning   = "running"
	statusPaused    = "paused"
	statusCompleted = "completed"
	statusFailed    = "failed"
	statusCancelled = "cancelled"
)

// JobsHandler handles job-related HTTP requests.
type JobsHandler struct {
	repo          database.JobRepositoryInterface
	executionRepo database.ExecutionRepositoryInterface
	scheduler     SchedulerInterface
	log           infralogger.Logger
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

// SetLogger sets the logger for observability.
func (h *JobsHandler) SetLogger(log infralogger.Logger) {
	h.log = log
}

// allowedJobSortFields maps API field names to database column expressions.
var allowedJobSortFields = map[string]string{
	"created_at":  "created_at",
	"updated_at":  "updated_at",
	"status":      "status",
	"source_name": "COALESCE(source_name, '')",
	"next_run_at": "next_run_at",
	"last_run_at": "started_at", // last_run_at maps to started_at in DB
}

// ListJobs handles GET /api/v1/jobs
func (h *JobsHandler) ListJobs(c *gin.Context) {
	// Parse pagination
	limit, offset := parseLimitOffset(c, defaultLimit, defaultOffset)
	limit = clampLimit(limit, MaxPageSize)

	// Parse sorting
	sortBy, sortOrder := parseSortParams(c, allowedJobSortFields, "created_at", "desc")

	// Parse filters
	status := c.Query("status")
	sourceID := c.Query("source_id")
	search := c.Query("search")

	// Build params
	listParams := database.ListJobsParams{
		Status:    status,
		SourceID:  sourceID,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Limit:     limit,
		Offset:    offset,
	}

	countParams := database.CountJobsParams{
		Status:   status,
		SourceID: sourceID,
		Search:   search,
	}

	// Get jobs from database
	jobs, err := h.repo.List(c.Request.Context(), listParams)
	if err != nil {
		respondInternalError(c, "Failed to retrieve jobs")
		return
	}

	total, err := h.repo.Count(c.Request.Context(), countParams)
	if err != nil {
		respondInternalError(c, "Failed to get total count")
		return
	}

	// Map sortBy back to external name for response
	externalSortBy := c.DefaultQuery("sort_by", "created_at")

	c.JSON(http.StatusOK, gin.H{
		"jobs":       jobs,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
		"sort_by":    externalSortBy,
		"sort_order": sortOrder,
	})
}

// GetJob handles GET /api/v1/jobs/:id
func (h *JobsHandler) GetJob(c *gin.Context) {
	id := c.Param("id")

	if id == "" || id == undefinedID {
		respondBadRequest(c, "Invalid job ID")
		return
	}

	job, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondNotFound(c, "Job")
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
	status := statusPending
	if req.IntervalMinutes != nil && req.ScheduleEnabled {
		status = statusScheduled
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
	wasInserted, err := h.repo.CreateOrUpdate(c.Request.Context(), job)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create job: " + err.Error(),
		})
		return
	}

	if h.log != nil {
		if wasInserted {
			h.log.Info("Job created", infralogger.String("job_id", job.ID), infralogger.String("source_id", job.SourceID))
		} else {
			h.log.Info("Job updated on create request", infralogger.String("job_id", job.ID), infralogger.String("source_id", job.SourceID))
		}
	}

	// Add deprecation warning headers (Phase 3 migration)
	c.Header("Deprecation", "true")
	c.Header("Sunset", "2026-06-01")
	c.Header("X-Deprecation-Notice", "POST /api/v1/jobs is deprecated. Use source-manager to create sources; jobs are created automatically.")

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

	// If job is currently running, attempt to cancel via scheduler
	// Note: If the scheduler doesn't have this job in its active list,
	// we still proceed to update the database status. The scheduler
	// might not have the job if execution already finished or if the
	// scheduler was restarted.
	if job.Status == statusRunning && h.scheduler != nil {
		// Attempt to cancel - ignore "job not currently running" errors
		// since the job may have finished between status check and cancel
		_ = h.scheduler.CancelJob(id)
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

	// Jobs can be retried from completed, failed, or cancelled states
	retryableStatuses := map[string]bool{
		"completed": true,
		"failed":    true,
		"cancelled": true,
	}
	if !retryableStatuses[job.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Jobs can only be retried from completed, failed, or cancelled status",
		})
		return
	}

	// Reset job for retry
	job.Status = statusPending
	job.CurrentRetryCount = 0
	job.ErrorMessage = nil
	job.CompletedAt = nil
	job.CancelledAt = nil

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

	if id == "" || id == undefinedID {
		respondBadRequest(c, "Invalid job ID")
		return
	}

	limit, offset := parseLimitOffset(c, defaultLimit, defaultOffset)

	executions, err := h.executionRepo.ListByJobID(c.Request.Context(), id, limit, offset)
	if err != nil {
		respondInternalError(c, "Failed to retrieve executions")
		return
	}

	total, err := h.executionRepo.CountByJobID(c.Request.Context(), id)
	if err != nil {
		respondInternalError(c, "Failed to get total count")
		return
	}

	c.JSON(http.StatusOK, ExecutionsListResponse{
		Executions: executions,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
	})
}

// GetJobStatusCounts handles GET /api/v1/jobs/status-counts
func (h *JobsHandler) GetJobStatusCounts(c *gin.Context) {
	counts, err := h.repo.CountByStatus(c.Request.Context())
	if err != nil {
		respondInternalError(c, "Failed to retrieve job status counts")
		return
	}

	c.JSON(http.StatusOK, counts)
}

// GetJobStats handles GET /api/v1/jobs/:id/stats
func (h *JobsHandler) GetJobStats(c *gin.Context) {
	id := c.Param("id")

	if id == "" || id == undefinedID {
		respondBadRequest(c, "Invalid job ID")
		return
	}

	stats, err := h.executionRepo.GetJobStats(c.Request.Context(), id)
	if err != nil {
		respondInternalError(c, "Failed to retrieve job statistics")
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetExecution handles GET /api/v1/executions/:id
func (h *JobsHandler) GetExecution(c *gin.Context) {
	id := c.Param("id")

	execution, err := h.executionRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondNotFound(c, "Execution")
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

// GetSchedulerDistribution returns the current schedule distribution.
// GET /api/v1/scheduler/distribution
func (h *JobsHandler) GetSchedulerDistribution(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	dist := h.scheduler.GetDistribution()
	if dist == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
			"message": "Load balancing is disabled",
		})
		return
	}

	c.JSON(http.StatusOK, dist)
}

// PostSchedulerRebalance triggers a full schedule rebalance.
// POST /api/v1/scheduler/rebalance
func (h *JobsHandler) PostSchedulerRebalance(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	result, err := h.scheduler.FullRebalance()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// PostSchedulerRebalancePreview previews what a rebalance would do.
// POST /api/v1/scheduler/rebalance/preview
func (h *JobsHandler) PostSchedulerRebalancePreview(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	result, err := h.scheduler.PreviewRebalance()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ForceRun handles POST /api/v2/jobs/:id/force-run (run scheduled job now).
// Sets next_run_at to now so the V1 interval scheduler picks the job up on its next poll.
func (h *JobsHandler) ForceRun(c *gin.Context) {
	id := c.Param("id")
	if id == "" || id == undefinedID {
		respondBadRequest(c, "Invalid job ID")
		return
	}

	job, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil {
		respondNotFound(c, "Job")
		return
	}

	switch job.Status {
	case statusRunning:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job is already running"})
		return
	case statusCompleted, statusFailed, statusCancelled:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job in terminal state: " + job.Status})
		return
	case statusScheduled, statusPaused, statusPending:
		// allowed
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job status cannot run now: " + job.Status})
		return
	}

	now := time.Now()
	job.NextRunAt = &now
	if updateErr := h.repo.Update(c.Request.Context(), job); updateErr != nil {
		respondInternalError(c, "Failed to schedule job for immediate run")
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Job queued for immediate execution",
		"job_id":  job.ID,
	})
}
