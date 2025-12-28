package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Route represents a routing rule: source → channel with filters
type Route struct {
	ID              uuid.UUID      `json:"id" db:"id"`
	SourceID        uuid.UUID      `json:"source_id" db:"source_id"`
	ChannelID       uuid.UUID      `json:"channel_id" db:"channel_id"`
	MinQualityScore int            `json:"min_quality_score" db:"min_quality_score"` // 0-100
	Topics          pq.StringArray `json:"topics" db:"topics"`                       // e.g., ['crime', 'news']
	Enabled         bool           `json:"enabled" db:"enabled"`
	CreatedAt       time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at" db:"updated_at"`
}

// RouteWithDetails represents a route with joined source and channel details
type RouteWithDetails struct {
	Route
	SourceName        string `json:"source_name" db:"source_name"`
	SourceIndexPattern string `json:"source_index_pattern" db:"source_index_pattern"`
	ChannelName        string `json:"channel_name" db:"channel_name"`
	ChannelDescription string `json:"channel_description" db:"channel_description"`
}

// RouteCreateRequest represents the request payload for creating a route
type RouteCreateRequest struct {
	SourceID        uuid.UUID `json:"source_id" binding:"required"`
	ChannelID       uuid.UUID `json:"channel_id" binding:"required"`
	MinQualityScore *int      `json:"min_quality_score" binding:"omitempty,min=0,max=100"` // Defaults to 50
	Topics          []string  `json:"topics"`                                              // Optional
	Enabled         *bool     `json:"enabled"`                                             // Defaults to true
}

// RouteUpdateRequest represents the request payload for updating a route
type RouteUpdateRequest struct {
	MinQualityScore *int     `json:"min_quality_score" binding:"omitempty,min=0,max=100"`
	Topics          []string `json:"topics"`
	Enabled         *bool    `json:"enabled"`
}

// Validate validates the route create request
func (r *RouteCreateRequest) Validate() error {
	// Additional validation logic can be added here
	return nil
}

// Validate validates the route update request
func (r *RouteUpdateRequest) Validate() error {
	// At least one field must be provided
	if r.MinQualityScore == nil && r.Topics == nil && r.Enabled == nil {
		return ErrNoFieldsToUpdate
	}
	return nil
}
