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

// onboardSourceArgs holds the arguments for onboarding a source
type onboardSourceArgs struct {
	Name                 string         `json:"name"`
	URL                  string         `json:"url"`
	SourceType           string         `json:"source_type"`
	Selectors            map[string]any `json:"selectors"`
	CrawlIntervalMinutes int            `json:"crawl_interval_minutes"`
	CrawlIntervalType    string         `json:"crawl_interval_type"`
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
	const maxOnboardSteps = 2
	result := map[string]any{}
	stepsCompleted := make([]string, 0, maxOnboardSteps)

	// Step 1: Create the source
	source, err := s.sourceClient.CreateSource(ctx, client.CreateSourceRequest{
		Name:      args.Name,
		URL:       args.URL,
		Type:      args.SourceType,
		Selectors: args.Selectors,
		Enabled:   true,
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

// Source Manager tool handlers

func (s *Server) handleAddSource(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Name      string         `json:"name"`
		URL       string         `json:"url"`
		Type      string         `json:"type"`
		Selectors map[string]any `json:"selectors"`
		Active    bool           `json:"active"`
		FeedURL   *string        `json:"feed_url"`
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
		Enabled:   args.Active,
		FeedURL:   args.FeedURL,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create source: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"source_id":  source.ID,
		"name":       source.Name,
		"url":        source.URL,
		"type":       source.Type,
		"active":     source.Enabled,
		"created_at": source.CreatedAt,
		"message":    "Source created successfully",
	})
}

func (s *Server) handleListSources(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}

	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
		}
	}

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

	// Return compact summaries (omit selectors and timestamps to reduce token usage)
	summaries := make([]map[string]any, 0, len(paginatedSources))
	for i := range paginatedSources {
		summary := map[string]any{
			"id":     paginatedSources[i].ID,
			"name":   paginatedSources[i].Name,
			"url":    paginatedSources[i].URL,
			"type":   paginatedSources[i].Type,
			"active": paginatedSources[i].Enabled,
		}
		if paginatedSources[i].FeedURL != nil {
			summary["feed_url"] = *paginatedSources[i].FeedURL
		}
		summaries = append(summaries, summary)
	}

	return s.successResponse(id, map[string]any{
		"sources": summaries,
		"count":   len(summaries),
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
		FeedURL   *string        `json:"feed_url"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.SourceID == "" {
		return s.errorResponse(id, InvalidParams, "source_id is required")
	}

	// GET current source first (source-manager PUT does full replacement)
	req, mergeErr := s.buildMergedUpdateRequest(ctx, args.SourceID, args.Name, args.URL, args.Selectors, args.Active, args.FeedURL)
	if mergeErr != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get current source: %v", mergeErr))
	}

	source, err := s.sourceClient.UpdateSource(ctx, args.SourceID, req)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to update source: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"source_id":  source.ID,
		"name":       source.Name,
		"url":        source.URL,
		"active":     source.Enabled,
		"updated_at": source.UpdatedAt,
		"message":    "Source updated successfully",
	})
}

// buildMergedUpdateRequest fetches the current source and merges only the provided fields,
// since source-manager PUT does full replacement.
func (s *Server) buildMergedUpdateRequest(
	ctx context.Context, sourceID, name, url string, selectors map[string]any, active *bool, feedURL *string,
) (client.UpdateSourceRequest, error) {
	current, err := s.sourceClient.GetSource(ctx, sourceID)
	if err != nil {
		return client.UpdateSourceRequest{}, err
	}

	req := client.UpdateSourceRequest{
		Name:      current.Name,
		URL:       current.URL,
		Type:      current.Type,
		Selectors: current.Selectors,
		FeedURL:   current.FeedURL,
	}
	currentEnabled := current.Enabled
	req.Enabled = &currentEnabled

	if name != "" {
		req.Name = name
	}
	if url != "" {
		req.URL = url
	}
	if selectors != nil {
		req.Selectors = selectors
	}
	if active != nil {
		req.Enabled = active
	}
	if feedURL != nil {
		req.FeedURL = feedURL
	}

	return req, nil
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

// Community tool handlers

func (s *Server) handleListCommunities(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}

	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
		}
	}

	limit := max(args.Limit, 0)
	if limit == 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	offset := max(args.Offset, 0)

	communities, total, err := s.sourceClient.ListCommunities(ctx, limit, offset)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list communities: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"communities": communities,
		"count":       len(communities),
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	})
}

func (s *Server) handleGetCommunity(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		CommunityID string `json:"community_id"`
		Slug        string `json:"slug"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.CommunityID == "" && args.Slug == "" {
		return s.errorResponse(id, InvalidParams, "either community_id or slug is required")
	}

	var community *client.Community
	var err error

	if args.CommunityID != "" {
		community, err = s.sourceClient.GetCommunity(ctx, args.CommunityID)
	} else {
		community, err = s.sourceClient.GetCommunityBySlug(ctx, args.Slug)
	}

	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get community: %v", err))
	}

	return s.successResponse(id, community)
}

