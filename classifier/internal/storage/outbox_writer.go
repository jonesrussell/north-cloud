// Package storage provides storage adapters for the classifier service.
// outbox_writer.go writes classified content to the publisher's transactional outbox.
package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// OutboxWriter writes classified content to the publisher outbox for guaranteed delivery.
// The outbox table lives in the publisher database, ensuring at-least-once delivery
// even if Redis is temporarily unavailable.
type OutboxWriter struct {
	db     *sql.DB
	logger infralogger.Logger
}

// NewOutboxWriter creates a new outbox writer.
// The db connection should be to the publisher database, not the classifier database.
func NewOutboxWriter(db *sql.DB, logger infralogger.Logger) *OutboxWriter {
	return &OutboxWriter{
		db:     db,
		logger: logger,
	}
}

// Write adds a single classified content item to the outbox for publishing.
// This is idempotent via ON CONFLICT - duplicate content_ids are ignored.
func (w *OutboxWriter) Write(ctx context.Context, content *domain.ClassifiedContent) error {
	query := `
		INSERT INTO classified_outbox (
			content_id, source_name, index_name, content_type, topics,
			quality_score, is_crime_related, crime_subcategory,
			title, body, url, published_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (content_id) DO NOTHING`

	// Determine crime subcategory if applicable
	var crimeSubcat *string
	isCrimeRelated := false
	for _, topic := range content.Topics {
		if isCrimeSubcategory(topic) {
			isCrimeRelated = true
			crimeSubcat = &topic
			break
		}
	}

	indexName := content.SourceName + "_classified_content"

	_, err := w.db.ExecContext(ctx, query,
		content.ID,
		content.SourceName,
		indexName,
		content.ContentType,
		pq.Array(content.Topics),
		content.QualityScore,
		isCrimeRelated,
		crimeSubcat,
		content.Title,
		content.RawText, // Body field
		content.URL,
		content.PublishedDate,
	)
	if err != nil {
		return fmt.Errorf("write to outbox: %w", err)
	}

	if w.logger != nil {
		w.logger.Debug("wrote to outbox",
			infralogger.String("content_id", content.ID),
			infralogger.String("source", content.SourceName),
			infralogger.String("content_type", content.ContentType))
	}

	return nil
}

// WriteBatch writes multiple entries in a single transaction.
// This is more efficient than individual writes when processing batches.
func (w *OutboxWriter) WriteBatch(ctx context.Context, contents []*domain.ClassifiedContent) error {
	if len(contents) == 0 {
		return nil
	}

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO classified_outbox (
			content_id, source_name, index_name, content_type, topics,
			quality_score, is_crime_related, crime_subcategory,
			title, body, url, published_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (content_id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for _, content := range contents {
		var crimeSubcat *string
		isCrimeRelated := false
		for _, topic := range content.Topics {
			if isCrimeSubcategory(topic) {
				isCrimeRelated = true
				crimeSubcat = &topic
				break
			}
		}

		indexName := content.SourceName + "_classified_content"

		_, err = stmt.ExecContext(ctx,
			content.ID,
			content.SourceName,
			indexName,
			content.ContentType,
			pq.Array(content.Topics),
			content.QualityScore,
			isCrimeRelated,
			crimeSubcat,
			content.Title,
			content.RawText,
			content.URL,
			content.PublishedDate,
		)
		if err != nil {
			return fmt.Errorf("insert content %s: %w", content.ID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	if w.logger != nil {
		w.logger.Info("wrote batch to outbox",
			infralogger.Int("count", len(contents)))
	}

	return nil
}

// isCrimeSubcategory checks if the topic is a crime sub-category
func isCrimeSubcategory(topic string) bool {
	switch topic {
	case "violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice":
		return true
	default:
		return false
	}
}
