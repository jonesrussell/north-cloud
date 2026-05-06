// Package catalogue provides an SQLite-backed store for poll checkpoints and
// alert deduplication / rescission state.
//
// Layer: L1.  Imports: domain (L0), stdlib, mattn/go-sqlite3 (driver only).
// CGO is required because go-sqlite3 is a CGO binding.
package catalogue

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver — CGO required.
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// ErrNotFound is returned by LookupAlert when no matching row exists.
var ErrNotFound = errors.New("catalogue: alert not found")

// PollCheckpoint holds HTTP-caching metadata for one (source, feed) pair.
type PollCheckpoint struct {
	SourceID            string
	FeedURL             string
	LastPolledAt        time.Time
	LastEtag            string
	LastModified        string
	LastStatus          int
	ConsecutiveFailures int
}

// CatalogEntry holds deduplication state for one alert within a source.
type CatalogEntry struct {
	SourceID    string
	AlertID     string
	LastSeenAt  time.Time
	IsActive    bool
	ContentHash string
}

// Store wraps an *sql.DB and provides the catalogue operations.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at path, runs embedded migrations,
// and returns a ready-to-use Store.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("catalogue: open db: %w", err)
	}

	if pingErr := db.PingContext(ctx); pingErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("catalogue: ping db: %w", pingErr)
	}

	s := &Store{db: db}
	if migrErr := s.runMigrations(ctx); migrErr != nil {
		_ = db.Close()
		return nil, migrErr
	}

	return s, nil
}

// runMigrations applies embedded SQL migration files in lexicographic order.
// Each file is executed inside its own transaction; already-applied statements
// using IF NOT EXISTS are idempotent.
func (s *Store) runMigrations(ctx context.Context) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("catalogue: read migrations dir: %w", err)
	}

	for _, entry := range entries {
		// Only apply up migrations here.
		name := entry.Name()
		if len(name) < len(upSuffix) || name[len(name)-len(upSuffix):] != upSuffix {
			continue
		}

		sqlBytes, readErr := migrationFS.ReadFile("migrations/" + name)
		if readErr != nil {
			return fmt.Errorf("catalogue: read migration %s: %w", name, readErr)
		}

		if execErr := s.execMigration(ctx, string(sqlBytes)); execErr != nil {
			return fmt.Errorf("catalogue: apply migration %s: %w", name, execErr)
		}
	}

	return nil
}

const upSuffix = ".up.sql"

func (s *Store) execMigration(ctx context.Context, ddl string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if _, execErr := tx.ExecContext(ctx, ddl); execErr != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec ddl: %w", execErr)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("commit: %w", commitErr)
	}

	return nil
}

// LoadCheckpoint returns the checkpoint for (sourceID, feedURL), or nil if none exists.
func (s *Store) LoadCheckpoint(ctx context.Context, sourceID, feedURL string) (*PollCheckpoint, error) {
	const q = `
		SELECT source_id, feed_url, last_polled_at, last_etag, last_modified,
		       last_status, consecutive_failures
		FROM poll_checkpoint
		WHERE source_id = ? AND feed_url = ?`

	row := s.db.QueryRowContext(ctx, q, sourceID, feedURL)

	var c PollCheckpoint
	var lastPolledAt string

	err := row.Scan(
		&c.SourceID, &c.FeedURL, &lastPolledAt,
		&c.LastEtag, &c.LastModified, &c.LastStatus, &c.ConsecutiveFailures,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // Intentional: nil means no checkpoint exists yet.
	}

	if err != nil {
		return nil, fmt.Errorf("catalogue: load checkpoint: %w", err)
	}

	parsed, parseErr := time.Parse(time.RFC3339, lastPolledAt)
	if parseErr != nil {
		return nil, fmt.Errorf("catalogue: parse last_polled_at: %w", parseErr)
	}

	c.LastPolledAt = parsed

	return &c, nil
}

// SaveCheckpoint upserts the checkpoint for (sourceID, feedURL).
func (s *Store) SaveCheckpoint(ctx context.Context, c PollCheckpoint) error {
	const q = `
		INSERT INTO poll_checkpoint
		    (source_id, feed_url, last_polled_at, last_etag, last_modified, last_status, consecutive_failures)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_id, feed_url) DO UPDATE SET
		    last_polled_at       = excluded.last_polled_at,
		    last_etag            = excluded.last_etag,
		    last_modified        = excluded.last_modified,
		    last_status          = excluded.last_status,
		    consecutive_failures = excluded.consecutive_failures`

	_, err := s.db.ExecContext(ctx, q,
		c.SourceID, c.FeedURL, c.LastPolledAt.UTC().Format(time.RFC3339),
		c.LastEtag, c.LastModified, c.LastStatus, c.ConsecutiveFailures,
	)
	if err != nil {
		return fmt.Errorf("catalogue: save checkpoint: %w", err)
	}

	return nil
}

