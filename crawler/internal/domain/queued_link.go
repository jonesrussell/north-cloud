// Package domain provides domain models used across the application.
package domain

import (
	"time"
)

// QueuedLink represents a link discovered during crawling that should be crawled later.
type QueuedLink struct {
	ID           string     `json:"id" db:"id"`
	SourceID     string     `json:"source_id" db:"source_id"`
	SourceName   string     `json:"source_name" db:"source_name"`
	URL          string     `json:"url" db:"url"`
	ParentURL    *string    `json:"parent_url,omitempty" db:"parent_url"`
	Depth        int        `json:"depth" db:"depth"`
	DiscoveredAt time.Time  `json:"discovered_at" db:"discovered_at"`
	QueuedAt     time.Time  `json:"queued_at" db:"queued_at"`
	Status       string     `json:"status" db:"status"`
	Priority     int        `json:"priority" db:"priority"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

