// Package v2 implements the V2 HTTP API for the crawler service.
package v2

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	v2domain "github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/domain"
)

const (
	defaultLimit  = 50
	defaultOffset = 0
	undefinedID   = "undefined"
)

// JobRepositoryV2 defines the interface for V2 job repository operations.
type JobRepositoryV2 interface {
	// Standard CRUD
	GetByID(ctx any, id string) (*domain.Job, error)
	List(ctx any, status string, limit, offset int) ([]*domain.Job, error)
	Count(ctx any, status string) (int64, error)
	Create(ctx any, job *domain.Job) error
	Update(ctx any, job *domain.Job) error
	Delete(ctx any, id string) error

	// V2 operations
	GetV2ByID(ctx any, id string) (*v2domain.JobV2, error)
	ListV2(ctx any, status string, priority *int, limit, offset int) ([]*v2domain.JobV2, error)
	CreateV2(ctx any, job *v2domain.JobV2) error
	UpdateV2(ctx any, job *v2domain.JobV2) error

	// Job control
	PauseJob(ctx any, id string) error
	ResumeJob(ctx any, id string) error
	CancelJob(ctx any, id string) error
}

// SchedulerV2Interface defines the interface for V2 scheduler operations.
type SchedulerV2Interface interface {
	ScheduleJob(ctx any, job *v2domain.JobV2) error
	CancelJob(jobID string) error
	ForceRun(ctx any, jobID string) error
	GetHealth() SchedulerHealth
	GetWorkerStatus() WorkerStatus
	DrainWorkers(ctx any) error
	ResumeWorkers(ctx any) error
}

