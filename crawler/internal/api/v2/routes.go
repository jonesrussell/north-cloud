package v2

import (
	"github.com/gin-gonic/gin"
)

// SetupV2Routes configures all V2 API routes.
func SetupV2Routes(
	router *gin.RouterGroup,
	jobsHandler *JobsHandlerV2,
	schedulerHandler *SchedulerHandlerV2,
	triggersHandler *TriggersHandlerV2,
) {
	// Job routes
	setupJobRoutes(router, jobsHandler)

	// Scheduler routes
	setupSchedulerRoutes(router, schedulerHandler)

	// Trigger routes
	setupTriggerRoutes(router, triggersHandler)
}

// setupJobRoutes configures V2 job-related endpoints.
func setupJobRoutes(v2 *gin.RouterGroup, handler *JobsHandlerV2) {
	if handler == nil {
		return
	}

	// CRUD
	v2.GET("/jobs", handler.ListJobs)
	v2.POST("/jobs", handler.CreateJob)
	v2.GET("/jobs/:id", handler.GetJob)
	v2.PUT("/jobs/:id", handler.UpdateJob)
	v2.DELETE("/jobs/:id", handler.DeleteJob)

	// Job control
	v2.POST("/jobs/:id/pause", handler.PauseJob)
	v2.POST("/jobs/:id/resume", handler.ResumeJob)
	v2.POST("/jobs/:id/cancel", handler.CancelJob)
	v2.POST("/jobs/:id/force-run", handler.ForceRun)
}

// setupSchedulerRoutes configures V2 scheduler-related endpoints.
func setupSchedulerRoutes(v2 *gin.RouterGroup, handler *SchedulerHandlerV2) {
	if handler == nil {
		return
	}

	v2.GET("/scheduler/health", handler.GetHealth)
	v2.GET("/scheduler/workers", handler.GetWorkers)
	v2.GET("/scheduler/metrics", handler.GetMetrics)
	v2.POST("/scheduler/drain", handler.Drain)
	v2.POST("/scheduler/resume", handler.Resume)
}

// setupTriggerRoutes configures V2 trigger-related endpoints.
func setupTriggerRoutes(v2 *gin.RouterGroup, handler *TriggersHandlerV2) {
	if handler == nil {
		return
	}

	// Webhook trigger endpoint
	v2.POST("/triggers/webhook", handler.HandleWebhook)
	v2.POST("/triggers/webhook/*path", handler.HandleWebhookPath)

	// Trigger management
	v2.GET("/triggers/status", handler.TriggerStatus)
	v2.GET("/triggers/webhooks", handler.ListWebhooks)
	v2.GET("/triggers/channels", handler.ListChannels)
	v2.POST("/triggers/webhooks", handler.RegisterWebhook)
	v2.POST("/triggers/channels", handler.RegisterChannel)
	v2.DELETE("/triggers/jobs/:id", handler.UnregisterJob)
}
