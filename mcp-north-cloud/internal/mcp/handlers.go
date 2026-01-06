package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

// Crawler tool handlers

func (s *Server) handleStartCrawl(id any, arguments json.RawMessage) *Response {
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

	job, err := s.crawlerClient.CreateJob(client.CreateJobRequest{
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

func (s *Server) handleScheduleCrawl(id any, arguments json.RawMessage) *Response {
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

	job, err := s.crawlerClient.CreateJob(client.CreateJobRequest{
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

func (s *Server) handleListCrawlJobs(id any, arguments json.RawMessage) *Response {
	var args struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		// Empty args is okay for listing all jobs
		args.Status = ""
	}

	jobs, err := s.crawlerClient.ListJobs(args.Status)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list jobs: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

func (s *Server) handlePauseCrawlJob(id any, arguments json.RawMessage) *Response {
	var args struct {
		JobID string `json:"job_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.JobID == "" {
		return s.errorResponse(id, InvalidParams, "job_id is required")
	}

	job, err := s.crawlerClient.PauseJob(args.JobID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to pause job: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "Job paused successfully",
	})
}

func (s *Server) handleResumeCrawlJob(id any, arguments json.RawMessage) *Response {
	var args struct {
		JobID string `json:"job_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.JobID == "" {
		return s.errorResponse(id, InvalidParams, "job_id is required")
	}

	job, err := s.crawlerClient.ResumeJob(args.JobID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to resume job: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "Job resumed successfully",
	})
}

func (s *Server) handleCancelCrawlJob(id any, arguments json.RawMessage) *Response {
	var args struct {
		JobID string `json:"job_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.JobID == "" {
		return s.errorResponse(id, InvalidParams, "job_id is required")
	}

	job, err := s.crawlerClient.CancelJob(args.JobID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to cancel job: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "Job cancelled successfully",
	})
}

func (s *Server) handleGetCrawlStats(id any, arguments json.RawMessage) *Response {
	var args struct {
		JobID string `json:"job_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.JobID == "" {
		return s.errorResponse(id, InvalidParams, "job_id is required")
	}

	stats, err := s.crawlerClient.GetJobStats(args.JobID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get stats: %v", err))
	}

	return s.successResponse(id, stats)
}

// Source Manager tool handlers

func (s *Server) handleAddSource(id any, arguments json.RawMessage) *Response {
	var args struct {
		Name      string         `json:"name"`
		URL       string         `json:"url"`
		Type      string         `json:"type"`
		Selectors map[string]any `json:"selectors"`
		Active    bool           `json:"active"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Name == "" || args.URL == "" || args.Type == "" || args.Selectors == nil {
		return s.errorResponse(id, InvalidParams, "name, url, type, and selectors are required")
	}

	source, err := s.sourceClient.CreateSource(client.CreateSourceRequest{
		Name:      args.Name,
		URL:       args.URL,
		Type:      args.Type,
		Selectors: args.Selectors,
		Active:    args.Active,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create source: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"source_id":  source.ID,
		"name":       source.Name,
		"url":        source.URL,
		"type":       source.Type,
		"active":     source.Active,
		"created_at": source.CreatedAt,
		"message":    "Source created successfully",
	})
}

func (s *Server) handleListSources(id any, _ json.RawMessage) *Response {
	sources, err := s.sourceClient.ListSources()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list sources: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"sources": sources,
		"count":   len(sources),
	})
}

func (s *Server) handleUpdateSource(id any, arguments json.RawMessage) *Response {
	var args struct {
		SourceID  string         `json:"source_id"`
		Name      string         `json:"name"`
		URL       string         `json:"url"`
		Selectors map[string]any `json:"selectors"`
		Active    *bool          `json:"active"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.SourceID == "" {
		return s.errorResponse(id, InvalidParams, "source_id is required")
	}

	source, err := s.sourceClient.UpdateSource(args.SourceID, client.UpdateSourceRequest{
		Name:      args.Name,
		URL:       args.URL,
		Selectors: args.Selectors,
		Active:    args.Active,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to update source: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"source_id":  source.ID,
		"name":       source.Name,
		"url":        source.URL,
		"active":     source.Active,
		"updated_at": source.UpdatedAt,
		"message":    "Source updated successfully",
	})
}

func (s *Server) handleDeleteSource(id any, arguments json.RawMessage) *Response {
	var args struct {
		SourceID string `json:"source_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.SourceID == "" {
		return s.errorResponse(id, InvalidParams, "source_id is required")
	}

	if err := s.sourceClient.DeleteSource(args.SourceID); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to delete source: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"source_id": args.SourceID,
		"message":   "Source deleted successfully",
	})
}

func (s *Server) handleTestSource(id any, arguments json.RawMessage) *Response {
	var args struct {
		URL       string         `json:"url"`
		Selectors map[string]any `json:"selectors"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.URL == "" || args.Selectors == nil {
		return s.errorResponse(id, InvalidParams, "url and selectors are required")
	}

	result, err := s.sourceClient.TestCrawl(client.TestCrawlRequest{
		URL:       args.URL,
		Selectors: args.Selectors,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to test source: %v", err))
	}

	return s.successResponse(id, result)
}

// Publisher tool handlers

func (s *Server) handleCreateRoute(id any, arguments json.RawMessage) *Response {
	var args struct {
		SourceID        string   `json:"source_id"`
		ChannelID       string   `json:"channel_id"`
		MinQualityScore int      `json:"min_quality_score"`
		Topics          []string `json:"topics"`
		Active          bool     `json:"active"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.SourceID == "" || args.ChannelID == "" {
		return s.errorResponse(id, InvalidParams, "source_id and channel_id are required")
	}

	route, err := s.publisherClient.CreateRoute(client.CreateRouteRequest{
		SourceID:        args.SourceID,
		ChannelID:       args.ChannelID,
		MinQualityScore: args.MinQualityScore,
		Topics:          args.Topics,
		Active:          args.Active,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create route: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"route_id":          route.ID,
		"source_id":         route.SourceID,
		"channel_id":        route.ChannelID,
		"min_quality_score": route.MinQualityScore,
		"topics":            route.Topics,
		"active":            route.Active,
		"created_at":        route.CreatedAt,
		"message":           "Publishing route created successfully",
	})
}

func (s *Server) handleListRoutes(id any, arguments json.RawMessage) *Response {
	var args struct {
		SourceID  string `json:"source_id"`
		ChannelID string `json:"channel_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		// Empty args is okay
		args.SourceID = ""
		args.ChannelID = ""
	}

	routes, err := s.publisherClient.ListRoutes(args.SourceID, args.ChannelID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list routes: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"routes": routes,
		"count":  len(routes),
	})
}

func (s *Server) handleDeleteRoute(id any, arguments json.RawMessage) *Response {
	var args struct {
		RouteID string `json:"route_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.RouteID == "" {
		return s.errorResponse(id, InvalidParams, "route_id is required")
	}

	if err := s.publisherClient.DeleteRoute(args.RouteID); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to delete route: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"route_id": args.RouteID,
		"message":  "Route deleted successfully",
	})
}

func (s *Server) handlePreviewRoute(id any, arguments json.RawMessage) *Response {
	var args struct {
		RouteID string `json:"route_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.RouteID == "" {
		return s.errorResponse(id, InvalidParams, "route_id is required")
	}

	articles, err := s.publisherClient.PreviewRoute(args.RouteID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to preview route: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"articles": articles,
		"count":    len(articles),
	})
}

func (s *Server) handleGetPublishHistory(id any, arguments json.RawMessage) *Response {
	var args struct {
		ChannelName string `json:"channel_name"`
		Limit       int    `json:"limit"`
		Offset      int    `json:"offset"`
	}

	_ = json.Unmarshal(arguments, &args) // Empty args is okay, use defaults

	history, err := s.publisherClient.GetPublishHistory(args.ChannelName, args.Limit, args.Offset)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get publish history: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"history": history,
		"count":   len(history),
	})
}

func (s *Server) handleGetPublisherStats(id any, _ json.RawMessage) *Response {
	stats, err := s.publisherClient.GetStats()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get stats: %v", err))
	}

	return s.successResponse(id, stats)
}

// Search tool handlers

func (s *Server) handleSearchArticles(id any, arguments json.RawMessage) *Response {
	var args client.SearchRequest

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Query == "" {
		return s.errorResponse(id, InvalidParams, "query is required")
	}

	result, err := s.searchClient.Search(args)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to search: %v", err))
	}

	return s.successResponse(id, result)
}

// Classifier tool handlers

func (s *Server) handleClassifyArticle(id any, arguments json.RawMessage) *Response {
	var args client.ClassifyRequest

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Title == "" || args.RawText == "" || args.URL == "" {
		return s.errorResponse(id, InvalidParams, "title, raw_text, and url are required")
	}

	result, err := s.classifierClient.Classify(args)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to classify article: %v", err))
	}

	return s.successResponse(id, result)
}

// Index Manager tool handlers

func (s *Server) handleListIndexes(id any, _ json.RawMessage) *Response {
	indexes, err := s.indexClient.ListIndices()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list indexes: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"indexes": indexes,
		"count":   len(indexes),
	})
}

// Helper methods

func (s *Server) successResponse(id, data any) *Response {
	result := map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": formatResult(data),
			},
		},
		"isError": false,
	}

	resultJSON, _ := json.Marshal(result)

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  json.RawMessage(resultJSON),
	}
}

func (s *Server) errorResponse(id any, code int, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ErrorObject{
			Code:    code,
			Message: message,
		},
	}
}

func formatResult(data any) string {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(jsonData)
}
