package metrics

import "time"

// RecentArticle represents a recently posted article
type RecentArticle struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	City     string    `json:"city"`
	PostedAt time.Time `json:"posted_at"`
}

// Stats represents aggregated statistics
type Stats struct {
	TotalPosted  int64       `json:"total_posted"`
	TotalSkipped int64       `json:"total_skipped"`
	TotalErrors  int64       `json:"total_errors"`
	Cities       []CityStats `json:"cities"`
	LastSync     time.Time   `json:"last_sync"`
}

// CityStats represents statistics for a specific city
type CityStats struct {
	Name    string `json:"name"`
	Posted  int64  `json:"posted"`
	Skipped int64  `json:"skipped"`
	Errors  int64  `json:"errors"`
}