const (
	defaultNearbyRadiusKm = 50.0
	defaultNearbyLimit    = 10
)

func (s *Server) handleFindNearbyCommunities(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Lat      float64 `json:"lat"`
		Lng      float64 `json:"lng"`
		RadiusKm float64 `json:"radius_km"`
		Limit    int     `json:"limit"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Lat == 0 && args.Lng == 0 {
		return s.errorResponse(id, InvalidParams, "lat and lng are required")
	}

	if args.RadiusKm <= 0 {
		args.RadiusKm = defaultNearbyRadiusKm
	}
	if args.Limit <= 0 {
		args.Limit = defaultNearbyLimit
	}

	communities, err := s.sourceClient.FindNearbyCommunities(ctx, args.Lat, args.Lng, args.RadiusKm, args.Limit)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to find nearby communities: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"communities": communities,
		"count":       len(communities),
		"radius_km":   args.RadiusKm,
	})
}

func (s *Server) handleAddCommunity(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args client.Community

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Name == "" || args.Slug == "" {
		return s.errorResponse(id, InvalidParams, "name and slug are required")
	}

	community, err := s.sourceClient.CreateCommunity(ctx, args)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create community: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"community_id": community.ID,
		"name":         community.Name,
		"slug":         community.Slug,
		"message":      "Community created successfully",
	})
}

func (s *Server) handleUpdateCommunity(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		CommunityID string `json:"community_id"`
		client.Community
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.CommunityID == "" {
		return s.errorResponse(id, InvalidParams, "community_id is required")
	}

	community, err := s.sourceClient.UpdateCommunity(ctx, args.CommunityID, args.Community)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to update community: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"community_id": community.ID,
		"name":         community.Name,
		"slug":         community.Slug,
		"message":      "Community updated successfully",
	})
}

func (s *Server) handleLinkSources(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		DryRun *bool `json:"dry_run"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	dryRun := true
	if args.DryRun != nil {
		dryRun = *args.DryRun
	}

	result, err := s.sourceClient.LinkSources(ctx, dryRun)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to link sources: %v", err))
	}

	return s.successResponse(id, result)
}

// People and Band Office tool handlers

func (s *Server) handleListPeople(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		CommunityID string `json:"community_id"`
		CurrentOnly *bool  `json:"current_only"`
		Limit       int    `json:"limit"`
		Offset      int    `json:"offset"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.CommunityID == "" {
		return s.errorResponse(id, InvalidParams, "community_id is required")
	}

	currentOnly := true
	if args.CurrentOnly != nil {
		currentOnly = *args.CurrentOnly
	}

	limit := max(args.Limit, 0)
	if limit == 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	offset := max(args.Offset, 0)

	people, total, err := s.sourceClient.ListPeople(ctx, args.CommunityID, currentOnly, limit, offset)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list people: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"people": people,
		"count":  len(people),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleGetPerson(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		PersonID string `json:"person_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.PersonID == "" {
		return s.errorResponse(id, InvalidParams, "person_id is required")
	}

	person, err := s.sourceClient.GetPerson(ctx, args.PersonID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get person: %v", err))
	}

	return s.successResponse(id, person)
}

func (s *Server) handleAddPerson(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		CommunityID string  `json:"community_id"`
		Name        string  `json:"name"`
		Role        string  `json:"role"`
		RoleTitle   *string `json:"role_title"`
		Email       *string `json:"email"`
		Phone       *string `json:"phone"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.CommunityID == "" || args.Name == "" || args.Role == "" {
		return s.errorResponse(id, InvalidParams, "community_id, name, and role are required")
	}

	person, err := s.sourceClient.CreatePerson(ctx, args.CommunityID, client.Person{
		Name:      args.Name,
		Role:      args.Role,
		IsCurrent: true,
		RoleTitle: args.RoleTitle,
		Email:     args.Email,
		Phone:     args.Phone,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create person: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"person_id":    person.ID,
		"name":         person.Name,
		"role":         person.Role,
		"community_id": args.CommunityID,
		"message":      "Person created successfully",
	})
}

func (s *Server) handleGetBandOffice(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		CommunityID string `json:"community_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.CommunityID == "" {
		return s.errorResponse(id, InvalidParams, "community_id is required")
	}

	office, err := s.sourceClient.GetBandOffice(ctx, args.CommunityID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get band office: %v", err))
	}

	return s.successResponse(id, office)
}

