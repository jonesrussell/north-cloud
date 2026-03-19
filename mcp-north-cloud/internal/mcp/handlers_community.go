package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

// Community tool handlers

const (
	defaultNearbyRadiusKm = 50.0
	defaultNearbyLimit    = 10
)

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
