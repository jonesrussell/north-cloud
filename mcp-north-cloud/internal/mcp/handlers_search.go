package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

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
