// Package domain provides extended domain models for scheduler V2.
package domain

import (
	"time"

	basedomain "github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/queue"
)

const (
	// defaultTimeoutSeconds is the default job timeout (1 hour).
	defaultTimeoutSeconds = 3600

	// currentSchedulerVersion is the current scheduler version.
	currentSchedulerVersion = 2
)

// ScheduleType represents the type of scheduling for a job.
type ScheduleType string

const (
	// ScheduleTypeCron uses cron expressions for scheduling.
	ScheduleTypeCron ScheduleType = "cron"

	// ScheduleTypeInterval uses simple intervals for scheduling.
	ScheduleTypeInterval ScheduleType = "interval"

	// ScheduleTypeImmediate runs the job immediately once.
	ScheduleTypeImmediate ScheduleType = "immediate"

	// ScheduleTypeEvent triggers based on external events.
	ScheduleTypeEvent ScheduleType = "event"
)

// IsValid returns true if the schedule type is valid.
func (s ScheduleType) IsValid() bool {
	switch s {
	case ScheduleTypeCron, ScheduleTypeInterval, ScheduleTypeImmediate, ScheduleTypeEvent:
		return true
	default:
		return false
	}
}

// JobV2 extends the base Job with V2 scheduler features.
type JobV2 struct {
	*basedomain.Job

	// V2 Scheduling fields
	ScheduleType   ScheduleType `db:"schedule_type"   json:"schedule_type"`
	CronExpression *string      `db:"cron_expression" json:"cron_expression,omitempty"`
	Priority       int          `db:"priority"        json:"priority"`
	TimeoutSeconds int          `db:"timeout_seconds" json:"timeout_seconds"`

	// Dependencies
	DependsOn []string `db:"depends_on" json:"depends_on,omitempty"`

	// Event triggers
	TriggerWebhook *string `db:"trigger_webhook" json:"trigger_webhook,omitempty"`
	TriggerChannel *string `db:"trigger_channel" json:"trigger_channel,omitempty"`

	// Scheduler version
	SchedulerVersion int `db:"scheduler_version" json:"scheduler_version"`
}

// NewJobV2 creates a new V2 job from a base job.
func NewJobV2(job *basedomain.Job) *JobV2 {
	return &JobV2{
		Job:              job,
		ScheduleType:     ScheduleTypeInterval, // Default
		Priority:         int(queue.PriorityNormal),
		TimeoutSeconds:   defaultTimeoutSeconds,
		SchedulerVersion: currentSchedulerVersion,
	}
}

// GetPriority returns the job's priority as a queue.Priority.
func (j *JobV2) GetPriority() queue.Priority {
	priority, err := queue.ParsePriority(j.Priority)
	if err != nil {
		return queue.PriorityNormal
	}
	return priority
}

// GetTimeout returns the job's timeout as a duration.
func (j *JobV2) GetTimeout() time.Duration {
	if j.TimeoutSeconds <= 0 {
		return time.Hour
	}
	return time.Duration(j.TimeoutSeconds) * time.Second
}

// IsEventTriggered returns true if the job is triggered by events.
func (j *JobV2) IsEventTriggered() bool {
	return j.ScheduleType == ScheduleTypeEvent
}

// IsCronScheduled returns true if the job uses cron scheduling.
func (j *JobV2) IsCronScheduled() bool {
	return j.ScheduleType == ScheduleTypeCron && j.CronExpression != nil
}

// IsIntervalScheduled returns true if the job uses interval scheduling.
func (j *JobV2) IsIntervalScheduled() bool {
	return j.ScheduleType == ScheduleTypeInterval
}

// IsImmediate returns true if the job should run immediately.
func (j *JobV2) IsImmediate() bool {
	return j.ScheduleType == ScheduleTypeImmediate
}

// HasDependencies returns true if the job has dependencies.
func (j *JobV2) HasDependencies() bool {
	return len(j.DependsOn) > 0
}

// JobCreateRequest represents a request to create a V2 job.
type JobCreateRequest struct {
	// Base fields
	SourceID string `binding:"required"     json:"source_id"`
	URL      string `binding:"required,url" json:"url"`

	// V2 scheduling
	ScheduleType    ScheduleType `json:"schedule_type"`
	CronExpression  *string      `json:"cron_expression,omitempty"`
	IntervalMinutes *int         `json:"interval_minutes,omitempty"`
	IntervalType    string       `json:"interval_type,omitempty"`

	// Priority and timeout
	Priority       *int `json:"priority,omitempty"`
	TimeoutSeconds *int `json:"timeout_seconds,omitempty"`

	// Dependencies
	DependsOn []string `json:"depends_on,omitempty"`

	// Event triggers
	TriggerWebhook *string `json:"trigger_webhook,omitempty"`
	TriggerChannel *string `json:"trigger_channel,omitempty"`

	// Scheduling enabled
	ScheduleEnabled bool `json:"schedule_enabled"`
}

// JobUpdateRequest represents a request to update a V2 job.
type JobUpdateRequest struct {
	// V2 scheduling
	ScheduleType    *ScheduleType `json:"schedule_type,omitempty"`
	CronExpression  *string       `json:"cron_expression,omitempty"`
	IntervalMinutes *int          `json:"interval_minutes,omitempty"`
	IntervalType    *string       `json:"interval_type,omitempty"`

	// Priority and timeout
	Priority       *int `json:"priority,omitempty"`
	TimeoutSeconds *int `json:"timeout_seconds,omitempty"`

	// Dependencies
	DependsOn []string `json:"depends_on,omitempty"`

	// Event triggers
	TriggerWebhook *string `json:"trigger_webhook,omitempty"`
	TriggerChannel *string `json:"trigger_channel,omitempty"`

	// Scheduling enabled
	ScheduleEnabled *bool `json:"schedule_enabled,omitempty"`
}
