package domain

import "time"

// DeliveryStatus tracks where a delivery is in its lifecycle.
type DeliveryStatus string

const (
	StatusPending    DeliveryStatus = "pending"
	StatusPublishing DeliveryStatus = "publishing"
	StatusDelivered  DeliveryStatus = "delivered"
	StatusRetrying   DeliveryStatus = "retrying"
	StatusFailed     DeliveryStatus = "failed"
)

// Delivery represents a single attempt to publish content to a platform.
type Delivery struct {
	ID          string         `db:"id"            json:"id"`
	ContentID   string         `db:"content_id"    json:"content_id"`
	Platform    string         `db:"platform"      json:"platform"`
	Account     string         `db:"account"       json:"account"`
	Status      DeliveryStatus `db:"status"        json:"status"`
	PlatformID  *string        `db:"platform_id"   json:"platform_id,omitempty"`
	PlatformURL *string        `db:"platform_url"  json:"platform_url,omitempty"`
	Error       *string        `db:"error"         json:"error,omitempty"`
	Attempts    int            `db:"attempts"      json:"attempts"`
	MaxAttempts int            `db:"max_attempts"  json:"max_attempts"`
	NextRetryAt *time.Time     `db:"next_retry_at" json:"next_retry_at,omitempty"`
	LastErrorAt *time.Time     `db:"last_error_at" json:"last_error_at,omitempty"`
	CreatedAt   time.Time      `db:"created_at"    json:"created_at"`
	DeliveredAt *time.Time     `db:"delivered_at"  json:"delivered_at,omitempty"`
}

// DeliveryEvent is emitted on Redis when a delivery status changes.
type DeliveryEvent struct {
	ContentID   string    `json:"content_id"`
	ContentType string    `json:"content_type"`
	DeliveryID  string    `json:"delivery_id"`
	Platform    string    `json:"platform"`
	Account     string    `json:"account"`
	Status      string    `json:"status"`
	PlatformID  string    `json:"platform_id,omitempty"`
	PlatformURL string    `json:"platform_url,omitempty"`
	Error       string    `json:"error,omitempty"`
	RetryAfter  *int      `json:"retry_after,omitempty"`
	Attempts    int       `json:"attempts"`
	Timestamp   time.Time `json:"timestamp"`
}

// DeliveryResult holds the platform-assigned identifiers after a successful publish.
type DeliveryResult struct {
	PlatformID  string
	PlatformURL string
}
