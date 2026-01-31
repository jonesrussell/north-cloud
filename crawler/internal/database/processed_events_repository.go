package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ProcessedEventsRepository handles database operations for event idempotency.
type ProcessedEventsRepository struct {
	db *sqlx.DB
}

// NewProcessedEventsRepository creates a new processed events repository.
func NewProcessedEventsRepository(db *sqlx.DB) *ProcessedEventsRepository {
	return &ProcessedEventsRepository{db: db}
}

// RecordProcessedEvent records an event as processed for idempotency.
func (r *ProcessedEventsRepository) RecordProcessedEvent(ctx context.Context, eventID uuid.UUID) error {
	query := `
		INSERT INTO processed_events (event_id, processed_at)
		VALUES ($1, NOW())
		ON CONFLICT (event_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("record processed event: %w", err)
	}

	return nil
}

// IsEventProcessed checks if an event has already been processed.
func (r *ProcessedEventsRepository) IsEventProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`

	err := r.db.GetContext(ctx, &exists, query, eventID)
	if err != nil {
		return false, fmt.Errorf("check event processed: %w", err)
	}

	return exists, nil
}
