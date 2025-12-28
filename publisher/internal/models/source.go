package models

import (
	"time"

	"github.com/google/uuid"
)

// Source represents an Elasticsearch index to monitor for articles
type Source struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`                   // e.g., "sudbury_com"
	IndexPattern string    `json:"index_pattern" db:"index_pattern"` // e.g., "sudbury_com_classified_content"
	Enabled      bool      `json:"enabled" db:"enabled"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// SourceCreateRequest represents the request payload for creating a source
type SourceCreateRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=255"`
	IndexPattern string `json:"index_pattern" binding:"required,min=1,max=255"`
	Enabled      *bool  `json:"enabled"` // Pointer to allow omission (defaults to true)
}

// SourceUpdateRequest represents the request payload for updating a source
type SourceUpdateRequest struct {
	Name         *string `json:"name" binding:"omitempty,min=1,max=255"`
	IndexPattern *string `json:"index_pattern" binding:"omitempty,min=1,max=255"`
	Enabled      *bool   `json:"enabled"`
}

// Validate validates the source create request
func (r *SourceCreateRequest) Validate() error {
	// Additional validation logic can be added here
	return nil
}

// Validate validates the source update request
func (r *SourceUpdateRequest) Validate() error {
	// At least one field must be provided
	if r.Name == nil && r.IndexPattern == nil && r.Enabled == nil {
		return ErrNoFieldsToUpdate
	}
	return nil
}
