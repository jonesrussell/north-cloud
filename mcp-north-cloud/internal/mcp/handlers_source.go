package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

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
		SourceID                string         `json:"source_id"`
		Name                    string         `json:"name"`
		URL                     string         `json:"url"`
		Selectors               map[string]any `json:"selectors"`
		Active                  *bool          `json:"active"`
		FeedURL                 *string        `json:"feed_url"`
		FeedPollIntervalMinutes *int           `json:"feed_poll_interval_minutes"`
		IngestionMode           string         `json:"ingestion_mode"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.SourceID == "" {
		return s.errorResponse(id, InvalidParams, "source_id is required")
	}

	// GET current source first (source-manager PUT does full replacement)
	req, mergeErr := s.buildMergedUpdateRequest(
		ctx, args.SourceID, args.Name, args.URL, args.Selectors,
		args.Active, args.FeedURL, args.FeedPollIntervalMinutes, args.IngestionMode,
	)
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
	ctx context.Context, sourceID, name, url string, selectors map[string]any,
	active *bool, feedURL *string, feedPollIntervalMinutes *int, ingestionMode string,
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
	if feedPollIntervalMinutes != nil {
		req.FeedPollIntervalMinutes = feedPollIntervalMinutes
	}
	if ingestionMode != "" {
		req.IngestionMode = ingestionMode
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

func (s *Server) handleEnableFeed(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		SourceID string `json:"source_id"`
	}

	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "Invalid arguments: "+err.Error())
	}

	if args.SourceID == "" {
		return s.errorResponse(id, InvalidParams, "source_id is required")
	}

	if err := s.sourceClient.EnableFeed(ctx, args.SourceID); err != nil {
		return s.errorResponse(id, InternalError, fmt.Sprintf("Failed to enable feed: %v", err))
	}

	return s.successResponse(id, map[string]any{
		"source_id": args.SourceID,
		"message":   "Feed enabled — crawler will resume polling on next cycle",
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
