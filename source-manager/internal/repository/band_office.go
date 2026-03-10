package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

// bandOfficeColumns is the SELECT column list for the band_offices table.
const bandOfficeColumns = `id, community_id, data_source, verified, created_at, updated_at,
	address_line1, address_line2, city, province, postal_code,
	phone, fax, email, toll_free, office_hours, source_url, verified_at`

// BandOfficeRepository provides CRUD operations for the band_offices table.
type BandOfficeRepository struct {
	db     *sql.DB
	logger infralogger.Logger
}

// NewBandOfficeRepository creates a new BandOfficeRepository.
func NewBandOfficeRepository(db *sql.DB, log infralogger.Logger) *BandOfficeRepository {
	return &BandOfficeRepository{
		db:     db,
		logger: log,
	}
}

// scanBandOffice scans a single row into a BandOffice struct.
func scanBandOffice(row interface{ Scan(...any) error }) (*models.BandOffice, error) {
	var bo models.BandOffice
	scanErr := row.Scan(
		&bo.ID, &bo.CommunityID, &bo.DataSource, &bo.Verified, &bo.CreatedAt, &bo.UpdatedAt,
		&bo.AddressLine1, &bo.AddressLine2, &bo.City, &bo.Province, &bo.PostalCode,
		&bo.Phone, &bo.Fax, &bo.Email, &bo.TollFree, &bo.OfficeHours, &bo.SourceURL, &bo.VerifiedAt,
	)
	if scanErr != nil {
		return nil, fmt.Errorf("scan band office: %w", scanErr)
	}
	return &bo, nil
}

// Create inserts a new band office. ID and timestamps are set automatically.
func (r *BandOfficeRepository) Create(ctx context.Context, bo *models.BandOffice) error {
	bo.ID = uuid.New().String()
	bo.CreatedAt = time.Now()
	bo.UpdatedAt = time.Now()

	query := `
		INSERT INTO band_offices (
			id, community_id, data_source, verified, created_at, updated_at,
			address_line1, address_line2, city, province, postal_code,
			phone, fax, email, toll_free, office_hours, source_url, verified_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18
		)`

	_, err := r.db.ExecContext(ctx, query,
		bo.ID, bo.CommunityID, bo.DataSource, bo.Verified, bo.CreatedAt, bo.UpdatedAt,
		bo.AddressLine1, bo.AddressLine2, bo.City, bo.Province, bo.PostalCode,
		bo.Phone, bo.Fax, bo.Email, bo.TollFree, bo.OfficeHours, bo.SourceURL, bo.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("create band office: %w", err)
	}

	return nil
}

// GetByCommunity returns the band office for a community, or nil if not found.
func (r *BandOfficeRepository) GetByCommunity(ctx context.Context, communityID string) (*models.BandOffice, error) {
	query := `SELECT ` + bandOfficeColumns + ` FROM band_offices WHERE community_id = $1`
	row := r.db.QueryRowContext(ctx, query, communityID)

	bo, err := scanBandOffice(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
		}
		return nil, fmt.Errorf("get band office by community: %w", err)
	}

	return bo, nil
}

// Update modifies an existing band office by ID.
// Note: updated_at is handled by the set_band_offices_updated_at DB trigger.
func (r *BandOfficeRepository) Update(ctx context.Context, bo *models.BandOffice) error {
	query := `
		UPDATE band_offices SET
			community_id = $2, data_source = $3, verified = $4,
			address_line1 = $5, address_line2 = $6, city = $7, province = $8, postal_code = $9,
			phone = $10, fax = $11, email = $12, toll_free = $13, office_hours = $14,
			source_url = $15, verified_at = $16
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		bo.ID, bo.CommunityID, bo.DataSource, bo.Verified,
		bo.AddressLine1, bo.AddressLine2, bo.City, bo.Province, bo.PostalCode,
		bo.Phone, bo.Fax, bo.Email, bo.TollFree, bo.OfficeHours,
		bo.SourceURL, bo.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("update band office: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("update band office rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("update band office: not found")
	}

	return nil
}

// DeleteByCommunity removes a band office by community ID.
func (r *BandOfficeRepository) DeleteByCommunity(ctx context.Context, communityID string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM band_offices WHERE community_id = $1", communityID)
	if err != nil {
		return fmt.Errorf("delete band office: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("delete band office rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("delete band office: not found")
	}

	return nil
}

// Upsert inserts or updates a band office by community_id.
// On update, created_at is preserved (DB ignores EXCLUDED.created_at); the Go
// struct is restored to its original value so the caller sees the correct timestamp.
func (r *BandOfficeRepository) Upsert(ctx context.Context, bo *models.BandOffice) error {
	originalCreatedAt := bo.CreatedAt

	if bo.ID == "" {
		bo.ID = uuid.New().String()
	}
	bo.CreatedAt = time.Now()
	bo.UpdatedAt = time.Now()

	query := `
		INSERT INTO band_offices (
			id, community_id, data_source, verified, created_at, updated_at,
			address_line1, address_line2, city, province, postal_code,
			phone, fax, email, toll_free, office_hours, source_url, verified_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17, $18
		)
		ON CONFLICT (community_id) DO UPDATE SET
			data_source = EXCLUDED.data_source, verified = EXCLUDED.verified,
			updated_at = EXCLUDED.updated_at,
			address_line1 = EXCLUDED.address_line1, address_line2 = EXCLUDED.address_line2,
			city = EXCLUDED.city, province = EXCLUDED.province, postal_code = EXCLUDED.postal_code,
			phone = EXCLUDED.phone, fax = EXCLUDED.fax, email = EXCLUDED.email,
			toll_free = EXCLUDED.toll_free, office_hours = EXCLUDED.office_hours,
			source_url = EXCLUDED.source_url, verified_at = EXCLUDED.verified_at
		RETURNING id`

	if err := r.db.QueryRowContext(ctx, query,
		bo.ID, bo.CommunityID, bo.DataSource, bo.Verified, bo.CreatedAt, bo.UpdatedAt,
		bo.AddressLine1, bo.AddressLine2, bo.City, bo.Province, bo.PostalCode,
		bo.Phone, bo.Fax, bo.Email, bo.TollFree, bo.OfficeHours, bo.SourceURL, bo.VerifiedAt,
	).Scan(&bo.ID); err != nil {
		return fmt.Errorf("upsert band office: %w", err)
	}

	// Restore original created_at on update path (DB did not overwrite it).
	if !originalCreatedAt.IsZero() {
		bo.CreatedAt = originalCreatedAt
	}

	return nil
}
