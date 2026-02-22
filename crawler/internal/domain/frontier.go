package domain

import "time"

// FrontierURL status constants.
const (
	FrontierStatusPending  = "pending"
	FrontierStatusFetching = "fetching"
	FrontierStatusFetched  = "fetched"
	FrontierStatusFailed   = "failed"
	FrontierStatusDead     = "dead"
)

// FrontierURL origin constants.
const (
	FrontierOriginFeed    = "feed"
	FrontierOriginSitemap = "sitemap"
	FrontierOriginSpider  = "spider"
	FrontierOriginManual  = "manual"
)

// Priority bounds and defaults.
const (
	FrontierMinPriority     = 1
	FrontierMaxPriority     = 10
	FrontierDefaultPriority = 5
)

// Origin bonus values for priority calculation.
const (
	FrontierFeedBonus          = 2
	FrontierSitemapBonus       = 1
	FrontierSpiderArticleBonus = 1 // spider-discovered article URLs
)

// FrontierURL represents a URL in the frontier queue.
type FrontierURL struct {
	// Identity
	ID       string `db:"id"        json:"id"`
	URL      string `db:"url"       json:"url"`
	URLHash  string `db:"url_hash"  json:"url_hash"`
	Host     string `db:"host"      json:"host"`
	SourceID string `db:"source_id" json:"source_id"`

	// Discovery
	Origin    string  `db:"origin"     json:"origin"`
	ParentURL *string `db:"parent_url" json:"parent_url,omitempty"`
	Depth     int     `db:"depth"      json:"depth"`

	// Scheduling
	Priority    int       `db:"priority"      json:"priority"`
	Status      string    `db:"status"        json:"status"`
	NextFetchAt time.Time `db:"next_fetch_at" json:"next_fetch_at"`

	// Fetch state
	LastFetchedAt *time.Time `db:"last_fetched_at" json:"last_fetched_at,omitempty"`
	FetchCount    int        `db:"fetch_count"     json:"fetch_count"`
	ContentHash   *string    `db:"content_hash"    json:"content_hash,omitempty"`
	ETag          *string    `db:"etag"            json:"etag,omitempty"`
	LastModified  *string    `db:"last_modified"   json:"last_modified,omitempty"`

	// Retry
	RetryCount int     `db:"retry_count" json:"retry_count"`
	LastError  *string `db:"last_error"  json:"last_error,omitempty"`

	// Timestamps
	DiscoveredAt time.Time `db:"discovered_at" json:"discovered_at"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// HostState tracks per-host politeness and robots.txt cache.
type HostState struct {
	Host            string     `db:"host"              json:"host"`
	LastFetchAt     *time.Time `db:"last_fetch_at"     json:"last_fetch_at,omitempty"`
	MinDelayMs      int        `db:"min_delay_ms"      json:"min_delay_ms"`
	RobotsTxt       *string    `db:"robots_txt"        json:"robots_txt,omitempty"`
	RobotsFetchedAt *time.Time `db:"robots_fetched_at" json:"robots_fetched_at,omitempty"`
	RobotsTTLHours  int        `db:"robots_ttl_hours"  json:"robots_ttl_hours"`
	CreatedAt       time.Time  `db:"created_at"        json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"        json:"updated_at"`
}

// FeedState tracks polling state for a source's feed.
type FeedState struct {
	SourceID          string     `db:"source_id"          json:"source_id"`
	FeedURL           string     `db:"feed_url"           json:"feed_url"`
	LastPolledAt      *time.Time `db:"last_polled_at"     json:"last_polled_at,omitempty"`
	LastETag          *string    `db:"last_etag"          json:"last_etag,omitempty"`
	LastModified      *string    `db:"last_modified"      json:"last_modified,omitempty"`
	LastItemCount     int        `db:"last_item_count"    json:"last_item_count"`
	ConsecutiveErrors int        `db:"consecutive_errors" json:"consecutive_errors"`
	LastError         *string    `db:"last_error"         json:"last_error,omitempty"`
	LastErrorType     *string    `db:"last_error_type"    json:"last_error_type,omitempty"`
	CreatedAt         time.Time  `db:"created_at"         json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"         json:"updated_at"`
}
