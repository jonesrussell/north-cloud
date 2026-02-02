package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Channel represents a custom routing channel with embedded rules
type Channel struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	Name         string    `db:"name"          json:"name"`
	Slug         string    `db:"slug"          json:"slug"`
	RedisChannel string    `db:"redis_channel" json:"redis_channel"`
	Description  string    `db:"description"   json:"description"`
	Rules        Rules     `db:"-"             json:"rules"`
	RulesJSON    []byte    `db:"rules"         json:"-"`
	RulesVersion int       `db:"rules_version" json:"rules_version"`
	Enabled      bool      `db:"enabled"       json:"enabled"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// ParseRules parses RulesJSON into Rules struct
func (c *Channel) ParseRules() error {
	if len(c.RulesJSON) == 0 {
		c.Rules = Rules{}
		return nil
	}
	return json.Unmarshal(c.RulesJSON, &c.Rules)
}

// ChannelCreateRequest represents the request payload for creating a channel
type ChannelCreateRequest struct {
	Name         string `binding:"required,min=1,max=255" json:"name"`
	Slug         string `binding:"required,min=1,max=255" json:"slug"`
	RedisChannel string `binding:"required,min=1,max=255" json:"redis_channel"`
	Description  string `binding:"max=1000"               json:"description"`
	Rules        *Rules `json:"rules"`
	Enabled      *bool  `json:"enabled"`
}

// ChannelUpdateRequest represents the request payload for updating a channel
type ChannelUpdateRequest struct {
	Name         *string `binding:"omitempty,min=1,max=255" json:"name"`
	Slug         *string `binding:"omitempty,min=1,max=255" json:"slug"`
	RedisChannel *string `binding:"omitempty,min=1,max=255" json:"redis_channel"`
	Description  *string `binding:"omitempty,max=1000"      json:"description"`
	Rules        *Rules  `json:"rules"`
	Enabled      *bool   `json:"enabled"`
}

// Validate validates the channel create request
func (r *ChannelCreateRequest) Validate() error {
	return nil
}

// Validate validates the channel update request
func (r *ChannelUpdateRequest) Validate() error {
	if r.Name == nil && r.Slug == nil && r.RedisChannel == nil &&
		r.Description == nil && r.Rules == nil && r.Enabled == nil {
		return ErrNoFieldsToUpdate
	}
	return nil
}
