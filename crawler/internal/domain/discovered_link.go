// Package domain provides domain models used across the application.
package domain

import (
	"time"
)

// DiscoveredLink represents a link discovered during crawling for tracking/auditing purposes.
type DiscoveredLink struct {
	ID           string    `db:"id"            json:"id"`
	SourceID     string    `db:"source_id"     json:"source_id"`
	SourceName   string    `db:"source_name"   json:"source_name"`
	URL          string    `db:"url"           json:"url"`
	ParentURL    *string   `db:"parent_url"    json:"parent_url,omitempty"`
	Depth        int       `db:"depth"         json:"depth"`
	Domain       string    `db:"domain"        json:"domain"`
	HTTPStatus   *int16    `db:"http_status"   json:"http_status,omitempty"`
	ContentType  *string   `db:"content_type"  json:"content_type,omitempty"`
	DiscoveredAt time.Time `db:"discovered_at" json:"discovered_at"`
	QueuedAt     time.Time `db:"queued_at"     json:"queued_at"`
	Status       string    `db:"status"        json:"status"`
	Priority     int       `db:"priority"      json:"priority"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}
