package models

import (
	"time"

	"github.com/google/uuid"
)

// Source represents an Elasticsearch index to monitor for articles
type Source struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	Name         string    `db:"name"          json:"name"`          // e.g., "sudbury_com"
	IndexPattern string    `db:"index_pattern" json:"index_pattern"` // e.g., "sudbury_com_classified_content"
	Enabled      bool      `db:"enabled"       json:"enabled"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// SourceCreateRequest represents the request payload for creating a source
type SourceCreateRequest struct {
	Name         string `binding:"required,min=1,max=255" json:"name"`
	IndexPattern string `binding:"required,min=1,max=255" json:"index_pattern"`
	Enabled      *bool  `json:"enabled"` // Pointer to allow omission (defaults to true)
}

// SourceUpdateRequest represents the request payload for updating a source
type SourceUpdateRequest struct {
	Name         *string `binding:"omitempty,min=1,max=255" json:"name"`
	IndexPattern *string `binding:"omitempty,min=1,max=255" json:"index_pattern"`
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
