package mcp

import (
	"context"
	"encoding/json"
	"fmt"

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
