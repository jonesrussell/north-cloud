package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrIndexMetadataNotFound is returned when index metadata is not found
var ErrIndexMetadataNotFound = errors.New("index metadata not found")

// MigrationHistory represents a migration history record
type MigrationHistory struct {
	ID            int
	IndexName     string
	FromVersion   sql.NullString
	ToVersion     sql.NullString
	MigrationType string
	Status        string
	ErrorMessage  sql.NullString
	CreatedAt     time.Time
	CompletedAt   sql.NullTime
}

// IndexMetadata represents index metadata
type IndexMetadata struct {
	ID             int
	IndexName      string
	IndexType      string
	SourceName     sql.NullString
	MappingVersion string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Status         string
}

// RecordMigration records a migration in the history
func (c *Connection) RecordMigration(ctx context.Context, mh *MigrationHistory) error {
	query := `
		INSERT INTO migration_history 
		(index_name, from_version, to_version, migration_type, status, error_message, created_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	var completedAt sql.NullTime
	if !mh.CompletedAt.Time.IsZero() {
		completedAt = mh.CompletedAt
	}

	var fromVersion sql.NullString
	if mh.FromVersion.Valid {
		fromVersion = mh.FromVersion
	}

	var toVersion sql.NullString
	if mh.ToVersion.Valid {
		toVersion = mh.ToVersion
	}

	err := c.DB.QueryRowContext(ctx, query,
		mh.IndexName,
		fromVersion,
		toVersion,
		mh.MigrationType,
		mh.Status,
		mh.ErrorMessage,
		mh.CreatedAt,
		completedAt,
	).Scan(&mh.ID)

	if err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	return nil
}

// UpdateMigrationStatus updates the status of a migration
func (c *Connection) UpdateMigrationStatus(ctx context.Context, id int, status, errorMsg string) error {
	query := `
		UPDATE migration_history 
		SET status = $1, error_message = $2, completed_at = $3
		WHERE id = $4
	`

	var errMsg sql.NullString
	if errorMsg != "" {
		errMsg = sql.NullString{String: errorMsg, Valid: true}
	}

	_, err := c.DB.ExecContext(ctx, query, status, errMsg, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update migration status: %w", err)
	}

	return nil
}

// GetIndexMetadata retrieves metadata for an index
func (c *Connection) GetIndexMetadata(ctx context.Context, indexName string) (*IndexMetadata, error) {
	query := `
		SELECT id, index_name, index_type, source_name, mapping_version, created_at, updated_at, status
		FROM index_metadata
		WHERE index_name = $1
	`

	metadata := &IndexMetadata{}
	var sourceName sql.NullString

	err := c.DB.QueryRowContext(ctx, query, indexName).Scan(
		&metadata.ID,
		&metadata.IndexName,
		&metadata.IndexType,
		&sourceName,
		&metadata.MappingVersion,
		&metadata.CreatedAt,
		&metadata.UpdatedAt,
		&metadata.Status,
	)

	if err == sql.ErrNoRows {
		return nil, ErrIndexMetadataNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get index metadata: %w", err)
	}

	if sourceName.Valid {
		metadata.SourceName = sourceName
	}

	return metadata, nil
}

// SaveIndexMetadata saves or updates index metadata
func (c *Connection) SaveIndexMetadata(ctx context.Context, metadata *IndexMetadata) error {
	query := `
		INSERT INTO index_metadata 
		(index_name, index_type, source_name, mapping_version, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (index_name) 
		DO UPDATE SET 
			index_type = EXCLUDED.index_type,
			source_name = EXCLUDED.source_name,
			mapping_version = EXCLUDED.mapping_version,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	var sourceName sql.NullString
	if metadata.SourceName.Valid {
		sourceName = metadata.SourceName
	}

	err := c.DB.QueryRowContext(ctx, query,
		metadata.IndexName,
		metadata.IndexType,
		sourceName,
		metadata.MappingVersion,
		metadata.Status,
		time.Now(),
		time.Now(),
	).Scan(&metadata.ID)

	if err != nil {
		return fmt.Errorf("failed to save index metadata: %w", err)
	}

	return nil
}

// scanIndexMetadataRows scans rows into IndexMetadata slice
func scanIndexMetadataRows(rows *sql.Rows) ([]*IndexMetadata, error) {
	var metadataList []*IndexMetadata
	for rows.Next() {
		metadata := &IndexMetadata{}
		var sourceName sql.NullString

		if scanErr := rows.Scan(
			&metadata.ID,
			&metadata.IndexName,
			&metadata.IndexType,
			&sourceName,
			&metadata.MappingVersion,
			&metadata.CreatedAt,
			&metadata.UpdatedAt,
			&metadata.Status,
		); scanErr != nil {
			return nil, fmt.Errorf("failed to scan index metadata: %w", scanErr)
		}

		if sourceName.Valid {
			metadata.SourceName = sourceName
		}

		metadataList = append(metadataList, metadata)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return metadataList, nil
}

// ListIndexMetadataBySource lists all index metadata for a source
func (c *Connection) ListIndexMetadataBySource(ctx context.Context, sourceName string) ([]*IndexMetadata, error) {
	query := `
		SELECT id, index_name, index_type, source_name, mapping_version, created_at, updated_at, status
		FROM index_metadata
		WHERE source_name = $1 AND status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := c.DB.QueryContext(ctx, query, sourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to list index metadata: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	return scanIndexMetadataRows(rows)
}

// ListIndexMetadataByType lists all index metadata for an index type
func (c *Connection) ListIndexMetadataByType(ctx context.Context, indexType string) ([]*IndexMetadata, error) {
	query := `
		SELECT id, index_name, index_type, source_name, mapping_version, created_at, updated_at, status
		FROM index_metadata
		WHERE index_type = $1 AND status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := c.DB.QueryContext(ctx, query, indexType)
	if err != nil {
		return nil, fmt.Errorf("failed to list index metadata: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	return scanIndexMetadataRows(rows)
}

// ListAllActiveMetadata returns all active index metadata records.
func (c *Connection) ListAllActiveMetadata(ctx context.Context) ([]*IndexMetadata, error) {
	query := `
		SELECT id, index_name, index_type, source_name, mapping_version, created_at, updated_at, status
		FROM index_metadata
		WHERE status = 'active'
		ORDER BY index_name
	`

	rows, err := c.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list all index metadata: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanIndexMetadataRows(rows)
}

// DeleteIndexMetadata marks an index as deleted
func (c *Connection) DeleteIndexMetadata(ctx context.Context, indexName string) error {
	query := `
		UPDATE index_metadata 
		SET status = 'deleted', updated_at = $1
		WHERE index_name = $2
	`

	_, err := c.DB.ExecContext(ctx, query, time.Now(), indexName)
	if err != nil {
		return fmt.Errorf("failed to delete index metadata: %w", err)
	}

	return nil
}
