package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

// Auth tool handlers

func (s *Server) handleGetAuthToken(ctx context.Context, id any, _ json.RawMessage) *Response {
	token, expiresAt, err := s.authClient.GenerateToken()
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to generate token: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"token":      token,
		"expires_at": expiresAt.Format("2006-01-02T15:04:05Z"),
		"usage":      "curl -H 'Authorization: Bearer <token>' http://SERVICE:PORT/api/v1/...",
		"message":    "Token generated successfully. Valid for 24 hours.",
	})
}

// Workflow tool handlers (high-level, multi-service)

const defaultMinQualityScore = 50

// onboardSourceArgs holds the arguments for onboarding a source
type onboardSourceArgs struct {
	Name                 string         `json:"name"`
	URL                  string         `json:"url"`
	SourceType           string         `json:"source_type"`
	Selectors            map[string]any `json:"selectors"`
	CrawlIntervalMinutes int            `json:"crawl_interval_minutes"`
	CrawlIntervalType    string         `json:"crawl_interval_type"`
	ChannelID            string         `json:"channel_id"`
	MinQualityScore      int            `json:"min_quality_score"`
	Topics               []string       `json:"topics"`
}

func (s *Server) handleOnboardSource(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args onboardSourceArgs
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if errResp := s.validateOnboardArgs(id, args); errResp != nil {
		return errResp
	}

	return s.executeOnboardWorkflow(ctx, id, args)
}

func (s *Server) validateOnboardArgs(id any, args onboardSourceArgs) *Response {
	if args.Name == "" || args.URL == "" || args.SourceType == "" || args.Selectors == nil {
		return s.errorResponse(id, InvalidParams, "name, url, source_type, and selectors are required")
	}
	if args.CrawlIntervalMinutes > 0 && args.CrawlIntervalType == "" {
		return s.errorResponse(id, InvalidParams, "crawl_interval_type is required when crawl_interval_minutes is set")
	}
	return nil
}

