package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PublishHistory represents an audit trail entry for a published article
type PublishHistory struct {
	ID           uuid.UUID      `json:"id" db:"id"`
	RouteID      *uuid.UUID     `json:"route_id,omitempty" db:"route_id"` // Nullable if route is deleted
	ArticleID    string         `json:"article_id" db:"article_id"`       // Elasticsearch document ID
	ArticleTitle string         `json:"article_title" db:"article_title"`
	ArticleURL   string         `json:"article_url" db:"article_url"`
	ChannelName  string         `json:"channel_name" db:"channel_name"` // Denormalized for faster querying
	SourceName   string         `json:"source_name" db:"source_name"`   // Joined from sources table, defaults to "Unknown" if route deleted
	PublishedAt  time.Time      `json:"published_at" db:"published_at"`
	QualityScore int            `json:"quality_score" db:"quality_score"`
	Topics       pq.StringArray `json:"topics" db:"topics"`
}

// PublishHistoryCreateRequest represents the data needed to create a publish history entry
type PublishHistoryCreateRequest struct {
	RouteID      uuid.UUID `json:"route_id"`
	ArticleID    string    `json:"article_id" binding:"required"`
	ArticleTitle string    `json:"article_title"`
	ArticleURL   string    `json:"article_url"`
	ChannelName  string    `json:"channel_name" binding:"required"`
	QualityScore int       `json:"quality_score"`
	Topics       []string  `json:"topics"`
}

// PublishHistoryFilter represents filter criteria for querying publish history
type PublishHistoryFilter struct {
	ChannelName string     `form:"channel_name"`
	ArticleID   string     `form:"article_id"`
	StartDate   *time.Time `form:"start_date" time_format:"2006-01-02"`
	EndDate     *time.Time `form:"end_date" time_format:"2006-01-02"`
	Limit       int        `form:"limit" binding:"omitempty,min=1,max=1000"` // Default 100
	Offset      int        `form:"offset" binding:"omitempty,min=0"`
}