// SchedulerHealth represents scheduler health information.
type SchedulerHealth struct {
	Running        bool   `json:"running"`
	IsLeader       bool   `json:"is_leader"`
	WorkersActive  int    `json:"workers_active"`
	WorkersBusy    int    `json:"workers_busy"`
	QueueDepth     int    `json:"queue_depth"`
	CronJobsCount  int    `json:"cron_jobs_count"`
	LastCheckAt    string `json:"last_check_at"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
	CircuitBreaker string `json:"circuit_breaker_status"`
}

// WorkerStatus represents worker pool status.
type WorkerStatus struct {
	Total     int            `json:"total"`
	Active    int            `json:"active"`
	Idle      int            `json:"idle"`
	Draining  bool           `json:"draining"`
	Workers   []WorkerDetail `json:"workers"`
	QueueSize int            `json:"queue_size"`
}

// WorkerDetail represents individual worker details.
type WorkerDetail struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	JobID   string `json:"job_id,omitempty"`
	Started string `json:"started,omitempty"`
}

// JobsHandlerV2 handles V2 job-related HTTP requests.
type JobsHandlerV2 struct {
	repo      JobRepositoryV2
	scheduler SchedulerV2Interface
}

// NewJobsHandlerV2 creates a new V2 jobs handler.
func NewJobsHandlerV2(repo JobRepositoryV2) *JobsHandlerV2 {
	return &JobsHandlerV2{
		repo: repo,
	}
}

// SetScheduler sets the scheduler for the V2 jobs handler.
func (h *JobsHandlerV2) SetScheduler(sched SchedulerV2Interface) {
	h.scheduler = sched
}

// ListJobs handles GET /api/v2/jobs
func (h *JobsHandlerV2) ListJobs(c *gin.Context) {
	status := c.Query("status")
	priorityStr := c.Query("priority")
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

	var priority *int
	if priorityStr != "" {
		p, parseErr := strconv.Atoi(priorityStr)
		if parseErr == nil {
			priority = &p
		}
	}

	jobs, err := h.repo.ListV2(c.Request.Context(), status, priority, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve jobs",
		})
		return
	}

	total, err := h.repo.Count(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get total count",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":   jobs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetJob handles GET /api/v2/jobs/:id
func (h *JobsHandlerV2) GetJob(c *gin.Context) {
	id := c.Param("id")

	if id == "" || id == undefinedID {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid job ID",
		})
		return
	}

	job, err := h.repo.GetV2ByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// CreateJob handles POST /api/v2/jobs
func (h *JobsHandlerV2) CreateJob(c *gin.Context) {
	var req v2domain.JobCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate schedule type
	if req.ScheduleType != "" && !req.ScheduleType.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid schedule_type. Must be one of: cron, interval, immediate, event",
		})
		return
	}

	// Build base job
	baseJob := &domain.Job{
		ID:              uuid.New().String(),
		SourceID:        req.SourceID,
		URL:             req.URL,
		Status:          "pending",
		ScheduleEnabled: req.ScheduleEnabled,
	}

	// Handle interval scheduling (for backward compatibility)
	if req.IntervalMinutes != nil {
		baseJob.IntervalMinutes = req.IntervalMinutes
		baseJob.IntervalType = req.IntervalType
	}

	// Create V2 job
	job := v2domain.NewJobV2(baseJob)

	// Set V2 specific fields
	if req.ScheduleType != "" {
		job.ScheduleType = req.ScheduleType
	}
	if req.CronExpression != nil {
		job.CronExpression = req.CronExpression
	}
	if req.Priority != nil {
		job.Priority = *req.Priority
	}
	if req.TimeoutSeconds != nil {
		job.TimeoutSeconds = *req.TimeoutSeconds
	}
	if len(req.DependsOn) > 0 {
		job.DependsOn = req.DependsOn
	}
	if req.TriggerWebhook != nil {
		job.TriggerWebhook = req.TriggerWebhook
	}
	if req.TriggerChannel != nil {
		job.TriggerChannel = req.TriggerChannel
	}

	// Update base job status based on schedule type
	if req.ScheduleEnabled {
		job.Status = "scheduled"
	}

	// Create job in repository
	if err := h.repo.CreateV2(c.Request.Context(), job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create job: " + err.Error(),
		})
		return
	}

	// Schedule job if scheduler is available
	if h.scheduler != nil && req.ScheduleEnabled {
		if schedErr := h.scheduler.ScheduleJob(c.Request.Context(), job); schedErr != nil {
			// Log but don't fail - job was created
			c.JSON(http.StatusCreated, gin.H{
				"job":     job,
				"warning": "Job created but scheduling failed: " + schedErr.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusCreated, job)
}

// UpdateJob handles PUT /api/v2/jobs/:id
func (h *JobsHandlerV2) UpdateJob(c *gin.Context) {
	id := c.Param("id")

	var req v2domain.JobUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	// Get existing job
	job, err := h.repo.GetV2ByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	// Update fields
	if req.ScheduleType != nil {
		job.ScheduleType = *req.ScheduleType
	}
	if req.CronExpression != nil {
		job.CronExpression = req.CronExpression
	}
	if req.IntervalMinutes != nil {
		job.IntervalMinutes = req.IntervalMinutes
	}
	if req.IntervalType != nil {
		job.IntervalType = *req.IntervalType
	}
	if req.Priority != nil {
		job.Priority = *req.Priority
	}
	if req.TimeoutSeconds != nil {
		job.TimeoutSeconds = *req.TimeoutSeconds
	}
	if len(req.DependsOn) > 0 {
		job.DependsOn = req.DependsOn
	}
	if req.TriggerWebhook != nil {
		job.TriggerWebhook = req.TriggerWebhook
	}
	if req.TriggerChannel != nil {
		job.TriggerChannel = req.TriggerChannel
	}
	if req.ScheduleEnabled != nil {
		job.ScheduleEnabled = *req.ScheduleEnabled
	}

	// Update in repository
	if updateErr := h.repo.UpdateV2(c.Request.Context(), job); updateErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update job: " + updateErr.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// DeleteJob handles DELETE /api/v2/jobs/:id
func (h *JobsHandlerV2) DeleteJob(c *gin.Context) {
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

// PauseJob handles POST /api/v2/jobs/:id/pause
func (h *JobsHandlerV2) PauseJob(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.PauseJob(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	job, err := h.repo.GetV2ByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Job paused but failed to retrieve updated status",
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// ResumeJob handles POST /api/v2/jobs/:id/resume
func (h *JobsHandlerV2) ResumeJob(c *gin.Context) {
	id := c.Param("id")

	if err := h.repo.ResumeJob(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	job, err := h.repo.GetV2ByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Job resumed but failed to retrieve updated status",
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// CancelJob handles POST /api/v2/jobs/:id/cancel
func (h *JobsHandlerV2) CancelJob(c *gin.Context) {
	id := c.Param("id")

	job, err := h.repo.GetV2ByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	// If job is running, cancel via scheduler
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

	job, err = h.repo.GetV2ByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Job cancelled but failed to retrieve updated status",
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// ForceRun handles POST /api/v2/jobs/:id/force-run
func (h *JobsHandlerV2) ForceRun(c *gin.Context) {
	id := c.Param("id")

	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	job, err := h.repo.GetV2ByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	// Force run via scheduler
	if forceErr := h.scheduler.ForceRun(c.Request.Context(), id); forceErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to force run job: " + forceErr.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Job queued for immediate execution",
		"job_id":  job.ID,
	})
}
