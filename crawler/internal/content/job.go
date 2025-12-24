// Package content provides content processing types and interfaces.
package content

import (
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// JobStatus represents the status of a job.
type JobStatus string

const (
	// JobStatusPending indicates the job is waiting to be processed.
	JobStatusPending JobStatus = "pending"
	// JobStatusProcessing indicates the job is being processed.
	JobStatusProcessing JobStatus = "processing"
	// JobStatusCompleted indicates the job has been completed.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed indicates the job has failed.
	JobStatusFailed JobStatus = "failed"
)

// Job represents a crawling job.
type Job struct {
	// ID is the unique identifier for the job.
	ID string
	// URL is the URL to crawl.
	URL string
	// Status is the current status of the job.
	Status JobStatus
	// CreatedAt is when the job was created.
	CreatedAt time.Time
	// UpdatedAt is when the job was last updated.
	UpdatedAt time.Time
	// Items are the items found during crawling.
	Items []*Item
}

// Item represents a crawled item.
type Item struct {
	// ID is the unique identifier for the item.
	ID string
	// URL is the URL of the item.
	URL string
	// Type is the type of content.
	Type domain.Type
	// Status is the current status of the item.
	Status JobStatus
	// Source is the source of the item.
	Source string
	// CreatedAt is when the item was created.
	CreatedAt time.Time
	// UpdatedAt is when the item was last updated.
	UpdatedAt time.Time
}

// JobValidator validates jobs before processing.
type JobValidator interface {
	// ValidateJob validates a job before processing.
	ValidateJob(job *Job) error
}

// JobValidatorFunc is a function type that implements JobValidator.
type JobValidatorFunc func(job *Job) error

// ValidateJob implements JobValidator.
func (f JobValidatorFunc) ValidateJob(job *Job) error {
	return f(job)
}
