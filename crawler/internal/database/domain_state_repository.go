package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// argsPerDomain is the number of SQL arguments per domain in bulk upsert operations.
const argsPerDomain = 3

// DomainStateRepository handles database operations for discovered domain states.
type DomainStateRepository struct {
	db *sqlx.DB
}

// NewDomainStateRepository creates a new domain state repository.
func NewDomainStateRepository(db *sqlx.DB) *DomainStateRepository {
	return &DomainStateRepository{db: db}
}

// Upsert creates or updates a domain state record.
// The INSERT ... ON CONFLICT is atomic. The follow-up setStatusTimestamp
// is a separate statement; a crash between them would leave the timestamp
// stale but not corrupt state. Accepted trade-off for simplicity.
func (r *DomainStateRepository) Upsert(ctx context.Context, domainName, status string, notes *string) error {
	now := time.Now()

	query := `
		INSERT INTO discovered_domain_states (domain, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (domain)
		DO UPDATE SET
			status = EXCLUDED.status,
			notes = EXCLUDED.notes,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query, domainName, status, notes, now)
	if err != nil {
		return fmt.Errorf("upsert domain state: %w", err)
	}

	return r.setStatusTimestamp(ctx, domainName, status)
}

// setStatusTimestamp updates the appropriate timestamp field based on status.
func (r *DomainStateRepository) setStatusTimestamp(ctx context.Context, domainName, status string) error {
	switch status {
	case domain.DomainStatusIgnored:
		_, err := r.db.ExecContext(ctx,
			`UPDATE discovered_domain_states SET ignored_at = NOW() WHERE domain = $1`,
			domainName)
		if err != nil {
			return fmt.Errorf("set ignored_at: %w", err)
		}
	case domain.DomainStatusPromoted:
		_, err := r.db.ExecContext(ctx,
			`UPDATE discovered_domain_states SET promoted_at = NOW() WHERE domain = $1`,
			domainName)
		if err != nil {
			return fmt.Errorf("set promoted_at: %w", err)
		}
	}

	return nil
}

// BulkUpsert updates states for multiple domains in a single transaction.
func (r *DomainStateRepository) BulkUpsert(ctx context.Context, domains []string, status string, notes *string) (int, error) {
	if len(domains) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	affected, execErr := r.executeBulkUpsert(ctx, tx, domains, status, notes)
	if execErr != nil {
		return 0, execErr
	}

	if tsErr := r.setBulkStatusTimestamps(ctx, tx, domains, status); tsErr != nil {
		return 0, tsErr
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return 0, fmt.Errorf("commit: %w", commitErr)
	}

	return affected, nil
}

// executeBulkUpsert builds and runs the batch INSERT ... ON CONFLICT query.
func (r *DomainStateRepository) executeBulkUpsert(
	ctx context.Context,
	tx *sqlx.Tx,
	domains []string,
	status string,
	notes *string,
) (int, error) {
	placeholders := make([]string, 0, len(domains))
	args := make([]any, 0, len(domains)*argsPerDomain)
	argIdx := 1

	for _, d := range domains {
		placeholders = append(placeholders,
			fmt.Sprintf("($%d, $%d, $%d, NOW(), NOW())", argIdx, argIdx+1, argIdx+argsPerDomain-1))
		args = append(args, d, status, notes)
		argIdx += argsPerDomain
	}

	query := fmt.Sprintf(`
		INSERT INTO discovered_domain_states (domain, status, notes, created_at, updated_at)
		VALUES %s
		ON CONFLICT (domain)
		DO UPDATE SET
			status = EXCLUDED.status,
			notes = EXCLUDED.notes,
			updated_at = NOW()
	`, strings.Join(placeholders, ", "))

	result, execErr := tx.ExecContext(ctx, query, args...)
	if execErr != nil {
		return 0, fmt.Errorf("bulk upsert: %w", execErr)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return 0, fmt.Errorf("rows affected: %w", rowsErr)
	}

	return int(affected), nil
}

// setBulkStatusTimestamps updates the appropriate timestamp field for all domains in a bulk operation.
func (r *DomainStateRepository) setBulkStatusTimestamps(
	ctx context.Context,
	tx *sqlx.Tx,
	domains []string,
	status string,
) error {
	var column string

	switch status {
	case domain.DomainStatusIgnored:
		column = "ignored_at"
	case domain.DomainStatusPromoted:
		column = "promoted_at"
	default:
		return nil
	}

	query, args, err := sqlx.In(
		fmt.Sprintf("UPDATE discovered_domain_states SET %s = NOW() WHERE domain IN (?)", column),
		domains,
	)
	if err != nil {
		return fmt.Errorf("build bulk timestamp query: %w", err)
	}

	query = tx.Rebind(query)

	if _, execErr := tx.ExecContext(ctx, query, args...); execErr != nil {
		return fmt.Errorf("set bulk %s: %w", column, execErr)
	}

	return nil
}

// GetByDomain returns a domain state by domain name, or nil if not found.
func (r *DomainStateRepository) GetByDomain(ctx context.Context, domainName string) (*domain.DomainState, error) {
	var state domain.DomainState
	query := `SELECT * FROM discovered_domain_states WHERE domain = $1`

	err := r.db.GetContext(ctx, &state, query, domainName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract — default state is "active"
		}

		return nil, fmt.Errorf("get domain state: %w", err)
	}

	return &state, nil
}
