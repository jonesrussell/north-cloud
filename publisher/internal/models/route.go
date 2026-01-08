package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Route represents a routing rule: source → channel with filters
type Route struct {
	ID              uuid.UUID      `db:"id"                json:"id"`
	SourceID        uuid.UUID      `db:"source_id"         json:"source_id"`
	ChannelID       uuid.UUID      `db:"channel_id"        json:"channel_id"`
	MinQualityScore int            `db:"min_quality_score" json:"min_quality_score"` // 0-100
	Topics          pq.StringArray `db:"topics"            json:"topics"`            // e.g., ['crime', 'news']
	Enabled         bool           `db:"enabled"           json:"enabled"`
	CreatedAt       time.Time      `db:"created_at"        json:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"        json:"updated_at"`
}

// RouteWithDetails represents a route with joined source and channel details
type RouteWithDetails struct {
	Route
	SourceName         string `db:"source_name"          json:"source_name"`
	SourceIndexPattern string `db:"source_index_pattern" json:"source_index_pattern"`
	ChannelName        string `db:"channel_name"         json:"channel_name"`
	ChannelDescription string `db:"channel_description"  json:"channel_description"`
}

// RouteCreateRequest represents the request payload for creating a route
type RouteCreateRequest struct {
	SourceID        uuid.UUID `binding:"required"                json:"source_id"`
	ChannelID       uuid.UUID `binding:"required"                json:"channel_id"`
	MinQualityScore *int      `binding:"omitempty,min=0,max=100" json:"min_quality_score"` // Defaults to 50
	Topics          []string  `json:"topics"`                                              // Optional
	Enabled         *bool     `json:"enabled"`                                             // Defaults to true
}

// RouteUpdateRequest represents the request payload for updating a route
type RouteUpdateRequest struct {
	MinQualityScore *int     `binding:"omitempty,min=0,max=100" json:"min_quality_score"`
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
