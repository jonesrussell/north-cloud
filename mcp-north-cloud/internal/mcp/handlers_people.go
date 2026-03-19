package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/mcp-north-cloud/internal/client"
)

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
