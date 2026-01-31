// Package job provides core job service functionality.
package job

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infraevents "github.com/north-cloud/infrastructure/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// ErrJobNotFoundBySourceID is returned when no job exists for a given source ID.
var ErrJobNotFoundBySourceID = errors.New("job not found for source_id")

// Error definitions for event handling.
var (
	errInvalidSourceCreatedPayload = errors.New("invalid payload type for SOURCE_CREATED")
	errInvalidSourceUpdatedPayload = errors.New("invalid payload type for SOURCE_UPDATED")
)

// Schedule-affecting fields that trigger job reschedule on update.
var scheduleAffectingFields = map[string]bool{
	"rate_limit": true,
	"max_depth":  true,
	"priority":   true,
}

// Default job settings.
const (
	defaultMaxRetries          = 3
	defaultRetryBackoffSeconds = 60
	defaultSchedulerVersion    = 1
)

// Repository defines the interface for job persistence operations.
type Repository interface {
	FindBySourceID(ctx context.Context, sourceID uuid.UUID) (*domain.Job, error)
	UpsertAutoManaged(ctx context.Context, job *domain.Job) error
	DeleteBySourceID(ctx context.Context, sourceID uuid.UUID) error
	UpdateStatusBySourceID(ctx context.Context, sourceID uuid.UUID, status string) error
	RecordProcessedEvent(ctx context.Context, eventID uuid.UUID) error
	IsEventProcessed(ctx context.Context, eventID uuid.UUID) (bool, error)
}

// EventService manages job lifecycle based on source events.
// It implements the events.EventHandler interface.
type EventService struct {
	repo             Repository
	scheduleComputer *ScheduleComputer
	sourceClient     sources.Client
	log              infralogger.Logger
}

// NewEventService creates a new event-driven job service.
func NewEventService(
	repo Repository,
	scheduleComputer *ScheduleComputer,
	sourceClient sources.Client,
	log infralogger.Logger,
) *EventService {
	return &EventService{
		repo:             repo,
		scheduleComputer: scheduleComputer,
		sourceClient:     sourceClient,
		log:              log,
	}
}

// HandleSourceCreated creates an auto-managed job when a source is created.
func (s *EventService) HandleSourceCreated(ctx context.Context, event infraevents.SourceEvent) error {
	// Check idempotency
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		s.logDebug("Skipping already processed event",
			infralogger.String("event_id", event.EventID.String()),
			infralogger.String("event_type", string(event.EventType)),
		)
		return nil
	}

	// Extract payload
	payload, ok := event.Payload.(infraevents.SourceCreatedPayload)
	if !ok {
		return s.recordAndReturn(ctx, event.EventID, errInvalidSourceCreatedPayload)
	}

	// Skip if source is disabled
	if !payload.Enabled {
		s.logInfo("Skipping job creation for disabled source",
			infralogger.String("source_id", event.SourceID.String()),
			infralogger.String("source_name", payload.Name),
		)
		return s.recordEvent(ctx, event.EventID)
	}

	// Compute schedule
	schedule := s.scheduleComputer.ComputeSchedule(ScheduleInput{
		RateLimit: payload.RateLimit,
		MaxDepth:  payload.MaxDepth,
		Priority:  payload.Priority,
	})

	// Calculate next run time
	nextRunAt := timePtr(time.Now().Add(schedule.InitialDelay))

	// Create auto-managed job
	job := &domain.Job{
		ID:                  uuid.New().String(),
		SourceID:            event.SourceID.String(),
		SourceName:          &payload.Name,
		URL:                 payload.URL,
		IntervalMinutes:     &schedule.IntervalMinutes,
		IntervalType:        schedule.IntervalType,
		NextRunAt:           nextRunAt,
		Status:              "pending",
		AutoManaged:         true,
		Priority:            schedule.NumericPriority,
		ScheduleEnabled:     true,
		MaxRetries:          defaultMaxRetries,
		RetryBackoffSeconds: defaultRetryBackoffSeconds,
		SchedulerVersion:    defaultSchedulerVersion,
	}

	if upsertErr := s.repo.UpsertAutoManaged(ctx, job); upsertErr != nil {
		return fmt.Errorf("create auto-managed job: %w", upsertErr)
	}

	s.logInfo("Created auto-managed job for source",
		infralogger.String("source_id", event.SourceID.String()),
		infralogger.String("job_id", job.ID),
		infralogger.Int("interval_minutes", schedule.IntervalMinutes),
	)

	return s.recordEvent(ctx, event.EventID)
}