func (s *Server) executeOnboardWorkflow(ctx context.Context, id any, args onboardSourceArgs) *Response {
	const maxOnboardSteps = 3
	result := map[string]any{}
	stepsCompleted := make([]string, 0, maxOnboardSteps)

	// Step 1: Create the source
	source, err := s.sourceClient.CreateSource(ctx, client.CreateSourceRequest{
		Name:      args.Name,
		URL:       args.URL,
		Type:      args.SourceType,
		Selectors: args.Selectors,
		Active:    true,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create source: %v", err))
	}
	result["source_id"] = source.ID
	result["source_name"] = source.Name
	stepsCompleted = append(stepsCompleted, "source_created")

	// Step 2: Start or schedule crawl
	stepsCompleted, err = s.onboardCrawlStep(ctx, args, source.ID, result, stepsCompleted)
	if err != nil {
		result["crawl_error"] = err.Error()
		result["steps_completed"] = stepsCompleted
		return s.successResponse(id, result)
	}

	// Step 3: Create publishing route (optional)
	if args.ChannelID != "" {
		stepsCompleted, err = s.onboardRouteStep(ctx, args, result, stepsCompleted)
		if err != nil {
			result["route_error"] = err.Error()
			result["steps_completed"] = stepsCompleted
			return s.successResponse(id, result)
		}
	}

	result["steps_completed"] = stepsCompleted
	result["message"] = fmt.Sprintf("Source '%s' onboarded successfully with %d steps completed", args.Name, len(stepsCompleted))
	return s.successResponse(id, result)
}

func (s *Server) onboardCrawlStep(
	ctx context.Context, args onboardSourceArgs, sourceID string, result map[string]any, steps []string,
) ([]string, error) {
	var job *client.Job
	var err error

	if args.CrawlIntervalMinutes > 0 {
		job, err = s.crawlerClient.CreateJob(ctx, client.CreateJobRequest{
			SourceID:        sourceID,
			URL:             args.URL,
			ScheduleEnabled: true,
			IntervalMinutes: args.CrawlIntervalMinutes,
			IntervalType:    args.CrawlIntervalType,
		})
		if err != nil {
			return steps, err
		}
		result["crawl_scheduled"] = true
		result["crawl_interval"] = fmt.Sprintf("%d %s", args.CrawlIntervalMinutes, args.CrawlIntervalType)
		steps = append(steps, "crawl_scheduled")
	} else {
		job, err = s.crawlerClient.CreateJob(ctx, client.CreateJobRequest{
			SourceID:        sourceID,
			URL:             args.URL,
			ScheduleEnabled: false,
		})
		if err != nil {
			return steps, err
		}
		result["crawl_scheduled"] = false
		steps = append(steps, "crawl_started")
	}

	result["job_id"] = job.ID
	result["job_status"] = job.Status
	return steps, nil
}

func (s *Server) onboardRouteStep(ctx context.Context, args onboardSourceArgs, result map[string]any, steps []string) ([]string, error) {
	// First, create a publisher source (Elasticsearch index mapping)
	// Sanitize name: lowercase, replace spaces/special chars with underscores
	sanitizedName := sanitizeSourceName(args.Name)
	indexPattern := sanitizedName + "_classified_content"

	pubSource, err := s.publisherClient.CreatePublisherSource(ctx, client.CreatePublisherSourceRequest{
		Name:         sanitizedName,
		IndexPattern: indexPattern,
	})
	if err != nil {
		return steps, fmt.Errorf("failed to create publisher source: %w", err)
	}
	result["publisher_source_id"] = pubSource.ID
	result["index_pattern"] = indexPattern
	steps = append(steps, "publisher_source_created")

	// Now create the route using the publisher source ID
	minQuality := args.MinQualityScore
	if minQuality == 0 {
		minQuality = defaultMinQualityScore
	}

	route, err := s.publisherClient.CreateRoute(ctx, client.CreateRouteRequest{
		SourceID:        pubSource.ID,
		ChannelID:       args.ChannelID,
		MinQualityScore: minQuality,
		Topics:          args.Topics,
		Active:          true,
	})
	if err != nil {
		return steps, fmt.Errorf("failed to create route: %w", err)
	}

	result["route_id"] = route.ID
	result["channel_id"] = args.ChannelID
	return append(steps, "route_created"), nil
}

// sanitizeSourceName converts a source name to a valid index prefix
// e.g., "My News Site" → "my_news_site"
func sanitizeSourceName(name string) string {
	// Lowercase
	result := strings.ToLower(name)
	// Replace spaces and special chars with underscores
	replacer := strings.NewReplacer(
		" ", "_",
		"-", "_",
		".", "_",
		"/", "_",
		"\\", "_",
	)
	result = replacer.Replace(result)
	// Remove consecutive underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	// Trim underscores from ends
	result = strings.Trim(result, "_")
	return result
}

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

const (
	defaultLimit = 20
	maxLimit     = 100
)

func (s *Server) handleListCrawlJobs(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Status string `json:"status"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}

	_ = json.Unmarshal(arguments, &args) // Empty args is okay

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

// Source Manager tool handlers

func (s *Server) handleAddSource(ctx context.Context, id any, arguments json.RawMessage) *Response {
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

	source, err := s.sourceClient.CreateSource(ctx, client.CreateSourceRequest{
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

func (s *Server) handleListSources(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}

	_ = json.Unmarshal(arguments, &args) // Empty args is okay

	sources, err := s.sourceClient.ListSources(ctx)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list sources: %v", err))
	}

	// Apply pagination in MCP layer
	total := len(sources)
	limit := max(args.Limit, 0)
	if limit == 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)

	offset := max(args.Offset, 0)

	var paginatedSources []client.Source
	if offset >= total {
		paginatedSources = []client.Source{}
	} else {
		end := min(offset+limit, total)
		paginatedSources = sources[offset:end]
	}

	return s.successResponse(id, map[string]any{
		"sources": paginatedSources,
		"count":   len(paginatedSources),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (s *Server) handleUpdateSource(ctx context.Context, id any, arguments json.RawMessage) *Response {
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

	source, err := s.sourceClient.UpdateSource(ctx, args.SourceID, client.UpdateSourceRequest{
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

func (s *Server) handleDeleteSource(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		SourceID string `json:"source_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.SourceID == "" {
		return s.errorResponse(id, InvalidParams, "source_id is required")
	}

	if err := s.sourceClient.DeleteSource(ctx, args.SourceID); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to delete source: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"source_id": args.SourceID,
		"message":   "Source deleted successfully",
	})
}

