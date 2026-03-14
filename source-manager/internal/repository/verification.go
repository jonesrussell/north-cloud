package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/aiverify"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

const (
	defaultPendingLimit = 50
	maxPendingLimit     = 200
)

// Compile-time interface check.
var _ aiverify.Repository = (*VerificationRepository)(nil)

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

// --- AI Verification Worker Methods ---

// ListUnverifiedUnscoredPeople returns people not yet evaluated by AI.
func (r *VerificationRepository) ListUnverifiedUnscoredPeople(
	ctx context.Context, limit int,
) ([]aiverify.VerificationRecord, error) {
	query := `SELECT p.id, p.name, p.role, p.email, p.phone, p.source_url,
		c.name AS community_name, c.province
		FROM people p
		JOIN communities c ON c.id = p.community_id
		WHERE p.verified = false AND p.verification_confidence IS NULL
		ORDER BY p.created_at ASC LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list unscored people: %w", err)
	}
	defer rows.Close()

	records := make([]aiverify.VerificationRecord, 0, limit)
	for rows.Next() {
		rec, scanErr := scanUnscoredPerson(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, rec)
	}
	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("list unscored people rows: %w", closeErr)
	}

	return records, nil
}

// scanUnscoredPerson scans a single row from the unscored-people query.
func scanUnscoredPerson(rows *sql.Rows) (aiverify.VerificationRecord, error) {
	var rec aiverify.VerificationRecord
	var name, role, communityName sql.NullString
	var email, phone, sourceURL sql.NullString
	var province sql.NullString

	scanErr := rows.Scan(&rec.ID, &name, &role, &email, &phone, &sourceURL,
		&communityName, &province)
	if scanErr != nil {
		return rec, fmt.Errorf("scan unscored person: %w", scanErr)
	}

	rec.EntityType = "person"
	rec.Input = aiverify.VerifyInput{
		RecordType:    "person",
		Name:          name.String,
		Role:          role.String,
		Email:         email.String,
		Phone:         phone.String,
		SourceURL:     sourceURL.String,
		CommunityName: communityName.String,
		Province:      province.String,
	}

	return rec, nil
}

// ListUnverifiedUnscoredBandOffices returns band offices not yet evaluated by AI.
func (r *VerificationRepository) ListUnverifiedUnscoredBandOffices(
	ctx context.Context, limit int,
) ([]aiverify.VerificationRecord, error) {
	query := `SELECT bo.id, bo.phone, bo.fax, bo.email, bo.toll_free,
		bo.address_line1, bo.address_line2, bo.city, bo.postal_code,
		bo.office_hours, bo.source_url,
		c.name AS community_name, c.province
		FROM band_offices bo
		JOIN communities c ON c.id = bo.community_id
		WHERE bo.verified = false AND bo.verification_confidence IS NULL
		ORDER BY bo.created_at ASC LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list unscored band offices: %w", err)
	}
	defer rows.Close()

	records := make([]aiverify.VerificationRecord, 0, limit)
	for rows.Next() {
		rec, scanErr := scanUnscoredBandOffice(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, rec)
	}
	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("list unscored band offices rows: %w", closeErr)
	}

	return records, nil
}

// scanUnscoredBandOffice scans a single row from the unscored-band-offices query.
func scanUnscoredBandOffice(rows *sql.Rows) (aiverify.VerificationRecord, error) {
	var rec aiverify.VerificationRecord
	var phone, fax, email, tollFree sql.NullString
	var addressLine1, addressLine2, city, postalCode sql.NullString
	var officeHours, sourceURL sql.NullString
	var communityName, province sql.NullString

	scanErr := rows.Scan(&rec.ID, &phone, &fax, &email, &tollFree,
		&addressLine1, &addressLine2, &city, &postalCode,
		&officeHours, &sourceURL,
		&communityName, &province)
	if scanErr != nil {
		return rec, fmt.Errorf("scan unscored band office: %w", scanErr)
	}

	rec.EntityType = "band_office"
	rec.Input = aiverify.VerifyInput{
		RecordType:    "band_office",
		Phone:         phone.String,
		Fax:           fax.String,
		Email:         email.String,
		TollFree:      tollFree.String,
		AddressLine1:  addressLine1.String,
		AddressLine2:  addressLine2.String,
		City:          city.String,
		PostalCode:    postalCode.String,
		OfficeHours:   officeHours.String,
		SourceURL:     sourceURL.String,
		CommunityName: communityName.String,
		Province:      province.String,
	}

	return rec, nil
}

// UpdatePersonVerificationResult writes AI verification scores.
func (r *VerificationRepository) UpdatePersonVerificationResult(
	ctx context.Context, id string, confidence float64, issues string,
) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE people SET verification_confidence = $2,
			verification_issues = $3 WHERE id = $1`,
		id, confidence, issues,
	)
	if err != nil {
		return fmt.Errorf("update person verification: %w", err)
	}
	return checkRowsAffected(result, "update person verification")
}

// UpdateBandOfficeVerificationResult writes AI verification scores.
func (r *VerificationRepository) UpdateBandOfficeVerificationResult(
	ctx context.Context, id string, confidence float64, issues string,
) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE band_offices SET verification_confidence = $2,
			verification_issues = $3 WHERE id = $1`,
		id, confidence, issues,
	)
	if err != nil {
		return fmt.Errorf("update band office verification: %w", err)
	}
	return checkRowsAffected(result, "update band office verification")
}

