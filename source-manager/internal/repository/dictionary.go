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

const (
	defaultDictLimit = 50
	maxDictLimit     = 200
)

// dictColumns is the SELECT column list for the dictionary_entries table.
const dictColumns = `id, lemma, word_class, word_class_normalized,
	definitions, inflections, examples, word_family, media,
	attribution, license, consent_public_display, consent_ai_training,
	consent_derivative_works, content_hash, source_url, created_at, updated_at`

// DictionaryRepository provides CRUD operations for the dictionary_entries table.
type DictionaryRepository struct {
	db     *sql.DB
	logger infralogger.Logger
}

// NewDictionaryRepository creates a new DictionaryRepository.
func NewDictionaryRepository(db *sql.DB, log infralogger.Logger) *DictionaryRepository {
	return &DictionaryRepository{
		db:     db,
		logger: log,
	}
}

// scanDictEntry scans a single row into a DictionaryEntry struct.
func scanDictEntry(row interface{ Scan(...any) error }) (*models.DictionaryEntry, error) {
	var e models.DictionaryEntry
	scanErr := row.Scan(
		&e.ID, &e.Lemma, &e.WordClass, &e.WordClassNormalized,
		&e.Definitions, &e.Inflections, &e.Examples, &e.WordFamily, &e.Media,
		&e.Attribution, &e.License, &e.ConsentPublicDisplay, &e.ConsentAITraining,
		&e.ConsentDerivativeWorks, &e.ContentHash, &e.SourceURL, &e.CreatedAt, &e.UpdatedAt,
	)
	if scanErr != nil {
		return nil, fmt.Errorf("scan dictionary entry: %w", scanErr)
	}
	return &e, nil
}

// Create inserts a new dictionary entry. ID and timestamps are set automatically.
func (r *DictionaryRepository) Create(ctx context.Context, e *models.DictionaryEntry) error {
	e.ID = uuid.New().String()
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()

	query := `
		INSERT INTO dictionary_entries (
			id, lemma, word_class, word_class_normalized,
			definitions, inflections, examples, word_family, media,
			attribution, license, consent_public_display, consent_ai_training,
			consent_derivative_works, content_hash, source_url, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17, $18
		)`

	_, err := r.db.ExecContext(ctx, query,
		e.ID, e.Lemma, e.WordClass, e.WordClassNormalized,
		e.Definitions, e.Inflections, e.Examples, e.WordFamily, e.Media,
		e.Attribution, e.License, e.ConsentPublicDisplay, e.ConsentAITraining,
		e.ConsentDerivativeWorks, e.ContentHash, e.SourceURL, e.CreatedAt, e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create dictionary entry: %w", err)
	}

	return nil
}

// GetByID returns a dictionary entry by ID, or nil if not found.
func (r *DictionaryRepository) GetByID(ctx context.Context, id string) (*models.DictionaryEntry, error) {
	query := `SELECT ` + dictColumns + ` FROM dictionary_entries WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	e, err := scanDictEntry(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
		}
		return nil, fmt.Errorf("get dictionary entry by id: %w", err)
	}

	return e, nil
}

// normalizeDictLimit clamps limit to the allowed range.
func normalizeDictLimit(limit int) int {
	if limit <= 0 || limit > maxDictLimit {
		return defaultDictLimit
	}
	return limit
}

// List returns dictionary entries where consent_public_display is true.
func (r *DictionaryRepository) List(
	ctx context.Context, filter models.DictionaryEntryFilter,
) ([]models.DictionaryEntry, error) {
	limit := normalizeDictLimit(filter.Limit)

	query := `SELECT ` + dictColumns + `
		FROM dictionary_entries
		WHERE consent_public_display = TRUE
		ORDER BY lemma ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("list dictionary entries: %w", err)
	}
	defer rows.Close()

	entries := make([]models.DictionaryEntry, 0, limit)
	for rows.Next() {
		e, scanErr := scanDictEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, *e)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("list dictionary entries rows: %w", closeErr)
	}

	return entries, nil
}

// Count returns the number of dictionary entries where consent_public_display is true.
func (r *DictionaryRepository) Count(
	ctx context.Context, _ models.DictionaryEntryFilter,
) (int, error) {
	query := `SELECT COUNT(*) FROM dictionary_entries WHERE consent_public_display = TRUE`

	var count int
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count dictionary entries: %w", err)
	}

	return count, nil
}

// Search returns dictionary entries matching a full-text query with consent filtering.
func (r *DictionaryRepository) Search(
	ctx context.Context, q string, filter models.DictionaryEntryFilter,
) ([]models.DictionaryEntry, error) {
	limit := normalizeDictLimit(filter.Limit)

	query := `SELECT ` + dictColumns + `
		FROM dictionary_entries
		WHERE consent_public_display = TRUE
			AND search_vector @@ plainto_tsquery('english', $1)
		ORDER BY ts_rank(search_vector, plainto_tsquery('english', $1)) DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, q, limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("search dictionary entries: %w", err)
	}
	defer rows.Close()

	entries := make([]models.DictionaryEntry, 0, limit)
	for rows.Next() {
		e, scanErr := scanDictEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, *e)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("search dictionary entries rows: %w", closeErr)
	}

	return entries, nil
}

// UpsertByContentHash inserts or updates a dictionary entry keyed by content_hash.
// This enables idempotent bulk imports.
func (r *DictionaryRepository) UpsertByContentHash(
	ctx context.Context, e *models.DictionaryEntry,
) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now

	query := `
		INSERT INTO dictionary_entries (
			id, lemma, word_class, word_class_normalized,
			definitions, inflections, examples, word_family, media,
			attribution, license, consent_public_display, consent_ai_training,
			consent_derivative_works, content_hash, source_url, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17, $18
		)
		ON CONFLICT (content_hash) DO UPDATE SET
			lemma = EXCLUDED.lemma,
			word_class = EXCLUDED.word_class,
			word_class_normalized = EXCLUDED.word_class_normalized,
			definitions = EXCLUDED.definitions,
			inflections = EXCLUDED.inflections,
			examples = EXCLUDED.examples,
			word_family = EXCLUDED.word_family,
			media = EXCLUDED.media,
			attribution = EXCLUDED.attribution,
			license = EXCLUDED.license,
			consent_public_display = EXCLUDED.consent_public_display,
			consent_ai_training = EXCLUDED.consent_ai_training,
			consent_derivative_works = EXCLUDED.consent_derivative_works,
			source_url = EXCLUDED.source_url,
			updated_at = EXCLUDED.updated_at`

	_, err := r.db.ExecContext(ctx, query,
		e.ID, e.Lemma, e.WordClass, e.WordClassNormalized,
		e.Definitions, e.Inflections, e.Examples, e.WordFamily, e.Media,
		e.Attribution, e.License, e.ConsentPublicDisplay, e.ConsentAITraining,
		e.ConsentDerivativeWorks, e.ContentHash, e.SourceURL, e.CreatedAt, e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert dictionary entry: %w", err)
	}

	return nil
}