func (s *Server) handleTestSource(ctx context.Context, id any, arguments json.RawMessage) *Response {
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

	result, err := s.sourceClient.TestCrawl(ctx, client.TestCrawlRequest{
		URL:       args.URL,
		Selectors: args.Selectors,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to test source: %v", err))
	}

	return s.successResponse(id, result)
}

// Publisher tool handlers

func (s *Server) handleCreateRoute(ctx context.Context, id any, arguments json.RawMessage) *Response {
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

	route, err := s.publisherClient.CreateRoute(ctx, client.CreateRouteRequest{
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

func (s *Server) handleListRoutes(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		SourceID  string `json:"source_id"`
		ChannelID string `json:"channel_id"`
		Limit     int    `json:"limit"`
		Offset    int    `json:"offset"`
	}

	_ = json.Unmarshal(arguments, &args) // Empty args is okay

	routes, err := s.publisherClient.ListRoutes(ctx, args.SourceID, args.ChannelID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list routes: %v", err))
	}

	// Apply pagination in MCP layer
	total := len(routes)
	limit := max(args.Limit, 0)
	if limit == 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)

	offset := max(args.Offset, 0)

	if offset >= total {
		routes = []client.Route{}
	} else {
		end := min(offset+limit, total)
		routes = routes[offset:end]
	}

	return s.successResponse(id, map[string]any{
		"routes": routes,
		"count":  len(routes),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleCreateChannel(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Enabled     *bool  `json:"enabled"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Name == "" {
		return s.errorResponse(id, InvalidParams, "name is required")
	}

	channel, err := s.publisherClient.CreateChannel(ctx, client.CreateChannelRequest{
		Name:        args.Name,
		Description: args.Description,
		Enabled:     args.Enabled,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create channel: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"channel_id":  channel.ID,
		"name":        channel.Name,
		"description": channel.Description,
		"enabled":     channel.Active,
		"created_at":  channel.CreatedAt,
		"message":     "Channel created successfully",
	})
}

func (s *Server) handleListChannels(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ActiveOnly bool `json:"active_only"`
	}

	_ = json.Unmarshal(arguments, &args) // Empty args is okay

	channels, err := s.publisherClient.ListChannels(ctx)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list channels: %v", err))
	}

	// Filter by active if requested
	if args.ActiveOnly {
		filtered := make([]client.Channel, 0, len(channels))
		for i := range channels {
			if channels[i].Active {
				filtered = append(filtered, channels[i])
			}
		}
		channels = filtered
	}

	return s.successResponse(id, map[string]any{
		"channels": channels,
		"count":    len(channels),
	})
}

func (s *Server) handleDeleteRoute(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		RouteID string `json:"route_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.RouteID == "" {
		return s.errorResponse(id, InvalidParams, "route_id is required")
	}

	if err := s.publisherClient.DeleteRoute(ctx, args.RouteID); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to delete route: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"route_id": args.RouteID,
		"message":  "Route deleted successfully",
	})
}

func (s *Server) handlePreviewRoute(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		RouteID string `json:"route_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.RouteID == "" {
		return s.errorResponse(id, InvalidParams, "route_id is required")
	}

	articles, err := s.publisherClient.PreviewRoute(ctx, args.RouteID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to preview route: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"articles": articles,
		"count":    len(articles),
	})
}

func (s *Server) handleGetPublishHistory(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ChannelName string `json:"channel_name"`
		Limit       int    `json:"limit"`
		Offset      int    `json:"offset"`
	}

	_ = json.Unmarshal(arguments, &args) // Empty args is okay, use defaults

	// Apply limit/offset defaults and cap (Phase E: response size safeguard)
	limit := max(args.Limit, 0)
	if limit == 0 {
		limit = 50 // Tool schema default for publish history
	}
	limit = min(limit, maxLimit)
	offset := max(args.Offset, 0)

	history, err := s.publisherClient.GetPublishHistory(ctx, args.ChannelName, limit, offset)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get publish history: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"history": history,
		"count":   len(history),
		"limit":   limit,
		"offset":  offset,
	})
}

func (s *Server) handleGetPublisherStats(ctx context.Context, id any, _ json.RawMessage) *Response {
	stats, err := s.publisherClient.GetStats(ctx)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get stats: %v", err))
	}

	return s.successResponse(id, stats)
}

// Search tool handlers

func (s *Server) handleSearchArticles(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args client.SearchRequest

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Query == "" {
		return s.errorResponse(id, InvalidParams, "query is required")
	}

	result, err := s.searchClient.Search(ctx, args)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to search: %v", err))
	}

	return s.successResponse(id, result)
}

// Classifier tool handlers

func (s *Server) handleClassifyArticle(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args client.ClassifyRequest

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Title == "" || args.RawText == "" || args.URL == "" {
		return s.errorResponse(id, InvalidParams, "title, raw_text, and url are required")
	}

	result, err := s.classifierClient.Classify(ctx, args)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to classify article: %v", err))
	}

	return s.successResponse(id, result)
}

// Index Manager tool handlers

func (s *Server) handleListIndexes(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}

	_ = json.Unmarshal(arguments, &args) // Empty args is okay

	indexes, err := s.indexClient.ListIndices(ctx)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list indexes: %v", err))
	}

	// Apply pagination in MCP layer
	total := len(indexes)
	limit := max(args.Limit, 0)
	if limit == 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)

	offset := max(args.Offset, 0)

	var paginatedIndexes []string
	if offset >= total {
		paginatedIndexes = []string{}
	} else {
		end := min(offset+limit, total)
		paginatedIndexes = indexes[offset:end]
	}

	return s.successResponse(id, map[string]any{
		"indexes": paginatedIndexes,
		"count":   len(paginatedIndexes),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// Development tool handlers

const taskTest = "test"

func (s *Server) handleLintFile(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		FilePath    string `json:"file_path"`
		ServiceName string `json:"service_name"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	projectRoot := s.detectProjectRoot()
	if projectRoot == "" {
		return s.errorResponse(id, InvalidParams, "Could not determine project root")
	}

	var serviceDir string
	var lintCommand *exec.Cmd
	var lintType string
	var setupErr error

	if args.ServiceName != "" {
		serviceDir, lintCommand, lintType, setupErr = s.setupServiceLint(ctx, projectRoot, args.ServiceName)
	} else if args.FilePath != "" {
		serviceDir, lintCommand, lintType, setupErr = s.setupFileLint(ctx, projectRoot, args.FilePath)
	} else {
		return s.errorResponse(id, InvalidParams, "Either file_path or service_name must be provided")
	}

	if setupErr != nil {
		return s.errorResponse(id, InvalidParams, setupErr.Error())
	}

	return s.executeLintCommand(id, lintCommand, lintType, serviceDir)
}

// detectProjectRoot determines the project root directory
func (s *Server) detectProjectRoot() string {
	projectRoot := "/home/jones/dev/north-cloud"
	if _, statErr := os.Stat(projectRoot); os.IsNotExist(statErr) {
		cwd, getwdErr := os.Getwd()
		if getwdErr != nil {
			return ""
		}
		for {
			if _, statErr2 := os.Stat(filepath.Join(cwd, ".cursor")); statErr2 == nil {
				return cwd
			}
			parent := filepath.Dir(cwd)
			if parent == cwd {
				break
			}
			cwd = parent
		}
		return ""
	}
	return projectRoot
}

// setupServiceLint configures linting for an entire service
func (s *Server) setupServiceLint(ctx context.Context, projectRoot, serviceName string) (
	serviceDir string, lintCommand *exec.Cmd, lintType string, err error,
) {
	serviceDir = filepath.Join(projectRoot, serviceName)
	if _, statErr := os.Stat(filepath.Join(serviceDir, "package.json")); statErr == nil {
		cmd := exec.CommandContext(ctx, "npm", "run", "lint")
		cmd.Dir = serviceDir
		return serviceDir, cmd, "frontend", nil
	}

	if _, statErr := os.Stat(filepath.Join(serviceDir, "Taskfile.yml")); statErr == nil {
		cmd := exec.CommandContext(ctx, "task", "lint")
		cmd.Dir = serviceDir
		return serviceDir, cmd, "go", nil
	}

	return "", nil, "", fmt.Errorf("service '%s' not found or doesn't have a lint configuration", serviceName)
}

// setupFileLint configures linting for a specific file
func (s *Server) setupFileLint(ctx context.Context, projectRoot, filePath string) (
	serviceDir string, lintCommand *exec.Cmd, lintType string, err error,
) {
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(projectRoot, filePath)
	}

	relPath, relErr := filepath.Rel(projectRoot, filePath)
	if relErr != nil {
		return "", nil, "", fmt.Errorf("file path '%s' is not within project root", filePath)
	}

	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) == 0 {
		return "", nil, "", errors.New("could not determine service from file path")
	}

	serviceDir = filepath.Join(projectRoot, parts[0])
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".go":
		return s.setupGoFileLint(ctx, serviceDir, parts[0])
	case ".vue", ".ts", ".js", ".tsx", ".jsx":
		return s.setupFrontendFileLint(ctx, serviceDir, parts[0])
	default:
		return "", nil, "", fmt.Errorf(
			"file type '%s' is not supported for linting. Supported: .go, .vue, .ts, .js, .tsx, .jsx",
			ext,
		)
	}
}