func (s *Server) handleUpsertBandOffice(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		CommunityID  string  `json:"community_id"`
		AddressLine1 *string `json:"address_line1"`
		City         *string `json:"city"`
		Province     *string `json:"province"`
		PostalCode   *string `json:"postal_code"`
		Phone        *string `json:"phone"`
		Email        *string `json:"email"`
		OfficeHours  *string `json:"office_hours"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.CommunityID == "" {
		return s.errorResponse(id, InvalidParams, "community_id is required")
	}

	office, err := s.sourceClient.UpsertBandOffice(ctx, args.CommunityID, client.BandOffice{
		AddressLine1: args.AddressLine1,
		City:         args.City,
		Province:     args.Province,
		PostalCode:   args.PostalCode,
		Phone:        args.Phone,
		Email:        args.Email,
		OfficeHours:  args.OfficeHours,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to upsert band office: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"band_office_id": office.ID,
		"community_id":   args.CommunityID,
		"message":        "Band office saved successfully",
	})
}

// Publisher tool handlers

func (s *Server) handleCreateChannel(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Name         string               `json:"name"`
		Slug         string               `json:"slug"`
		RedisChannel string               `json:"redis_channel"`
		Description  string               `json:"description"`
		Rules        *client.ChannelRules `json:"rules"`
		Enabled      *bool                `json:"enabled"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Name == "" || args.Slug == "" || args.RedisChannel == "" {
		return s.errorResponse(id, InvalidParams, "name, slug, and redis_channel are required")
	}

	channel, err := s.publisherClient.CreateChannel(ctx, client.CreateChannelRequest{
		Name:         args.Name,
		Slug:         args.Slug,
		RedisChannel: args.RedisChannel,
		Description:  args.Description,
		Rules:        args.Rules,
		Enabled:      args.Enabled,
	})
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to create channel: %v", err))
	}

	return s.successResponse(id, channel)
}

func (s *Server) handleListChannels(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ActiveOnly bool `json:"active_only"`
	}

	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
		}
	}

	channels, err := s.publisherClient.ListChannels(ctx)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to list channels: %v", err))
	}

	// Filter by enabled if requested
	if args.ActiveOnly {
		filtered := make([]client.Channel, 0, len(channels))
		for i := range channels {
			if channels[i].Enabled {
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

func (s *Server) handleDeleteChannel(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ChannelID string `json:"channel_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.ChannelID == "" {
		return s.errorResponse(id, InvalidParams, "channel_id is required")
	}

	if err := s.publisherClient.DeleteChannel(ctx, args.ChannelID); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to delete channel: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"channel_id": args.ChannelID,
		"message":    "Channel deleted successfully",
	})
}

func (s *Server) handlePreviewChannel(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ChannelID string `json:"channel_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.ChannelID == "" {
		return s.errorResponse(id, InvalidParams, "channel_id is required")
	}

	preview, err := s.publisherClient.PreviewChannel(ctx, args.ChannelID)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to preview channel: %v", err))
	}

	return s.successResponse(id, preview)
}

func (s *Server) handleGetPublishHistory(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		ChannelName string `json:"channel_name"`
		Limit       int    `json:"limit"`
		Offset      int    `json:"offset"`
	}

	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
		}
	}

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

func (s *Server) handleSearchContent(ctx context.Context, id any, arguments json.RawMessage) *Response {
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

func (s *Server) handleClassifyContent(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args client.ClassifyRequest

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.Title == "" || args.RawText == "" || args.URL == "" {
		return s.errorResponse(id, InvalidParams, "title, raw_text, and url are required")
	}

	result, err := s.classifierClient.Classify(ctx, args)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to classify content: %v", err))
	}

	return s.successResponse(id, result)
}

// Observability tool handlers

func (s *Server) handleGetGrafanaAlerts(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		IncludeSilenced bool `json:"include_silenced"`
	}

	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
		}
	}

	alerts, err := s.grafanaClient.GetActiveAlerts(ctx)
	if err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to get Grafana alerts: %v", err))
	}

	// Filter out silenced alerts if not requested
	if !args.IncludeSilenced {
		filtered := make([]client.Alert, 0, len(alerts))
		for i := range alerts {
			if len(alerts[i].Status.SilencedBy) == 0 {
				filtered = append(filtered, alerts[i])
			}
		}
		alerts = filtered
	}

	// Build a summary for easier consumption
	firingCount := 0
	silencedCount := 0
	for i := range alerts {
		if alerts[i].Status.State == "active" {
			firingCount++
		}
		if len(alerts[i].Status.SilencedBy) > 0 {
			silencedCount++
		}
	}

	return s.successResponse(id, map[string]any{
		"alerts":         alerts,
		"total_count":    len(alerts),
		"firing_count":   firingCount,
		"silenced_count": silencedCount,
		"message":        fmt.Sprintf("Found %d active alerts (%d firing, %d silenced)", len(alerts), firingCount, silencedCount),
	})
}

// Index Manager tool handlers

func (s *Server) handleListIndexes(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}

	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
		}
	}

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
	// Sanitize internal error messages to avoid leaking service URLs, response bodies, etc.
	// InvalidParams messages are our own validation text and are safe to pass through.
	if code == InternalError {
		message = sanitizeErrorMessage(message)
	}
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
