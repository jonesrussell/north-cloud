// Package job_test provides tests for the job package.
package job_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infraevents "github.com/north-cloud/infrastructure/events"
)

// MockJobRepository implements job.Repository for testing.
type MockJobRepository struct {
	jobs            map[uuid.UUID]*domain.Job
	processedEvents map[uuid.UUID]bool
	findBySourceErr error
	upsertErr       error
	deleteErr       error
	updateStatusErr error
	recordEventErr  error
	isProcessedErr  error
}

// NewMockJobRepository creates a new mock repository.
func NewMockJobRepository() *MockJobRepository {
	return &MockJobRepository{
		jobs:            make(map[uuid.UUID]*domain.Job),
		processedEvents: make(map[uuid.UUID]bool),
	}
}

// FindBySourceID returns a job by source ID.
func (m *MockJobRepository) FindBySourceID(_ context.Context, sourceID uuid.UUID) (*domain.Job, error) {
	if m.findBySourceErr != nil {
		return nil, m.findBySourceErr
	}
	if j, ok := m.jobs[sourceID]; ok {
		return j, nil
	}
	return nil, job.ErrJobNotFoundBySourceID
}

// UpsertAutoManaged creates or updates an auto-managed job.
func (m *MockJobRepository) UpsertAutoManaged(_ context.Context, j *domain.Job) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	sourceID, parseErr := uuid.Parse(j.SourceID)
	if parseErr != nil {
		return parseErr
	}
	m.jobs[sourceID] = j
	return nil
}

// DeleteBySourceID deletes a job by source ID.
func (m *MockJobRepository) DeleteBySourceID(_ context.Context, sourceID uuid.UUID) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.jobs, sourceID)
	return nil
}

// UpdateStatusBySourceID updates a job's status by source ID.
func (m *MockJobRepository) UpdateStatusBySourceID(_ context.Context, sourceID uuid.UUID, status string) error {
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	if j, ok := m.jobs[sourceID]; ok {
		j.Status = status
	}
	return nil
}

// RecordProcessedEvent records an event as processed.
func (m *MockJobRepository) RecordProcessedEvent(_ context.Context, eventID uuid.UUID) error {
	if m.recordEventErr != nil {
		return m.recordEventErr
	}
	m.processedEvents[eventID] = true
	return nil
}

// IsEventProcessed checks if an event has been processed.
func (m *MockJobRepository) IsEventProcessed(_ context.Context, eventID uuid.UUID) (bool, error) {
	if m.isProcessedErr != nil {
		return false, m.isProcessedErr
	}
	return m.processedEvents[eventID], nil
}

// MockSourceClient implements sources.Client for testing.
type MockSourceClient struct {
	sources      map[uuid.UUID]*sources.Source
	getSourceErr error
}

// NewMockSourceClient creates a new mock source client.
func NewMockSourceClient() *MockSourceClient {
	return &MockSourceClient{
		sources: make(map[uuid.UUID]*sources.Source),
	}
}

// GetSource returns a source by ID.
func (m *MockSourceClient) GetSource(_ context.Context, sourceID uuid.UUID) (*sources.Source, error) {
	if m.getSourceErr != nil {
		return nil, m.getSourceErr
	}
	if s, ok := m.sources[sourceID]; ok {
		return s, nil
	}
	return nil, sources.ErrSourceNotFound
}

// AddSource adds a source to the mock.
func (m *MockSourceClient) AddSource(s *sources.Source) {
	m.sources[s.ID] = s
}

// --- Tests ---

func TestEventService_HandleSourceCreated_CreatesJob(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()

	service := job.NewEventService(repo, scheduleComputer, sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceCreated,
		SourceID:  sourceID,
		Timestamp: time.Now(),
		Payload: infraevents.SourceCreatedPayload{
			Name:      "Test Source",
			URL:       "https://example.com",
			RateLimit: 10,
			MaxDepth:  2,
			Enabled:   true,
			Priority:  "normal",
		},
	}

	err := service.HandleSourceCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job was created
	createdJob, findErr := repo.FindBySourceID(context.Background(), sourceID)
	if findErr != nil {
		t.Fatalf("expected job to be found, got error: %v", findErr)
	}

	if !createdJob.AutoManaged {
		t.Error("expected job to be auto-managed")
	}

	if createdJob.SourceID != sourceID.String() {
		t.Errorf("expected source_id %s, got %s", sourceID, createdJob.SourceID)
	}

	if createdJob.URL != "https://example.com" {
		t.Errorf("expected URL https://example.com, got %s", createdJob.URL)
	}

	if createdJob.Status != "pending" {
		t.Errorf("expected status pending, got %s", createdJob.Status)
	}

	// Verify event was recorded
	if !repo.processedEvents[eventID] {
		t.Error("expected event to be recorded as processed")
	}
}