// setupGoFileLint configures linting for a Go file
func (s *Server) setupGoFileLint(ctx context.Context, serviceDir, serviceName string) (
	dir string, lintCommand *exec.Cmd, lintType string, err error,
) {
	if _, statErr := os.Stat(filepath.Join(serviceDir, "Taskfile.yml")); statErr == nil {
		cmd := exec.CommandContext(ctx, "task", "lint")
		cmd.Dir = serviceDir
		return serviceDir, cmd, "go", nil
	}
	return "", nil, "", fmt.Errorf("service directory '%s' doesn't have a Taskfile.yml for linting", serviceName)
}

// setupFrontendFileLint configures linting for a frontend file
func (s *Server) setupFrontendFileLint(ctx context.Context, serviceDir, serviceName string) (
	dir string, lintCommand *exec.Cmd, lintType string, err error,
) {
	if _, statErr := os.Stat(filepath.Join(serviceDir, "package.json")); statErr == nil {
		cmd := exec.CommandContext(ctx, "npm", "run", "lint")
		cmd.Dir = serviceDir
		return serviceDir, cmd, "frontend", nil
	}
	return "", nil, "", fmt.Errorf("service directory '%s' doesn't have a package.json for linting", serviceName)
}

// executeLintCommand runs the lint command and returns the response
func (s *Server) executeLintCommand(id any, lintCommand *exec.Cmd, lintType, serviceDir string) *Response {
	output, err := lintCommand.CombinedOutput()
	outputStr := string(output)
	displayOutput, outputTruncated, outputTotalBytes := truncateCommandOutput(outputStr, maxCommandOutputBytes)

	result := map[string]any{
		"lint_type":   lintType,
		"service_dir": serviceDir,
		"command":     strings.Join(lintCommand.Args, " "),
		"output":      displayOutput,
		"success":     err == nil,
	}
	if outputTruncated {
		result["output_truncated"] = true
		result["output_total_bytes"] = outputTotalBytes
	}
	if err != nil {
		result["error"] = err.Error()
		if lintCommand.ProcessState != nil {
			result["exit_code"] = lintCommand.ProcessState.ExitCode()
		} else {
			result["exit_code"] = -1
		}
	}

	return s.successResponse(id, result)
}

