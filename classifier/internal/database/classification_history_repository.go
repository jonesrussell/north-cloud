package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/lib/pq"
)

// ClassificationHistoryRepository handles database operations for classification history.
type ClassificationHistoryRepository struct {
	db *sqlx.DB
}

// NewClassificationHistoryRepository creates a new classification history repository.
func NewClassificationHistoryRepository(db *sqlx.DB) *ClassificationHistoryRepository {
	return &ClassificationHistoryRepository{db: db}
}

// ClassificationStats represents overall classification statistics.
type ClassificationStats struct {
	TotalClassified     int            `json:"total_classified"`
	AvgQualityScore     float64        `json:"avg_quality_score"`
	CrimeRelated        int            `json:"crime_related"`
	AvgProcessingTimeMs float64        `json:"avg_processing_time_ms"`
	ContentTypes        map[string]int `json:"content_types"`
}

// TopicStat represents statistics for a single topic.
type TopicStat struct {
	Topic      string  `db:"topic"       json:"topic"`
	Count      int     `db:"count"       json:"count"`
	AvgQuality float64 `db:"avg_quality" json:"avg_quality,omitempty"`
}

// SourceStat represents statistics for a single source.
type SourceStat struct {
	SourceName string  `db:"source_name" json:"source_name"`
	Count      int     `db:"count"       json:"count"`
	AvgQuality float64 `db:"avg_quality" json:"avg_quality,omitempty"`
}

// Create inserts a new classification history record.
func (r *ClassificationHistoryRepository) Create(ctx context.Context, history *domain.ClassificationHistory) error {
	query := `
		INSERT INTO classification_history (
			content_id, content_url, source_name, content_type, content_subtype,
			quality_score, topics, source_reputation_score,
			classifier_version, classification_method, model_version, confidence,
			processing_time_ms
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, classified_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		history.ContentID,
		history.ContentURL,
		history.SourceName,
		history.ContentType,
		history.ContentSubtype,
		history.QualityScore,
		pq.Array(history.Topics),
		history.SourceReputationScore,
		history.ClassifierVersion,
		history.ClassificationMethod,
		history.ModelVersion,
		history.Confidence,
		history.ProcessingTimeMs,
	).Scan(&history.ID, &history.ClassifiedAt)

	if err != nil {
		return fmt.Errorf("failed to create classification history: %w", err)
	}

	return nil
}

// GetByContentID retrieves a classification history record by content ID.
func (r *ClassificationHistoryRepository) GetByContentID(ctx context.Context, contentID string) (*domain.ClassificationHistory, error) {
	var history domain.ClassificationHistory
	query := `
		SELECT id, content_id, content_url, source_name, content_type, content_subtype,
		       quality_score, topics, source_reputation_score,
		       classifier_version, classification_method, model_version, confidence,
		       processing_time_ms, classified_at
		FROM classification_history
		WHERE content_id = $1
		ORDER BY classified_at DESC
		LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, query, contentID).Scan(
		&history.ID,
		&history.ContentID,
		&history.ContentURL,
		&history.SourceName,
		&history.ContentType,
		&history.ContentSubtype,
		&history.QualityScore,
		pq.Array(&history.Topics),
		&history.SourceReputationScore,
		&history.ClassifierVersion,
		&history.ClassificationMethod,
		&history.ModelVersion,
		&history.Confidence,
		&history.ProcessingTimeMs,
		&history.ClassifiedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("classification history not found: %s", contentID)
		}
		return nil, fmt.Errorf("failed to get classification history: %w", err)
	}

	return &history, nil
}

// GetStats retrieves overall classification statistics.
// If startDate is provided, filters results to include only classifications on or after that date.
func (r *ClassificationHistoryRepository) GetStats(ctx context.Context, startDate *time.Time) (*ClassificationStats, error) {
	var stats ClassificationStats

	// Build query with optional date filter
	query := `
		SELECT
			COUNT(*) as total_classified,
			COALESCE(AVG(quality_score), 0) as avg_quality_score,
			SUM(CASE WHEN 'crime' = ANY(topics) THEN 1 ELSE 0 END) as crime_related,
			COALESCE(AVG(processing_time_ms), 0) as avg_processing_time_ms
		FROM classification_history
	`
	args := []any{}

	if startDate != nil {
		query += " WHERE classified_at >= $1"
		args = append(args, *startDate)
	}

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.TotalClassified,
		&stats.AvgQualityScore,
		&stats.CrimeRelated,
		&stats.AvgProcessingTimeMs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get classification stats: %w", err)
	}

	// Get content type distribution with same date filter
	stats.ContentTypes = make(map[string]int)
	typeQuery := `
		SELECT content_type, COUNT(*) as count
		FROM classification_history
		WHERE content_type IS NOT NULL
	`
	if startDate != nil {
		typeQuery += " AND classified_at >= $1"
	}

	typeQuery += " GROUP BY content_type"

	typeArgs := []any{}
	if startDate != nil {
		typeArgs = append(typeArgs, *startDate)
	}

	rows, err := r.db.QueryContext(ctx, typeQuery, typeArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get content type distribution: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var contentType string
		var count int
		if err = rows.Scan(&contentType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan content type: %w", err)
		}
		stats.ContentTypes[contentType] = count
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating content types: %w", err)
	}

	return &stats, nil
}

// GetTopicStats retrieves topic distribution statistics.
func (r *ClassificationHistoryRepository) GetTopicStats(ctx context.Context) ([]*TopicStat, error) {
	var stats []*TopicStat

	// Unnest topics array and aggregate
	query := `
		SELECT
			unnest(topics) as topic,
			COUNT(*) as count,
			COALESCE(AVG(quality_score), 0) as avg_quality
		FROM classification_history
		WHERE topics IS NOT NULL AND array_length(topics, 1) > 0
		GROUP BY topic
		ORDER BY count DESC
		LIMIT 20
	`

	err := r.db.SelectContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic stats: %w", err)
	}

	return stats, nil
}

// GetSourceStats retrieves source distribution statistics.
func (r *ClassificationHistoryRepository) GetSourceStats(ctx context.Context) ([]*SourceStat, error) {
	var stats []*SourceStat

	query := `
		SELECT
			source_name,
			COUNT(*) as count,
			COALESCE(AVG(quality_score), 0) as avg_quality
		FROM classification_history
		GROUP BY source_name
		ORDER BY count DESC
		LIMIT 50
	`

	err := r.db.SelectContext(ctx, &stats, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get source stats: %w", err)
	}

	return stats, nil
}

// GetSourceStatsByName retrieves statistics for a specific source.
func (r *ClassificationHistoryRepository) GetSourceStatsByName(ctx context.Context, sourceName string) (*SourceStat, error) {
	var stat SourceStat

	query := `
		SELECT
			source_name,
			COUNT(*) as count,
			COALESCE(AVG(quality_score), 0) as avg_quality
		FROM classification_history
		WHERE source_name = $1
		GROUP BY source_name
	`

	err := r.db.GetContext(ctx, &stat, query, sourceName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &SourceStat{SourceName: sourceName, Count: 0, AvgQuality: 0}, nil
		}
		return nil, fmt.Errorf("failed to get source stats: %w", err)
	}

	return &stat, nil
}
