package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

// Crawler tool handlers

func (s *Server) handleStartCrawl(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		SourceID string `json:"source_id"`
		URL      string `json:"url"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.SourceID == "" || args.URL == "" {
		return s.errorResponse(id, InvalidParams, "source_id and url are required")
	}

	job, err := s.crawlerClient.CreateJob(ctx, client.CreateJobRequest{
		SourceID:        args.SourceID,
		URL:             args.URL,
		ScheduleEnabled: false,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create job: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"job_id":     job.ID,
		"source_id":  job.SourceID,
		"url":        job.URL,
		"status":     job.Status,
		"created_at": job.CreatedAt,
		"message":    "Crawl job created successfully. Job will run immediately.",
	})
}

func (s *Server) handleScheduleCrawl(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		SourceID        string `json:"source_id"`
		URL             string `json:"url"`
		IntervalMinutes int    `json:"interval_minutes"`
		IntervalType    string `json:"interval_type"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.SourceID == "" || args.URL == "" || args.IntervalMinutes <= 0 || args.IntervalType == "" {
		return s.errorResponse(id, InvalidParams, "source_id, url, interval_minutes, and interval_type are required")
	}

	// Validate interval_type enum
	validIntervalTypes := map[string]bool{
		"minutes": true,
		"hours":   true,
		"days":    true,
	}
	if !validIntervalTypes[args.IntervalType] {
		return s.errorResponse(id, InvalidParams, "interval_type must be 'minutes', 'hours', or 'days'")
	}

	job, err := s.crawlerClient.CreateJob(ctx, client.CreateJobRequest{
		SourceID:        args.SourceID,
		URL:             args.URL,
		ScheduleEnabled: true,
		IntervalMinutes: args.IntervalMinutes,
		IntervalType:    args.IntervalType,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create job: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"job_id":           job.ID,
		"source_id":        job.SourceID,
		"url":              job.URL,
		"status":           job.Status,
		"interval_minutes": job.IntervalMinutes,
		"interval_type":    job.IntervalType,
		"next_run_at":      job.NextRunAt,
		"created_at":       job.CreatedAt,
		"message":          fmt.Sprintf("Scheduled crawl job created. Runs every %d %s.", job.IntervalMinutes, job.IntervalType),
	})
}

func (s *Server) handleListCrawlJobs(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Status string `json:"status"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}

	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
		}
	}

	jobs, err := s.crawlerClient.ListJobs(ctx, args.Status)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list jobs: %v", err))
	}

	// Apply pagination in MCP layer (until backend supports it)
	total := len(jobs)
	limit := max(args.Limit, 0)
	if limit == 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)

	offset := max(args.Offset, 0)

	// Slice the results
	var paginatedJobs []client.Job
	if offset >= total {
		paginatedJobs = []client.Job{}
	} else {
		end := min(offset+limit, total)
		paginatedJobs = jobs[offset:end]
	}

	return s.successResponse(id, map[string]any{
		"jobs":   paginatedJobs,
		"count":  len(paginatedJobs),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleControlCrawlJob(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		JobID  string `json:"job_id"`
		Action string `json:"action"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.JobID == "" {
		return s.errorResponse(id, InvalidParams, "job_id is required")
	}

	if args.Action == "" {
		return s.errorResponse(id, InvalidParams, "action is required")
	}

	var job *client.Job
	var err error
	var actionMessage string

	switch args.Action {
	case "pause":
		job, err = s.crawlerClient.PauseJob(ctx, args.JobID)
		actionMessage = "paused"
	case "resume":
		job, err = s.crawlerClient.ResumeJob(ctx, args.JobID)
		actionMessage = "resumed"
	case "cancel":
		job, err = s.crawlerClient.CancelJob(ctx, args.JobID)
		actionMessage = "cancelled"
	default:
		return s.errorResponse(id, InvalidParams, "action must be 'pause', 'resume', or 'cancel'")
	}

	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to %s job: %v", args.Action, err))
	}

	return s.successResponse(id, map[string]any{
		"job_id":  job.ID,
		"status":  job.Status,
		"action":  args.Action,
		"message": fmt.Sprintf("Job %s successfully", actionMessage),
	})
}

func (s *Server) handleGetCrawlStats(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		JobID string `json:"job_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.JobID == "" {
		return s.errorResponse(id, InvalidParams, "job_id is required")
	}

	stats, err := s.crawlerClient.GetJobStats(ctx, args.JobID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get stats: %v", err))
	}

	return s.successResponse(id, stats)
}