func (s *Server) handleBuildService(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ServiceName string `json:"service_name"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}
	if args.ServiceName == "" {
		return s.errorResponse(id, InvalidParams, "service_name is required")
	}
	return s.executeBuildTestCommand(ctx, id, args.ServiceName, "build", false)
}

func (s *Server) handleTestService(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ServiceName  string `json:"service_name"`
		WithCoverage bool   `json:"with_coverage"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}
	if args.ServiceName == "" {
		return s.errorResponse(id, InvalidParams, "service_name is required")
	}
	taskName := taskTest
	if args.WithCoverage {
		taskName = "test:coverage"
	}
	return s.executeBuildTestCommand(ctx, id, args.ServiceName, taskName, true)
}

// maxCommandOutputBytes caps the output field size in build/test/lint responses to avoid large MCP payloads.
const maxCommandOutputBytes = 6000

// truncateCommandOutput returns output truncated to the last maxBytes characters when over limit.
// It preserves the tail so failure messages and stack traces remain visible.
// truncationNoteReserve is the number of bytes reserved for the truncation note prefix.
const truncationNoteReserve = 60

// Returns (possibly truncated output, wasTruncated, original length in bytes).
func truncateCommandOutput(s string, maxBytes int) (out string, truncated bool, totalBytes int) {
	totalBytes = len(s)
	if totalBytes <= maxBytes {
		return s, false, totalBytes
	}
	tailStart := totalBytes - (maxBytes - truncationNoteReserve)
	if tailStart < 0 {
		tailStart = 0
	}
	note := fmt.Sprintf("... (output truncated, showing last %d of %d bytes)\n", totalBytes-tailStart, totalBytes)
	return note + s[tailStart:], true, totalBytes
}

