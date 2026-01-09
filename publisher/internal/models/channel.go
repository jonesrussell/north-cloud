package models

import (
	"time"

	"github.com/google/uuid"
)

// Channel represents a Redis pub/sub channel for routing articles by topic
type Channel struct {
	ID          uuid.UUID `db:"id"          json:"id"`
	Name        string    `db:"name"        json:"name"`        // e.g., "articles:crime", "articles:news"
	Description string    `db:"description" json:"description"` // Human-readable description
	Enabled     bool      `db:"enabled"     json:"enabled"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}

// ChannelCreateRequest represents the request payload for creating a channel
type ChannelCreateRequest struct {
	Name        string `binding:"required,min=1,max=255" json:"name"`
	Description string `binding:"max=1000"               json:"description"`
	Enabled     *bool  `json:"enabled"` // Pointer to allow omission (defaults to true)
}

// ChannelUpdateRequest represents the request payload for updating a channel
type ChannelUpdateRequest struct {
	Name        *string `binding:"omitempty,min=1,max=255" json:"name"`
	Description *string `binding:"omitempty,max=1000"      json:"description"`
	Enabled     *bool   `json:"enabled"`
}

// Validate validates the channel create request
func (r *ChannelCreateRequest) Validate() error {
	// Validate channel name format: articles:{topic}
	// Additional validation can be added here
	return nil
}

// Validate validates the channel update request
func (r *ChannelUpdateRequest) Validate() error {
	// At least one field must be provided
	if r.Name == nil && r.Description == nil && r.Enabled == nil {
		return ErrNoFieldsToUpdate
	}
	return nil
}
