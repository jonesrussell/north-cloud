package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

const (
	defaultPersonLimit = 50
	maxPersonLimit     = 200
)

// personColumns is the SELECT column list for the people table.
const personColumns = `id, community_id, name, slug, role, data_source, is_current, verified,
	created_at, updated_at, role_title, email, phone, term_start, term_end, source_url, verified_at`

// PersonRepository provides CRUD operations for the people table.
type PersonRepository struct {
	db     *sql.DB
	logger infralogger.Logger
}

// NewPersonRepository creates a new PersonRepository.
func NewPersonRepository(db *sql.DB, log infralogger.Logger) *PersonRepository {
	return &PersonRepository{
		db:     db,
		logger: log,
	}
}

// scanPerson scans a single row into a Person struct.
func scanPerson(row interface{ Scan(...any) error }) (*models.Person, error) {
	var p models.Person
	scanErr := row.Scan(
		&p.ID, &p.CommunityID, &p.Name, &p.Slug, &p.Role, &p.DataSource, &p.IsCurrent, &p.Verified,
		&p.CreatedAt, &p.UpdatedAt, &p.RoleTitle, &p.Email, &p.Phone, &p.TermStart, &p.TermEnd,
		&p.SourceURL, &p.VerifiedAt,
	)
	if scanErr != nil {
		return nil, fmt.Errorf("scan person: %w", scanErr)
	}
	return &p, nil
}

// Create inserts a new person. ID and timestamps are set automatically.
func (r *PersonRepository) Create(ctx context.Context, p *models.Person) error {
	p.ID = uuid.New().String()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	query := `
		INSERT INTO people (
			id, community_id, name, slug, role, data_source, is_current, verified,
			created_at, updated_at, role_title, email, phone, term_start, term_end,
			source_url, verified_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15,
			$16, $17
		)`

	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.CommunityID, p.Name, p.Slug, p.Role, p.DataSource, p.IsCurrent, p.Verified,
		p.CreatedAt, p.UpdatedAt, p.RoleTitle, p.Email, p.Phone, p.TermStart, p.TermEnd,
		p.SourceURL, p.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("create person: %w", err)
	}

	return nil
}

// GetByID returns a person by ID, or nil if not found.
func (r *PersonRepository) GetByID(ctx context.Context, id string) (*models.Person, error) {
	query := `SELECT ` + personColumns + ` FROM people WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	p, err := scanPerson(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
		}
		return nil, fmt.Errorf("get person by id: %w", err)
	}

	return p, nil
}

// Update modifies an existing person by ID.
// Note: updated_at is handled by the set_people_updated_at DB trigger.
func (r *PersonRepository) Update(ctx context.Context, p *models.Person) error {
	query := `
		UPDATE people SET
			community_id = $2, name = $3, slug = $4, role = $5, data_source = $6,
			is_current = $7, verified = $8, role_title = $9,
			email = $10, phone = $11, term_start = $12, term_end = $13,
			source_url = $14, verified_at = $15
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		p.ID, p.CommunityID, p.Name, p.Slug, p.Role, p.DataSource,
		p.IsCurrent, p.Verified, p.RoleTitle,
		p.Email, p.Phone, p.TermStart, p.TermEnd,
		p.SourceURL, p.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("update person: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("update person rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("update person: not found")
	}

	return nil
}

// Delete removes a person by ID.
func (r *PersonRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM people WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete person: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("delete person rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("delete person: not found")
	}

	return nil
}

// buildPersonWhere constructs a WHERE clause from a PersonFilter.
// CommunityID is always required (added by caller).
func buildPersonWhere(filter models.PersonFilter) (where string, args []any) {
	var conditions []string
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("community_id = $%d", argIdx))
	args = append(args, filter.CommunityID)
	argIdx++

	if filter.Role != "" {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, filter.Role)
	}

	if filter.CurrentOnly {
		conditions = append(conditions, "is_current = true")
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}

// ListByCommunity returns people for a community with optional filters.
// Returns error if CommunityID is empty.
func (r *PersonRepository) ListByCommunity(
	ctx context.Context, filter models.PersonFilter,
) ([]models.Person, error) {
	if filter.CommunityID == "" {
		return nil, errors.New("list people: community_id is required")
	}

	where, args := buildPersonWhere(filter)
	argIdx := len(args) + 1

	limit := filter.Limit
	if limit <= 0 || limit > maxPersonLimit {
		limit = defaultPersonLimit
	}

	//nolint:gosec // G201: query uses only constant column names and integer placeholders
	query := fmt.Sprintf(`SELECT `+personColumns+`
		FROM people%s ORDER BY name ASC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	defer rows.Close()

	people := make([]models.Person, 0, limit)
	for rows.Next() {
		p, scanErr := scanPerson(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		people = append(people, *p)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("list people rows: %w", closeErr)
	}

	return people, nil
}

// Count returns the number of people matching the filter.
// Returns error if CommunityID is empty.
func (r *PersonRepository) Count(ctx context.Context, filter models.PersonFilter) (int, error) {
	if filter.CommunityID == "" {
		return 0, errors.New("count people: community_id is required")
	}

	where, args := buildPersonWhere(filter)
	query := "SELECT COUNT(*) FROM people" + where

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count people: %w", err)
	}

	return count, nil
}

// ArchiveTerm archives a person's current term to people_history and marks them
// as no longer current. Runs in a transaction; rolls back on any failure.
// Returns error if personID is not found.
func (r *PersonRepository) ArchiveTerm(ctx context.Context, personID string) error {
	tx, txErr := r.db.BeginTx(ctx, nil)
	if txErr != nil {
		return fmt.Errorf("archive term begin tx: %w", txErr)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	// Step 1: SELECT current person
	query := `SELECT ` + personColumns + ` FROM people WHERE id = $1`
	row := tx.QueryRowContext(ctx, query, personID)

	p, scanErr := scanPerson(row)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return errors.New("archive term: person not found")
		}
		return fmt.Errorf("archive term select: %w", scanErr)
	}

	if !p.IsCurrent {
		return errors.New("archive term: person is not current")
	}

	// Step 2: INSERT snapshot into people_history
	historyID := uuid.New().String()
	insertQuery := `
		INSERT INTO people_history (
			id, person_id, community_id, name, role,
			term_start, term_end, data_source, source_url, archived_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	now := time.Now()
	_, insertErr := tx.ExecContext(ctx, insertQuery,
		historyID, p.ID, p.CommunityID, p.Name, p.Role,
		p.TermStart, p.TermEnd, p.DataSource, p.SourceURL, now,
	)
	if insertErr != nil {
		return fmt.Errorf("archive term insert history: %w", insertErr)
	}

	// Step 3: UPDATE person — mark as not current, set term_end
	updateQuery := `UPDATE people SET is_current = false, term_end = $2, updated_at = $3 WHERE id = $1`
	_, updateErr := tx.ExecContext(ctx, updateQuery, personID, now, now)
	if updateErr != nil {
		return fmt.Errorf("archive term update person: %w", updateErr)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("archive term commit: %w", commitErr)
	}

	return nil
}
