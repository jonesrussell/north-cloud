package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// SourceReputationListFilter holds pagination and filter params for List.
type SourceReputationListFilter struct {
	Page      int
	PageSize  int
	SortBy    string // reputation, category, total_articles, last_classified_at
	SortOrder string // asc, desc
	Search    string // ILIKE on source_name
	Category  string // filter by category
}

const (
	// Default reputation score (matches classifier.defaultReputationScore)
	defaultReputationScore = 50
)

// SourceReputationRepository handles database operations for source reputation.
type SourceReputationRepository struct {
	db *sqlx.DB
}

// NewSourceReputationRepository creates a new source reputation repository.
func NewSourceReputationRepository(db *sqlx.DB) *SourceReputationRepository {
	return &SourceReputationRepository{db: db}
}

// GetSource retrieves a source by its name.
func (r *SourceReputationRepository) GetSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error) {
	var source domain.SourceReputation
	query := `
		SELECT id, source_name, source_url, category, reputation_score,
		       total_articles, average_quality_score, spam_count,
		       last_classified_at, created_at, updated_at
		FROM source_reputation
		WHERE source_name = $1
	`

	err := r.db.GetContext(ctx, &source, query, sourceName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("source not found: %s", sourceName)
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return &source, nil
}

// CreateSource inserts a new source into the database.
func (r *SourceReputationRepository) CreateSource(ctx context.Context, source *domain.SourceReputation) error {
	query := `
		INSERT INTO source_reputation (
			source_name, source_url, category, reputation_score,
			total_articles, average_quality_score, spam_count, last_classified_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		source.SourceName,
		source.SourceURL,
		source.Category,
		source.ReputationScore,
		source.TotalArticles,
		source.AverageQualityScore,
		source.SpamCount,
		source.LastClassifiedAt,
	).Scan(&source.ID, &source.CreatedAt, &source.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create source: %w", err)
	}

	return nil
}

// UpdateSource updates an existing source.
func (r *SourceReputationRepository) UpdateSource(ctx context.Context, source *domain.SourceReputation) error {
	query := `
		UPDATE source_reputation
		SET source_url = $1, category = $2, reputation_score = $3,
		    total_articles = $4, average_quality_score = $5, spam_count = $6,
		    last_classified_at = $7
		WHERE source_name = $8
		RETURNING updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		source.SourceURL,
		source.Category,
		source.ReputationScore,
		source.TotalArticles,
		source.AverageQualityScore,
		source.SpamCount,
		source.LastClassifiedAt,
		source.SourceName,
	).Scan(&source.UpdatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("source not found: %s", source.SourceName)
		}
		return fmt.Errorf("failed to update source: %w", err)
	}

	return nil
}

// GetOrCreateSource retrieves a source or creates it if it doesn't exist.
func (r *SourceReputationRepository) GetOrCreateSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error) {
	// Try to get existing source
	source, err := r.GetSource(ctx, sourceName)
	if err == nil {
		return source, nil
	}

	// Source doesn't exist, create it
	newSource := &domain.SourceReputation{
		SourceName:          sourceName,
		Category:            "unknown",
		ReputationScore:     defaultReputationScore, // Default neutral score (matches classifier.defaultReputationScore)
		TotalArticles:       0,
		AverageQualityScore: 0.0,
		SpamCount:           0,
	}

	err = r.CreateSource(ctx, newSource)
	if err != nil {
		// Handle potential race condition where another goroutine created it
		existingSource, getErr := r.GetSource(ctx, sourceName)
		if getErr == nil {
			return existingSource, nil
		}
		return nil, fmt.Errorf("failed to create or get source: %w", err)
	}

	return newSource, nil
}

// List retrieves sources with pagination, sorting, and filtering.
func (r *SourceReputationRepository) List(ctx context.Context, filter SourceReputationListFilter) ([]*domain.SourceReputation, int, error) {
	offset := (filter.Page - 1) * filter.PageSize

	whereClause, countArgs := buildListWhere(filter)
	countQuery := `SELECT COUNT(*) FROM source_reputation WHERE 1=1` + whereClause
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count sources: %w", err)
	}

	orderClause := buildListOrder(filter)
	argCount := len(countArgs)
	const (
		limitParamIdx  = 1
		offsetParamIdx = 2
	)
	limitPlaceholder := argCount + limitParamIdx
	offsetPlaceholder := argCount + offsetParamIdx
	query := `
		SELECT id, source_name, source_url, category, reputation_score,
		       total_articles, average_quality_score, spam_count,
		       last_classified_at, created_at, updated_at
		FROM source_reputation
		WHERE 1=1` + whereClause + orderClause + fmt.Sprintf(`
		LIMIT $%d OFFSET $%d`, limitPlaceholder, offsetPlaceholder)

	args := append(append([]any{}, countArgs...), filter.PageSize, offset)
	var sources []*domain.SourceReputation
	err = r.db.SelectContext(ctx, &sources, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list sources: %w", err)
	}

	return sources, total, nil
}

func buildListWhere(filter SourceReputationListFilter) (whereClause string, args []any) {
	var clauses []string
	args = make([]any, 0)
	pos := 1

	if filter.Search != "" {
		clauses = append(clauses, fmt.Sprintf("source_name ILIKE $%d", pos))
		args = append(args, "%"+filter.Search+"%")
		pos++
	}
	if filter.Category != "" {
		clauses = append(clauses, fmt.Sprintf("category = $%d", pos))
		args = append(args, filter.Category)
	}

	if len(clauses) == 0 {
		whereClause = ""
		return
	}
	whereClause = " AND " + strings.Join(clauses, " AND ")
	return
}

func buildListOrder(filter SourceReputationListFilter) string {
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "reputation"
	}
	columnMap := map[string]string{
		"name":               "source_name",
		"reputation":         "reputation_score",
		"category":           "category",
		"total_articles":     "total_articles",
		"total_classified":   "total_articles",
		"last_classified_at": "last_classified_at",
		"last_updated":       "last_classified_at",
	}
	column, ok := columnMap[sortBy]
	if !ok {
		column = "reputation_score"
	}
	order := strings.ToUpper(filter.SortOrder)
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}
	return fmt.Sprintf(" ORDER BY %s %s, total_articles DESC", column, order)
}
