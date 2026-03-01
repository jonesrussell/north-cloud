package domain

import "time"

// ContentType represents the kind of content being published.
type ContentType string

const (
	BlogPost            ContentType = "blog_post"
	SocialUpdate        ContentType = "social_update"
	ProductAnnouncement ContentType = "product_announcement"
)

// PublishMessage is the inbound message describing content to publish.
type PublishMessage struct {
	ContentID   string            `json:"content_id"`
	Type        ContentType       `json:"type"`
	Title       string            `json:"title,omitempty"`
	Body        string            `json:"body,omitempty"`
	Summary     string            `json:"summary"`
	URL         string            `json:"url,omitempty"`
	Images      []string          `json:"images,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Project     string            `json:"project"`
	Targets     []TargetConfig    `json:"targets,omitempty"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Source      string            `json:"source"`
}

// TargetConfig specifies a destination platform and account for publishing.
type TargetConfig struct {
	Platform string  `json:"platform"`
	Account  string  `json:"account"`
	Override *string `json:"override,omitempty"`
}