// AutoRejectPerson deletes a low-confidence person record.
func (r *VerificationRepository) AutoRejectPerson(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM people WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("auto reject person: %w", err)
	}
	return checkRowsAffected(result, "auto reject person")
}

// AutoRejectBandOffice deletes a low-confidence band office record.
func (r *VerificationRepository) AutoRejectBandOffice(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM band_offices WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("auto reject band office: %w", err)
	}
	return checkRowsAffected(result, "auto reject band office")
}

const (
	highConfidenceThreshold   = 0.9
	mediumConfidenceThreshold = 0.5
)

// VerificationStats holds aggregate counts for the verification dashboard.
type VerificationStats struct {
	PendingPeople      int `json:"pending_people"`
	PendingBandOffices int `json:"pending_band_offices"`
	ScoredPeople       int `json:"scored_people"`
	ScoredBandOffices  int `json:"scored_band_offices"`
	HighConfidence     int `json:"high_confidence"`
	MediumConfidence   int `json:"medium_confidence"`
	LowConfidence      int `json:"low_confidence"`
}

// GetStats returns aggregate counts for the verification dashboard.
func (r *VerificationRepository) GetStats(ctx context.Context) (*VerificationStats, error) {
	stats := &VerificationStats{}

	if scanErr := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM people WHERE verified = false",
	).Scan(&stats.PendingPeople); scanErr != nil {
		return nil, fmt.Errorf("count pending people: %w", scanErr)
	}
	if scanErr := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM band_offices WHERE verified = false",
	).Scan(&stats.PendingBandOffices); scanErr != nil {
		return nil, fmt.Errorf("count pending band offices: %w", scanErr)
	}
	if scanErr := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM people WHERE verified = false AND verification_confidence IS NOT NULL",
	).Scan(&stats.ScoredPeople); scanErr != nil {
		return nil, fmt.Errorf("count scored people: %w", scanErr)
	}
	if scanErr := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM band_offices WHERE verified = false AND verification_confidence IS NOT NULL",
	).Scan(&stats.ScoredBandOffices); scanErr != nil {
		return nil, fmt.Errorf("count scored band offices: %w", scanErr)
	}

	high, medium, low, bracketErr := r.countByConfidenceBracket(ctx)
	if bracketErr != nil {
		return nil, bracketErr
	}
	stats.HighConfidence = high
	stats.MediumConfidence = medium
	stats.LowConfidence = low

	return stats, nil
}

func (r *VerificationRepository) countByConfidenceBracket(ctx context.Context) (high, medium, low int, err error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE verification_confidence >= $1),
			COUNT(*) FILTER (WHERE verification_confidence >= $2 AND verification_confidence < $1),
			COUNT(*) FILTER (WHERE verification_confidence < $2)
		FROM (
			SELECT verification_confidence FROM people
			WHERE verified = false AND verification_confidence IS NOT NULL
			UNION ALL
			SELECT verification_confidence FROM band_offices
			WHERE verified = false AND verification_confidence IS NOT NULL
		) combined`

	if scanErr := r.db.QueryRowContext(ctx, query, highConfidenceThreshold, mediumConfidenceThreshold).
		Scan(&high, &medium, &low); scanErr != nil {
		return 0, 0, 0, fmt.Errorf("count by confidence bracket: %w", scanErr)
	}
	return high, medium, low, nil
}

// BulkVerifyPeople marks multiple people as verified.
func (r *VerificationRepository) BulkVerifyPeople(ctx context.Context, ids []string) (int, error) {
	return r.bulkApply(ctx, ids,
		"UPDATE people SET verified = true, verified_at = NOW() WHERE id = $1")
}

// BulkRejectPeople deletes multiple unverified people.
func (r *VerificationRepository) BulkRejectPeople(ctx context.Context, ids []string) (int, error) {
	return r.bulkApply(ctx, ids,
		"DELETE FROM people WHERE id = $1 AND verified = false")
}

// BulkVerifyBandOffices marks multiple band offices as verified.
func (r *VerificationRepository) BulkVerifyBandOffices(ctx context.Context, ids []string) (int, error) {
	return r.bulkApply(ctx, ids,
		"UPDATE band_offices SET verified = true, verified_at = NOW() WHERE id = $1")
}

// BulkRejectBandOffices deletes multiple unverified band offices.
func (r *VerificationRepository) BulkRejectBandOffices(ctx context.Context, ids []string) (int, error) {
	return r.bulkApply(ctx, ids,
		"DELETE FROM band_offices WHERE id = $1 AND verified = false")
}

// bulkApply executes query for each ID and returns total rows affected.
func (r *VerificationRepository) bulkApply(ctx context.Context, ids []string, query string) (int, error) {
	count := 0
	for _, id := range ids {
		result, execErr := r.db.ExecContext(ctx, query, id)
		if execErr != nil {
			return count, fmt.Errorf("bulk apply %s: %w", id, execErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return count, fmt.Errorf("rows affected %s: %w", id, rowsErr)
		}
		count += int(rows)
	}
	return count, nil
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
