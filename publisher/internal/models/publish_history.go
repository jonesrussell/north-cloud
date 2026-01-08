package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PublishHistory represents an audit trail entry for a published article
type PublishHistory struct {
	ID           uuid.UUID      `db:"id"            json:"id"`
	RouteID      *uuid.UUID     `db:"route_id"      json:"route_id,omitempty"` // Nullable if route is deleted
	ArticleID    string         `db:"article_id"    json:"article_id"`         // Elasticsearch document ID
	ArticleTitle string         `db:"article_title" json:"article_title"`
	ArticleURL   string         `db:"article_url"   json:"article_url"`
	ChannelName  string         `db:"channel_name"  json:"channel_name"` // Denormalized for faster querying
	SourceName   string         `db:"source_name"   json:"source_name"`  // Joined from sources table, defaults to "Unknown" if route deleted
	PublishedAt  time.Time      `db:"published_at"  json:"published_at"`
	QualityScore int            `db:"quality_score" json:"quality_score"`
	Topics       pq.StringArray `db:"topics"        json:"topics"`
}

// PublishHistoryCreateRequest represents the data needed to create a publish history entry
type PublishHistoryCreateRequest struct {
	RouteID      uuid.UUID `json:"route_id"`
	ArticleID    string    `binding:"required"   json:"article_id"`
	ArticleTitle string    `json:"article_title"`
	ArticleURL   string    `json:"article_url"`
	ChannelName  string    `binding:"required"   json:"channel_name"`
	QualityScore int       `json:"quality_score"`
	Topics       []string  `json:"topics"`
}

// PublishHistoryFilter represents filter criteria for querying publish history
type PublishHistoryFilter struct {
	ChannelName string     `form:"channel_name"`
	ArticleID   string     `form:"article_id"`
	StartDate   *time.Time `form:"start_date"                  time_format:"2006-01-02"`
	EndDate     *time.Time `form:"end_date"                    time_format:"2006-01-02"`
	Limit       int        `binding:"omitempty,min=1,max=1000" form:"limit"` // Default 100
	Offset      int        `binding:"omitempty,min=0"          form:"offset"`
}