// HandleSourceUpdated reschedules a job if schedule-affecting fields changed.
func (s *EventService) HandleSourceUpdated(ctx context.Context, event infraevents.SourceEvent) error {
	// Check idempotency
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		s.logDebug("Skipping already processed event",
			infralogger.String("event_id", event.EventID.String()),
			infralogger.String("event_type", string(event.EventType)),
		)
		return nil
	}

	// Extract payload
	payload, ok := event.Payload.(infraevents.SourceUpdatedPayload)
	if !ok {
		return s.recordAndReturn(ctx, event.EventID, errInvalidSourceUpdatedPayload)
	}

	// Check if any schedule-affecting fields changed
	if !s.hasScheduleAffectingChanges(payload.ChangedFields) {
		s.logDebug("No schedule-affecting changes, skipping reschedule",
			infralogger.String("source_id", event.SourceID.String()),
		)
		return s.recordEvent(ctx, event.EventID)
	}

	// Fetch current source data
	source, fetchErr := s.sourceClient.GetSource(ctx, event.SourceID)
	if fetchErr != nil {
		if errors.Is(fetchErr, sources.ErrSourceNotFound) {
			s.logWarn("Source not found for update event",
				infralogger.String("source_id", event.SourceID.String()),
			)
			return s.recordEvent(ctx, event.EventID)
		}
		return fmt.Errorf("fetch source: %w", fetchErr)
	}

	// Find existing job
	existingJob, findErr := s.repo.FindBySourceID(ctx, event.SourceID)
	if findErr != nil {
		if errors.Is(findErr, ErrJobNotFoundBySourceID) {
			s.logWarn("No job found for source update",
				infralogger.String("source_id", event.SourceID.String()),
			)
			return s.recordEvent(ctx, event.EventID)
		}
		return fmt.Errorf("find job by source_id: %w", findErr)
	}

	// Recompute schedule
	schedule := s.scheduleComputer.ComputeSchedule(ScheduleInput{
		RateLimit:    source.RateLimit,
		MaxDepth:     source.MaxDepth,
		Priority:     source.Priority,
		FailureCount: existingJob.FailureCount,
	})

	// Update job with new schedule
	existingJob.IntervalMinutes = &schedule.IntervalMinutes
	existingJob.IntervalType = schedule.IntervalType
	existingJob.Priority = schedule.NumericPriority

	if upsertErr := s.repo.UpsertAutoManaged(ctx, existingJob); upsertErr != nil {
		return fmt.Errorf("update auto-managed job: %w", upsertErr)
	}

	s.logInfo("Rescheduled job for source update",
		infralogger.String("source_id", event.SourceID.String()),
		infralogger.String("job_id", existingJob.ID),
		infralogger.Int("new_interval_minutes", schedule.IntervalMinutes),
	)

	return s.recordEvent(ctx, event.EventID)
}

// HandleSourceDeleted deletes the job associated with a source.
func (s *EventService) HandleSourceDeleted(ctx context.Context, event infraevents.SourceEvent) error {
	// Check idempotency
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		s.logDebug("Skipping already processed event",
			infralogger.String("event_id", event.EventID.String()),
			infralogger.String("event_type", string(event.EventType)),
		)
		return nil
	}

	// Delete job by source ID
	if deleteErr := s.repo.DeleteBySourceID(ctx, event.SourceID); deleteErr != nil {
		return fmt.Errorf("delete job by source_id: %w", deleteErr)
	}

	s.logInfo("Deleted job for source",
		infralogger.String("source_id", event.SourceID.String()),
	)

	return s.recordEvent(ctx, event.EventID)
}

