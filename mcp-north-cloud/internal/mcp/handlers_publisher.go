package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

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
