// Package api implements the HTTP API for the search service.
package api

// SearchRequest represents the structure of the search request
type SearchRequest struct {
	Query string `json:"query"`
	Index string `json:"index"`
	Size  int    `json:"size"`
}

// SearchResponse represents the structure of the search response
type SearchResponse struct {
	Results []any `json:"results"`
	Total   int   `json:"total"`
}

// APIError represents an error response from the API.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *APIError) Error() string {
	return e.Message
}

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

// JobsListResponse represents a list of jobs response.
type JobsListResponse struct {
	Jobs  []any `json:"jobs"`
	Total int   `json:"total"`
}
