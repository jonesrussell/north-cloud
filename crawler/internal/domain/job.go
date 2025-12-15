// Package domain provides domain models used across the application.
package domain

import (
	"time"
)

// Job represents a crawling job.
type Job struct {
	ID              string     `json:"id" db:"id"`
	SourceID        string     `json:"source_id" db:"source_id"`
	SourceName      *string    `json:"source_name,omitempty" db:"source_name"`
	URL             string     `json:"url" db:"url"`
	ScheduleTime    *string    `json:"schedule_time,omitempty" db:"schedule_time"`
	ScheduleEnabled bool       `json:"schedule_enabled" db:"schedule_enabled"`
	Status          string     `json:"status" db:"status"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
	StartedAt       *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	ErrorMessage    *string    `json:"error_message,omitempty" db:"error_message"`
}

// Item represents a crawled item from a job.
type Item struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	URL       string    `json:"url"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
