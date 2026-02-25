package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PublishHistory represents an audit trail entry for a published content item
type PublishHistory struct {
	ID           uuid.UUID      `db:"id"            json:"id"`
	RouteID      *uuid.UUID     `db:"route_id"      json:"channel_id,omitempty"` // Repurposed: stores channel_id for Layer 2
	ContentID    string         `db:"article_id"    json:"content_id"`           // Elasticsearch document ID
	ContentTitle string         `db:"article_title" json:"content_title"`
	ContentURL   string         `db:"article_url"   json:"content_url"`
	ChannelName  string         `db:"channel_name"  json:"channel_name"` // Channel name (e.g., "content:crime" or "streetcode:crime_feed")
	PublishedAt  time.Time      `db:"published_at"  json:"published_at"`
	QualityScore int            `db:"quality_score" json:"quality_score"`
	Topics       pq.StringArray `db:"topics"        json:"topics"`
}

// PublishHistoryCreateRequest represents the data needed to create a publish history entry
type PublishHistoryCreateRequest struct {
	ChannelID    *uuid.UUID `json:"channel_id,omitempty"` // Optional: custom channel ID (Layer 2)
	ContentID    string     `binding:"required"          json:"content_id"`
	ContentTitle string     `json:"content_title"`
	ContentURL   string     `json:"content_url"`
	ChannelName  string     `binding:"required"          json:"channel_name"`
	QualityScore int        `json:"quality_score"`
	Topics       []string   `json:"topics"`
}

// PublishHistoryFilter represents filter criteria for querying publish history
type PublishHistoryFilter struct {
	ChannelName string     `form:"channel_name"`
	ContentID   string     `form:"content_id"`
	StartDate   *time.Time `form:"start_date"                  time_format:"2006-01-02"`
	EndDate     *time.Time `form:"end_date"                    time_format:"2006-01-02"`
	Limit       int        `binding:"omitempty,min=1,max=1000" form:"limit"` // Default 100
	Offset      int        `binding:"omitempty,min=0"          form:"offset"`
}
