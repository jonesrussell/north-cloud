// Package api implements the HTTP API for the crawler service.
package api

// CreateJobRequest represents a job creation request.
type CreateJobRequest struct {
	SourceID        string `json:"source_id" binding:"required"`
	SourceName      string `json:"source_name"`
	URL             string `json:"url" binding:"required"`
	ScheduleTime    string `json:"schedule_time"`
	ScheduleEnabled bool   `json:"schedule_enabled"`
}

// UpdateJobRequest represents a job update request.
type UpdateJobRequest struct {
	SourceID        string `json:"source_id"`
	SourceName      string `json:"source_name"`
	URL             string `json:"url"`
	ScheduleTime    string `json:"schedule_time"`
	ScheduleEnabled *bool  `json:"schedule_enabled"`
	Status          string `json:"status"`
}
