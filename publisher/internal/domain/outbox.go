// Package domain contains the core domain models for the publisher service.
package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when an entity is not found in the database.
var ErrNotFound = errors.New("entity not found")

// ErrInvalidOutboxEntry is returned when creating an outbox entry with invalid fields.
var ErrInvalidOutboxEntry = errors.New("invalid outbox entry")

// OutboxStatus represents the state of an outbox entry
type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusPublishing OutboxStatus = "publishing"
	OutboxStatusPublished  OutboxStatus = "published"
	OutboxStatusFailed     OutboxStatus = "failed"
)

// OutboxEntry represents a classified document awaiting publication
type OutboxEntry struct {
	ID               string       `db:"id"                json:"id"`
	ContentID        string       `db:"content_id"        json:"content_id"`
	SourceName       string       `db:"source_name"       json:"source_name"`
	IndexName        string       `db:"index_name"        json:"index_name"`
	ContentType      string       `db:"content_type"      json:"content_type"`
	Topics           []string     `db:"topics"            json:"topics"`
	QualityScore     int          `db:"quality_score"     json:"quality_score"`
	IsCrimeRelated   bool         `db:"is_crime_related"  json:"is_crime_related"`
	CrimeSubcategory *string      `db:"crime_subcategory" json:"crime_subcategory,omitempty"`
	Title            string       `db:"title"             json:"title"`
	Body             string       `db:"body"              json:"body"`
	URL              string       `db:"url"               json:"url"`
	PublishedDate    *time.Time   `db:"published_date"    json:"published_date,omitempty"`
	Status           OutboxStatus `db:"status"            json:"status"`
	RetryCount       int          `db:"retry_count"       json:"retry_count"`
	MaxRetries       int          `db:"max_retries"       json:"max_retries"`
	ErrorMessage     *string      `db:"error_message"     json:"error_message,omitempty"`
	CreatedAt        time.Time    `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time    `db:"updated_at"        json:"updated_at"`
	PublishedAt      *time.Time   `db:"published_at"      json:"published_at,omitempty"`
	NextRetryAt      *time.Time   `db:"next_retry_at"     json:"next_retry_at,omitempty"`
}

// RoutingKey returns the Redis channel for this entry based on content type and topics
func (o *OutboxEntry) RoutingKey() string {
	// Crime content gets specific channels based on subcategory
	if o.IsCrimeRelated && o.CrimeSubcategory != nil {
		return "articles:crime:" + *o.CrimeSubcategory
	}
	if o.IsCrimeRelated {
		return "articles:crime"
	}

	// Route by content type
	switch o.ContentType {
	case "article":
		return "articles:news"
	case "video":
		return "content:video"
	case "image":
		return "content:image"
	default:
		return "content:other"
	}
}

// ShouldRetry returns true if the entry can be retried
func (o *OutboxEntry) ShouldRetry() bool {
	return o.RetryCount < o.MaxRetries
}

// IsExhausted returns true if all retries have been exhausted
func (o *OutboxEntry) IsExhausted() bool {
	return o.RetryCount >= o.MaxRetries
}

// ToPublishMessage converts to Redis message format
func (o *OutboxEntry) ToPublishMessage() map[string]any {
	return map[string]any{
		"id":                o.ContentID,
		"source":            o.SourceName,
		"index":             o.IndexName,
		"content_type":      o.ContentType,
		"topics":            o.Topics,
		"quality_score":     o.QualityScore,
		"is_crime_related":  o.IsCrimeRelated,
		"crime_subcategory": o.CrimeSubcategory,
		"title":             o.Title,
		"body":              o.Body,
		"url":               o.URL,
		"published_date":    o.PublishedDate,
		"publisher": map[string]any{
			"outbox_id":    o.ID,
			"published_at": time.Now().UTC().Format(time.RFC3339),
			"channel":      o.RoutingKey(),
		},
	}
}

// OutboxStats holds outbox statistics for monitoring
type OutboxStats struct {
	Pending              int64   `json:"pending"`
	Publishing           int64   `json:"publishing"`
	Published            int64   `json:"published"`
	FailedRetryable      int64   `json:"failed_retryable"`
	FailedExhausted      int64   `json:"failed_exhausted"`
	AvgPublishLagSeconds float64 `json:"avg_publish_lag_seconds"`
}

// OutboxSourceStats holds per-source statistics
type OutboxSourceStats struct {
	SourceName string `json:"source_name"`
	Pending    int64  `json:"pending"`
	Published  int64  `json:"published"`
	Failed     int64  `json:"failed"`
}

const (
	defaultOutboxMaxRetries = 5
	minQualityScore         = 0
	maxQualityScore         = 100
)

// NewOutboxEntry creates a new outbox entry with validation.
// Returns an error if required fields are empty or quality score is out of range.
func NewOutboxEntry(contentID, sourceName, indexName, contentType, title, body, url string) (*OutboxEntry, error) {
	if contentID == "" {
		return nil, fmt.Errorf("%w: content_id is required", ErrInvalidOutboxEntry)
	}
	if sourceName == "" {
		return nil, fmt.Errorf("%w: source_name is required", ErrInvalidOutboxEntry)
	}
	if indexName == "" {
		return nil, fmt.Errorf("%w: index_name is required", ErrInvalidOutboxEntry)
	}

	now := time.Now()
	return &OutboxEntry{
		ContentID:   contentID,
		SourceName:  sourceName,
		IndexName:   indexName,
		ContentType: contentType,
		Title:       title,
		Body:        body,
		URL:         url,
		Topics:      []string{}, // Initialize to empty, never nil
		Status:      OutboxStatusPending,
		MaxRetries:  defaultOutboxMaxRetries,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// SetQualityScore sets the quality score with validation (0-100).
func (o *OutboxEntry) SetQualityScore(score int) error {
	if score < minQualityScore || score > maxQualityScore {
		return fmt.Errorf("%w: quality_score must be between %d and %d, got %d",
			ErrInvalidOutboxEntry, minQualityScore, maxQualityScore, score)
	}
	o.QualityScore = score
	return nil
}

// SetCrimeStatus sets crime-related fields together to maintain consistency.
// If isCrime is false, subcategory is always set to nil.
func (o *OutboxEntry) SetCrimeStatus(isCrime bool, subcategory string) {
	o.IsCrimeRelated = isCrime
	if isCrime && subcategory != "" {
		o.CrimeSubcategory = &subcategory
	} else {
		o.CrimeSubcategory = nil
	}
}
