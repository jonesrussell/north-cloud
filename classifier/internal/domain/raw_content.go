package domain

import "time"

// RawContent represents minimally-processed crawled content
// This is the input to the classifier service from Elasticsearch
type RawContent struct {
	// Core identifiers
	ID         string `json:"id"`
	URL        string `json:"url"`
	SourceName string `json:"source_name"`

	// Raw content
	Title   string `json:"title"`
	RawHTML string `json:"raw_html,omitempty"` // Optional, can be large
	RawText string `json:"raw_text"`

	// Open Graph metadata (extracted by crawler)
	OGType        string `json:"og_type,omitempty"`
	OGTitle       string `json:"og_title,omitempty"`
	OGDescription string `json:"og_description,omitempty"`
	OGImage       string `json:"og_image,omitempty"`
	OGURL         string `json:"og_url,omitempty"`

	// Basic metadata
	MetaDescription string `json:"meta_description,omitempty"`
	MetaKeywords    string `json:"meta_keywords,omitempty"`
	CanonicalURL    string `json:"canonical_url,omitempty"`

	// Timestamps
	CrawledAt     time.Time  `json:"crawled_at"`
	PublishedDate *time.Time `json:"published_date,omitempty"`

	// Processing status
	ClassificationStatus string     `json:"classification_status"` // "pending", "classified", "failed"
	ClassifiedAt         *time.Time `json:"classified_at,omitempty"`

	// Quick metrics
	WordCount int `json:"word_count"`

	// Meta holds additional metadata from the crawler (e.g. detected_content_type from IsStructuredContentPage)
	Meta map[string]any `json:"meta,omitempty"`
}

// ClassificationStatus constants
const (
	StatusPending    = "pending"
	StatusClassified = "classified"
	StatusFailed     = "failed"
)
