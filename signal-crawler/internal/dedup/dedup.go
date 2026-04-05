package dedup

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// Store tracks which signals have already been ingested.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite dedup database. Use ":memory:" for testing.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open dedup db: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS seen (
		source      TEXT     NOT NULL,
		external_id TEXT     NOT NULL,
		first_seen  DATETIME NOT NULL DEFAULT (datetime('now')),
		PRIMARY KEY (source, external_id)
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create seen table: %w", err)
	}

	return &Store{db: db}, nil
}

// Seen checks whether a signal has already been ingested.
func (s *Store) Seen(ctx context.Context, source, externalID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM seen WHERE source = ? AND external_id = ?`,
		source, externalID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query seen: %w", err)
	}
	return count > 0, nil
}

// Mark records a signal as ingested.
func (s *Store) Mark(ctx context.Context, source, externalID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO seen (source, external_id) VALUES (?, ?)`,
		source, externalID,
	)
	if err != nil {
		return fmt.Errorf("mark seen: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