// executeBuildTestCommand runs build or test for a service and returns structured output.
func (s *Server) executeBuildTestCommand(ctx context.Context, id any, serviceName, taskName string, isTest bool) *Response {
	projectRoot := s.detectProjectRoot()
	if projectRoot == "" {
		return s.errorResponse(id, InvalidParams, "Could not determine project root")
	}
	serviceDir := filepath.Join(projectRoot, serviceName)

	var cmd *exec.Cmd
	var isGo bool
	if _, statErr := os.Stat(filepath.Join(serviceDir, "Taskfile.yml")); statErr == nil {
		_, goModErr := os.Stat(filepath.Join(serviceDir, "go.mod"))
		isGo = (goModErr == nil)
		// Frontend services (dashboard, search-frontend) have Taskfile but no test:coverage
		runTask := taskName
		if taskName == "test:coverage" && !isGo {
			runTask = taskTest
		}
		cmd = exec.CommandContext(ctx, "task", runTask)
		cmd.Dir = serviceDir
	} else if _, pkgJSONErr := os.Stat(filepath.Join(serviceDir, "package.json")); pkgJSONErr == nil {
		isGo = false
		script := "build"
		if isTest {
			script = taskTest
		}
		cmd = exec.CommandContext(ctx, "npm", "run", script)
		cmd.Dir = serviceDir
	} else {
		return s.errorResponse(id, InvalidParams,
			fmt.Sprintf("service '%s' not found or doesn't have Taskfile.yml or package.json", serviceName))
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	displayOutput, outputTruncated, outputTotalBytes := truncateCommandOutput(outputStr, maxCommandOutputBytes)

	result := map[string]any{
		"success": err == nil,
		"service": serviceName,
		"command": strings.Join(cmd.Args, " "),
		"output":  displayOutput,
	}
	if outputTruncated {
		result["output_truncated"] = true
		result["output_total_bytes"] = outputTotalBytes
	}
	populateBuildTestErrorResult(result, cmd, err, outputStr, isGo)

	return s.successResponse(id, result)
}

func populateBuildTestErrorResult(result map[string]any, cmd *exec.Cmd, cmdErr error, outputStr string, isGo bool) {
	if cmdErr == nil {
		return
	}
	result["error"] = cmdErr.Error()
	if cmd.ProcessState != nil {
		result["exit_code"] = cmd.ProcessState.ExitCode()
	} else {
		result["exit_code"] = -1
	}
	if isGo {
		if parsed := parseGoErrors(outputStr); len(parsed) > 0 {
			result["errors"] = parsed
		}
	}
}

// goError represents a parsed Go compiler or test error.
type goError struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column,omitempty"`
	Message string `json:"message"`
}

// parseGoErrors extracts file:line:column: message patterns from Go build/test output.
func parseGoErrors(output string) []goError {
	// Matches: path/to/file.go:42:5: message or path/to/file.go:42: message
	re := regexp.MustCompile(`(?m)^([^:]+):(\d+)(?::(\d+))?:\s*(.+)$`)
	matches := re.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return nil
	}
	errs := make([]goError, 0, len(matches))
	for _, m := range matches {
		e := goError{File: m[1], Message: m[4]}
		if line, err := strconv.Atoi(m[2]); err == nil {
			e.Line = line
		}
		if m[3] != "" {
			if col, colErr := strconv.Atoi(m[3]); colErr == nil {
				e.Column = col
			}
		}
		errs = append(errs, e)
	}
	return errs
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

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to marshal result: %v", err))
	}

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