func TestEventService_HandleSourceCreated_SkipsDisabled(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()

	service := job.NewEventService(repo, scheduleComputer, sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceCreated,
		SourceID:  sourceID,
		Timestamp: time.Now(),
		Payload: infraevents.SourceCreatedPayload{
			Name:      "Disabled Source",
			URL:       "https://disabled.com",
			RateLimit: 10,
			MaxDepth:  2,
			Enabled:   false, // Source is disabled
			Priority:  "normal",
		},
	}

	err := service.HandleSourceCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify no job was created
	_, findErr := repo.FindBySourceID(context.Background(), sourceID)
	if !errors.Is(findErr, job.ErrJobNotFoundBySourceID) {
		t.Error("expected no job to be created for disabled source")
	}

	// Event should still be recorded
	if !repo.processedEvents[eventID] {
		t.Error("expected event to be recorded as processed")
	}
}

func TestEventService_HandleSourceDeleted_DeletesJob(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()

	service := job.NewEventService(repo, scheduleComputer, sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	// Pre-create a job
	existingJob := &domain.Job{
		ID:          uuid.New().String(),
		SourceID:    sourceID.String(),
		URL:         "https://example.com",
		AutoManaged: true,
		Status:      "scheduled",
	}
	repo.jobs[sourceID] = existingJob

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceDeleted,
		SourceID:  sourceID,
		Timestamp: time.Now(),
		Payload: infraevents.SourceDeletedPayload{
			Name:           "Test Source",
			DeletionReason: "user requested",
		},
	}

	err := service.HandleSourceDeleted(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job was deleted
	_, findErr := repo.FindBySourceID(context.Background(), sourceID)
	if !errors.Is(findErr, job.ErrJobNotFoundBySourceID) {
		t.Error("expected job to be deleted")
	}

	// Verify event was recorded
	if !repo.processedEvents[eventID] {
		t.Error("expected event to be recorded as processed")
	}
}

func TestEventService_HandleSourceDisabled_PausesJob(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()

	service := job.NewEventService(repo, scheduleComputer, sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	// Pre-create a job
	existingJob := &domain.Job{
		ID:          uuid.New().String(),
		SourceID:    sourceID.String(),
		URL:         "https://example.com",
		AutoManaged: true,
		Status:      "scheduled",
	}
	repo.jobs[sourceID] = existingJob

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceDisabled,
		SourceID:  sourceID,
		Timestamp: time.Now(),
		Payload: infraevents.SourceTogglePayload{
			Reason:    "maintenance",
			ToggledBy: "user",
		},
	}

	err := service.HandleSourceDisabled(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job status was updated to paused
	foundJob, findErr := repo.FindBySourceID(context.Background(), sourceID)
	if findErr != nil {
		t.Fatalf("expected job to be found, got error: %v", findErr)
	}

	if foundJob.Status != "paused" {
		t.Errorf("expected status paused, got %s", foundJob.Status)
	}

	// Verify event was recorded
	if !repo.processedEvents[eventID] {
		t.Error("expected event to be recorded as processed")
	}
}

func TestEventService_HandleSourceEnabled_ResumesJob(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()

	service := job.NewEventService(repo, scheduleComputer, sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	// Add source to client for fetch
	sourceClient.AddSource(&sources.Source{
		ID:        sourceID,
		Name:      "Test Source",
		URL:       "https://example.com",
		RateLimit: 10,
		MaxDepth:  2,
		Enabled:   true,
		Priority:  "normal",
	})

	// Pre-create a paused job
	existingJob := &domain.Job{
		ID:          uuid.New().String(),
		SourceID:    sourceID.String(),
		URL:         "https://example.com",
		AutoManaged: true,
		Status:      "paused",
	}
	repo.jobs[sourceID] = existingJob

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceEnabled,
		SourceID:  sourceID,
		Timestamp: time.Now(),
		Payload: infraevents.SourceTogglePayload{
			Reason:    "ready to crawl",
			ToggledBy: "user",
		},
	}

	err := service.HandleSourceEnabled(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job status was updated to pending
	foundJob, findErr := repo.FindBySourceID(context.Background(), sourceID)
	if findErr != nil {
		t.Fatalf("expected job to be found, got error: %v", findErr)
	}

	if foundJob.Status != "pending" {
		t.Errorf("expected status pending, got %s", foundJob.Status)
	}

	// Verify event was recorded
	if !repo.processedEvents[eventID] {
		t.Error("expected event to be recorded as processed")
	}
}

func TestEventService_Idempotency_SkipsDuplicateEvents(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()

	service := job.NewEventService(repo, scheduleComputer, sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	// Mark event as already processed
	repo.processedEvents[eventID] = true

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceCreated,
		SourceID:  sourceID,
		Timestamp: time.Now(),
		Payload: infraevents.SourceCreatedPayload{
			Name:      "Test Source",
			URL:       "https://example.com",
			RateLimit: 10,
			MaxDepth:  2,
			Enabled:   true,
			Priority:  "normal",
		},
	}

	err := service.HandleSourceCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify no job was created (event was skipped)
	_, findErr := repo.FindBySourceID(context.Background(), sourceID)
	if !errors.Is(findErr, job.ErrJobNotFoundBySourceID) {
		t.Error("expected no job to be created for duplicate event")
	}
}

func TestEventService_HandleSourceUpdated_ReschedulesJob(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()

	service := job.NewEventService(repo, scheduleComputer, sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	// Add source to client for fetch
	sourceClient.AddSource(&sources.Source{
		ID:        sourceID,
		Name:      "Test Source",
		URL:       "https://example.com",
		RateLimit: 5, // Changed rate limit
		MaxDepth:  3, // Changed max depth
		Enabled:   true,
		Priority:  "high", // Changed priority
	})

	// Pre-create a job with old settings
	intervalMinutes := 60
	existingJob := &domain.Job{
		ID:              uuid.New().String(),
		SourceID:        sourceID.String(),
		URL:             "https://example.com",
		AutoManaged:     true,
		Status:          "scheduled",
		IntervalMinutes: &intervalMinutes,
		IntervalType:    "minutes",
		Priority:        50,
	}
	repo.jobs[sourceID] = existingJob

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceUpdated,
		SourceID:  sourceID,
		Timestamp: time.Now(),
		Payload: infraevents.SourceUpdatedPayload{
			ChangedFields: []string{"rate_limit", "max_depth", "priority"},
			Previous: map[string]any{
				"rate_limit": 10,
				"max_depth":  2,
				"priority":   "normal",
			},
			Current: map[string]any{
				"rate_limit": 5,
				"max_depth":  3,
				"priority":   "high",
			},
		},
	}

	err := service.HandleSourceUpdated(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job was updated with new schedule
	updatedJob, findErr := repo.FindBySourceID(context.Background(), sourceID)
	if findErr != nil {
		t.Fatalf("expected job to be found, got error: %v", findErr)
	}

	// The schedule should be different from the original
	if updatedJob.IntervalMinutes == nil || *updatedJob.IntervalMinutes == intervalMinutes {
		t.Error("expected interval to be recalculated")
	}

	// Verify event was recorded
	if !repo.processedEvents[eventID] {
		t.Error("expected event to be recorded as processed")
	}
}

func TestEventService_ImplementsEventHandler(t *testing.T) {
	t.Helper()

	// Compile-time check that EventService implements events.EventHandler
	var _ events.EventHandler = (*job.EventService)(nil)
}

func TestEventService_HandleSourceEnabled_CreatesJobIfNotExists(t *testing.T) {
	t.Helper()

	repo := NewMockJobRepository()
	sourceClient := NewMockSourceClient()
	scheduleComputer := job.NewScheduleComputer()

	service := job.NewEventService(repo, scheduleComputer, sourceClient, nil)

	sourceID := uuid.New()
	eventID := uuid.New()

	// Add source to client for fetch
	sourceClient.AddSource(&sources.Source{
		ID:        sourceID,
		Name:      "New Source",
		URL:       "https://new-source.com",
		RateLimit: 10,
		MaxDepth:  2,
		Enabled:   true,
		Priority:  "normal",
	})

	// No existing job

	event := infraevents.SourceEvent{
		EventID:   eventID,
		EventType: infraevents.SourceEnabled,
		SourceID:  sourceID,
		Timestamp: time.Now(),
		Payload: infraevents.SourceTogglePayload{
			Reason:    "enabled for first time",
			ToggledBy: "user",
		},
	}

	err := service.HandleSourceEnabled(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify job was created
	createdJob, findErr := repo.FindBySourceID(context.Background(), sourceID)
	if findErr != nil {
		t.Fatalf("expected job to be created, got error: %v", findErr)
	}

	if !createdJob.AutoManaged {
		t.Error("expected job to be auto-managed")
	}

	if createdJob.Status != "pending" {
		t.Errorf("expected status pending, got %s", createdJob.Status)
	}

	// Verify event was recorded
	if !repo.processedEvents[eventID] {
		t.Error("expected event to be recorded as processed")
	}
}
