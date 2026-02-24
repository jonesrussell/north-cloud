// Package database: discovery decision log for deterministic audit of the Source Candidate Pipeline.

package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// DecisionLogRepository persists discovery_decision_log entries (resolution, risk, approval, creation, frontier seed).
type DecisionLogRepository struct {
	db *sqlx.DB
}

// NewDecisionLogRepository creates a new decision log repository.
func NewDecisionLogRepository(db *sqlx.DB) *DecisionLogRepository {
	return &DecisionLogRepository{db: db}
}

// LogEntry represents one row in discovery_decision_log.
type LogEntry struct {
	Stage      string
	Reason     string
	Inputs     map[string]any
	Outputs    map[string]any
	OccurredAt time.Time
}

// Insert appends a decision log entry. OccurredAt defaults to now if zero.
func (r *DecisionLogRepository) Insert(ctx context.Context, stage, reason string, inputs, outputs map[string]any) error {
	if inputs == nil {
		inputs = map[string]any{}
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}
	var outputsJSON []byte
	if outputs != nil {
		outputsJSON, err = json.Marshal(outputs)
		if err != nil {
			return fmt.Errorf("marshal outputs: %w", err)
		}
	}

	query := `INSERT INTO discovery_decision_log (stage, reason, inputs, outputs) VALUES ($1, $2, $3, $4)`
	_, execErr := r.db.ExecContext(ctx, query, stage, reason, inputsJSON, outputsJSON)
	if execErr != nil {
		return fmt.Errorf("insert decision log: %w", execErr)
	}
	return nil
}

// InsertWithTime is like Insert but sets occurred_at explicitly (for replay/audit).
func (r *DecisionLogRepository) InsertWithTime(ctx context.Context, occurredAt time.Time, stage, reason string, inputs, outputs map[string]any) error {
	if inputs == nil {
		inputs = map[string]any{}
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}
	var outputsJSON []byte
	if outputs != nil {
		outputsJSON, err = json.Marshal(outputs)
		if err != nil {
			return fmt.Errorf("marshal outputs: %w", err)
		}
	}

	query := `INSERT INTO discovery_decision_log (occurred_at, stage, reason, inputs, outputs) VALUES ($1, $2, $3, $4, $5)`
	_, execErr := r.db.ExecContext(ctx, query, occurredAt, stage, reason, inputsJSON, outputsJSON)
	if execErr != nil {
		return fmt.Errorf("insert decision log: %w", execErr)
	}
	return nil
}