// HandleSourceEnabled resumes or creates a job when a source is enabled.
func (s *EventService) HandleSourceEnabled(ctx context.Context, event infraevents.SourceEvent) error {
	// Check idempotency
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		s.logDebug("Skipping already processed event",
			infralogger.String("event_id", event.EventID.String()),
			infralogger.String("event_type", string(event.EventType)),
		)
		return nil
	}

	// Try to find existing job
	existingJob, findErr := s.repo.FindBySourceID(ctx, event.SourceID)
	if findErr != nil && !errors.Is(findErr, ErrJobNotFoundBySourceID) {
		return fmt.Errorf("find job by source_id: %w", findErr)
	}

	// Fetch source data
	source, fetchErr := s.sourceClient.GetSource(ctx, event.SourceID)
	if fetchErr != nil {
		if errors.Is(fetchErr, sources.ErrSourceNotFound) {
			s.logWarn("Source not found for enable event",
				infralogger.String("source_id", event.SourceID.String()),
			)
			return s.recordEvent(ctx, event.EventID)
		}
		return fmt.Errorf("fetch source: %w", fetchErr)
	}

	if existingJob != nil {
		// Resume existing job
		existingJob.Status = "pending"
		existingJob.NextRunAt = timePtr(time.Now())
		existingJob.IsPaused = false

		if upsertErr := s.repo.UpsertAutoManaged(ctx, existingJob); upsertErr != nil {
			return fmt.Errorf("resume job: %w", upsertErr)
		}

		s.logInfo("Resumed job for enabled source",
			infralogger.String("source_id", event.SourceID.String()),
			infralogger.String("job_id", existingJob.ID),
		)
	} else {
		// Create new job
		schedule := s.scheduleComputer.ComputeSchedule(ScheduleInput{
			RateLimit: source.RateLimit,
			MaxDepth:  source.MaxDepth,
			Priority:  source.Priority,
		})

		job := &domain.Job{
			ID:                  uuid.New().String(),
			SourceID:            event.SourceID.String(),
			SourceName:          &source.Name,
			URL:                 source.URL,
			IntervalMinutes:     &schedule.IntervalMinutes,
			IntervalType:        schedule.IntervalType,
			NextRunAt:           timePtr(time.Now()),
			Status:              "pending",
			AutoManaged:         true,
			Priority:            schedule.NumericPriority,
			ScheduleEnabled:     true,
			MaxRetries:          defaultMaxRetries,
			RetryBackoffSeconds: defaultRetryBackoffSeconds,
			SchedulerVersion:    defaultSchedulerVersion,
		}

		if upsertErr := s.repo.UpsertAutoManaged(ctx, job); upsertErr != nil {
			return fmt.Errorf("create job for enabled source: %w", upsertErr)
		}

		s.logInfo("Created job for enabled source",
			infralogger.String("source_id", event.SourceID.String()),
			infralogger.String("job_id", job.ID),
		)
	}

	return s.recordEvent(ctx, event.EventID)
}

// HandleSourceDisabled pauses the job associated with a source.
func (s *EventService) HandleSourceDisabled(ctx context.Context, event infraevents.SourceEvent) error {
	// Check idempotency
	processed, err := s.repo.IsEventProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check event processed: %w", err)
	}
	if processed {
		s.logDebug("Skipping already processed event",
			infralogger.String("event_id", event.EventID.String()),
			infralogger.String("event_type", string(event.EventType)),
		)
		return nil
	}

	// Update job status to paused
	if updateErr := s.repo.UpdateStatusBySourceID(ctx, event.SourceID, "paused"); updateErr != nil {
		return fmt.Errorf("pause job: %w", updateErr)
	}

	s.logInfo("Paused job for disabled source",
		infralogger.String("source_id", event.SourceID.String()),
	)

	return s.recordEvent(ctx, event.EventID)
}

// hasScheduleAffectingChanges checks if any schedule-affecting fields changed.
func (s *EventService) hasScheduleAffectingChanges(changedFields []string) bool {
	for _, field := range changedFields {
		if scheduleAffectingFields[field] {
			return true
		}
	}
	return false
}

// recordEvent records an event as processed.
func (s *EventService) recordEvent(ctx context.Context, eventID uuid.UUID) error {
	if recordErr := s.repo.RecordProcessedEvent(ctx, eventID); recordErr != nil {
		return fmt.Errorf("record processed event: %w", recordErr)
	}
	return nil
}

// recordAndReturn records the event and returns the provided error.
func (s *EventService) recordAndReturn(ctx context.Context, eventID uuid.UUID, err error) error {
	if recordErr := s.repo.RecordProcessedEvent(ctx, eventID); recordErr != nil {
		s.logWarn("Failed to record processed event",
			infralogger.String("event_id", eventID.String()),
			infralogger.String("record_error", recordErr.Error()),
		)
	}
	return err
}

// logInfo logs at info level if logger is not nil.
func (s *EventService) logInfo(msg string, fields ...infralogger.Field) {
	if s.log != nil {
		s.log.Info(msg, fields...)
	}
}

// logDebug logs at debug level if logger is not nil.
func (s *EventService) logDebug(msg string, fields ...infralogger.Field) {
	if s.log != nil {
		s.log.Debug(msg, fields...)
	}
}

// logWarn logs at warn level if logger is not nil.
func (s *EventService) logWarn(msg string, fields ...infralogger.Field) {
	if s.log != nil {
		s.log.Warn(msg, fields...)
	}
}

// timePtr returns a pointer to the given time.
func timePtr(t time.Time) *time.Time {
	return &t
}
