package domain

import "time"

// ContentListItem is a content record with a rolled-up delivery summary for list views.
type ContentListItem struct {
	ID              string           `db:"id"                           json:"id"`
	Type            ContentType      `db:"type"                         json:"type"`
	Title           string           `db:"title"                        json:"title"`
	Summary         string           `db:"summary"                      json:"summary"`
	URL             string           `db:"url"                          json:"url"`
	Project         string           `db:"project"                      json:"project"`
	Source          string           `db:"source"                       json:"source"`
	Published       bool             `db:"published"                    json:"published"`
	ScheduledAt     *time.Time       `db:"scheduled_at"                 json:"scheduled_at,omitempty"`
	CreatedAt       time.Time        `db:"created_at"                   json:"created_at"`
	DeliverySummary *DeliverySummary `json:"delivery_summary,omitempty"`
}

// DeliverySummary is a count of deliveries by status for a single content item.
type DeliverySummary struct {
	Total     int `db:"total"     json:"total"`
	Pending   int `db:"pending"   json:"pending"`
	Delivered int `db:"delivered" json:"delivered"`
	Failed    int `db:"failed"    json:"failed"`
	Retrying  int `db:"retrying"  json:"retrying"`
}

// ContentListFilter holds query parameters for listing content.
type ContentListFilter struct {
	Offset int
	Limit  int
	Status string
	Type   string
}
