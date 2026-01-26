package v2

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/observability"
)

// SchedulerHandlerV2 handles V2 scheduler-related HTTP requests.
type SchedulerHandlerV2 struct {
	scheduler SchedulerV2Interface
	metrics   *observability.Metrics
}

// NewSchedulerHandlerV2 creates a new V2 scheduler handler.
func NewSchedulerHandlerV2(metrics *observability.Metrics) *SchedulerHandlerV2 {
	return &SchedulerHandlerV2{
		metrics: metrics,
	}
}

// SetScheduler sets the scheduler for the handler.
func (h *SchedulerHandlerV2) SetScheduler(sched SchedulerV2Interface) {
	h.scheduler = sched
}

// GetHealth handles GET /api/v2/scheduler/health
func (h *SchedulerHandlerV2) GetHealth(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":  "Scheduler not available",
			"status": "unavailable",
		})
		return
	}

	health := h.scheduler.GetHealth()

	c.JSON(http.StatusOK, gin.H{
		"status":                "ok",
		"running":               health.Running,
		"is_leader":             health.IsLeader,
		"workers_active":        health.WorkersActive,
		"workers_busy":          health.WorkersBusy,
		"queue_depth":           health.QueueDepth,
		"cron_jobs_count":       health.CronJobsCount,
		"last_check_at":         health.LastCheckAt,
		"uptime_seconds":        health.UptimeSeconds,
		"circuit_breaker_state": health.CircuitBreaker,
	})
}

// GetWorkers handles GET /api/v2/scheduler/workers
func (h *SchedulerHandlerV2) GetWorkers(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	status := h.scheduler.GetWorkerStatus()

	c.JSON(http.StatusOK, gin.H{
		"total":      status.Total,
		"active":     status.Active,
		"idle":       status.Idle,
		"draining":   status.Draining,
		"workers":    status.Workers,
		"queue_size": status.QueueSize,
	})
}

// GetMetrics handles GET /api/v2/scheduler/metrics
func (h *SchedulerHandlerV2) GetMetrics(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	// Get health info for some metrics
	health := h.scheduler.GetHealth()
	workerStatus := h.scheduler.GetWorkerStatus()

	// Return combined metrics
	c.JSON(http.StatusOK, gin.H{
		"scheduler": gin.H{
			"running":    health.Running,
			"is_leader":  health.IsLeader,
			"uptime_sec": health.UptimeSeconds,
		},
		"workers": gin.H{
			"pool_size": workerStatus.Total,
			"active":    workerStatus.Active,
			"idle":      workerStatus.Idle,
			"draining":  workerStatus.Draining,
		},
		"queue": gin.H{
			"depth": health.QueueDepth,
		},
		"cron": gin.H{
			"jobs_count": health.CronJobsCount,
		},
		"circuit_breaker": gin.H{
			"state": health.CircuitBreaker,
		},
	})
}

// Drain handles POST /api/v2/scheduler/drain
func (h *SchedulerHandlerV2) Drain(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	if err := h.scheduler.DrainWorkers(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to drain workers: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workers draining started",
		"status":  "draining",
	})
}

// Resume handles POST /api/v2/scheduler/resume
func (h *SchedulerHandlerV2) Resume(c *gin.Context) {
	if h.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scheduler not available",
		})
		return
	}

	if err := h.scheduler.ResumeWorkers(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to resume workers: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Workers resumed",
		"status":  "running",
	})
}
