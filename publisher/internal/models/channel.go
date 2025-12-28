package models

import (
	"time"

	"github.com/google/uuid"
)

// Channel represents a Redis pub/sub channel for routing articles by topic
type Channel struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`               // e.g., "articles:crime", "articles:news"
	Description string    `json:"description" db:"description"` // Human-readable description
	Enabled     bool      `json:"enabled" db:"enabled"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ChannelCreateRequest represents the request payload for creating a channel
type ChannelCreateRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=255"`
	Description string `json:"description" binding:"max=1000"`
	Enabled     *bool  `json:"enabled"` // Pointer to allow omission (defaults to true)
}

// ChannelUpdateRequest represents the request payload for updating a channel
type ChannelUpdateRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=1,max=255"`
	Description *string `json:"description" binding:"omitempty,max=1000"`
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