// IncrementConsecutiveFailures increments the failure counter for (sourceID, feedURL).
// If no checkpoint exists, the row is inserted with consecutive_failures = 1.
func (s *Store) IncrementConsecutiveFailures(ctx context.Context, sourceID, feedURL string) error {
	const q = `
		INSERT INTO poll_checkpoint (source_id, feed_url, last_polled_at, consecutive_failures)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(source_id, feed_url) DO UPDATE SET
		    consecutive_failures = consecutive_failures + 1`

	_, err := s.db.ExecContext(ctx, q, sourceID, feedURL, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("catalogue: increment failures: %w", err)
	}

	return nil
}

// ResetConsecutiveFailures sets consecutive_failures = 0 for (sourceID, feedURL).
func (s *Store) ResetConsecutiveFailures(ctx context.Context, sourceID, feedURL string) error {
	const q = `
		UPDATE poll_checkpoint
		SET    consecutive_failures = 0
		WHERE  source_id = ? AND feed_url = ?`

	_, err := s.db.ExecContext(ctx, q, sourceID, feedURL)
	if err != nil {
		return fmt.Errorf("catalogue: reset failures: %w", err)
	}

	return nil
}

// LookupAlert returns the CatalogEntry for (sourceID, alertID).
// Returns ErrNotFound when no row exists.
func (s *Store) LookupAlert(ctx context.Context, sourceID, alertID string) (*CatalogEntry, error) {
	const q = `
		SELECT source_id, alert_id, last_seen_at, is_active, content_hash
		FROM alert_catalogue
		WHERE source_id = ? AND alert_id = ?`

	row := s.db.QueryRowContext(ctx, q, sourceID, alertID)

	var e CatalogEntry
	var lastSeenAt string
	var isActiveInt int

	err := row.Scan(&e.SourceID, &e.AlertID, &lastSeenAt, &isActiveInt, &e.ContentHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("catalogue: lookup alert: %w", err)
	}

	parsed, parseErr := time.Parse(time.RFC3339, lastSeenAt)
	if parseErr != nil {
		return nil, fmt.Errorf("catalogue: parse last_seen_at: %w", parseErr)
	}

	e.LastSeenAt = parsed
	e.IsActive = isActiveInt != 0

	return &e, nil
}

// MarkSeen upserts a CatalogEntry, always setting is_active = 1.
// If the alert was previously rescinded, this reactivates it.
func (s *Store) MarkSeen(ctx context.Context, e CatalogEntry) error {
	const q = `
		INSERT INTO alert_catalogue (source_id, alert_id, last_seen_at, is_active, content_hash)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(source_id, alert_id) DO UPDATE SET
		    last_seen_at  = excluded.last_seen_at,
		    is_active     = 1,
		    content_hash  = excluded.content_hash`

	_, err := s.db.ExecContext(ctx, q,
		e.SourceID, e.AlertID, e.LastSeenAt.UTC().Format(time.RFC3339), e.ContentHash,
	)
	if err != nil {
		return fmt.Errorf("catalogue: mark seen: %w", err)
	}

	return nil
}

// RescindAbsent returns alert IDs that are still active (is_active = 1) but whose
// last_seen_at is before pollStartedAt. The caller should call MarkRescinded for
// each returned ID.
func (s *Store) RescindAbsent(ctx context.Context, sourceID string, pollStartedAt time.Time) ([]string, error) {
	const q = `
		SELECT alert_id
		FROM alert_catalogue
		WHERE source_id = ?
		  AND is_active = 1
		  AND last_seen_at < ?`

	rows, err := s.db.QueryContext(ctx, q, sourceID, pollStartedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("catalogue: rescind absent query: %w", err)
	}
	defer rows.Close()

	var ids []string

	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, fmt.Errorf("catalogue: rescind absent scan: %w", scanErr)
		}

		ids = append(ids, id)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("catalogue: rescind absent rows: %w", rowsErr)
	}

	return ids, nil
}

// MarkRescinded sets is_active = 0 for (sourceID, alertID).
func (s *Store) MarkRescinded(ctx context.Context, sourceID, alertID string) error {
	const q = `
		UPDATE alert_catalogue
		SET    is_active = 0
		WHERE  source_id = ? AND alert_id = ?`

	_, err := s.db.ExecContext(ctx, q, sourceID, alertID)
	if err != nil {
		return fmt.Errorf("catalogue: mark rescinded: %w", err)
	}

	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("catalogue: close db: %w", err)
	}

	return nil
}
