package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

const (
	defaultPendingLimit = 50
	maxPendingLimit     = 200
)

// PendingItem wraps either a Person or BandOffice with its entity type.
type PendingItem struct {
	Type       string             `json:"type"` // "person" or "band_office"
	Person     *models.Person     `json:"person,omitempty"`
	BandOffice *models.BandOffice `json:"band_office,omitempty"`
}

// VerificationRepository provides queries for the verification queue.
type VerificationRepository struct {
	db     *sql.DB
	logger infralogger.Logger
}

// NewVerificationRepository creates a new VerificationRepository.
func NewVerificationRepository(db *sql.DB, log infralogger.Logger) *VerificationRepository {
	return &VerificationRepository{
		db:     db,
		logger: log,
	}
}

// ListPending returns unverified people and band offices.
// Filter by entityType: "person", "band_office", or "" for both.
func (r *VerificationRepository) ListPending(
	ctx context.Context, entityType string, limit, offset int,
) ([]PendingItem, int, error) {
	if limit <= 0 || limit > maxPendingLimit {
		limit = defaultPendingLimit
	}

	items := make([]PendingItem, 0, limit)
	totalCount := 0

	if entityType == "" || entityType == "person" {
		people, count, err := r.listUnverifiedPeople(ctx, limit, offset)
		if err != nil {
			return nil, 0, err
		}
		for i := range people {
			items = append(items, PendingItem{Type: "person", Person: &people[i]})
		}
		totalCount += count
	}

	if entityType == "" || entityType == "band_office" {
		offices, count, err := r.listUnverifiedBandOffices(ctx, limit, offset)
		if err != nil {
			return nil, 0, err
		}
		for i := range offices {
			items = append(items, PendingItem{Type: "band_office", BandOffice: &offices[i]})
		}
		totalCount += count
	}

	return items, totalCount, nil
}

//nolint:dupl // mirrors listUnverifiedBandOffices but scans a different type
func (r *VerificationRepository) listUnverifiedPeople(
	ctx context.Context, limit, offset int,
) ([]models.Person, int, error) {
	var count int
	countErr := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM people WHERE verified = false",
	).Scan(&count)
	if countErr != nil {
		return nil, 0, fmt.Errorf("count unverified people: %w", countErr)
	}

	query := `SELECT ` + personColumns + `
		FROM people WHERE verified = false
		ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list unverified people: %w", err)
	}
	defer rows.Close()

	people := make([]models.Person, 0, limit)
	for rows.Next() {
		p, scanErr := scanPerson(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		people = append(people, *p)
	}
	if closeErr := rows.Err(); closeErr != nil {
		return nil, 0, fmt.Errorf("list unverified people rows: %w", closeErr)
	}

	return people, count, nil
}

//nolint:dupl // mirrors listUnverifiedPeople but scans a different type
func (r *VerificationRepository) listUnverifiedBandOffices(
	ctx context.Context, limit, offset int,
) ([]models.BandOffice, int, error) {
	var count int
	countErr := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM band_offices WHERE verified = false",
	).Scan(&count)
	if countErr != nil {
		return nil, 0, fmt.Errorf("count unverified band offices: %w", countErr)
	}

	query := `SELECT ` + bandOfficeColumns + `
		FROM band_offices WHERE verified = false
		ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list unverified band offices: %w", err)
	}
	defer rows.Close()

	offices := make([]models.BandOffice, 0, limit)
	for rows.Next() {
		bo, scanErr := scanBandOffice(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		offices = append(offices, *bo)
	}
	if closeErr := rows.Err(); closeErr != nil {
		return nil, 0, fmt.Errorf("list unverified band offices rows: %w", closeErr)
	}

	return offices, count, nil
}

// VerifyPerson marks a person as verified.
func (r *VerificationRepository) VerifyPerson(ctx context.Context, id string) error {
	now := time.Now()
	result, err := r.db.ExecContext(ctx,
		"UPDATE people SET verified = true, verified_at = $2 WHERE id = $1",
		id, now,
	)
	if err != nil {
		return fmt.Errorf("verify person: %w", err)
	}
	return checkRowsAffected(result, "verify person")
}

// VerifyBandOffice marks a band office as verified.
func (r *VerificationRepository) VerifyBandOffice(ctx context.Context, id string) error {
	now := time.Now()
	result, err := r.db.ExecContext(ctx,
		"UPDATE band_offices SET verified = true, verified_at = $2 WHERE id = $1",
		id, now,
	)
	if err != nil {
		return fmt.Errorf("verify band office: %w", err)
	}
	return checkRowsAffected(result, "verify band office")
}

// RejectPerson archives and deletes a person.
func (r *VerificationRepository) RejectPerson(ctx context.Context, id string) error {
	// Look up the person first to confirm it exists and is unverified.
	query := `SELECT ` + personColumns + ` FROM people WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	p, scanErr := scanPerson(row)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return errors.New("reject person: not found")
		}
		return fmt.Errorf("reject person lookup: %w", scanErr)
	}
	if p.Verified {
		return errors.New("reject person: already verified")
	}

	// Delete (no archive needed for rejected scraped data).
	result, err := r.db.ExecContext(ctx, "DELETE FROM people WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("reject person delete: %w", err)
	}
	return checkRowsAffected(result, "reject person delete")
}

// RejectBandOffice deletes an unverified band office.
func (r *VerificationRepository) RejectBandOffice(ctx context.Context, id string) error {
	query := `SELECT ` + bandOfficeColumns + ` FROM band_offices WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	bo, scanErr := scanBandOffice(row)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return errors.New("reject band office: not found")
		}
		return fmt.Errorf("reject band office lookup: %w", scanErr)
	}
	if bo.Verified {
		return errors.New("reject band office: already verified")
	}

	result, err := r.db.ExecContext(ctx, "DELETE FROM band_offices WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("reject band office delete: %w", err)
	}
	return checkRowsAffected(result, "reject band office delete")
}

// checkRowsAffected returns an error if no rows were affected.
func checkRowsAffected(result sql.Result, action string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", action, err)
	}
	if rows == 0 {
		return fmt.Errorf("%s: not found", action)
	}
	return nil
}
