package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// GetCursor retrieves the current polling cursor
func (r *Repository) GetCursor(ctx context.Context) ([]any, error) {
	var cursorJSON []byte
	query := `SELECT last_sort FROM publisher_cursor WHERE id = 1`

	err := r.db.GetContext(ctx, &cursorJSON, query)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []any{}, nil
		}
		return nil, fmt.Errorf("failed to get cursor: %w", err)
	}

	var cursor []any
	if unmarshalErr := json.Unmarshal(cursorJSON, &cursor); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor: %w", unmarshalErr)
	}

	return cursor, nil
}

// UpdateCursor updates the polling cursor
func (r *Repository) UpdateCursor(ctx context.Context, cursor []any) error {
	cursorJSON, err := json.Marshal(cursor)
	if err != nil {
		return fmt.Errorf("failed to marshal cursor: %w", err)
	}

	query := `
		INSERT INTO publisher_cursor (id, last_sort, updated_at)
		VALUES (1, $1, $2)
		ON CONFLICT (id) DO UPDATE SET
			last_sort = EXCLUDED.last_sort,
			updated_at = EXCLUDED.updated_at
	`

	_, err = r.db.ExecContext(ctx, query, cursorJSON, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update cursor: %w", err)
	}

	return nil
}
